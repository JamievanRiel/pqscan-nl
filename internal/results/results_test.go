package results

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestCountsAddAndPct(t *testing.T) {
	var c Counts
	for _, s := range []Status{StatusPQDefault, StatusPQDefault, StatusPQSupported, StatusClassic, StatusUnreachable} {
		c.Add(s)
	}
	want := Counts{Total: 5, PQDefault: 2, PQSupported: 1, Classic: 1, Unreachable: 1}
	if c != want {
		t.Fatalf("counts = %+v, want %+v", c, want)
	}
	if got := c.Reachable(); got != 4 {
		t.Fatalf("Reachable = %d, want 4", got)
	}
	if got := c.Pct(c.PQDefault); got != 50 {
		t.Fatalf("Pct = %v, want 50", got)
	}
	if got := (Counts{Total: 2, Unreachable: 2}).Pct(0); got != 0 {
		t.Fatalf("Pct with nothing reachable = %v, want 0", got)
	}
}

func TestJSONLRoundTrip(t *testing.T) {
	at := time.Date(2026, 9, 28, 2, 14, 7, 0, time.UTC)
	in := []Record{
		{Domain: "example.nl", Host: "www.example.nl", Sectors: []string{"banks"}, TrancoRank: 1234, Status: StatusPQDefault, Group: "X25519MLKEM768", CertValid: true, ScannedAt: at},
		{Domain: "down.nl", Status: StatusUnreachable, Error: "dns", ScannedAt: at},
	}
	var buf bytes.Buffer
	if err := WriteJSONL(&buf, in); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), `"asn"`) {
		t.Errorf("empty fields must be omitted, got %s", buf.String())
	}
	out, err := ReadJSONL[Record](strings.NewReader(buf.String() + "\n\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[0].Domain != "example.nl" || out[0].Sectors[0] != "banks" ||
		!out[0].ScannedAt.Equal(at) || out[1].Error != "dns" {
		t.Fatalf("round trip mismatch: %+v", out)
	}
}

func TestReadJSONLReportsLine(t *testing.T) {
	_, err := ReadJSONL[Target](strings.NewReader("{\"domain\":\"a.nl\"}\nnot json\n"))
	if err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("err = %v, want an error mentioning line 2", err)
	}
}
