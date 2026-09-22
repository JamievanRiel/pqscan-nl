package report

import (
	"bytes"
	"cmp"
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
	"time"

	"github.com/JamievanRiel/pqscan-nl/internal/results"
)

//go:embed templates/*.html assets/*
var files embed.FS

// pageData is what every page template renders.
type pageData struct {
	Title, Page    string
	Latest         results.Summary
	LatestDate     string // "22 September 2026"
	ScanWindow     string // "22 September 2026, 06:54 to 06:59 UTC"
	Revision       string // short commit of the pqscan build, "" if unknown
	Year           int
	Scanned        int            // domains in the latest scan
	Tail           results.Counts // Tranco domains ranked below Latest.TrancoTopRank
	Headline       float64
	HeadLo, HeadHi float64
	UnreachablePct float64 // of the TrancoTop domains
	MinSample      int
	Findings       []string
	Concentration  string
	Waffle         template.HTML
	Trend          template.HTML
	TrendRows      []trendRow
	ByRank         template.HTML
	Sectors        template.HTML
	Networks       template.HTML
	Groups, TLS    []shareRow
	Causes         []countRow
	Downloads      []download
}

type trendRow struct {
	Date        string
	N           int
	Pct, Lo, Hi float64
}

// shareRow is one row of a protocol table.
type shareRow struct {
	Name   string
	Hybrid bool
	N      int
	Pct    float64
}

type countRow struct {
	Name string
	N    int
}

// domainEntry is one row of domains.json. Keys are short because the file
// holds about 20,000 entries.
type domainEntry struct {
	Domain   string `json:"d"`
	Host     string `json:"h,omitempty"`
	Status   string `json:"s"`
	Provider string `json:"p,omitempty"`
}

var pages = []string{"index", "methodology", "data", "search"}

var pageTitles = map[string]string{
	"index":       "How quantum-safe is .nl?",
	"methodology": "Methodology · How quantum-safe is .nl?",
	"data":        "Data · How quantum-safe is .nl?",
	"search":      "Look up a domain · How quantum-safe is .nl?",
}

var sectorNames = map[string]string{
	"banks":      "Banks",
	"government": "Government",
	"hospitals":  "Hospitals",
	"webshops":   "Web shops",
}

var causeNames = map[string]string{
	"dns":     "DNS failure",
	"timeout": "Timeout",
	"refused": "Connection refused",
	"tls":     "TLS error",
	"other":   "Other error",
}

var groupNames = map[string]string{
	"CurveP256": "P-256",
	"CurveP384": "P-384",
	"CurveP521": "P-521",
}

var funcs = template.FuncMap{
	"pct": pct1,
	"num": thousands,
}

// download is one file on the data page.
type download struct {
	Path, Description, Format, Size string
}

