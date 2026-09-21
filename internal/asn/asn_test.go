package asn

import (
	"compress/gzip"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JamievanRiel/pqscan-nl/internal/results"
)

// The KPN row is deliberately out of order: Parse must sort.
const fixture = "1.0.0.0\t1.0.0.255\t13335\tUS\tCLOUDFLARENET\n" +
	"1.0.1.0\t1.0.3.255\t0\tNone\tNot routed\n" +
	"5.0.0.0\t5.0.0.127\t1136\tNL\tKPN KPN B.V.\n" +
	"2.0.0.0\t2.0.0.255\t3320\tDE\tDTAG Internet service provider operations\n"

func TestLookup(t *testing.T) {
	db, err := Parse(strings.NewReader(fixture))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		ip   string
		asn  uint32
		org  string
		want bool
	}{
		{"1.0.0.0", 13335, "CLOUDFLARENET", true},
		{"1.0.0.255", 13335, "CLOUDFLARENET", true},
		{"1.0.2.1", 0, "", false}, // not routed rows are skipped
		{"2.0.0.10", 3320, "DTAG Internet service provider operations", true},
		{"5.0.0.128", 0, "", false}, // gap after a range
		{"0.0.0.1", 0, "", false},   // before the first range
		{"::ffff:5.0.0.1", 1136, "KPN KPN B.V.", true},
		{"2001:db8::1", 0, "", false},
	}
	for _, tt := range tests {
		asn, org, ok := db.Lookup(netip.MustParseAddr(tt.ip))
		if asn != tt.asn || org != tt.org || ok != tt.want {
			t.Errorf("Lookup(%s) = %d, %q, %v; want %d, %q, %v", tt.ip, asn, org, ok, tt.asn, tt.org, tt.want)
		}
	}
}

func TestParseRejectsBadRow(t *testing.T) {
	_, err := Parse(strings.NewReader("1.0.0.0\t1.0.0.255\tx\tUS\tA\n"))
	if err == nil || !strings.Contains(err.Error(), "line 1") {
		t.Fatalf("err = %v, want an error mentioning line 1", err)
	}
}

func TestLoadGzip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ip2asn-v4.tsv.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	gz.Write([]byte(fixture))
	gz.Close()
	f.Close()

	db, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if asn, _, ok := db.Lookup(netip.MustParseAddr("5.0.0.1")); !ok || asn != 1136 {
		t.Fatalf("Lookup after Load = %d, %v; want 1136, true", asn, ok)
	}
}

func TestAnnotate(t *testing.T) {
	db, err := Parse(strings.NewReader(fixture))
	if err != nil {
		t.Fatal(err)
	}
	recs := []results.Record{{IP: "5.0.0.1"}, {IP: ""}, {IP: "9.9.9.9"}}
	db.Annotate(recs)
	if recs[0].ASN != 1136 || recs[0].ASOrg != "KPN KPN B.V." {
		t.Errorf("recs[0] = %+v", recs[0])
	}
	if recs[1].ASN != 0 || recs[2].ASN != 0 {
		t.Errorf("unexpected annotation: %+v %+v", recs[1], recs[2])
	}
}
