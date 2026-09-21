// Package asn maps IPv4 addresses to autonomous systems (hosting networks)
// with the ip2asn-v4 table from iptoasn.com (public domain, PDDL v1.0).
package asn

import (
	"bufio"
	"cmp"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/JamievanRiel/pqscan-nl/internal/results"
)

type entry struct {
	start, end uint32
	asn        uint32
	org        string
}

// DB is an in-memory table of IPv4 ranges, sorted by start address.
type DB struct{ entries []entry }

// Parse reads the tab-separated ip2asn-v4 format: range start, range end,
// AS number, country code, AS description. Ranges with AS number 0 (not
// routed) are skipped.
func Parse(r io.Reader) (*DB, error) {
	var db DB
	sc := bufio.NewScanner(r)
	line := 0
	for sc.Scan() {
		line++
		text := sc.Text()
		if strings.TrimSpace(text) == "" {
			continue
		}
		f := strings.Split(text, "\t")
		if len(f) < 5 {
			return nil, fmt.Errorf("ip2asn line %d: want 5 fields, got %d", line, len(f))
		}
		start, err1 := parseIPv4(f[0])
		end, err2 := parseIPv4(f[1])
		n, err3 := strconv.ParseUint(f[2], 10, 32)
		if err := errors.Join(err1, err2, err3); err != nil {
			return nil, fmt.Errorf("ip2asn line %d: %w", line, err)
		}
		if n == 0 {
			continue
		}
		db.entries = append(db.entries, entry{start: start, end: end, asn: uint32(n), org: f[4]})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	slices.SortFunc(db.entries, func(a, b entry) int { return cmp.Compare(a.start, b.start) })
	return &db, nil
}

func parseIPv4(s string) (uint32, error) {
	a, err := netip.ParseAddr(s)
	if err != nil || !a.Is4() {
		return 0, fmt.Errorf("not an IPv4 address: %q", s)
	}
	b := a.As4()
	return binary.BigEndian.Uint32(b[:]), nil
}

// Load reads an ip2asn file. A name ending in .gz is decompressed.
func Load(path string) (*DB, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var r io.Reader = f
	if strings.HasSuffix(path, ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		r = gz
	}
	return Parse(r)
}

// Lookup returns the AS number and description of the range holding ip.
func (db *DB) Lookup(ip netip.Addr) (uint32, string, bool) {
	ip = ip.Unmap()
	if !ip.Is4() {
		return 0, "", false
	}
	b := ip.As4()
	v := binary.BigEndian.Uint32(b[:])
	// Find the first range starting after v; the candidate is the one before it.
	i := sort.Search(len(db.entries), func(i int) bool { return db.entries[i].start > v })
	if i == 0 {
		return 0, "", false
	}
	e := db.entries[i-1]
	if v > e.end {
		return 0, "", false
	}
	return e.asn, e.org, true
}

// Annotate fills ASN and ASOrg on every record whose IP is in the table.
func (db *DB) Annotate(recs []results.Record) {
	for i := range recs {
		ip, err := netip.ParseAddr(recs[i].IP)
		if err != nil {
			continue
		}
		if n, org, ok := db.Lookup(ip); ok {
			recs[i].ASN, recs[i].ASOrg = n, org
		}
	}
}
