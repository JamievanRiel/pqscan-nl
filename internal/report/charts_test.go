package report

import (
	"strings"
	"testing"
)

func TestTrendSVG(t *testing.T) {
	if TrendSVG(nil) != "" {
		t.Error("no points must draw nothing")
	}
	one := string(TrendSVG([]TrendPoint{{Date: "2026-09-28", Pct: 40, Lo: 30, Hi: 50}}))
	if strings.Contains(one, "<polyline") || strings.Contains(one, `class="band"`) || strings.Count(one, `class="dot"`) != 1 || !strings.Contains(one, `class="whisker"`) {
		t.Errorf("one point should be a dot with a whisker, without line or band: %s", one)
	}
	two := string(TrendSVG([]TrendPoint{{Date: "2026-09-21", Pct: 20, Lo: 15, Hi: 25}, {Date: "2026-09-28", Pct: 40, Lo: 35, Hi: 45}}))
	for _, want := range []string{"<polyline", `<polygon class="band"`, "2026-09-21: 20.0% (95% CI 15.0–25.0%)", "2026-09-28: 40.0%", ">100%<", ">0%<",
		`x1="64"`, "latest scan 2026-09-28: 40.0%", `class="hit"`} {
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

func TestDotPlot(t *testing.T) {
	if DotPlot(nil) != "" {
		t.Error("no rows must draw nothing")
	}
	html := string(DotPlot([]DotRow{
		{Label: "A&B", N: 1752, Est: 45.1, Lo: 42.7, Hi: 47.4},
		{Label: "None"},
		{Label: "Tail", N: 20, Est: 75, Lo: 53.1, Hi: 88.8, Muted: true},
	}))
	for _, want := range []string{
		`<span class="sr">A&amp;B: 45.1% (95% CI 42.7–47.4%), n = 1,752</span>`,
		`title="A&amp;B: 45.1% (95% CI 42.7–47.4%), n = 1,752"`,
		`<span class="dp-label" aria-hidden="true">A&amp;B <span class="dp-n">n&nbsp;=&nbsp;1,752</span></span>`,
		`<span class="dp-ci" style="left:42.7%;width:4.7%"></span><span class="dp-dot" style="left:45.1%"></span>`,
		`<span class="dp-value" aria-hidden="true">45.1%</span>`,
		`<span class="sr">None: no reachable domains</span>`,
		`<span class="dp-value" aria-hidden="true">–</span>`,
		`<div class="dp-sep" role="presentation"></div><div class="dp-row muted" role="listitem"`,
		`<span style="left:50%">50%</span>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("dot plot lacks %q: %s", want, html)
		}
	}
	if strings.Contains(html, "A&B") {
		t.Error("labels must be escaped")
	}
	if strings.Count(html, "dp-sep") != 1 {
		t.Error("only the first muted row after a normal one gets a separator")
	}
}

func TestWaffle(t *testing.T) {
	if Waffle(nil, 60) != "" {
		t.Error("no cells must draw nothing")
	}
	svg := string(Waffle([]bool{true, false, true, false, false}, 3))
	for _, want := range []string{
		`viewBox="0 0 28 18"`,
		`aria-label="5 squares, one per reachable domain in order of Tranco rank; 2 are post-quantum by default"`,
		`<path class="w-pq" d="M0 0h8v8h-8zM20 0h8v8h-8z"><title>2 domains post-quantum by default</title></path>`,
		`<path class="w-other" d="M10 0h8v8h-8zM0 10h8v8h-8zM10 10h8v8h-8z"><title>3 domains not post-quantum by default</title></path>`,
	} {
		if !strings.Contains(svg, want) {
			t.Errorf("waffle lacks %q: %s", want, svg)
		}
	}
}
