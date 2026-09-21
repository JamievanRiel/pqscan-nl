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
func TrendSVG(points []TrendPoint) template.HTML {
	if len(points) == 0 {
		return ""
	}
	const w, h, left, right, top, bottom = 640.0, 220.0, 44.0, 16.0, 12.0, 28.0
	plotW, plotH := w-left-right, h-top-bottom
	x := func(i int) float64 {
		if len(points) == 1 {
			return left + plotW/2
		}
		return left + plotW*float64(i)/float64(len(points)-1)
	}
	y := func(pct float64) float64 { return top + plotH*(1-pct/100) }

	var b strings.Builder
	fmt.Fprintf(&b, `<svg class="chart" viewBox="0 0 %g %g" role="img" aria-label="Share of top .nl sites using post-quantum key exchange by default, per scan">`, w, h)
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
		last := i == len(points)-1
		if len(points) > 6 && i != 0 && !last {
			continue
		}
		anchor := "middle"
		if len(points) > 1 && i == 0 {
			anchor = "start"
		} else if len(points) > 1 && last {
			anchor = "end"
		}
		fmt.Fprintf(&b, `<text class="tick" x="%.1f" y="%g" text-anchor="%s">%s</text>`, x(i), h-8, anchor, template.HTMLEscapeString(p.Date))
	}
	b.WriteString(`</svg>`)
	return template.HTML(b.String())
}

// BarRow is one bar: a label and its counts.
type BarRow struct {
	Label  string
	Counts results.Counts
}

// StackedBarsSVG draws one full-width bar per row, split into the pq-default,
// pq-supported and classic shares of the reachable domains.
func StackedBarsSVG(rows []BarRow) template.HTML {
	if len(rows) == 0 {
		return ""
	}
	const w, labelW, valueW, rowH, barH = 640.0, 170.0, 56.0, 34.0, 20.0
	barW := w - labelW - valueW
	var b strings.Builder
	fmt.Fprintf(&b, `<svg class="chart" viewBox="0 0 %g %g" role="img" aria-label="Post-quantum key exchange per group of sites">`, w, rowH*float64(len(rows)))
	for i, r := range rows {
		c := r.Counts
		label := template.HTMLEscapeString(r.Label)
		yTop := float64(i)*rowH + (rowH-barH)/2
		fmt.Fprintf(&b, `<text class="label" x="0" y="%.1f" dominant-baseline="middle">%s</text>`, yTop+barH/2, label)
		xPos := labelW
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
			segW := barW * c.Pct(seg.n) / 100
			fmt.Fprintf(&b, `<rect class="%s" x="%.1f" y="%.1f" width="%.1f" height="%g"><title>%s, %s: %d of %d (%.1f%%)</title></rect>`,
				seg.class, xPos, yTop, segW, barH, label, seg.name, seg.n, c.Reachable(), c.Pct(seg.n))
			xPos += segW
		}
		fmt.Fprintf(&b, `<text class="value" x="%g" y="%.1f" text-anchor="end" dominant-baseline="middle">%.0f%%</text>`, w, yTop+barH/2, c.Pct(c.PQDefault))
	}
	b.WriteString(`</svg>`)
	return template.HTML(b.String())
}
