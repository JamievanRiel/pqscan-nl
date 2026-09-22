package main

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/JamievanRiel/pqscan-nl/internal/results"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunLists(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "top-1m.csv.zip")
	f, _ := os.Create(zipPath)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("top-1m.csv")
	w.Write([]byte("1,google.com\r\n2,nu.nl\r\n3,ing.nl\r\n4,optout.nl\r\n"))
	zw.Close()
	f.Close()
	writeFile(t, filepath.Join(dir, "sectors", "banks.csv"), "domain,name,source\ning.nl,ING,https://x\nasnbank.nl,ASN Bank,https://x\n")
	writeFile(t, filepath.Join(dir, "exclude.txt"), "# opt-outs\noptout.nl\n")
	out := filepath.Join(dir, "targets.jsonl")

	var stderr bytes.Buffer
	err := run(context.Background(), []string{"lists", "-tranco", zipPath, "-sectors", filepath.Join(dir, "sectors"),
		"-exclude", filepath.Join(dir, "exclude.txt"), "-o", out}, &bytes.Buffer{}, &stderr)
	if err != nil {
		t.Fatalf("run: %v (stderr: %s)", err, stderr.String())
	}
	b, _ := os.ReadFile(out)
	got, err := results.ReadJSONL[results.Target](bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	want := []results.Target{
		{Domain: "nu.nl", TrancoRank: 2},
		{Domain: "ing.nl", TrancoRank: 3, Sectors: []string{"banks"}},
		{Domain: "asnbank.nl", Sectors: []string{"banks"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestRunReport(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2026, 9, 28, 2, 0, 0, 0, time.UTC)
	recs := []results.Record{
		{Domain: "a.nl", Host: "a.nl", TrancoRank: 1, Status: results.StatusPQDefault, ASN: 13335, ASOrg: "CLOUDFLARENET", ScannedAt: at},
		{Domain: "b.nl", TrancoRank: 2, Status: results.StatusUnreachable, Error: "dns", ScannedAt: at},
	}
	var buf bytes.Buffer
	results.WriteJSONL(&buf, recs)
	scan := filepath.Join(dir, "scan.jsonl")
	writeFile(t, scan, buf.String())
	banks := "domain,name,source\ning.nl,ING,https://x\n"
	writeFile(t, filepath.Join(dir, "sectors", "banks.csv"), banks)
	writeFile(t, filepath.Join(dir, "sectors", "README"), "not a list")

	var stdout, stderr bytes.Buffer
	err := run(context.Background(), []string{"report", "-scan", scan, "-tranco-id", " 64X5X\n",
		"-summaries", filepath.Join(dir, "summaries"), "-sectors", filepath.Join(dir, "sectors"), "-site", filepath.Join(dir, "site")}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v (stderr: %s)", err, stderr.String())
	}
	if stdout.String() != "2026-09-28\n" {
		t.Errorf("stdout = %q, want the scan date", stdout.String())
	}
	summary, err := os.ReadFile(filepath.Join(dir, "summaries", "2026-09-28.json"))
	if err != nil || !strings.Contains(string(summary), `"tranco_list_id": "64X5X"`) {
		t.Errorf("summary = %s, err %v", summary, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "site", "index.html")); err != nil {
		t.Error(err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "site", "sectors", "banks.csv")); err != nil || string(b) != banks {
		t.Errorf("published banks.csv = %q, err %v", b, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "site", "sectors", "README")); err == nil {
		t.Error("only CSV files belong in site/sectors")
	}
}

func TestRunReportNeedsTrancoID(t *testing.T) {
	err := run(context.Background(), []string{"report", "-scan", "x.jsonl"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "tranco-id") {
		t.Fatalf("err = %v, want an error about -tranco-id", err)
	}
}

func TestCheckUnreachable(t *testing.T) {
	recs := func(unreachable, total int) []results.Record {
		out := make([]results.Record, total)
		for i := range out {
			out[i].Status = results.StatusClassic
			if i < unreachable {
				out[i].Status = results.StatusUnreachable
			}
		}
		return out
	}
	if err := checkUnreachable(recs(3, 10), 0.30); err != nil {
		t.Errorf("30%% unreachable must pass: %v", err)
	}
	if err := checkUnreachable(recs(4, 10), 0.30); err == nil {
		t.Error("40% unreachable must fail")
	}
	if err := checkUnreachable(nil, 0.30); err == nil {
		t.Error("no records must fail")
	}
}

func TestUnknownCommand(t *testing.T) {
	var stderr bytes.Buffer
	if err := run(context.Background(), []string{"nope"}, &bytes.Buffer{}, &stderr); err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Error("usage must be printed")
	}
}
