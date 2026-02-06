package llm

import (
	"sort"
	"sync"
	"time"

	alexerrors "alex/internal/errors"
)

// HealthState represents the health status of a provider.
type HealthState string

const (
	HealthStateHealthy  HealthState = "healthy"
	HealthStateDegraded HealthState = "degraded" // circuit half-open or high error rate
	HealthStateDown     HealthState = "down"     // circuit open
)

// LatencyStats holds percentile and average latency measurements.
type LatencyStats struct {
	P50 time.Duration `json:"p50"`
	P95 time.Duration `json:"p95"`
	Avg time.Duration `json:"avg"`
}

// ProviderHealth is a point-in-time snapshot of a provider's health.
type ProviderHealth struct {
	Provider     string      `json:"provider"`
	Model        string      `json:"model"`
	State        HealthState `json:"state"`
	LastError    string      `json:"last_error,omitempty"`
	FailureCount int         `json:"failure_count"`
	LastChecked  time.Time   `json:"last_checked"`
	Latency      LatencyStats `json:"latency"`
}

const (
	latencyWindowSize      = 100
	errorRateWindowSize    = 100
	errorRateHealthy       = 0.05 // < 5%
	errorRateDegraded      = 0.20 // 5-20%
)

// providerEntry stores per-provider tracking state.
type providerEntry struct {
	provider string
	model    string
	breaker  *alexerrors.CircuitBreaker

	// Latency ring buffer (last latencyWindowSize measurements).
	latencies [latencyWindowSize]time.Duration
	latCount  int  // total recorded (used to determine fill level)
	latIdx    int  // next write index

	// Error rate tracking (rolling window of success/failure outcomes).
	outcomes     [errorRateWindowSize]bool // true = error
	outcomeCount int
	outcomeIdx   int

	lastError   string
	failureCount int
}

// HealthRegistry tracks per-provider health via circuit breakers and latency/error metrics.
type HealthRegistry struct {
	mu      sync.RWMutex
	entries map[string]*providerEntry // key = "provider:model"
}

// NewHealthRegistry creates a new HealthRegistry.
func NewHealthRegistry() *HealthRegistry {
	return &HealthRegistry{
		entries: make(map[string]*providerEntry),
	}
}

func providerKey(provider, model string) string {
	return provider + ":" + model
}

// Register registers a circuit breaker for the given provider/model pair.
// If breaker is nil the entry is still created and health will be derived from error rate.
func (hr *HealthRegistry) Register(provider, model string, breaker *alexerrors.CircuitBreaker) {
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

// RecordLatency records a successful call latency for the given provider/model.
func (hr *HealthRegistry) RecordLatency(provider, model string, d time.Duration) {
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

// RecordError records a failed call for the given provider/model.
func (hr *HealthRegistry) RecordError(provider, model string, err error) {
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

// GetHealth returns a health snapshot for the given provider/model.
func (hr *HealthRegistry) GetHealth(provider, model string) ProviderHealth {
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

// GetAllHealth returns health snapshots for all registered providers, sorted by key.
func (hr *HealthRegistry) GetAllHealth() []ProviderHealth {
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
func (hr *HealthRegistry) buildHealth(e *providerEntry) ProviderHealth {
	return ProviderHealth{
		Provider:     e.provider,
		Model:        e.model,
		State:        hr.deriveState(e),
		LastError:    e.lastError,
		FailureCount: e.failureCount,
		LastChecked:  time.Now(),
		Latency:      hr.computeLatency(e),
	}
}

// deriveState determines the HealthState from circuit breaker state or error rate.
func (hr *HealthRegistry) deriveState(e *providerEntry) HealthState {
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
func (hr *HealthRegistry) computeLatency(e *providerEntry) LatencyStats {
	if e.latCount == 0 {
		return LatencyStats{}
	}

	// Copy filled portion and sort.
	buf := make([]time.Duration, e.latCount)
	copy(buf, e.latencies[:e.latCount])
	sort.Slice(buf, func(i, j int) bool { return buf[i] < buf[j] })

	var sum time.Duration
	for _, d := range buf {
		sum += d
	}

	return LatencyStats{
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
func (hr *HealthRegistry) getOrCreate(provider, model string) *providerEntry {
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
