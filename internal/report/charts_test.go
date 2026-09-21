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
	for _, want := range []string{"<polyline", "2026-09-21: 20.0%", "2026-09-28: 40.0%", ">100%<", ">0%<",
		`x1="64"`, "latest scan 2026-09-28: 40.0%"} {
		if !strings.Contains(two, want) {
			t.Errorf("trend chart lacks %q: %s", want, two)
		}
	}
	if strings.Contains(two, "tick mid") {
		t.Errorf("first and last date labels must not be hidden on phones: %s", two)
	}
	three := string(TrendSVG([]TrendPoint{{Date: "2026-09-14", Pct: 10}, {Date: "2026-09-21", Pct: 20}, {Date: "2026-09-28", Pct: 40}}))
	if !strings.Contains(three, `class="tick mid" x="`) || strings.Count(three, "tick mid") != 1 {
		t.Errorf("only the middle date label should have class mid: %s", three)
	}
}

func TestSectorBars(t *testing.T) {
	if SectorBars(nil) != "" {
		t.Error("no rows must draw nothing")
	}
	html := string(SectorBars([]BarRow{
		{Label: "A&B", Counts: results.Counts{Total: 5, PQDefault: 2, PQSupported: 1, Classic: 1, Unreachable: 1}},
		{Label: "None", Counts: results.Counts{Total: 1, Unreachable: 1}},
	}))
	// Bars are 100 units wide, so x and width are percentages of the 4 reachable domains.
	for _, want := range []string{
		`<span class="bar-label">A&amp;B</span>`,
		`viewBox="0 0 100 10" preserveAspectRatio="none"`,
		`aria-label="A&amp;B: 50% post-quantum by default, 25% supported, 25% classic only"`,
		`<rect class="seg-pq" x="0.0" y="0" width="50.0" height="10">`,
		`<rect class="seg-sup" x="50.0" y="0" width="25.0" height="10">`,
		`<rect class="seg-classic" x="75.0" y="0" width="25.0" height="10">`,
		"A&amp;B, post-quantum by default: 2 of 4 (50.0%)",
		`<span class="bar-value">50%</span>`,
		`aria-label="None: no reachable domains"`,
		`<span class="bar-value">–</span>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("sector bars lack %q: %s", want, html)
		}
	}
	if strings.Contains(html, "A&B") {
		t.Error("labels must be escaped")
	}
	if strings.Count(html, `class="bar-row"`) != 2 {
		t.Errorf("want one bar-row per row: %s", html)
	}
}
