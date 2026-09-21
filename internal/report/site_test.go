package report

import (
	"bytes"
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
	earlier.Tranco = results.Counts{Total: 6, PQDefault: 1, PQSupported: 1, Classic: 3, Unreachable: 1}

	out := t.TempDir()
	if err := BuildSite(out, []results.Summary{earlier, latest}, recs); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"index.html", "search.html", "methodology.html", "findings.html", "style.css", "search.js", "domains.json"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}

	index, err := os.ReadFile(filepath.Join(out, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"40.0%", "<polyline", "Banks", "Government", "KPN", `aria-current="page"`} {
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
