// Package targets builds the list of domains to scan from the Tranco list,
// the sector lists and the opt-out list.
package targets

import (
	"archive/zip"
	"bufio"
	"cmp"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/JamievanRiel/pqscan-nl/internal/results"
)

// NormalizeDomain lowercases a domain and strips surrounding whitespace,
// a trailing dot and a leading "www.".
func NormalizeDomain(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimSuffix(s, ".")
	return strings.TrimPrefix(s, "www.")
}

// ParseTranco reads Tranco "rank,domain" lines and keeps the domains ending
// in suffix. When a domain appears twice after normalization, the best rank wins.
func ParseTranco(r io.Reader, suffix string) ([]results.Target, error) {
	best := map[string]int{}
	var order []string
	sc := bufio.NewScanner(r)
	line := 0
	for sc.Scan() {
		line++
		text := strings.TrimSpace(sc.Text())
		if text == "" {
			continue
		}
		rankStr, domain, ok := strings.Cut(text, ",")
		if !ok {
			return nil, fmt.Errorf("tranco line %d: missing comma", line)
		}
		rank, err := strconv.Atoi(rankStr)
		if err != nil {
			return nil, fmt.Errorf("tranco line %d: bad rank %q", line, rankStr)
		}
		d := NormalizeDomain(domain)
		if !strings.HasSuffix(d, suffix) {
			continue
		}
		if prev, seen := best[d]; !seen {
			best[d] = rank
			order = append(order, d)
		} else if rank < prev {
			best[d] = rank
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	out := make([]results.Target, 0, len(order))
	for _, d := range order {
		out = append(out, results.Target{Domain: d, TrancoRank: best[d]})
	}
	return out, nil
}

// ReadTrancoZip parses the first .csv file inside a Tranco zip archive.
func ReadTrancoZip(path, suffix string) ([]results.Target, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	for _, f := range zr.File {
		if !strings.HasSuffix(f.Name, ".csv") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return ParseTranco(rc, suffix)
	}
	return nil, fmt.Errorf("%s: no .csv file in archive", path)
}

// ParseSectorCSV reads a sector list with the header "domain,name,source".
// Every row needs a valid domain and a non-empty source.
func ParseSectorCSV(r io.Reader, sector string) ([]results.Target, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = 3
	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", sector, err)
	}
	if !slices.Equal(header, []string{"domain", "name", "source"}) {
		return nil, fmt.Errorf("%s: header must be domain,name,source, got %v", sector, header)
	}
	var out []results.Target
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", sector, err)
		}
		d := NormalizeDomain(rec[0])
		if d == "" || !strings.Contains(d, ".") || strings.ContainsAny(d, " /:") {
			return nil, fmt.Errorf("%s: invalid domain %q", sector, rec[0])
		}
		if strings.TrimSpace(rec[2]) == "" {
			return nil, fmt.Errorf("%s: %s has no source", sector, d)
		}
		out = append(out, results.Target{Domain: d, Sectors: []string{sector}})
	}
	return out, nil
}

// LoadSectors reads every *.csv file in dir. The file name without ".csv"
// is the sector name.
func LoadSectors(dir string) ([]results.Target, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.csv"))
	if err != nil {
		return nil, err
	}
	slices.Sort(paths)
	var out []results.Target
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			return nil, err
		}
		ts, err := ParseSectorCSV(f, strings.TrimSuffix(filepath.Base(p), ".csv"))
		f.Close()
		if err != nil {
			return nil, err
		}
		out = append(out, ts...)
	}
	return out, nil
}

// ParseExclude reads one domain per line. Blank lines and "#" comments are ignored.
func ParseExclude(r io.Reader) (map[string]bool, error) {
	out := map[string]bool{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line, _, _ := strings.Cut(sc.Text(), "#")
		if d := NormalizeDomain(line); d != "" {
			out[d] = true
		}
	}
	return out, sc.Err()
}

// Merge combines Tranco and sector targets by domain and drops excluded
// domains. Ranked domains come first by rank, then sector-only domains
// alphabetically.
func Merge(tranco, sectors []results.Target, exclude map[string]bool) []results.Target {
	byDomain := map[string]*results.Target{}
	add := func(t results.Target) {
		if exclude[t.Domain] {
			return
		}
		cur, ok := byDomain[t.Domain]
		if !ok {
			byDomain[t.Domain] = &results.Target{Domain: t.Domain, TrancoRank: t.TrancoRank, Sectors: slices.Clone(t.Sectors)}
			return
		}
		if t.TrancoRank > 0 && (cur.TrancoRank == 0 || t.TrancoRank < cur.TrancoRank) {
			cur.TrancoRank = t.TrancoRank
		}
		for _, s := range t.Sectors {
			if !slices.Contains(cur.Sectors, s) {
				cur.Sectors = append(cur.Sectors, s)
			}
		}
	}
	for _, t := range tranco {
		add(t)
	}
	for _, t := range sectors {
		add(t)
	}
	out := make([]results.Target, 0, len(byDomain))
	for _, t := range byDomain {
		slices.Sort(t.Sectors)
		out = append(out, *t)
	}
	slices.SortFunc(out, func(a, b results.Target) int {
		switch {
		case a.TrancoRank > 0 && b.TrancoRank > 0 && a.TrancoRank != b.TrancoRank:
			return cmp.Compare(a.TrancoRank, b.TrancoRank)
		case a.TrancoRank > 0 && b.TrancoRank == 0:
			return -1
		case a.TrancoRank == 0 && b.TrancoRank > 0:
			return 1
		}
		return strings.Compare(a.Domain, b.Domain)
	})
	return out
}
