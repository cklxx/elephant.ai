package llm

import (
	"sort"
	"sync"
	"time"

	alexerrors "alex/internal/shared/errors"
)

// healthState represents the health status of a provider.
type healthState string

const (
	HealthStateHealthy  healthState = "healthy"
	HealthStateDegraded healthState = "degraded" // circuit half-open or high error rate
	HealthStateDown     healthState = "down"     // circuit open
)

// latencyStats holds percentile and average latency measurements.
type latencyStats struct {
	P50 time.Duration `json:"p50"`
	P95 time.Duration `json:"p95"`
	Avg time.Duration `json:"avg"`
}

// ProviderHealth is a point-in-time snapshot of a provider's health.
type ProviderHealth struct {
	Provider     string       `json:"provider"`
	Model        string       `json:"model"`
	State        healthState  `json:"state"`
	LastError    string       `json:"last_error,omitempty"`
	FailureCount int          `json:"failure_count"`
	ErrorRate    float64      `json:"error_rate"`
	HealthScore  float64      `json:"health_score"`
	LastChecked  time.Time    `json:"last_checked"`
	Latency      latencyStats `json:"latency"`
}

const (
	latencyWindowSize   = 100
	errorRateWindowSize = 100
	errorRateHealthy    = 0.05 // < 5%
	errorRateDegraded   = 0.20 // 5-20%
)

// providerEntry stores per-provider tracking state.
type providerEntry struct {
	provider string
	model    string
	breaker  *alexerrors.CircuitBreaker

	// Latency ring buffer (last latencyWindowSize measurements).
	latencies [latencyWindowSize]time.Duration
	latCount  int // total recorded (used to determine fill level)
	latIdx    int // next write index

	// Error rate tracking (rolling window of success/failure outcomes).
	outcomes     [errorRateWindowSize]bool // true = error
	outcomeCount int
	outcomeIdx   int

	lastError    string
	failureCount int
}

// healthRegistry tracks per-provider health via circuit breakers and latency/error metrics.
type healthRegistry struct {
	mu      sync.RWMutex
	entries map[string]*providerEntry // key = "provider:model"
}

// newHealthRegistry creates a new healthRegistry.
func newHealthRegistry() *healthRegistry {
	return &healthRegistry{
		entries: make(map[string]*providerEntry),
	}
}

func providerKey(provider, model string) string {
	return provider + ":" + model
}

// Register registers a circuit breaker for the given provider/model pair.
// If breaker is nil the entry is still created and health will be derived from error rate.
func (hr *healthRegistry) register(provider, model string, breaker *alexerrors.CircuitBreaker) {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	key := providerKey(provider, model)
	if e, ok := hr.entries[key]; ok {
		// Update breaker reference if re-registered.
		e.breaker = breaker
		return
	}
	hr.entries[key] = &providerEntry{
		provider: provider,
		model:    model,
		breaker:  breaker,
	}
}

// recordLatency records a successful call latency for the given provider/model.
func (hr *healthRegistry) recordLatency(provider, model string, d time.Duration) {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	e := hr.getOrCreate(provider, model)
	e.latencies[e.latIdx] = d
	e.latIdx = (e.latIdx + 1) % latencyWindowSize
	if e.latCount < latencyWindowSize {
		e.latCount++
	}

	// Record success outcome.
	e.outcomes[e.outcomeIdx] = false
	e.outcomeIdx = (e.outcomeIdx + 1) % errorRateWindowSize
	if e.outcomeCount < errorRateWindowSize {
		e.outcomeCount++
	}
}

// recordError records a failed call for the given provider/model.
func (hr *healthRegistry) recordError(provider, model string, err error) {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	e := hr.getOrCreate(provider, model)
	e.failureCount++
	if err != nil {
		e.lastError = err.Error()
	}

	// Record failure outcome.
	e.outcomes[e.outcomeIdx] = true
	e.outcomeIdx = (e.outcomeIdx + 1) % errorRateWindowSize
	if e.outcomeCount < errorRateWindowSize {
		e.outcomeCount++
	}
}

// getHealth returns a health snapshot for the given provider/model.
func (hr *healthRegistry) getHealth(provider, model string) ProviderHealth {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	key := providerKey(provider, model)
	e, ok := hr.entries[key]
	if !ok {
		return ProviderHealth{
			Provider:    provider,
			Model:       model,
			State:       HealthStateHealthy,
			LastChecked: time.Now(),
		}
	}
	return hr.buildHealth(e)
}

