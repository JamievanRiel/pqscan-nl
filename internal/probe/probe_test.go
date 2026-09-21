package probe

import (
	"context"
	"crypto/tls"
	"slices"
	"testing"
	"time"

	"github.com/JamievanRiel/pqscan-nl/internal/results"
)

func TestIsPQ(t *testing.T) {
	if !IsPQ(tls.X25519MLKEM768) || !IsPQ(tls.SecP256r1MLKEM768) || !IsPQ(tls.SecP384r1MLKEM1024) {
		t.Error("hybrid groups must count as PQ")
	}
	if IsPQ(tls.X25519) || IsPQ(tls.CurveP256) || IsPQ(0) {
		t.Error("classic groups must not count as PQ")
	}
}

func TestProbe(t *testing.T) {
	pki := newTestPKI(t)
	untrustedPKI := newTestPKI(t) // its CA is not in the prober's roots
	cert := func(names ...string) []tls.Certificate { return []tls.Certificate{pki.leaf(t, false, names...)} }

	preferClassicBase := &tls.Config{Certificates: cert("prefers-classic.nl")}
	servers := map[string]string{
		"pq.nl":      serveTLS(t, &tls.Config{Certificates: cert("pq.nl")}),
		"classic.nl": serveTLS(t, &tls.Config{Certificates: cert("classic.nl"), CurvePreferences: []tls.CurveID{tls.X25519, tls.CurveP256}}),
		"prefers-classic.nl": serveTLS(t, &tls.Config{GetConfigForClient: func(h *tls.ClientHelloInfo) (*tls.Config, error) {
			// Go servers ignore CurvePreferences order, so simulate a server
			// that supports PQ but picks classic whenever it is offered.
			c := preferClassicBase.Clone()
			if slices.Contains(h.SupportedCurves, tls.X25519) {
				c.CurvePreferences = []tls.CurveID{tls.X25519}
			} else {
				c.CurvePreferences = []tls.CurveID{tls.X25519MLKEM768}
			}
			return c, nil
		}}),
		"old.nl":         serveTLS(t, &tls.Config{Certificates: cert("old.nl"), MaxVersion: tls.VersionTLS12}),
		"expired.nl":     serveTLS(t, &tls.Config{Certificates: []tls.Certificate{pki.leaf(t, true, "expired.nl")}}),
		"untrusted.nl":   serveTLS(t, &tls.Config{Certificates: []tls.Certificate{untrustedPKI.leaf(t, false, "untrusted.nl")}}),
		"www.wwwonly.nl": serveTLS(t, &tls.Config{Certificates: cert("www.wwwonly.nl")}),
		"refused.nl":     closedAddr(t),
		"www.refused.nl": closedAddr(t),
		"nottls.nl":      serveGarbage(t),
		// Browsers offer X25519MLKEM768 as their only hybrid, so a server
		// with only a NIST-curve hybrid is not post-quantum by default.
		"nist-pq.nl": serveTLS(t, &tls.Config{Certificates: cert("nist-pq.nl"), CurvePreferences: []tls.CurveID{tls.SecP256r1MLKEM768, tls.X25519}}),
	}
	now := time.Now().UTC()
	p := &Prober{Dial: dialMap(servers), Timeout: 2 * time.Second, Roots: pki.roots, Now: func() time.Time { return now }}

	tests := []struct {
		domain, status, host, group, pqGroup, version, errClass string
		certValid                                               bool
	}{
		{"pq.nl", "pq-default", "pq.nl", "X25519MLKEM768", "X25519MLKEM768", "1.3", "", true},
		{"classic.nl", "classic", "classic.nl", "X25519", "", "1.3", "", true},
		{"prefers-classic.nl", "pq-supported", "prefers-classic.nl", "X25519", "X25519MLKEM768", "1.3", "", true},
		{"old.nl", "classic", "old.nl", "X25519", "", "1.2", "", true},
		{"nist-pq.nl", "pq-supported", "nist-pq.nl", "X25519", "SecP256r1MLKEM768", "1.3", "", true},
		{"expired.nl", "pq-default", "expired.nl", "X25519MLKEM768", "X25519MLKEM768", "1.3", "", false},
		{"untrusted.nl", "pq-default", "untrusted.nl", "X25519MLKEM768", "X25519MLKEM768", "1.3", "", false},
		{"wwwonly.nl", "pq-default", "www.wwwonly.nl", "X25519MLKEM768", "X25519MLKEM768", "1.3", "", true},
		{"refused.nl", "unreachable", "", "", "", "", "refused", false},
		{"missing.nl", "unreachable", "", "", "", "", "dns", false},
		{"nottls.nl", "unreachable", "", "", "", "", "tls", false},
	}
	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			r := p.Probe(context.Background(), results.Target{Domain: tt.domain, Sectors: []string{"banks"}, TrancoRank: 7})
			if string(r.Status) != tt.status || r.Host != tt.host || r.Group != tt.group || r.PQGroup != tt.pqGroup ||
				r.TLSVersion != tt.version || r.Error != tt.errClass || r.CertValid != tt.certValid {
				t.Fatalf("got status=%s host=%s group=%s pq=%s tls=%s err=%s valid=%v; want %+v",
					r.Status, r.Host, r.Group, r.PQGroup, r.TLSVersion, r.Error, r.CertValid, tt)
			}
			if r.Domain != tt.domain || r.TrancoRank != 7 || len(r.Sectors) != 1 || !r.ScannedAt.Equal(now) {
				t.Errorf("target fields not carried over: %+v", r)
			}
			if tt.status != "unreachable" && r.IP != "127.0.0.1" {
				t.Errorf("IP = %q, want 127.0.0.1", r.IP)
			}
		})
	}
	if r := p.Probe(context.Background(), results.Target{Domain: "pq.nl"}); r.CertIssuer != "Test CA" {
		t.Errorf("CertIssuer = %q, want Test CA", r.CertIssuer)
	}
}

func TestProbeTimeout(t *testing.T) {
	p := &Prober{Dial: dialMap(map[string]string{"slow.nl": serveSilent(t)}), Timeout: 150 * time.Millisecond}
	r := p.Probe(context.Background(), results.Target{Domain: "slow.nl"})
	if r.Status != results.StatusUnreachable || r.Error != "timeout" {
		t.Fatalf("got status=%s err=%s, want unreachable/timeout", r.Status, r.Error)
	}
}