// BuildSite writes the static website to outDir: the pages, the raw results
// of the latest scan, the domain index, every summary and the sector lists
// from sectorDir. summaries must be sorted oldest first and not be empty;
// latest holds the records of the newest scan.
func BuildSite(outDir, sectorDir string, summaries []results.Summary, latest []results.Record) error {
	if len(summaries) == 0 {
		return errors.New("no summaries to build the site from")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	date := summaries[len(summaries)-1].Date
	raw := "scan-" + date + ".jsonl.gz"
	if err := writeRawData(filepath.Join(outDir, raw), latest); err != nil {
		return err
	}
	if err := writeDomainIndex(filepath.Join(outDir, "domains.json"), date, latest); err != nil {
		return err
	}
	downloads := []download{
		{Path: raw, Description: "Raw results of the latest scan, one JSON record per domain", Format: "JSON Lines, gzip", Size: fileSize(filepath.Join(outDir, raw))},
		{Path: "domains.json", Description: "Latest status per domain, used by the search page", Format: "JSON", Size: fileSize(filepath.Join(outDir, "domains.json"))},
	}
	sectors, err := copySectorLists(sectorDir, outDir)
	if err != nil {
		return err
	}
	downloads = append(downloads, sectors...)
	for _, s := range slices.Backward(summaries) {
		path, err := WriteSummary(filepath.Join(outDir, "summaries"), s)
		if err != nil {
			return err
		}
		downloads = append(downloads, download{Path: "summaries/" + s.Date + ".json", Description: "Summary of the scan of " + s.Date, Format: "JSON", Size: fileSize(path)})
	}

	data := newPageData(summaries, latest, downloads)
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
	return nil
}

// copySectorLists copies the CSV files in dir to outDir/sectors and describes
// them for the data page.
func copySectorLists(dir, outDir string) ([]download, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.csv"))
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("%s: no sector CSV files", dir)
	}
	if err := os.MkdirAll(filepath.Join(outDir, "sectors"), 0o755); err != nil {
		return nil, err
	}
	var out []download
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		base := filepath.Base(p)
		if err := os.WriteFile(filepath.Join(outDir, "sectors", base), b, 0o644); err != nil {
			return nil, err
		}
		entries := strings.Count(strings.TrimSpace(string(b)), "\n") // lines after the header
		noun := "entries"
		if entries == 1 {
			noun = "entry"
		}
		out = append(out, download{
			Path:        "sectors/" + base,
			Description: fmt.Sprintf("%s sector list, %s %s with their sources", sectorName(strings.TrimSuffix(base, ".csv")), thousands(entries), noun),
			Format:      "CSV",
			Size:        humanSize(int64(len(b))),
		})
	}
	return out, nil
}

func fileSize(path string) string {
	fi, err := os.Stat(path)
	if err != nil {
		return ""
	}
	return humanSize(fi.Size())
}

// humanSize formats a file size: 900 B, 1.6 KB, 658 KB, 1.6 MB.
func humanSize(n int64) string {
	switch f := float64(n); {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 10*1024:
		return fmt.Sprintf("%.1f KB", f/1024)
	case n < 1024*1024:
		return fmt.Sprintf("%.0f KB", f/1024)
	default:
		return fmt.Sprintf("%.1f MB", f/(1024*1024))
	}
}

