package probe

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/JamievanRiel/pqscan-nl/internal/results"
)

// ScanAll probes every target with at most concurrency probes in flight and
// returns the records in target order. progress, if not nil, is called after
// each probe with the number finished so far, possibly from several goroutines.
func (p *Prober) ScanAll(ctx context.Context, targets []results.Target, concurrency int, progress func(done int)) []results.Record {
	concurrency = max(concurrency, 1)
	recs := make([]results.Record, len(targets))
	jobs := make(chan int)
	var done atomic.Int64
	var wg sync.WaitGroup
	for range min(concurrency, len(targets)) {
		wg.Go(func() {
			for i := range jobs {
				recs[i] = p.Probe(ctx, targets[i])
				n := done.Add(1)
				if progress != nil {
					progress(int(n))
				}
			}
		})
	}
	for i := range targets {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return recs
}
