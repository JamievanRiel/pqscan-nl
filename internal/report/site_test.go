package report

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/JamievanRiel/pqscan-nl/internal/results"
)

var update = flag.Bool("update", false, "rewrite golden files in testdata")

func TestThousands(t *testing.T) {
	for n, want := range map[int]string{0: "0", 999: "999", 1000: "1,000", 19077: "19,077", 1234567: "1,234,567", -4200: "-4,200"} {
		if got := thousands(n); got != want {
			t.Errorf("thousands(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestBuildSite(t *testing.T) {
	recs := fixtureRecords()
	latest := Summarize(recs, "64X5X", 20)
	earlier := latest
	earlier.Date = "2026-09-21"
	earlier.TrancoTop = results.Counts{Total: 6, PQDefault: 1, PQSupported: 1, Classic: 3, Unreachable: 1}

	out := t.TempDir()
	if err := BuildSite(out, fixtureSectors(t), []results.Summary{earlier, latest}, recs); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"index.html", "search.html", "methodology.html", "data.html", "style.css", "search.js", "domains.json", "scan-2026-09-28.jsonl.gz",
		"sectors/banks.csv", "sectors/government.csv", "summaries/2026-09-21.json", "summaries/2026-09-28.json"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}

	if _, err := os.Stat(filepath.Join(out, "findings.html")); err == nil {
		t.Error("findings.html must be gone")
	}
	index, err := os.ReadFile(filepath.Join(out, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"40.0%", "95% CI 11.8–76.9%", `class="waffle"`, `<polygon class="band"`, `<ol class="findings">`,
		"Figure 1.", "Figure 5.", "Table 3.", "Jamie van Riel", `aria-current="page"`, "250,001–1,000,000", "<code>X25519MLKEM768</code>"} {
		if !bytes.Contains(index, []byte(want)) {
			t.Errorf("index.html lacks %q", want)
		}
	}
	golden(t, "index.golden.html", index)
	for _, page := range []string{"methodology", "data"} {
		b, err := os.ReadFile(filepath.Join(out, page+".html"))
		if err != nil {
			t.Fatal(err)
		}
		golden(t, page+".golden.html", b)
	}

	search, _ := os.ReadFile(filepath.Join(out, "search.html"))
	if !bytes.Contains(search, []byte(`src="search.js"`)) {
		t.Error("search.html must load search.js")
	}

	var idx struct {
		Date    string `json:"date"`
		Domains []struct {
			D, H, S, P string
		} `json:"domains"`
	}
	b, _ := os.ReadFile(filepath.Join(out, "domains.json"))
	if err := json.Unmarshal(b, &idx); err != nil {
		t.Fatal(err)
	}
	if idx.Date != "2026-09-28" || len(idx.Domains) != len(recs) || idx.Domains[0].D != "a.nl" || idx.Domains[0].S != "pq-default" || idx.Domains[0].P != "Cloudflare" {
		t.Fatalf("unexpected domains.json: %s", b)
	}
	if strings.Contains(string(b), `"h":""`) {
		t.Error("empty host must be omitted")
	}

	f, err := os.Open(filepath.Join(out, "scan-2026-09-28.jsonl.gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := results.ReadJSONL[results.Record](zr)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != len(recs) || raw[0].Domain != recs[0].Domain || raw[0].Status != recs[0].Status {
		t.Fatalf("raw data holds %+v, want the scan records", raw)
	}
}

// The repository is private, so the public site must not link into it, and
// every relative link must resolve to a file in the site.
func TestBuildSiteLinks(t *testing.T) {
	recs := fixtureRecords()
	out := t.TempDir()
	if err := BuildSite(out, fixtureSectors(t), []results.Summary{Summarize(recs, "64X5X", 20)}, recs); err != nil {
		t.Fatal(err)
	}
	attr := regexp.MustCompile(`(?:href|src)="([^"#?]+)`)
	for _, page := range pages {
		b, err := os.ReadFile(filepath.Join(out, page+".html"))
		if err != nil {
			t.Fatal(err)
		}
		for _, bad := range []string{`github.com/JamievanRiel/pqscan-nl"`, "github.com/JamievanRiel/pqscan-nl/"} {
			if bytes.Contains(b, []byte(bad)) {
				t.Errorf("%s.html links to the private repository (%s)", page, bad)
			}
		}
		if !bytes.Contains(b, []byte(`href="scan-2026-09-28.jsonl.gz"`)) {
			t.Errorf("%s.html lacks a link to the raw data", page)
		}
		for _, m := range attr.FindAllSubmatch(b, -1) {
			link := string(m[1])
			if strings.Contains(link, ":") {
				continue // absolute URL or data URI
			}
			if _, err := os.Stat(filepath.Join(out, link)); err != nil {
				t.Errorf("%s.html links to missing %s", page, link)
			}
		}
	}
	methodology, _ := os.ReadFile(filepath.Join(out, "methodology.html"))
	if !bytes.Contains(methodology, []byte("github.com/JamievanRiel/pqscan-nl-optout/issues/new")) {
		t.Error("methodology.html lacks the opt-out link")
	}
	data, _ := os.ReadFile(filepath.Join(out, "data.html"))
	for _, want := range []string{`href="sectors/banks.csv"`, `href="summaries/2026-09-28.json"`, `id="cite"`, "@misc{vanriel2026pqscan,", "year         = {2026},"} {
		if !bytes.Contains(data, []byte(want)) {
			t.Errorf("data.html lacks %q", want)
		}
	}
}

func TestNewPageData(t *testing.T) {
	recs := fixtureRecords()
	d := newPageData([]results.Summary{Summarize(recs, "64X5X", 20)}, recs, nil)
	if d.Headline != 40 || pct1(d.HeadLo) != "11.8" || pct1(d.HeadHi) != "76.9" {
		t.Errorf("headline = %v (%v–%v), want 40 (11.8–76.9)", d.Headline, d.HeadLo, d.HeadHi)
	}
	if d.Tail != (results.Counts{Total: 1, PQDefault: 1}) {
		t.Errorf("tail = %+v", d.Tail)
	}
	if d.Scanned != len(recs) || d.LatestDate != "28 September 2026" || d.Year != 2026 {
		t.Errorf("scanned %d, date %q, year %d", d.Scanned, d.LatestDate, d.Year)
	}
	wantGroups := []shareRow{{Name: "X25519", N: 2, Pct: 40}, {Name: "X25519MLKEM768", Hybrid: true, N: 2, Pct: 40}, {Name: "P-256", N: 1, Pct: 20}}
	if !reflect.DeepEqual(d.Groups, wantGroups) {
		t.Errorf("groups = %+v", d.Groups)
	}
	if len(d.TLS) != 2 || d.TLS[0] != (shareRow{Name: "TLS 1.3", N: 4, Pct: 80}) {
		t.Errorf("tls = %+v", d.TLS)
	}
	if len(d.Causes) != 2 || d.Causes[0] != (countRow{Name: "DNS failure", N: 1}) {
		t.Errorf("causes = %+v", d.Causes)
	}
	if d.ScanWindow != "28 September 2026, 02:01 to 02:08 UTC" {
		t.Errorf("scan window = %q", d.ScanWindow)
	}
}

func TestBuildSiteNeedsSectorLists(t *testing.T) {
	recs := fixtureRecords()
	err := BuildSite(t.TempDir(), t.TempDir(), []results.Summary{Summarize(recs, "64X5X", 20)}, recs)
	if err == nil || !strings.Contains(err.Error(), "no sector CSV files") {
		t.Fatalf("err = %v, want an error about missing sector lists", err)
	}
}

// fixtureSectors writes two sector lists and returns their directory.
func fixtureSectors(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string]string{
		"banks.csv":      "domain,name,source\nc.nl,C Bank,https://example.org/banks\nbank.nl,Bank,https://example.org/banks\n",
		"government.csv": "domain,name,source\ngemeente.nl,Gemeente,https://example.org/gov\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestHumanSize(t *testing.T) {
	for n, want := range map[int64]string{900: "900 B", 1635: "1.6 KB", 673389: "658 KB", 1628541: "1.6 MB"} {
		if got := humanSize(n); got != want {
			t.Errorf("humanSize(%d) = %q, want %q", n, got, want)
		}
	}
}

// golden compares got with testdata/name, or rewrites it with -update.
func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run go test ./internal/report -update to create it)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs from the golden file; run go test ./internal/report -update and review the diff", name)
	}
}