func newPageData(summaries []results.Summary, latest []results.Record, downloads []download) pageData {
	s := summaries[len(summaries)-1]
	top := s.TrancoTop
	d := pageData{
		Latest:        s,
		LatestDate:    longDate(s.Date),
		ScanWindow:    scanWindow(s),
		Revision:      shortRevision(s.Revision),
		Year:          s.StartedAt.Year(),
		Scanned:       len(latest),
		Tail:          minus(s.Tranco, top),
		Headline:      top.Pct(top.PQDefault),
		MinSample:     MinSample,
		Findings:      KeyFindings(s),
		Concentration: Concentration(s),
		Downloads:     downloads,
	}
	d.HeadLo, d.HeadHi = Wilson(top.PQDefault, top.Reachable())
	if top.Total > 0 {
		d.UnreachablePct = 100 * float64(top.Unreachable) / float64(top.Total)
	}

	cells := waffleCells(latest, s.TrancoTopRank)
	d.Waffle = Waffle(cells, min(60, len(cells)))

	points := make([]TrendPoint, len(summaries))
	for i, sum := range summaries {
		c := sum.TrancoTop
		lo, hi := Wilson(c.PQDefault, c.Reachable())
		points[i] = TrendPoint{Date: sum.Date, Pct: c.Pct(c.PQDefault), Lo: lo, Hi: hi}
		d.TrendRows = append(d.TrendRows, trendRow{Date: sum.Date, N: c.Reachable(), Pct: points[i].Pct, Lo: lo, Hi: hi})
	}
	d.Trend = TrendSVG(points)

	var rank []DotRow
	for _, b := range s.ByRank {
		rank = append(rank, dotRow(thousands(b.From)+"–"+thousands(b.To), b.Counts, b.From > s.TrancoTopRank))
	}
	d.ByRank = DotPlot(rank)

	var sectors []DotRow
	for _, sec := range eligibleSectors(s.Sectors) {
		sectors = append(sectors, dotRow(sectorName(sec.name), sec.c, false))
	}
	d.Sectors = DotPlot(sectors)

	var nets []DotRow
	for _, p := range eligibleNetworks(s.Providers) {
		nets = append(nets, dotRow(fmt.Sprintf("%s (AS%d)", NetworkName(p.Org), p.ASN), p.Counts, false))
	}
	d.Networks = DotPlot(nets)

	d.Groups = shares(s.Groups, func(g string) (string, bool) {
		if n, ok := groupNames[g]; ok {
			return n, false
		}
		return g, strings.Contains(g, "MLKEM")
	})
	d.TLS = shares(s.TLSVersions, func(v string) (string, bool) { return "TLS " + v, false })
	for _, k := range slices.Sorted(maps.Keys(s.UnreachableByError)) {
		name := causeNames[k]
		if name == "" {
			name = k
		}
		d.Causes = append(d.Causes, countRow{Name: name, N: s.UnreachableByError[k]})
	}
	slices.SortStableFunc(d.Causes, func(a, b countRow) int { return cmp.Compare(b.N, a.N) })
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

func dotRow(label string, c results.Counts, muted bool) DotRow {
	lo, hi := Wilson(c.PQDefault, c.Reachable())
	return DotRow{Label: label, N: c.Reachable(), Est: c.Pct(c.PQDefault), Lo: lo, Hi: hi, Muted: muted}
}

// shares turns counts into table rows, largest first, then by name.
func shares(m map[string]int, name func(string) (string, bool)) []shareRow {
	total := sum(m)
	var out []shareRow
	for k, n := range m {
		label, hybrid := name(k)
		out = append(out, shareRow{Name: label, Hybrid: hybrid, N: n, Pct: 100 * float64(n) / float64(total)})
	}
	slices.SortFunc(out, func(a, b shareRow) int {
		if c := cmp.Compare(b.N, a.N); c != 0 {
			return c
		}
		return strings.Compare(a.Name, b.Name)
	})
	return out
}

// waffleCells returns, in rank order, whether each reachable domain ranked
// topRank or better is post-quantum by default.
func waffleCells(recs []results.Record, topRank int) []bool {
	var top []results.Record
	for _, r := range recs {
		if r.TrancoRank > 0 && r.TrancoRank <= topRank && r.Status != results.StatusUnreachable {
			top = append(top, r)
		}
	}
	slices.SortFunc(top, func(a, b results.Record) int { return cmp.Compare(a.TrancoRank, b.TrancoRank) })
	cells := make([]bool, len(top))
	for i, r := range top {
		cells[i] = r.Status == results.StatusPQDefault
	}
	return cells
}

// longDate turns 2026-09-22 into 22 September 2026.
func longDate(date string) string {
	t, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return date
	}
	return t.Format("2 January 2006")
}

func scanWindow(s results.Summary) string {
	a, b := s.StartedAt.UTC(), s.FinishedAt.UTC()
	if a.Format(time.DateOnly) == b.Format(time.DateOnly) {
		return a.Format("2 January 2006, 15:04") + " to " + b.Format("15:04") + " UTC"
	}
	return a.Format("2 January 2006 15:04") + " to " + b.Format("2 January 2006 15:04") + " UTC"
}

// shortRevision keeps the first 12 characters of a commit hash and a -dirty suffix.
func shortRevision(rev string) string {
	rev, dirty := strings.CutSuffix(rev, "-dirty")
	if len(rev) > 12 {
		rev = rev[:12]
	}
	if dirty {
		rev += "-dirty"
	}
	return rev
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
		idx.Domains = append(idx.Domains, domainEntry{Domain: r.Domain, Host: r.Host, Status: string(r.Status), Provider: NetworkName(r.ASOrg)})
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
