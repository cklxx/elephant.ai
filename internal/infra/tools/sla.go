package tools

import (
	"math"
	"sort"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	// slidingWindowSize is the number of recent calls tracked per tool for
	// success-rate calculation.
	slidingWindowSize = 100
)

// ToolSLA is a point-in-time snapshot of a tool's service level metrics.
type ToolSLA struct {
	ToolName     string
	P50Latency   time.Duration
	P95Latency   time.Duration
	P99Latency   time.Duration
	ErrorRate    float64
	CallCount    int64
	SuccessRate  float64
	CostUSDTotal float64
	CostUSDAvg   float64
}

// SLACollector records per-tool latency, error rate, and call count via
// Prometheus metrics, and maintains a sliding window for success-rate
// calculation.
type SLACollector struct {
	toolLatency     *prometheus.HistogramVec
	toolErrors      *prometheus.CounterVec
	toolCalls       *prometheus.CounterVec
	toolSuccessRate *prometheus.GaugeVec

	mu      sync.RWMutex
	windows map[string]*slidingWindow
}

// slidingWindow tracks the last N call outcomes (true = success, false = failure)
// along with latencies for percentile computation.
type slidingWindow struct {
	outcomes  []bool
	latencies []time.Duration
	pos       int
	full      bool
	total     int64
	costTotal float64
}

func newSlidingWindow(size int) *slidingWindow {
	return &slidingWindow{
		outcomes:  make([]bool, size),
		latencies: make([]time.Duration, size),
	}
}

func (w *slidingWindow) record(success bool, d time.Duration, costUSD float64) {
	w.outcomes[w.pos] = success
	w.latencies[w.pos] = d
	w.pos = (w.pos + 1) % len(w.outcomes)
	if !w.full && w.pos == 0 {
		w.full = true
	}
	w.total++
	w.costTotal += costUSD
}

func (w *slidingWindow) count() int {
	if w.full {
		return len(w.outcomes)
	}
	return w.pos
}

func (w *slidingWindow) successRate() float64 {
	n := w.count()
	if n == 0 {
		return 0
	}
	var successes int
	for i := 0; i < n; i++ {
		if w.outcomes[i] {
			successes++
		}
	}
	return float64(successes) / float64(n)
}

func (w *slidingWindow) errorRate() float64 {
	return 1.0 - w.successRate()
}

func (w *slidingWindow) percentile(p float64) time.Duration {
	n := w.count()
	if n == 0 {
		return 0
	}
	sorted := make([]time.Duration, n)
	copy(sorted, w.latencies[:n])
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(math.Ceil(p/100.0*float64(n))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	return sorted[idx]
}

// NewSLACollector creates a new SLACollector and registers Prometheus
// metrics with the provided registerer. If registerer is nil,
// prometheus.DefaultRegisterer is used.
func NewSLACollector(registerer prometheus.Registerer) *SLACollector {
	if registerer == nil {
		registerer = prometheus.DefaultRegisterer
	}

	toolLatency := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "alex",
		Subsystem: "tool_sla",
		Name:      "latency_seconds",
		Help:      "Tool execution latency in seconds, partitioned by tool name and status.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"tool_name", "status"})

	toolErrors := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "alex",
		Subsystem: "tool_sla",
		Name:      "errors_total",
		Help:      "Total tool execution errors, partitioned by tool name and error type.",
	}, []string{"tool_name", "error_type"})

	toolCalls := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "alex",
		Subsystem: "tool_sla",
		Name:      "calls_total",
		Help:      "Total tool calls, partitioned by tool name.",
	}, []string{"tool_name"})

	toolSuccessRate := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "alex",
		Subsystem: "tool_sla",
		Name:      "success_rate",
		Help:      "Sliding window success rate per tool (last 100 calls).",
	}, []string{"tool_name"})

	toolLatency = registerHistogramVec(registerer, toolLatency)
	toolErrors = registerCounterVec(registerer, toolErrors)
	toolCalls = registerCounterVec(registerer, toolCalls)
	toolSuccessRate = registerGaugeVec(registerer, toolSuccessRate)

	return &SLACollector{
		toolLatency:     toolLatency,
		toolErrors:      toolErrors,
		toolCalls:       toolCalls,
		toolSuccessRate: toolSuccessRate,
		windows:         make(map[string]*slidingWindow),
	}
}

