package backoff

import (
	"math"
	"time"
)

// ExponentialClamp returns base*2^attempt clamped by max when max > 0.
func ExponentialClamp(base, max time.Duration, attempt int) time.Duration {
	if base <= 0 {
		return 0
	}
	if attempt < 0 {
		attempt = 0
	}

	delay := base
	for i := 0; i < attempt; i++ {
		if max > 0 && delay >= max {
			return max
		}
		if delay > time.Duration(math.MaxInt64/2) {
			delay = time.Duration(math.MaxInt64)
			break
		}
		delay *= 2
	}

	if max > 0 && delay > max {
		return max
	}
	return delay
}

// ScaleJitter applies ±factor scaling to delay using sample in [0, 1].
func ScaleJitter(delay time.Duration, factor float64, sample float64) time.Duration {
	if delay <= 0 || factor == 0 {
		return delay
	}
	if sample < 0 {
		sample = 0
	}
	if sample > 1 {
		sample = 1
	}

	jitterRange := float64(delay) * factor
	adjusted := float64(delay) - jitterRange + (2 * jitterRange * sample)
	if adjusted < 0 {
		return 0
	}
	return time.Duration(adjusted)
}
