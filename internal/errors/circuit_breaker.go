package errors

import (
	"context"
	"fmt"
	"sync"
	"time"

	"alex/internal/logging"
)

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	// StateClosed - normal operation, requests allowed
	StateClosed CircuitState = iota
	// StateOpen - failing, requests blocked
	StateOpen
	// StateHalfOpen - testing if service recovered
	StateHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig configures circuit breaker behavior
type CircuitBreakerConfig struct {
	FailureThreshold int                                      // Number of consecutive failures to open circuit (default: 5)
	SuccessThreshold int                                      // Number of consecutive successes in half-open to close circuit (default: 2)
	Timeout          time.Duration                            // Time to wait before attempting half-open (default: 30s)
	OnStateChange    func(from, to CircuitState, name string) // Optional callback
}

// DefaultCircuitBreakerConfig returns sensible defaults
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          30 * time.Second,
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	name   string
	config CircuitBreakerConfig
	logger logging.Logger

	mu              sync.RWMutex
	state           CircuitState
	failureCount    int
	successCount    int
	lastFailureTime time.Time
	lastStateChange time.Time
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(name string, config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		name:            name,
		config:          config,
		logger:          logging.NewComponentLogger("circuit-breaker"),
		state:           StateClosed,
		lastStateChange: time.Now(),
	}
}

// Execute runs a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(ctx context.Context) error) error {
	// Check if request is allowed
	if err := cb.beforeRequest(); err != nil {
		return err
	}

	// Execute function
	err := fn(ctx)

	// Record result
	cb.afterRequest(err)

	return err
}

// ExecuteFunc is a helper to execute a function that returns a value
// This avoids the need for method generics
func ExecuteFunc[T any](cb *CircuitBreaker, ctx context.Context, fn func(ctx context.Context) (T, error)) (T, error) {
	var zeroValue T

	// Check if request is allowed
	if err := cb.beforeRequest(); err != nil {
		return zeroValue, err
	}

	// Execute function
	result, err := fn(ctx)

	// Record result
	cb.afterRequest(err)

	return result, err
}

// Allow checks whether a request can proceed under the circuit breaker.
// Callers that need to inspect responses should use Allow/Mark instead of Execute.
func (cb *CircuitBreaker) Allow() error {
	return cb.beforeRequest()
}

// Mark records a request outcome for the circuit breaker.
// Pass nil to mark success, or a non-nil error to record failure.
func (cb *CircuitBreaker) Mark(err error) {
	cb.afterRequest(err)
}

// beforeRequest checks if request should be allowed
func (cb *CircuitBreaker) beforeRequest() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		// Normal operation
		return nil

	case StateOpen:
		// Check if timeout has elapsed
		if time.Since(cb.lastFailureTime) >= cb.config.Timeout {
			// Transition to half-open
			cb.setState(StateHalfOpen)
			cb.successCount = 0
			cb.logger.Info("[%s] Circuit breaker transitioning to half-open (testing recovery)", cb.name)
			return nil
		}
		// Circuit is open, reject request
		return NewDegradedError(
			fmt.Errorf("circuit breaker open for %s", cb.name),
			fmt.Sprintf("Service '%s' is temporarily unavailable due to repeated failures. Circuit breaker will retry in %v.",
				cb.name, cb.config.Timeout-time.Since(cb.lastFailureTime)),
			"",
		)

	case StateHalfOpen:
		// Allow limited requests in half-open state
		return nil

	default:
		return fmt.Errorf("unknown circuit breaker state: %v", cb.state)
	}
}

// afterRequest records the result of a request
func (cb *CircuitBreaker) afterRequest(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err == nil {
		// Success
		cb.onSuccess()
	} else {
		// Failure
		cb.onFailure()
	}
}

// onSuccess handles successful requests
func (cb *CircuitBreaker) onSuccess() {
	switch cb.state {
	case StateClosed:
		// Reset failure count on success
		if cb.failureCount > 0 {
			cb.logger.Debug("[%s] Success, resetting failure count", cb.name)
			cb.failureCount = 0
		}

	case StateHalfOpen:
		// Increment success count
		cb.successCount++
		cb.logger.Debug("[%s] Success in half-open state (%d/%d)",
			cb.name, cb.successCount, cb.config.SuccessThreshold)

		// Check if we should close the circuit
		if cb.successCount >= cb.config.SuccessThreshold {
			cb.setState(StateClosed)
			cb.failureCount = 0
			cb.successCount = 0
			cb.logger.Info("[%s] Circuit breaker closed (service recovered)", cb.name)
		}

	case StateOpen:
		// Should not happen, but reset if it does
		cb.logger.Warn("[%s] Unexpected success in open state", cb.name)
	}
}

