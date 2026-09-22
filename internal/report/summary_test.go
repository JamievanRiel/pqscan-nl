package report

import (
	"reflect"
	"testing"
	"time"

	"github.com/JamievanRiel/pqscan-nl/internal/results"
)

func fixtureRecords() []results.Record {
	at := func(min int) time.Time { return time.Date(2026, 9, 28, 2, min, 0, 0, time.UTC) }
	return []results.Record{
		{Domain: "a.nl", Host: "a.nl", TrancoRank: 1, Status: results.StatusPQDefault, Group: "X25519MLKEM768", TLSVersion: "1.3", ASN: 13335, ASOrg: "CLOUDFLARENET", ScannedAt: at(3)},
		{Domain: "b.nl", Host: "www.b.nl", TrancoRank: 2, Status: results.StatusPQDefault, Group: "X25519MLKEM768", TLSVersion: "1.3", ASN: 13335, ASOrg: "CLOUDFLARENET", ScannedAt: at(1)},
		{Domain: "c.nl", Host: "c.nl", TrancoRank: 3, Sectors: []string{"banks"}, Status: results.StatusClassic, Group: "X25519", TLSVersion: "1.2", ASN: 1136, ASOrg: "KPN", ScannedAt: at(2)},
		{Domain: "d.nl", Host: "d.nl", TrancoRank: 4, Status: results.StatusPQSupported, Group: "X25519", TLSVersion: "1.3", ASN: 16509, ASOrg: "AMAZON-02", ScannedAt: at(4)},
		{Domain: "e.nl", TrancoRank: 5, Status: results.StatusUnreachable, Error: "timeout", ScannedAt: at(5)},
		{Domain: "bank.nl", Host: "bank.nl", Sectors: []string{"banks"}, Status: results.StatusPQDefault, Group: "X25519MLKEM768", TLSVersion: "1.3", ASN: 13335, ASOrg: "CLOUDFLARENET", ScannedAt: at(6)},
		{Domain: "gemeente.nl", Sectors: []string{"government"}, Status: results.StatusUnreachable, Error: "dns", ScannedAt: at(7)},
		{Domain: "f.nl", Host: "f.nl", TrancoRank: 6, Status: results.StatusClassic, Group: "CurveP256", TLSVersion: "1.3", ASN: 1136, ASOrg: "KPN", ScannedAt: at(8)},
		{Domain: "g.nl", Host: "g.nl", TrancoRank: 400000, Status: results.StatusPQDefault, Group: "X25519MLKEM768", TLSVersion: "1.3", ASN: 13335, ASOrg: "CLOUDFLARENET", ScannedAt: at(5)},
	}
}

func TestSummarize(t *testing.T) {
	got := Summarize(fixtureRecords(), "64X5X", 2)
	want := results.Summary{
		Date:          "2026-09-28",
		TrancoListID:  "64X5X",
		StartedAt:     time.Date(2026, 9, 28, 2, 1, 0, 0, time.UTC),
		FinishedAt:    time.Date(2026, 9, 28, 2, 8, 0, 0, time.UTC),
		Tranco:        results.Counts{Total: 7, PQDefault: 3, PQSupported: 1, Classic: 2, Unreachable: 1},
		TrancoTopRank: 250000,
		TrancoTop:     results.Counts{Total: 6, PQDefault: 2, PQSupported: 1, Classic: 2, Unreachable: 1},
		ByRank: []results.RankCounts{
			{From: 1, To: 10000, Counts: results.Counts{Total: 6, PQDefault: 2, PQSupported: 1, Classic: 2, Unreachable: 1}},
			{From: 10001, To: 50000},
			{From: 50001, To: 100000},
			{From: 100001, To: 250000},
			{From: 250001, To: 1000000, Counts: results.Counts{Total: 1, PQDefault: 1}},
		},
		Groups:      map[string]int{"X25519MLKEM768": 2, "X25519": 2, "CurveP256": 1},
		TLSVersions: map[string]int{"1.3": 4, "1.2": 1},
		Sectors: map[string]results.Counts{
			"banks":      {Total: 2, PQDefault: 1, Classic: 1},
			"government": {Total: 1, Unreachable: 1},
		},
		Providers: []results.ProviderCounts{
			{ASN: 1136, Org: "KPN", Counts: results.Counts{Total: 2, Classic: 2}},
			{ASN: 13335, Org: "CLOUDFLARENET", Counts: results.Counts{Total: 2, PQDefault: 2}},
		},
		UnreachableByError: map[string]int{"timeout": 1, "dns": 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestSummariesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	later := Summarize(fixtureRecords(), "64X5X", 20)
	earlier := later
	earlier.Date = "2026-09-21"
	for _, s := range []results.Summary{later, earlier} {
		if _, err := WriteSummary(dir, s); err != nil {
			t.Fatal(err)
		}
	}
	got, err := LoadSummaries(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Date != "2026-09-21" || got[1].Date != "2026-09-28" {
		t.Fatalf("dates = %v, want 2026-09-21 then 2026-09-28", []string{got[0].Date, got[1].Date})
	}
	if !reflect.DeepEqual(got[1], later) {
		t.Fatalf("round trip changed the summary:\n got  %+v\n want %+v", got[1], later)
	}
}
