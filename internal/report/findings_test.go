package report

import (
	"reflect"
	"strings"
	"testing"

	"github.com/JamievanRiel/pqscan-nl/internal/results"
)

func findingsSummary() results.Summary {
	return results.Summary{
		TrancoTopRank: 250000,
		TrancoTop:     results.Counts{Total: 110, PQDefault: 45, PQSupported: 1, Classic: 54, Unreachable: 10},
		ByRank: []results.RankCounts{
			{From: 1, To: 10000, Counts: results.Counts{Total: 10, PQDefault: 6, Classic: 4}},
			{From: 10001, To: 250000, Counts: results.Counts{Total: 100, PQDefault: 39, PQSupported: 1, Classic: 50, Unreachable: 10}},
			{From: 250001, To: 1000000, Counts: results.Counts{Total: 50, PQDefault: 40, Classic: 10}},
		},
		TLSVersions: map[string]int{"1.3": 90, "1.2": 10},
		Sectors: map[string]results.Counts{
			"banks":      {Total: 25, PQDefault: 20, Classic: 5},
			"government": {Total: 40, PQDefault: 8, Classic: 30, Unreachable: 2},
			"hospitals":  {Total: 10, PQDefault: 1, Classic: 9},
		},
		Providers: []results.ProviderCounts{
			{ASN: 13335, Org: "CLOUDFLARENET", Counts: results.Counts{Total: 40, PQDefault: 38, Classic: 2}},
			{ASN: 20857, Org: "TRANSIP-AS Amsterdam, the Netherlands", Counts: results.Counts{Total: 30, PQDefault: 1, Classic: 29}},
			{ASN: 16509, Org: "AMAZON-02", Counts: results.Counts{Total: 20, PQDefault: 5, PQSupported: 1, Classic: 14}},
			{ASN: 64500, Org: "SMALL-AS", Counts: results.Counts{Total: 10, PQDefault: 1, Classic: 9}},
		},
	}
}

func TestKeyFindings(t *testing.T) {
	want := []string{
		"45.0% (95% CI 35.6–54.8%) of the 100 reachable .nl domains in the Tranco top 250,000 use post-quantum key exchange by default.",
		"Adoption depends on the hosting network: among networks with at least 20 domains it ranges from 95.0% (Cloudflare) to 3.3% (TransIP).",
		"Cloudflare alone hosts 84.4% of all post-quantum domains; the three networks with the most post-quantum domains together host 97.8%.",
		"10.0% of reachable domains negotiate TLS 1.2 with a current browser, which rules out the hybrid key exchange.",
		"Adoption is 60.0% in the top 10,000 and 43.3% in ranks 10,001–250,000.",
		"Among the sectors, banks score highest (80.0%) and government lowest (21.1%).",
	}
	if got := KeyFindings(findingsSummary()); !reflect.DeepEqual(got, want) {
		t.Fatalf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// Each finding beyond the headline needs enough data to say something.
func TestKeyFindingsNeedData(t *testing.T) {
	s := results.Summary{
		TrancoTopRank: 250000,
		TrancoTop:     results.Counts{Total: 30, Classic: 30},
		ByRank:        []results.RankCounts{{From: 1, To: 10000}, {From: 10001, To: 250000, Counts: results.Counts{Total: 30, Classic: 30}}},
		TLSVersions:   map[string]int{"1.3": 30},
		Sectors:       map[string]results.Counts{"banks": {Total: 30, Classic: 30}},
		Providers:     []results.ProviderCounts{{ASN: 1, Org: "ONE", Counts: results.Counts{Total: 30, Classic: 30}}},
	}
	got := KeyFindings(s)
	if len(got) != 1 || !strings.HasPrefix(got[0], "0.0% (95% CI 0.0–11.4%) of the 30 reachable") {
		t.Fatalf("got %q, want only the headline", got)
	}
}
