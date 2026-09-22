package report

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/JamievanRiel/pqscan-nl/internal/results"
)

// MinSample is the number of reachable domains a network or sector needs to
// appear in the figures and findings. Below it, one domain moves the share by
// more than five percentage points.
const MinSample = 20

// KeyFindings returns the report's key findings as sentences. Each finding
// after the headline is only included when the data supports it.
func KeyFindings(s results.Summary) []string {
	top := s.TrancoTop
	lo, hi := Wilson(top.PQDefault, top.Reachable())
	out := []string{fmt.Sprintf("%s%% (95%% CI %s–%s%%) of the %s reachable .nl domains in the Tranco top %s use post-quantum key exchange by default.",
		pct1(top.Pct(top.PQDefault)), pct1(lo), pct1(hi), thousands(top.Reachable()), thousands(s.TrancoTopRank))}

	if nets := eligibleNetworks(s.Providers); len(nets) >= 2 {
		best, worst := nets[0], nets[len(nets)-1]
		out = append(out, fmt.Sprintf("Adoption depends on the hosting network: among networks with at least %d domains it ranges from %s%% (%s) to %s%% (%s).",
			MinSample, pct1(best.Pct(best.PQDefault)), NetworkName(best.Org), pct1(worst.Pct(worst.PQDefault)), NetworkName(worst.Org)))
	}
	if c := Concentration(s); c != "" {
		out = append(out, c)
	}
	if n12, n := s.TLSVersions["1.2"], sum(s.TLSVersions); n12 > 0 {
		out = append(out, fmt.Sprintf("%s%% of reachable domains negotiate TLS 1.2 with a current browser, which rules out the hybrid key exchange.",
			pct1(100*float64(n12)/float64(n))))
	}
	if first, last, ok := popularityBands(s); ok {
		out = append(out, fmt.Sprintf("Adoption is %s%% in the top %s and %s%% in ranks %s–%s.",
			pct1(first.Pct(first.PQDefault)), thousands(first.To), pct1(last.Pct(last.PQDefault)), thousands(last.From), thousands(last.To)))
	}
	if secs := eligibleSectors(s.Sectors); len(secs) >= 2 {
		best, worst := secs[0], secs[len(secs)-1]
		out = append(out, fmt.Sprintf("Among the sectors, %s score highest (%s%%) and %s lowest (%s%%).",
			strings.ToLower(sectorName(best.name)), pct1(best.pct), strings.ToLower(sectorName(worst.name)), pct1(worst.pct)))
	}
	return out
}

// Concentration says which share of the post-quantum domains the networks
// with the most of them host, or returns "" with fewer than three networks.
func Concentration(s results.Summary) string {
	if s.TrancoTop.PQDefault == 0 || len(s.Providers) < 3 {
		return ""
	}
	byPQ := slices.Clone(s.Providers)
	slices.SortStableFunc(byPQ, func(a, b results.ProviderCounts) int { return cmp.Compare(b.PQDefault, a.PQDefault) })
	total := float64(s.TrancoTop.PQDefault)
	three := byPQ[0].PQDefault + byPQ[1].PQDefault + byPQ[2].PQDefault
	return fmt.Sprintf("%s alone hosts %s%% of all post-quantum domains; the three networks with the most post-quantum domains together host %s%%.",
		NetworkName(byPQ[0].Org), pct1(100*float64(byPQ[0].PQDefault)/total), pct1(100*float64(three)/total))
}

// eligibleNetworks returns the providers with at least MinSample reachable
// domains, highest adoption first.
func eligibleNetworks(ps []results.ProviderCounts) []results.ProviderCounts {
	var out []results.ProviderCounts
	for _, p := range ps {
		if p.Reachable() >= MinSample {
			out = append(out, p)
		}
	}
	slices.SortStableFunc(out, func(a, b results.ProviderCounts) int { return cmp.Compare(b.Pct(b.PQDefault), a.Pct(a.PQDefault)) })
	return out
}

type sectorShare struct {
	name string
	pct  float64
	c    results.Counts
}

// eligibleSectors returns the sectors with at least MinSample reachable
// domains, highest adoption first, then by name.
func eligibleSectors(m map[string]results.Counts) []sectorShare {
	var out []sectorShare
	for _, name := range slices.Sorted(maps.Keys(m)) {
		if c := m[name]; c.Reachable() >= MinSample {
			out = append(out, sectorShare{name: name, pct: c.Pct(c.PQDefault), c: c})
		}
	}
	slices.SortStableFunc(out, func(a, b sectorShare) int { return cmp.Compare(b.pct, a.pct) })
	return out
}

// popularityBands returns the first rank band and the last band inside the
// headline population, when both have reachable domains.
func popularityBands(s results.Summary) (first, last results.RankCounts, ok bool) {
	var in []results.RankCounts
	for _, b := range s.ByRank {
		if b.To <= s.TrancoTopRank {
			in = append(in, b)
		}
	}
	if len(in) < 2 || in[0].Reachable() == 0 || in[len(in)-1].Reachable() == 0 {
		return first, last, false
	}
	return in[0], in[len(in)-1], true
}

func sum(m map[string]int) int {
	n := 0
	for _, v := range m {
		n += v
	}
	return n
}

// pct1 formats a percentage with one decimal: 45.0.
func pct1(f float64) string { return strconv.FormatFloat(f, 'f', 1, 64) }
