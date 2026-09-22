package report

import (
	"bytes"
	"compress/gzip"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/JamievanRiel/pqscan-nl/internal/results"
)

//go:embed templates/*.html assets/*
var files embed.FS

type providerRow struct {
	Org       string
	ASN       uint32
	Reachable int
	PQPct     float64
}

// pageData is what every page template renders.
type pageData struct {
	Title     string
	Page      string
	Latest    results.Summary
	Tail      results.Counts // Tranco domains ranked below Latest.TrancoTopRank
	Headline  float64
	Trend     template.HTML
	Sectors   template.HTML
	Providers []providerRow
	Causes    string // e.g. "DNS failure 1, timeout 1"
}

// domainEntry is one row of domains.json. Keys are short because the file
// holds about 20,000 entries.
type domainEntry struct {
	Domain   string `json:"d"`
	Host     string `json:"h,omitempty"`
	Status   string `json:"s"`
	Provider string `json:"p,omitempty"`
}

var pages = []string{"index", "search", "methodology", "findings"}

var pageTitles = map[string]string{
	"index":       "How quantum-safe is .nl?",
	"search":      "Look up a domain · How quantum-safe is .nl?",
	"methodology": "Methodology · How quantum-safe is .nl?",
	"findings":    "Findings · How quantum-safe is .nl?",
}

var sectorNames = map[string]string{
	"banks":      "Banks",
	"government": "Government",
	"hospitals":  "Hospitals",
	"webshops":   "Web shops",
}

var causeNames = map[string]string{
	"dns":     "DNS failure",
	"timeout": "timeout",
	"refused": "connection refused",
	"tls":     "TLS error",
	"other":   "other error",
}

var funcs = template.FuncMap{
	"pct": pct1,
	"num": thousands,
}

// BuildSite writes the static website to outDir. summaries must be sorted
// oldest first and not be empty; latest holds the records of the newest scan.
func BuildSite(outDir string, summaries []results.Summary, latest []results.Record) error {
	if len(summaries) == 0 {
		return errors.New("no summaries to build the site from")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	data := newPageData(summaries)
	for _, page := range pages {
		b, err := renderPage(page, data)
		if err != nil {
			return fmt.Errorf("%s: %w", page, err)
		}
		if err := os.WriteFile(filepath.Join(outDir, page+".html"), b, 0o644); err != nil {
			return err
		}
	}
	for _, name := range []string{"style.css", "search.js"} {
		b, err := files.ReadFile("assets/" + name)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(outDir, name), b, 0o644); err != nil {
			return err
		}
	}
	if err := writeDomainIndex(filepath.Join(outDir, "domains.json"), data.Latest.Date, latest); err != nil {
		return err
	}
	return writeRawData(filepath.Join(outDir, "scan-"+data.Latest.Date+".jsonl.gz"), latest)
}

func newPageData(summaries []results.Summary) pageData {
	latest := summaries[len(summaries)-1]
	d := pageData{
		Latest:   latest,
		Tail:     minus(latest.Tranco, latest.TrancoTop),
		Headline: latest.TrancoTop.Pct(latest.TrancoTop.PQDefault),
	}

	trend := make([]TrendPoint, len(summaries))
	for i, s := range summaries {
		trend[i] = TrendPoint{Date: s.Date, Pct: s.TrancoTop.Pct(s.TrancoTop.PQDefault)}
	}
	d.Trend = TrendSVG(trend)

	top := strconv.Itoa(latest.TrancoTopRank/1000) + "k"
	rows := []BarRow{
		{Label: "Tranco top " + top, Counts: latest.TrancoTop},
		{Label: "Tranco " + top + "–1M", Counts: d.Tail},
	}
	for _, name := range slices.Sorted(maps.Keys(latest.Sectors)) {
		rows = append(rows, BarRow{Label: sectorName(name), Counts: latest.Sectors[name]})
	}
	d.Sectors = SectorBars(rows)

	for _, p := range latest.Providers {
		d.Providers = append(d.Providers, providerRow{Org: p.Org, ASN: p.ASN, Reachable: p.Reachable(), PQPct: p.Pct(p.PQDefault)})
	}
	d.Causes = describeCauses(latest.UnreachableByError)
	return d
}

// minus subtracts b from a per status.
func minus(a, b results.Counts) results.Counts {
	return results.Counts{
		Total:       a.Total - b.Total,
		PQDefault:   a.PQDefault - b.PQDefault,
		PQSupported: a.PQSupported - b.PQSupported,
		Classic:     a.Classic - b.Classic,
		Unreachable: a.Unreachable - b.Unreachable,
	}
}

// describeCauses lists unreachable causes alphabetically: "DNS failure 1, timeout 1".
func describeCauses(m map[string]int) string {
	var parts []string
	for _, k := range slices.Sorted(maps.Keys(m)) {
		name := causeNames[k]
		if name == "" {
			name = k
		}
		parts = append(parts, name+" "+thousands(m[k]))
	}
	return strings.Join(parts, ", ")
}

func renderPage(page string, d pageData) ([]byte, error) {
	d.Page = page
	d.Title = pageTitles[page]
	t, err := template.New("layout.html").Funcs(funcs).ParseFS(files, "templates/layout.html", "templates/"+page+".html")
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout.html", d); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeDomainIndex(path, date string, recs []results.Record) error {
	idx := struct {
		Date    string        `json:"date"`
		Domains []domainEntry `json:"domains"`
	}{Date: date, Domains: make([]domainEntry, 0, len(recs))}
	for _, r := range recs {
		idx.Domains = append(idx.Domains, domainEntry{Domain: r.Domain, Host: r.Host, Status: string(r.Status), Provider: r.ASOrg})
	}
	slices.SortFunc(idx.Domains, func(a, b domainEntry) int { return strings.Compare(a.Domain, b.Domain) })
	b, err := json.Marshal(idx)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// writeRawData writes the records as gzipped JSONL, the same format as the
// scan output.
func writeRawData(path string, recs []results.Record) error {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if err := results.WriteJSONL(zw, recs); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func sectorName(s string) string {
	if n, ok := sectorNames[s]; ok {
		return n
	}
	return s
}

// thousands formats n with comma separators: 19077 becomes "19,077".
func thousands(n int) string {
	if n < 0 {
		return "-" + thousands(-n)
	}
	s := strconv.Itoa(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