// RecordExecution records a single tool execution's duration and outcome.
// It is safe to call on a nil receiver (no-op).
func (c *SLACollector) RecordExecution(toolName string, duration time.Duration, err error) {
	c.RecordExecutionWithCost(toolName, duration, err, 0)
}

// RecordExecutionWithCost records a single tool execution including optional
// per-call cost in USD.
func (c *SLACollector) RecordExecutionWithCost(toolName string, duration time.Duration, err error, costUSD float64) {
	if c == nil {
		return
	}

	status := "success"
	if err != nil {
		status = "error"
	}

	seconds := duration.Seconds()
	c.toolLatency.WithLabelValues(toolName, status).Observe(seconds)
	c.toolCalls.WithLabelValues(toolName).Inc()

	if err != nil {
		errType := classifyError(err)
		c.toolErrors.WithLabelValues(toolName, errType).Inc()
	}

	// Update sliding window.
	c.mu.Lock()
	w, ok := c.windows[toolName]
	if !ok {
		w = newSlidingWindow(slidingWindowSize)
		c.windows[toolName] = w
	}
	w.record(err == nil, duration, costUSD)
	rate := w.successRate()
	c.mu.Unlock()

	c.toolSuccessRate.WithLabelValues(toolName).Set(rate)
}

// GetSLA returns a point-in-time SLA snapshot for the named tool.
// It is safe to call on a nil receiver (returns zero-value ToolSLA).
func (c *SLACollector) GetSLA(toolName string) ToolSLA {
	if c == nil {
		return ToolSLA{ToolName: toolName}
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	w, ok := c.windows[toolName]
	if !ok {
		return ToolSLA{ToolName: toolName}
	}

	return ToolSLA{
		ToolName:     toolName,
		P50Latency:   w.percentile(50),
		P95Latency:   w.percentile(95),
		P99Latency:   w.percentile(99),
		ErrorRate:    w.errorRate(),
		CallCount:    w.total,
		SuccessRate:  w.successRate(),
		CostUSDTotal: w.costTotal,
		CostUSDAvg:   averageCostUSD(w.total, w.costTotal),
	}
}

func averageCostUSD(calls int64, total float64) float64 {
	if calls <= 0 {
		return 0
	}
	return total / float64(calls)
}

func registerHistogramVec(registerer prometheus.Registerer, collector *prometheus.HistogramVec) *prometheus.HistogramVec {
	if err := registerer.Register(collector); err != nil {
		if already, ok := err.(prometheus.AlreadyRegisteredError); ok {
			if existing, castOK := already.ExistingCollector.(*prometheus.HistogramVec); castOK {
				return existing
			}
			panic(err)
		}
		panic(err)
	}
	return collector
}

func registerCounterVec(registerer prometheus.Registerer, collector *prometheus.CounterVec) *prometheus.CounterVec {
	if err := registerer.Register(collector); err != nil {
		if already, ok := err.(prometheus.AlreadyRegisteredError); ok {
			if existing, castOK := already.ExistingCollector.(*prometheus.CounterVec); castOK {
				return existing
			}
			panic(err)
		}
		panic(err)
	}
	return collector
}

func registerGaugeVec(registerer prometheus.Registerer, collector *prometheus.GaugeVec) *prometheus.GaugeVec {
	if err := registerer.Register(collector); err != nil {
		if already, ok := err.(prometheus.AlreadyRegisteredError); ok {
			if existing, castOK := already.ExistingCollector.(*prometheus.GaugeVec); castOK {
				return existing
			}
			panic(err)
		}
		panic(err)
	}
	return collector
}

// classifyError returns a short error-type label for Prometheus.
func classifyError(err error) string {
	if err == nil {
		return "none"
	}
	msg := err.Error()
	switch {
	case containsAny(msg, "timeout", "deadline exceeded"):
		return "timeout"
	case containsAny(msg, "context canceled"):
		return "canceled"
	case containsAny(msg, "permission", "denied", "rejected"):
		return "permission"
	case containsAny(msg, "not found"):
		return "not_found"
	default:
		return "unknown"
	}
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
