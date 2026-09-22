package report

import (
	"math"
	"testing"
)

func TestWilson(t *testing.T) {
	for _, tc := range []struct {
		k, n   int
		lo, hi float64
	}{
		{786, 1752, 42.5479, 47.2006},
		{0, 10, 0, 27.7533},
		{10, 10, 72.2467, 100},
		{2, 5, 11.7621, 76.9276},
		{0, 0, 0, 0},
	} {
		lo, hi := Wilson(tc.k, tc.n)
		if math.Abs(lo-tc.lo) > 1e-4 || math.Abs(hi-tc.hi) > 1e-4 {
			t.Errorf("Wilson(%d, %d) = %.4f, %.4f; want %.4f, %.4f", tc.k, tc.n, lo, hi, tc.lo, tc.hi)
		}
	}
}