// onFailure handles failed requests
func (cb *CircuitBreaker) onFailure() {
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		// Increment failure count
		cb.failureCount++
		cb.logger.Debug("[%s] Failure in closed state (%d/%d)",
			cb.name, cb.failureCount, cb.config.FailureThreshold)

		// Check if we should open the circuit
		if cb.failureCount >= cb.config.FailureThreshold {
			cb.setState(StateOpen)
			cb.logger.Warn("[%s] Circuit breaker opened (too many failures)", cb.name)
		}

	case StateHalfOpen:
		// Any failure in half-open goes back to open
		cb.setState(StateOpen)
		cb.successCount = 0
		cb.logger.Warn("[%s] Circuit breaker reopened (test failed)", cb.name)

	case StateOpen:
		// Already open, just update timestamp
		cb.logger.Debug("[%s] Failure while circuit open", cb.name)
	}
}

// setState transitions to a new state
func (cb *CircuitBreaker) setState(newState CircuitState) {
	oldState := cb.state
	cb.state = newState
	cb.lastStateChange = time.Now()

	// Call state change callback if configured
	if cb.config.OnStateChange != nil {
		// Call in goroutine to avoid blocking
		go cb.config.OnStateChange(oldState, newState, cb.name)
	}
}

// State returns the current state of the circuit breaker
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Metrics returns current circuit breaker metrics
func (cb *CircuitBreaker) Metrics() CircuitBreakerMetrics {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return CircuitBreakerMetrics{
		Name:            cb.name,
		State:           cb.state,
		FailureCount:    cb.failureCount,
		SuccessCount:    cb.successCount,
		LastFailureTime: cb.lastFailureTime,
		LastStateChange: cb.lastStateChange,
	}
}

// Reset manually resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	oldState := cb.state
	cb.state = StateClosed
	cb.failureCount = 0
	cb.successCount = 0
	cb.lastStateChange = time.Now()

	cb.logger.Info("[%s] Circuit breaker manually reset from %s to closed", cb.name, oldState)
}

// CircuitBreakerMetrics contains circuit breaker statistics
type CircuitBreakerMetrics struct {
	Name            string
	State           CircuitState
	FailureCount    int
	SuccessCount    int
	LastFailureTime time.Time
	LastStateChange time.Time
}

// CircuitBreakerManager manages multiple circuit breakers
type CircuitBreakerManager struct {
	breakers map[string]*CircuitBreaker
	config   CircuitBreakerConfig
	mu       sync.RWMutex
	logger   logging.Logger
}

// NewCircuitBreakerManager creates a new circuit breaker manager
func NewCircuitBreakerManager(config CircuitBreakerConfig) *CircuitBreakerManager {
	return &CircuitBreakerManager{
		breakers: make(map[string]*CircuitBreaker),
		config:   config,
		logger:   logging.NewComponentLogger("circuit-breaker-manager"),
	}
}

// Get returns a circuit breaker for the given name (creates if not exists)
func (m *CircuitBreakerManager) Get(name string) *CircuitBreaker {
	m.mu.RLock()
	if breaker, ok := m.breakers[name]; ok {
		m.mu.RUnlock()
		return breaker
	}
	m.mu.RUnlock()

	// Create new circuit breaker
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if breaker, ok := m.breakers[name]; ok {
		return breaker
	}

	breaker := NewCircuitBreaker(name, m.config)
	m.breakers[name] = breaker
	m.logger.Debug("Created circuit breaker for: %s", name)
	return breaker
}

// GetMetrics returns metrics for all circuit breakers
func (m *CircuitBreakerManager) GetMetrics() []CircuitBreakerMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := make([]CircuitBreakerMetrics, 0, len(m.breakers))
	for _, breaker := range m.breakers {
		metrics = append(metrics, breaker.Metrics())
	}
	return metrics
}

// ResetAll resets all circuit breakers
func (m *CircuitBreakerManager) ResetAll() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, breaker := range m.breakers {
		breaker.Reset()
	}
	m.logger.Info("Reset all circuit breakers")
}

// Remove removes a circuit breaker
func (m *CircuitBreakerManager) Remove(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.breakers, name)
	m.logger.Debug("Removed circuit breaker: %s", name)
}
