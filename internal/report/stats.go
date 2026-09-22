package report

import "math"

// Wilson returns the 95% Wilson score interval for k successes in n trials,
// in percent. Unlike the normal approximation it stays within 0–100% and
// holds up for small n and shares near 0 or 100%. n == 0 gives 0, 0.
func Wilson(k, n int) (lo, hi float64) {
	if n == 0 {
		return 0, 0
	}
	const z = 1.959963984540054 // 97.5th percentile of the standard normal distribution
	p, nf := float64(k)/float64(n), float64(n)
	denom := 1 + z*z/nf
	centre := (p + z*z/(2*nf)) / denom
	half := z * math.Sqrt(p*(1-p)/nf+z*z/(4*nf*nf)) / denom
	return 100 * max(0, centre-half), 100 * min(1, centre+half)
}
