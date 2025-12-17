package math

import (
	"math"
	"math/rand"
)

// GetPoissonDelay generates a random delay around a target mean (lambda) in milliseconds.
// Uses exponential distribution which models the time between events in a Poisson process.
// Time = -ln(U) * mean, where U is uniform random [0,1)
func GetPoissonDelay(targetMean float64) int {
	// Ensure we don't get -Inf from log(0)
	u := rand.Float64()
	for u == 0 {
		u = rand.Float64()
	}

	delay := int(-math.Log(u) * targetMean)

	// Clamp to bounds relative to the mean (0.1x to 10x)
	minDelay := int(0.1 * targetMean)
	maxDelay := int(10 * targetMean)

	if delay < minDelay {
		delay = minDelay
	}
	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}
