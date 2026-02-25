package backoff

import (
	"testing"
	"time"
)

func TestExponentialClamp(t *testing.T) {
	base := 100 * time.Millisecond
	max := 350 * time.Millisecond

	tests := []struct {
		name    string
		attempt int
		want    time.Duration
	}{
		{name: "negative attempt", attempt: -1, want: base},
		{name: "first attempt", attempt: 0, want: base},
		{name: "second attempt", attempt: 1, want: 200 * time.Millisecond},
		{name: "clamped at max", attempt: 4, want: max},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExponentialClamp(base, max, tt.attempt); got != tt.want {
				t.Fatalf("ExponentialClamp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExponentialClampWithoutMax(t *testing.T) {
	base := 125 * time.Millisecond
	got := ExponentialClamp(base, 0, 3)
	want := 1 * time.Second
	if got != want {
		t.Fatalf("ExponentialClamp() = %v, want %v", got, want)
	}
}

func TestScaleJitter(t *testing.T) {
	delay := 1 * time.Second

	tests := []struct {
		name   string
		factor float64
		sample float64
		want   time.Duration
	}{
		{name: "no jitter factor", factor: 0, sample: 0.3, want: delay},
		{name: "min sample", factor: 0.25, sample: 0, want: 750 * time.Millisecond},
		{name: "mid sample", factor: 0.25, sample: 0.5, want: delay},
		{name: "max sample", factor: 0.25, sample: 1, want: 1250 * time.Millisecond},
		{name: "clamp sample below range", factor: 0.25, sample: -2, want: 750 * time.Millisecond},
		{name: "clamp sample above range", factor: 0.25, sample: 2, want: 1250 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ScaleJitter(delay, tt.factor, tt.sample); got != tt.want {
				t.Fatalf("ScaleJitter() = %v, want %v", got, tt.want)
			}
		})
	}
}
