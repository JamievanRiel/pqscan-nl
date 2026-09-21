package report

import (
	"fmt"
	"html/template"
	"strings"

	"github.com/JamievanRiel/pqscan-nl/internal/results"
)

// TrendPoint is one scan on the trend chart.
type TrendPoint struct {
	Date string
	Pct  float64
}

// TrendSVG draws the share of pq-default domains per scan on a fixed 0–100% axis.
// Date labels between the first and the last have class "mid", so the
// stylesheet can hide them on narrow screens where the text is enlarged.
func TrendSVG(points []TrendPoint) template.HTML {
	if len(points) == 0 {
		return ""
	}
	const w, h, left, right, top, bottom = 640.0, 220.0, 64.0, 16.0, 12.0, 32.0
	plotW, plotH := w-left-right, h-top-bottom
	x := func(i int) float64 {
		if len(points) == 1 {
			return left + plotW/2
		}
		return left + plotW*float64(i)/float64(len(points)-1)
	}
	y := func(pct float64) float64 { return top + plotH*(1-pct/100) }

	var b strings.Builder
	latest := points[len(points)-1]
	fmt.Fprintf(&b, `<svg class="chart" viewBox="0 0 %g %g" role="img" aria-label="Share of top .nl sites using post-quantum key exchange by default, per scan; latest scan %s: %.1f%%">`,
		w, h, template.HTMLEscapeString(latest.Date), latest.Pct)
	for _, g := range []float64{0, 25, 50, 75, 100} {
		fmt.Fprintf(&b, `<line class="grid" x1="%g" x2="%g" y1="%.1f" y2="%.1f"/>`, left, w-right, y(g), y(g))
		fmt.Fprintf(&b, `<text class="tick" x="%g" y="%.1f" text-anchor="end" dominant-baseline="middle">%g%%</text>`, left-6, y(g), g)
	}
	if len(points) > 1 {
		coords := make([]string, len(points))
		for i, p := range points {
			coords[i] = fmt.Sprintf("%.1f,%.1f", x(i), y(p.Pct))
		}
		fmt.Fprintf(&b, `<polyline class="line" points="%s"/>`, strings.Join(coords, " "))
	}
	for i, p := range points {
		fmt.Fprintf(&b, `<circle class="dot" cx="%.1f" cy="%.1f" r="4"><title>%s: %.1f%%</title></circle>`,
			x(i), y(p.Pct), template.HTMLEscapeString(p.Date), p.Pct)
	}
	// Date labels: every scan while there are few, otherwise the first and last.
	for i, p := range points {
		first, last := i == 0, i == len(points)-1
		if len(points) > 6 && !first && !last {
			continue
		}
		class, anchor := "tick", "middle"
		switch {
		case len(points) == 1:
		case first:
			anchor = "start"
		case last:
			anchor = "end"
		default:
			class = "tick mid"
		}
		fmt.Fprintf(&b, `<text class="%s" x="%.1f" y="%g" text-anchor="%s">%s</text>`, class, x(i), h-8, anchor, template.HTMLEscapeString(p.Date))
	}
	b.WriteString(`</svg>`)
	return template.HTML(b.String())
}

// BarRow is one bar: a label and its counts.
type BarRow struct {
	Label  string
	Counts results.Counts
}

// SectorBars draws one row per BarRow: the label and the pq-default share as
// HTML text, and between them a bar split into the pq-default, pq-supported
// and classic shares of the reachable domains. Keeping the text out of the
// SVG lets it wrap and stay legible on narrow screens.
func SectorBars(rows []BarRow) template.HTML {
	if len(rows) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<div class="bars">`)
	for _, r := range rows {
		c := r.Counts
		label := template.HTMLEscapeString(r.Label)
		desc := fmt.Sprintf("%s: %.0f%% post-quantum by default, %.0f%% supported, %.0f%% classic only",
			label, c.Pct(c.PQDefault), c.Pct(c.PQSupported), c.Pct(c.Classic))
		value := fmt.Sprintf("%.0f%%", c.Pct(c.PQDefault))
		if c.Reachable() == 0 {
			desc = label + ": no reachable domains"
			value = "–"
		}
		fmt.Fprintf(&b, `<div class="bar-row"><span class="bar-label">%s</span>`, label)
		fmt.Fprintf(&b, `<svg class="bar" viewBox="0 0 100 10" preserveAspectRatio="none" role="img" aria-label="%s">`, desc)
		x := 0.0
		for _, seg := range []struct {
			class, name string
			n           int
		}{
			{"seg-pq", "post-quantum by default", c.PQDefault},
			{"seg-sup", "supported, not default", c.PQSupported},
			{"seg-classic", "classic only", c.Classic},
		} {
			if seg.n == 0 {
				continue
			}
			w := c.Pct(seg.n)
			fmt.Fprintf(&b, `<rect class="%s" x="%.1f" y="0" width="%.1f" height="10"><title>%s, %s: %d of %d (%.1f%%)</title></rect>`,
				seg.class, x, w, label, seg.name, seg.n, c.Reachable(), w)
			x += w
		}
		fmt.Fprintf(&b, `</svg><span class="bar-value">%s</span></div>`, value)
	}
	b.WriteString(`</div>`)
	return template.HTML(b.String())
}
