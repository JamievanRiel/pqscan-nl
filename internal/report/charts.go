package report

import (
	"fmt"
	"html/template"
	"strings"
)

// TrendPoint is one scan on the trend chart; Lo and Hi bound its 95% interval.
type TrendPoint struct {
	Date        string
	Pct, Lo, Hi float64
}

// TrendSVG draws the share of pq-default domains per scan on a fixed 0–100%
// axis, with the 95% interval as a band (a whisker for a single scan). Date
// labels between the first and the last have class "mid", so the stylesheet
// can hide them on narrow screens where the text is enlarged.
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
		band := make([]string, 0, 2*len(points))
		for i, p := range points {
			band = append(band, fmt.Sprintf("%.1f,%.1f", x(i), y(p.Hi)))
		}
		for i := len(points) - 1; i >= 0; i-- {
			band = append(band, fmt.Sprintf("%.1f,%.1f", x(i), y(points[i].Lo)))
		}
		fmt.Fprintf(&b, `<polygon class="band" points="%s"/>`, strings.Join(band, " "))
		coords := make([]string, len(points))
		for i, p := range points {
			coords[i] = fmt.Sprintf("%.1f,%.1f", x(i), y(p.Pct))
		}
		fmt.Fprintf(&b, `<polyline class="line" points="%s"/>`, strings.Join(coords, " "))
	} else {
		fmt.Fprintf(&b, `<line class="whisker" x1="%.1f" x2="%.1f" y1="%.1f" y2="%.1f"/>`, x(0), x(0), y(latest.Lo), y(latest.Hi))
	}
	for i, p := range points {
		fmt.Fprintf(&b, `<g class="pt"><circle class="hit" cx="%.1f" cy="%.1f" r="12"/><circle class="dot" cx="%.1f" cy="%.1f" r="4"/><title>%s: %.1f%% (95%% CI %.1f–%.1f%%)</title></g>`,
			x(i), y(p.Pct), x(i), y(p.Pct), template.HTMLEscapeString(p.Date), p.Pct, p.Lo, p.Hi)
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

// DotRow is one estimate on a dot plot. Est, Lo and Hi are percentages.
type DotRow struct {
	Label       string
	N           int // reachable domains behind the estimate
	Est, Lo, Hi float64
	Muted       bool // a comparison row outside the main population
}

// DotPlot draws estimates with their 95% intervals on a shared 0–100% axis.
// Labels, n and values are HTML so they wrap on narrow screens; the whisker
// and the dot are positioned in percent so the dot stays round at any width.
// A separator precedes the first muted row that follows a normal one.
func DotPlot(rows []DotRow) template.HTML {
	if len(rows) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<div class="dotplot" role="list">`)
	for i, r := range rows {
		if r.Muted && i > 0 && !rows[i-1].Muted {
			b.WriteString(`<div class="dp-sep" role="presentation"></div>`)
		}
		label := template.HTMLEscapeString(r.Label)
		desc := fmt.Sprintf("%s: %.1f%% (95%% CI %.1f–%.1f%%), n = %s", label, r.Est, r.Lo, r.Hi, thousands(r.N))
		plot := fmt.Sprintf(`<span class="dp-ci" style="left:%.1f%%;width:%.1f%%"></span><span class="dp-dot" style="left:%.1f%%"></span>`, r.Lo, r.Hi-r.Lo, r.Est)
		value := fmt.Sprintf("%.1f%%", r.Est)
		if r.N == 0 {
			desc, plot, value = label+": no reachable domains", "", "–"
		}
		class := "dp-row"
		if r.Muted {
			class += " muted"
		}
		fmt.Fprintf(&b, `<div class="%s" role="listitem" title="%s"><span class="sr">%s</span>`, class, desc, desc)
		fmt.Fprintf(&b, `<span class="dp-label" aria-hidden="true">%s <span class="dp-n">n&nbsp;=&nbsp;%s</span></span>`, label, thousands(r.N))
		fmt.Fprintf(&b, `<span class="dp-plot" aria-hidden="true">%s</span><span class="dp-value" aria-hidden="true">%s</span></div>`, plot, value)
	}
	b.WriteString(`<div class="dp-axis" aria-hidden="true"><span></span><span class="dp-ticks">`)
	for _, t := range []int{0, 25, 50, 75, 100} {
		fmt.Fprintf(&b, `<span style="left:%d%%">%d%%</span>`, t, t)
	}
	b.WriteString(`</span><span></span></div></div>`)
	return template.HTML(b.String())
}

// Waffle draws one square per domain, cols to a row, in the order given: the
// most popular domain at the top left. Filled squares are post-quantum by
// default. Each group is one path, which keeps thousands of squares small.
func Waffle(pq []bool, cols int) template.HTML {
	if len(pq) == 0 || cols <= 0 {
		return ""
	}
	const cell, pitch = 8, 10 // the 2-unit gap separates squares in the surface colour
	var on, off strings.Builder
	n := 0
	for i, v := range pq {
		p := &off
		if v {
			p, n = &on, n+1
		}
		fmt.Fprintf(p, "M%d %dh%dv%dh-%dz", i%cols*pitch, i/cols*pitch, cell, cell, cell)
	}
	rows := (len(pq) + cols - 1) / cols
	var b strings.Builder
	fmt.Fprintf(&b, `<svg class="waffle" viewBox="0 0 %d %d" role="img" aria-label="%s squares, one per reachable domain in order of Tranco rank; %s are post-quantum by default">`,
		cols*pitch-(pitch-cell), rows*pitch-(pitch-cell), thousands(len(pq)), thousands(n))
	if n > 0 {
		fmt.Fprintf(&b, `<path class="w-pq" d="%s"><title>%s domains post-quantum by default</title></path>`, on.String(), thousands(n))
	}
	if n < len(pq) {
		fmt.Fprintf(&b, `<path class="w-other" d="%s"><title>%s domains not post-quantum by default</title></path>`, off.String(), thousands(len(pq)-n))
	}
	b.WriteString(`</svg>`)
	return template.HTML(b.String())
}