// getAllHealth returns health snapshots for all registered providers, sorted by key.
func (hr *healthRegistry) getAllHealth() []ProviderHealth {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	result := make([]ProviderHealth, 0, len(hr.entries))
	for _, e := range hr.entries {
		result = append(result, hr.buildHealth(e))
	}
	sort.Slice(result, func(i, j int) bool {
		ki := providerKey(result[i].Provider, result[i].Model)
		kj := providerKey(result[j].Provider, result[j].Model)
		return ki < kj
	})
	return result
}

// buildHealth constructs a ProviderHealth from a providerEntry. Caller must hold at least RLock.
func (hr *healthRegistry) buildHealth(e *providerEntry) ProviderHealth {
	errRate := hr.computeErrorRate(e)
	lat := hr.computeLatency(e)
	return ProviderHealth{
		Provider:     e.provider,
		Model:        e.model,
		State:        hr.deriveState(e),
		LastError:    e.lastError,
		FailureCount: e.failureCount,
		ErrorRate:    errRate,
		HealthScore:  computeHealthScore(hr.deriveState(e), errRate, lat),
		LastChecked:  time.Now(),
		Latency:      lat,
	}
}

// computeErrorRate returns the error rate from the rolling window (0.0–1.0).
func (hr *healthRegistry) computeErrorRate(e *providerEntry) float64 {
	if e.outcomeCount == 0 {
		return 0
	}
	errors := 0
	for i := 0; i < e.outcomeCount; i++ {
		if e.outcomes[i] {
			errors++
		}
	}
	return float64(errors) / float64(e.outcomeCount)
}

// computeHealthScore returns a 0–100 score combining state, error rate, and latency.
func computeHealthScore(state healthState, errRate float64, lat latencyStats) float64 {
	var base float64
	switch state {
	case HealthStateHealthy:
		base = 100
	case HealthStateDegraded:
		base = 50
	case HealthStateDown:
		return 0
	}

	// Penalize by error rate (up to -40 points).
	base -= errRate * 40

	// Penalize high P95 latency (>5s starts deducting, up to -20 points).
	const latencyThreshold = 5 * time.Second
	if lat.P95 > latencyThreshold {
		penalty := float64(lat.P95-latencyThreshold) / float64(10*time.Second) * 20
		if penalty > 20 {
			penalty = 20
		}
		base -= penalty
	}

	if base < 0 {
		base = 0
	}
	return base
}

// deriveState determines the healthState from circuit breaker state or error rate.
func (hr *healthRegistry) deriveState(e *providerEntry) healthState {
	if e.breaker != nil {
		switch e.breaker.State() {
		case alexerrors.StateClosed:
			return HealthStateHealthy
		case alexerrors.StateHalfOpen:
			return HealthStateDegraded
		case alexerrors.StateOpen:
			return HealthStateDown
		}
	}

	// Fallback: derive from error rate in the rolling window.
	if e.outcomeCount == 0 {
		return HealthStateHealthy
	}
	errors := 0
	for i := 0; i < e.outcomeCount; i++ {
		if e.outcomes[i] {
			errors++
		}
	}
	rate := float64(errors) / float64(e.outcomeCount)
	switch {
	case rate > errorRateDegraded:
		return HealthStateDown
	case rate >= errorRateHealthy:
		return HealthStateDegraded
	default:
		return HealthStateHealthy
	}
}

// computeLatency calculates P50, P95, and Avg from the latency ring buffer.
func (hr *healthRegistry) computeLatency(e *providerEntry) latencyStats {
	if e.latCount == 0 {
		return latencyStats{}
	}

	// Copy filled portion and sort.
	buf := make([]time.Duration, e.latCount)
	copy(buf, e.latencies[:e.latCount])
	sort.Slice(buf, func(i, j int) bool { return buf[i] < buf[j] })

	var sum time.Duration
	for _, d := range buf {
		sum += d
	}

	return latencyStats{
		P50: percentile(buf, 0.50),
		P95: percentile(buf, 0.95),
		Avg: sum / time.Duration(len(buf)),
	}
}

// percentile returns the value at the given percentile (0-1) from a sorted slice.
func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(p * float64(len(sorted)-1))
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// getOrCreate returns the entry for the key, creating it if necessary. Caller must hold Lock.
func (hr *healthRegistry) getOrCreate(provider, model string) *providerEntry {
	key := providerKey(provider, model)
	if e, ok := hr.entries[key]; ok {
		return e
	}
	e := &providerEntry{
		provider: provider,
		model:    model,
	}
	hr.entries[key] = e
	return e
}
