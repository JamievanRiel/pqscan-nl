// Command govlist writes the government sector list from the Organisaties
// overheid export (https://organisaties.overheid.nl/archive/exportOO.xml).
//
//	curl -fsSL -o exportOO.xml https://organisaties.overheid.nl/archive/exportOO.xml
//	go run ./tools/govlist -i exportOO.xml -o lists/sectors/government.csv
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

const source = "https://organisaties.overheid.nl/archive/exportOO.xml"

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

type entry struct{ domain, name string }

func main() {
	in := flag.String("i", "exportOO.xml", "Organisaties overheid export")
	out := flag.String("o", "lists/sectors/government.csv", "output CSV")
	flag.Parse()
	if err := run(*in, *out, time.Now().Format(time.DateOnly)); err != nil {
		fmt.Fprintln(os.Stderr, "govlist:", err)
		os.Exit(1)
	}
}

func run(in, out, today string) error {
	f, err := os.Open(in)
	if err != nil {
		return err
	}
	defer f.Close()
	entries, err := parse(f, today)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.Write([]string{"domain", "name", "source"})
	for _, e := range entries {
		w.Write([]string{e.domain, e.name, source})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "%d government domains\n", len(entries))
	return os.WriteFile(out, buf.Bytes(), 0o644)
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
		out = append(out, entry{domain: d, name: names[d]})
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
