package math

import (
	"math"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetPoissonDelay_BoundsEnforced(t *testing.T) {
	// Seed for reproducibility in tests
	rand.Seed(42)

	targetMean := 1000.0
	minBound := int(0.1 * targetMean) // 100
	maxBound := int(10 * targetMean)  // 10000

	// Run many iterations to check bounds
	for i := 0; i < 10000; i++ {
		delay := GetPoissonDelay(targetMean)
		assert.GreaterOrEqual(t, delay, minBound, "delay should be >= min bound (0.1x mean)")
		assert.LessOrEqual(t, delay, maxBound, "delay should be <= max bound (10x mean)")
	}
}

func TestGetPoissonDelay_SmallMean(t *testing.T) {
	rand.Seed(42)

	targetMean := 10.0
	minBound := int(0.1 * targetMean) // 1
	maxBound := int(10 * targetMean)  // 100

	for i := 0; i < 1000; i++ {
		delay := GetPoissonDelay(targetMean)
		assert.GreaterOrEqual(t, delay, minBound)
		assert.LessOrEqual(t, delay, maxBound)
	}
}

func TestGetPoissonDelay_LargeMean(t *testing.T) {
	rand.Seed(42)

	targetMean := 100000.0
	minBound := int(0.1 * targetMean) // 10000
	maxBound := int(10 * targetMean)  // 1000000

	for i := 0; i < 1000; i++ {
		delay := GetPoissonDelay(targetMean)
		assert.GreaterOrEqual(t, delay, minBound)
		assert.LessOrEqual(t, delay, maxBound)
	}
}

func TestGetPoissonDelay_DistributionReasonable(t *testing.T) {
	rand.Seed(42)

	targetMean := 1000.0
	iterations := 10000

	var sum float64
	for i := 0; i < iterations; i++ {
		delay := GetPoissonDelay(targetMean)
		sum += float64(delay)
	}

	actualMean := sum / float64(iterations)

	// The actual mean of exponential distribution is lambda (targetMean)
	// Allow 20% tolerance since we're clamping values
	tolerance := 0.20 * targetMean
	assert.InDelta(t, targetMean, actualMean, tolerance,
		"mean of delays should be approximately the target mean")
}

func TestGetPoissonDelay_Variance(t *testing.T) {
	rand.Seed(42)

	targetMean := 1000.0
	iterations := 10000

	delays := make([]float64, iterations)
	var sum float64
	for i := 0; i < iterations; i++ {
		delay := float64(GetPoissonDelay(targetMean))
		delays[i] = delay
		sum += delay
	}

	mean := sum / float64(iterations)

	// Calculate variance
	var varianceSum float64
	for _, d := range delays {
		varianceSum += (d - mean) * (d - mean)
	}
	variance := varianceSum / float64(iterations)
	stdDev := math.Sqrt(variance)

	// Standard deviation should be significant (not zero)
	// For exponential distribution, stddev = mean, but clamping reduces this
	assert.Greater(t, stdDev, 0.0, "should have non-zero variance")
	assert.Greater(t, stdDev, targetMean*0.1, "variance should be meaningful")
}

func TestGetPoissonDelay_VerySmallMean(t *testing.T) {
	rand.Seed(42)

	// Edge case: very small mean (like 1 ms)
	targetMean := 1.0
	minBound := int(0.1 * targetMean) // 0
	maxBound := int(10 * targetMean)  // 10

	for i := 0; i < 1000; i++ {
		delay := GetPoissonDelay(targetMean)
		assert.GreaterOrEqual(t, delay, minBound)
		assert.LessOrEqual(t, delay, maxBound)
	}
}

func TestGetPoissonDelay_ReturnsInteger(t *testing.T) {
	rand.Seed(42)

	delay := GetPoissonDelay(1000.0)
	// Type assertion - if it compiles, it's an int
	var _ int = delay
	assert.IsType(t, 0, delay)
}

func TestGetPoissonDelay_DifferentSeeds(t *testing.T) {
	// Test that different seeds produce different sequences
	rand.Seed(1)
	seq1 := make([]int, 10)
	for i := range seq1 {
		seq1[i] = GetPoissonDelay(1000.0)
	}

	rand.Seed(2)
	seq2 := make([]int, 10)
	for i := range seq2 {
		seq2[i] = GetPoissonDelay(1000.0)
	}

	// Sequences should differ
	different := false
	for i := range seq1 {
		if seq1[i] != seq2[i] {
			different = true
			break
		}
	}
	assert.True(t, different, "different seeds should produce different sequences")
}

func TestGetPoissonDelay_ZeroMean(t *testing.T) {
	rand.Seed(42)

	// Edge case: zero mean
	// min = 0, max = 0, so all results should be 0
	targetMean := 0.0

	for i := 0; i < 100; i++ {
		delay := GetPoissonDelay(targetMean)
		assert.Equal(t, 0, delay, "zero mean should produce zero delay")
	}
}

func BenchmarkGetPoissonDelay(b *testing.B) {
	rand.Seed(42)
	for i := 0; i < b.N; i++ {
		GetPoissonDelay(1000.0)
	}
}
