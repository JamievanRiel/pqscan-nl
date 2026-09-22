package report

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
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
	if err := BuildSite(out, []results.Summary{earlier, latest}, recs); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"index.html", "search.html", "methodology.html", "findings.html", "style.css", "search.js", "domains.json", "scan-2026-09-28.jsonl.gz"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}

	index, err := os.ReadFile(filepath.Join(out, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"40.0%", "<polyline", "Banks", "Government", "KPN", `aria-current="page"`, "Tranco top 250k", "Tranco 250k–1M", "250,000"} {
		if !bytes.Contains(index, []byte(want)) {
			t.Errorf("index.html lacks %q", want)
		}
	}
	golden(t, "index.golden.html", index)

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
	if idx.Date != "2026-09-28" || len(idx.Domains) != len(recs) || idx.Domains[0].D != "a.nl" || idx.Domains[0].S != "pq-default" || idx.Domains[0].P != "CLOUDFLARENET" {
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

// The repository is private, so the public site must not link into it.
func TestBuildSiteLinks(t *testing.T) {
	recs := fixtureRecords()
	out := t.TempDir()
	if err := BuildSite(out, []results.Summary{Summarize(recs, "64X5X", 20)}, recs); err != nil {
		t.Fatal(err)
	}
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
	}
	methodology, _ := os.ReadFile(filepath.Join(out, "methodology.html"))
	for _, want := range []string{"github.com/JamievanRiel/pqscan-nl-optout/issues/new", `href="sectors/banks.csv"`, `href="sectors/government.csv"`} {
		if !bytes.Contains(methodology, []byte(want)) {
			t.Errorf("methodology.html lacks %q", want)
		}
	}
}

func TestBuildSiteNeedsSummaries(t *testing.T) {
	if err := BuildSite(t.TempDir(), nil, nil); err == nil {
		t.Fatal("expected an error without summaries")
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
