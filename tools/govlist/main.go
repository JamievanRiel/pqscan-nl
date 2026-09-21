// Command govlist writes the government sector list from the Organisaties
// overheid export (https://organisaties.overheid.nl/archive/exportOO.xml),
// plus the hand-maintained entries in lists/government-extra.csv for
// national services that the register files as organisation units.
//
//	curl -fsSL -o exportOO.xml https://organisaties.overheid.nl/archive/exportOO.xml
//	go run ./tools/govlist -i exportOO.xml -extra lists/government-extra.csv -o lists/sectors/government.csv
package main

import (
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/JamievanRiel/pqscan-nl/internal/targets"
)

const exportURL = "https://organisaties.overheid.nl/archive/exportOO.xml"

// kinds are the organisation types counted as government.
var kinds = map[string]bool{
	"Ministerie":                 true,
	"Provincie":                  true,
	"Gemeente":                   true,
	"Waterschap":                 true,
	"Agentschap":                 true,
	"Zelfstandig bestuursorgaan": true,
	"Inspectie":                  true,
	"Hoog College van Staat":     true,
	"Rechtspraak":                true,
	"Politie":                    true,
}

type organisatie struct {
	Naam      string   `xml:"naam"`
	Types     []string `xml:"types>type"`
	EindDatum string   `xml:"eindDatum"`
	Adressen  []struct {
		URL   string `xml:"url"`
		Label string `xml:"label"`
	} `xml:"contact>internetadressen>internetadres"`
}

type entry struct{ domain, name, source string }

func main() {
	in := flag.String("i", "exportOO.xml", "Organisaties overheid export")
	extra := flag.String("extra", "lists/government-extra.csv", "hand-maintained entries (domain,name,source) merged into the output")
	out := flag.String("o", "lists/sectors/government.csv", "output CSV")
	flag.Parse()
	if err := run(*in, *extra, *out, time.Now().Format(time.DateOnly)); err != nil {
		fmt.Fprintln(os.Stderr, "govlist:", err)
		os.Exit(1)
	}
}

func run(in, extraPath, out, today string) error {
	f, err := os.Open(in)
	if err != nil {
		return err
	}
	defer f.Close()
	register, err := parse(f, today)
	if err != nil {
		return err
	}
	extra, err := readExtra(extraPath)
	if err != nil {
		return err
	}
	entries := merge(register, extra)
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.Write([]string{"domain", "name", "source"})
	for _, e := range entries {
		w.Write([]string{e.domain, e.name, e.source})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "%d government domains, %d from %s\n", len(entries), len(entries)-len(register), extraPath)
	return os.WriteFile(out, buf.Bytes(), 0o644)
}

// readExtra reads the hand-maintained entries. The file follows the same
// rules as a sector list: header domain,name,source, a valid domain and a
// source on every row.
func readExtra(path string) ([]entry, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ts, err := targets.ParseSectorCSV(bytes.NewReader(b), path)
	if err != nil {
		return nil, err
	}
	rows, err := csv.NewReader(bytes.NewReader(b)).ReadAll()
	if err != nil {
		return nil, err
	}
	out := make([]entry, len(ts))
	for i, t := range ts {
		row := rows[i+1] // rows[0] is the header
		out[i] = entry{domain: t.Domain, name: strings.TrimSpace(row[1]), source: strings.TrimSpace(row[2])}
	}
	return out, nil
}

// merge adds the extra entries for domains the register does not have and
// sorts the result by domain. On a shared domain the register entry wins.
func merge(register, extra []entry) []entry {
	out := slices.Clone(register)
	seen := map[string]bool{}
	for _, e := range register {
		seen[e.domain] = true
	}
	for _, e := range extra {
		if !seen[e.domain] {
			seen[e.domain] = true
			out = append(out, e)
		}
	}
	slices.SortFunc(out, func(a, b entry) int { return strings.Compare(a.domain, b.domain) })
	return out
}

// parse returns one entry per domain of an active government organisation,
// sorted by domain. When organisations share a domain, the first one wins.
// Sub-organisations nested inside another organisation are skipped.
func parse(r io.Reader, today string) ([]entry, error) {
	dec := xml.NewDecoder(r)
	names := map[string]string{}
	depth := 0
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			// overheidsorganisaties > organisaties > organisatie
			if t.Name.Local != "organisatie" || depth != 3 {
				continue
			}
			var o organisatie
			if err := dec.DecodeElement(&o, &t); err != nil {
				return nil, err
			}
			depth--
			if d := o.domain(today); d != "" {
				if _, seen := names[d]; !seen {
					names[d] = strings.TrimSpace(o.Naam)
				}
			}
		case xml.EndElement:
			depth--
		}
	}
	var out []entry
	for _, d := range slices.Sorted(maps.Keys(names)) {
		out = append(out, entry{domain: d, name: names[d], source: exportURL})
	}
	return out, nil
}

// domain returns the organisation's main website domain, or "" when it has
// ended, is not a government type or has no website. Addresses hosted on
// organisaties.overheid.nl (the register itself) are always ignored.
func (o organisatie) domain(today string) string {
	if o.EindDatum != "" && o.EindDatum <= today {
		return ""
	}
	if !slices.ContainsFunc(o.Types, func(t string) bool { return kinds[t] }) {
		return ""
	}
	raw := ""
	// Look for an address with label "algemeen", skipping register self-links
	for _, a := range o.Adressen {
		if strings.EqualFold(strings.TrimSpace(a.Label), "algemeen") {
			if !isRegisterLink(a.URL) {
				raw = a.URL
				break
			}
		}
	}
	// Fall back to first non-register address if no "algemeen" found
	if raw == "" {
		for _, a := range o.Adressen {
			if !isRegisterLink(a.URL) {
				raw = a.URL
				break
			}
		}
	}
	return hostOf(raw)
}

// isRegisterLink reports whether the URL is hosted on organisaties.overheid.nl.
func isRegisterLink(rawURL string) bool {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return false
	}
	if !strings.Contains(rawURL, "://") {
		rawURL = "https://" + rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := u.Hostname()
	return strings.EqualFold(host, "organisaties.overheid.nl")
}

func hostOf(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	d := targets.NormalizeDomain(u.Hostname())
	if !strings.Contains(d, ".") {
		return ""
	}
	return d
}
