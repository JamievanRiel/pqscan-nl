package probe

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JamievanRiel/pqscan-nl/internal/results"
)

// raiseTo sets v to n if n is larger.
func raiseTo(v *atomic.Int64, n int64) {
	for {
		old := v.Load()
		if n <= old || v.CompareAndSwap(old, n) {
			return
		}
	}
}

func TestScanAll(t *testing.T) {
	pki := newTestPKI(t)
	pq := serveTLS(t, &tls.Config{Certificates: []tls.Certificate{pki.leaf(t, false, "pq.test")}})

	servers := map[string]string{}
	var targets []results.Target
	for i := range 20 {
		d := fmt.Sprintf("d%02d.nl", i)
		targets = append(targets, results.Target{Domain: d, TrancoRank: i + 1})
		if i%2 == 0 {
			servers[d] = pq // odd domains have no server: unreachable (dns)
		}
	}
	mapped := dialMap(servers)
	var inFlight, peak atomic.Int64
	p := &Prober{Timeout: 2 * time.Second, Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
		raiseTo(&peak, inFlight.Add(1))
		defer inFlight.Add(-1)
		time.Sleep(5 * time.Millisecond)
		return mapped(ctx, network, addr)
	}}

	var calls, last atomic.Int64
	recs := p.ScanAll(context.Background(), targets, 4, func(done int) {
		calls.Add(1)
		raiseTo(&last, int64(done))
	})

	if len(recs) != len(targets) {
		t.Fatalf("got %d records, want %d", len(recs), len(targets))
	}
	for i, r := range recs {
		want := results.StatusUnreachable
		if i%2 == 0 {
			want = results.StatusPQDefault
		}
		if r.Domain != targets[i].Domain || r.Status != want {
			t.Errorf("recs[%d] = %s %s, want %s %s", i, r.Domain, r.Status, targets[i].Domain, want)
		}
	}
	if peak.Load() > 4 {
		t.Errorf("peak concurrency %d, want at most 4", peak.Load())
	}
	if calls.Load() != 20 || last.Load() != 20 {
		t.Errorf("progress called %d times, last %d; want 20, 20", calls.Load(), last.Load())
	}
	if got := p.ScanAll(context.Background(), nil, 4, nil); len(got) != 0 {
		t.Errorf("ScanAll(nil) = %v, want empty", got)
	}
}
