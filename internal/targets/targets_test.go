package targets

import (
	"archive/zip"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/JamievanRiel/pqscan-nl/internal/results"
)

func TestNormalizeDomain(t *testing.T) {
	for in, want := range map[string]string{
		" WWW.Example.NL. ": "example.nl",
		"shop.example.nl":   "shop.example.nl",
		"":                  "",
	} {
		if got := NormalizeDomain(in); got != want {
			t.Errorf("NormalizeDomain(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseTranco(t *testing.T) {
	in := "1,google.com\r\n2,Example.NL\r\n3,bol.com\r\n4,www.example.nl\r\n5,nu.nl\r\n"
	got, err := ParseTranco(strings.NewReader(in), ".nl")
	if err != nil {
		t.Fatal(err)
	}
	want := []results.Target{{Domain: "example.nl", TrancoRank: 2}, {Domain: "nu.nl", TrancoRank: 5}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestParseTrancoBadLine(t *testing.T) {
	_, err := ParseTranco(strings.NewReader("1,a.nl\nx,b.nl\n"), ".nl")
	if err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("err = %v, want an error mentioning line 2", err)
	}
}

func TestReadTrancoZip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "top-1m.csv.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("top-1m.csv")
	if err != nil {
		t.Fatal(err)
	}
	w.Write([]byte("1,google.com\r\n2,nu.nl\r\n"))
	zw.Close()
	f.Close()

	got, err := ReadTrancoZip(path, ".nl")
	if err != nil {
		t.Fatal(err)
	}
	want := []results.Target{{Domain: "nu.nl", TrancoRank: 2}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestParseSectorCSV(t *testing.T) {
	in := "domain,name,source\nwww.ING.nl,ING,https://example.org/list\nbol.com,bol,https://example.org/list\n"
	got, err := ParseSectorCSV(strings.NewReader(in), "banks")
	if err != nil {
		t.Fatal(err)
	}
	want := []results.Target{
		{Domain: "ing.nl", Sectors: []string{"banks"}},
		{Domain: "bol.com", Sectors: []string{"banks"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestParseSectorCSVRejects(t *testing.T) {
	for name, in := range map[string]string{
		"bad header":  "url,name,source\na.nl,A,https://x\n",
		"no source":   "domain,name,source\na.nl,A,\n",
		"bad domain":  "domain,name,source\nhttps://a.nl,A,https://x\n",
		"wrong width": "domain,name,source\na.nl,A\n",
	} {
		if _, err := ParseSectorCSV(strings.NewReader(in), "x"); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestLoadSectors(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "banks.csv"), []byte("domain,name,source\ning.nl,ING,https://x\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "hospitals.csv"), []byte("domain,name,source\numcutrecht.nl,UMC Utrecht,https://x\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignored"), 0o644)

	got, err := LoadSectors(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []results.Target{
		{Domain: "ing.nl", Sectors: []string{"banks"}},
		{Domain: "umcutrecht.nl", Sectors: []string{"hospitals"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestParseExclude(t *testing.T) {
	got, err := ParseExclude(strings.NewReader("# opt-outs\nwww.Opted-Out.nl  # asked 2026-10-01\n\nother.nl\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"opted-out.nl": true, "other.nl": true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestMerge(t *testing.T) {
	tranco := []results.Target{
		{Domain: "nu.nl", TrancoRank: 5},
		{Domain: "ing.nl", TrancoRank: 3},
		{Domain: "gone.nl", TrancoRank: 1},
	}
	sectors := []results.Target{
		{Domain: "ing.nl", Sectors: []string{"banks"}},
		{Domain: "bol.com", Sectors: []string{"webshops"}},
		{Domain: "amsterdam.nl", Sectors: []string{"government"}},
		{Domain: "ing.nl", Sectors: []string{"banks"}},
	}
	got := Merge(tranco, sectors, map[string]bool{"gone.nl": true})
	want := []results.Target{
		{Domain: "ing.nl", TrancoRank: 3, Sectors: []string{"banks"}},
		{Domain: "nu.nl", TrancoRank: 5},
		{Domain: "amsterdam.nl", Sectors: []string{"government"}},
		{Domain: "bol.com", Sectors: []string{"webshops"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
