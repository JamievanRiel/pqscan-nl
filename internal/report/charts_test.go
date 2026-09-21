package report

import (
	"strings"
	"testing"

	"github.com/JamievanRiel/pqscan-nl/internal/results"
)

func TestTrendSVG(t *testing.T) {
	if TrendSVG(nil) != "" {
		t.Error("no points must draw nothing")
	}
	one := string(TrendSVG([]TrendPoint{{Date: "2026-09-28", Pct: 40}}))
	if strings.Contains(one, "<polyline") || strings.Count(one, "<circle") != 1 {
		t.Errorf("one point should be a single dot without a line: %s", one)
	}
	two := string(TrendSVG([]TrendPoint{{Date: "2026-09-21", Pct: 20}, {Date: "2026-09-28", Pct: 40}}))
	for _, want := range []string{"<polyline", "2026-09-21: 20.0%", "2026-09-28: 40.0%", ">100%<", ">0%<"} {
		if !strings.Contains(two, want) {
			t.Errorf("trend chart lacks %q: %s", want, two)
		}
	}
}

func TestStackedBarsSVG(t *testing.T) {
	if StackedBarsSVG(nil) != "" {
		t.Error("no rows must draw nothing")
	}
	svg := string(StackedBarsSVG([]BarRow{{Label: "A&B", Counts: results.Counts{Total: 5, PQDefault: 2, PQSupported: 1, Classic: 1, Unreachable: 1}}}))
	// The bar is 414 units wide (640 - 170 label - 56 value); shares are of the 4 reachable domains.
	for _, want := range []string{`width="207.0"`, `width="103.5"`, "A&amp;B", ">50%<", "2 of 4 (50.0%)"} {
		if !strings.Contains(svg, want) {
			t.Errorf("bar chart lacks %q: %s", want, svg)
		}
	}
	if strings.Contains(svg, "A&B") {
		t.Error("labels must be escaped")
	}
}
