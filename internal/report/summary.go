// Package report turns scan records into summaries and a static website.
package report

import (
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/JamievanRiel/pqscan-nl/internal/results"
)

// Summarize aggregates one scan. The headline counts and the providers use
// Tranco domains only; sector counts and unreachable causes use every record.
// Providers are the topN networks by number of reachable Tranco domains.
func Summarize(recs []results.Record, trancoListID string, topN int) results.Summary {
	s := results.Summary{
		TrancoListID:       trancoListID,
		Sectors:            map[string]results.Counts{},
		Providers:          []results.ProviderCounts{},
		UnreachableByError: map[string]int{},
	}
	providers := map[uint32]*results.ProviderCounts{}
	for i, r := range recs {
		if i == 0 || r.ScannedAt.Before(s.StartedAt) {
			s.StartedAt = r.ScannedAt
		}
		if r.ScannedAt.After(s.FinishedAt) {
			s.FinishedAt = r.ScannedAt
		}
		if r.TrancoRank > 0 {
			s.Tranco.Add(r.Status)
			if r.Status != results.StatusUnreachable && r.ASN != 0 {
				pc := providers[r.ASN]
				if pc == nil {
					pc = &results.ProviderCounts{ASN: r.ASN, Org: r.ASOrg}
					providers[r.ASN] = pc
				}
				pc.Add(r.Status)
			}
		}
		for _, sector := range r.Sectors {
			c := s.Sectors[sector]
			c.Add(r.Status)
			s.Sectors[sector] = c
		}
		if r.Status == results.StatusUnreachable {
			s.UnreachableByError[r.Error]++
		}
	}
	s.StartedAt, s.FinishedAt = s.StartedAt.UTC(), s.FinishedAt.UTC()
	s.Date = s.StartedAt.Format(time.DateOnly)

	for _, pc := range providers {
		s.Providers = append(s.Providers, *pc)
	}
	slices.SortFunc(s.Providers, func(a, b results.ProviderCounts) int {
		if c := cmp.Compare(b.Total, a.Total); c != 0 {
			return c
		}
		return cmp.Compare(a.ASN, b.ASN)
	})
	if len(s.Providers) > topN {
		s.Providers = s.Providers[:topN]
	}
	return s
}

// WriteSummary stores s as dir/<date>.json and returns the path.
func WriteSummary(dir string, s results.Summary) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, s.Date+".json")
	return path, os.WriteFile(path, append(b, '\n'), 0o644)
}

// LoadSummaries reads every *.json file in dir, oldest first.
func LoadSummaries(dir string) ([]results.Summary, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	var out []results.Summary
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var s results.Summary
		if err := json.Unmarshal(b, &s); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		out = append(out, s)
	}
	slices.SortFunc(out, func(a, b results.Summary) int { return strings.Compare(a.Date, b.Date) })
	return out, nil
}
