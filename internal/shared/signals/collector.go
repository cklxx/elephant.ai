// Package signals provides a centralized signal collection framework for
// recording, querying, and aggregating operational signals across the system.
// Signals capture tool outcomes, approval decisions, user feedback, retries,
// model switches, session abandonment, and latency outliers.
//
// This is an infrastructure-layer package following the port/adapter pattern.
package signals

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/segmentio/ksuid"
)

// ---------------------------------------------------------------------------
// Signal types
// ---------------------------------------------------------------------------

// SignalType classifies what kind of event a Signal represents.
type SignalType string

const (
	SignalToolSuccess      SignalType = "tool.success"
	SignalToolFailure      SignalType = "tool.failure"
	SignalToolTimeout      SignalType = "tool.timeout"
	SignalApprovalAccepted SignalType = "approval.accepted"
	SignalApprovalRejected SignalType = "approval.rejected"
	SignalUserFeedback     SignalType = "user.feedback"
	SignalRetry            SignalType = "retry"
	SignalModelSwitch      SignalType = "model.switch"
	SignalSessionAbandoned SignalType = "session.abandoned"
	SignalLatencyOutlier   SignalType = "latency.outlier"
)

// ---------------------------------------------------------------------------
// Core data structures
// ---------------------------------------------------------------------------

// Signal is a single recorded event in the system.
type Signal struct {
	ID        string            `json:"id"`
	Type      SignalType        `json:"type"`
	SessionID string            `json:"session_id"`
	UserID    string            `json:"user_id"`
	ToolName  string            `json:"tool_name,omitempty"`
	ModelName string            `json:"model_name,omitempty"`
	Value     float64           `json:"value"`
	Message   string            `json:"message,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// SignalQuery specifies filters for retrieving signals.
type SignalQuery struct {
	SessionID string      `json:"session_id,omitempty"`
	UserID    string      `json:"user_id,omitempty"`
	Types     []SignalType `json:"types,omitempty"`
	ToolName  string      `json:"tool_name,omitempty"`
	Since     time.Time   `json:"since,omitempty"`
	Until     time.Time   `json:"until,omitempty"`
	Limit     int         `json:"limit,omitempty"`
}

// SignalAggregate holds computed statistics over a set of signals.
type SignalAggregate struct {
	TotalCount       int                  `json:"total_count"`
	CountByType      map[SignalType]int   `json:"count_by_type"`
	AvgValue         float64              `json:"avg_value"`
	MaxValue         float64              `json:"max_value"`
	MinValue         float64              `json:"min_value"`
	ToolFailureRates map[string]float64   `json:"tool_failure_rates"`
	Period           time.Duration        `json:"period"`
}

// ---------------------------------------------------------------------------
// SignalCollector interface (port)
// ---------------------------------------------------------------------------

// SignalCollector is the primary port for recording and querying signals.
type SignalCollector interface {
	// Emit records a signal. The collector fills in ID and Timestamp if empty.
	Emit(ctx context.Context, signal Signal) error

	// Query returns signals matching the given filters.
	Query(ctx context.Context, query SignalQuery) ([]Signal, error)

	// Aggregate computes statistics over signals matching the query.
	Aggregate(ctx context.Context, query SignalQuery) (*SignalAggregate, error)

	// Flush persists any buffered signals.
	Flush(ctx context.Context) error
}

// ---------------------------------------------------------------------------
// InMemoryCollector (adapter)
// ---------------------------------------------------------------------------

const defaultMaxSize = 10000

// InMemoryCollector stores signals in a thread-safe ring buffer.
type InMemoryCollector struct {
	mu      sync.RWMutex
	buf     []Signal
	pos     int
	full    bool
	maxSize int
}

// NewInMemoryCollector creates a collector backed by a ring buffer of the
// given maximum size. If maxSize <= 0, defaultMaxSize is used.
func NewInMemoryCollector(maxSize int) *InMemoryCollector {
	if maxSize <= 0 {
		maxSize = defaultMaxSize
	}
	return &InMemoryCollector{
		buf:     make([]Signal, maxSize),
		maxSize: maxSize,
	}
}

// Emit stores a signal in the ring buffer.
func (c *InMemoryCollector) Emit(_ context.Context, s Signal) error {
	if s.ID == "" {
		s.ID = ksuid.New().String()
	}
	if s.Timestamp.IsZero() {
		s.Timestamp = time.Now()
	}

	c.mu.Lock()
	c.buf[c.pos] = s
	c.pos = (c.pos + 1) % c.maxSize
	if !c.full && c.pos == 0 {
		c.full = true
	}
	c.mu.Unlock()
	return nil
}

// snapshot returns an ordered copy of all stored signals (oldest first).
func (c *InMemoryCollector) snapshot() []Signal {
	count := c.count()
	if count == 0 {
		return nil
	}
	out := make([]Signal, 0, count)
	if c.full {
		// oldest entries start at c.pos (the next overwrite position)
		out = append(out, c.buf[c.pos:]...)
		out = append(out, c.buf[:c.pos]...)
	} else {
		out = append(out, c.buf[:c.pos]...)
	}
	return out
}

func (c *InMemoryCollector) count() int {
	if c.full {
		return c.maxSize
	}
	return c.pos
}

// Query returns signals matching the given filters.
func (c *InMemoryCollector) Query(_ context.Context, q SignalQuery) ([]Signal, error) {
	c.mu.RLock()
	all := c.snapshot()
	c.mu.RUnlock()

	return filterSignals(all, q), nil
}

// Aggregate computes statistics over matching signals.
func (c *InMemoryCollector) Aggregate(_ context.Context, q SignalQuery) (*SignalAggregate, error) {
	c.mu.RLock()
	all := c.snapshot()
	c.mu.RUnlock()

	matched := filterSignals(all, q)
	return computeAggregate(matched, q), nil
}

// Flush is a no-op for the in-memory collector.
func (c *InMemoryCollector) Flush(_ context.Context) error {
	return nil
}

// ---------------------------------------------------------------------------
// FileCollector (adapter)
// ---------------------------------------------------------------------------

const defaultFlushThreshold = 500

// FileCollector wraps an InMemoryCollector and persists signals to JSONL files
// under {basePath}/signals/{date}.jsonl. It auto-flushes when the buffer
// reaches a configurable threshold.
type FileCollector struct {
	mem            *InMemoryCollector
	basePath       string
	flushThreshold int

	mu       sync.Mutex
	pending  []Signal
	loadDays int
}

// NewFileCollector creates a FileCollector that stores JSONL files under
// basePath. maxSize controls the in-memory ring buffer size.
func NewFileCollector(basePath string, maxSize int) *FileCollector {
	return &FileCollector{
		mem:            NewInMemoryCollector(maxSize),
		basePath:       basePath,
		flushThreshold: defaultFlushThreshold,
		loadDays:       7,
	}
}

// SetFlushThreshold overrides the default auto-flush threshold.
func (f *FileCollector) SetFlushThreshold(n int) {
	f.mu.Lock()
	f.flushThreshold = n
	f.mu.Unlock()
}

// SetLoadDays configures how many days of history to reload.
func (f *FileCollector) SetLoadDays(days int) {
	f.mu.Lock()
	f.loadDays = days
	f.mu.Unlock()
}

// Load reads persisted signals from the last N days into the in-memory buffer.
func (f *FileCollector) Load(ctx context.Context) error {
	dir := filepath.Join(f.basePath, "signals")
	now := time.Now()
	for d := f.loadDays - 1; d >= 0; d-- {
		date := now.AddDate(0, 0, -d).Format("2006-01-02")
		path := filepath.Join(dir, date+".jsonl")
		signals, err := readJSONLFile(path)
		if err != nil {
			continue // file may not exist
		}
		for _, s := range signals {
			if err := f.mem.Emit(ctx, s); err != nil {
				return err
			}
		}
	}
	return nil
}

// Emit records a signal in-memory and buffers it for file persistence.
// Auto-flushes when the pending buffer reaches the threshold.
func (f *FileCollector) Emit(ctx context.Context, s Signal) error {
	if s.ID == "" {
		s.ID = ksuid.New().String()
	}
	if s.Timestamp.IsZero() {
		s.Timestamp = time.Now()
	}

	if err := f.mem.Emit(ctx, s); err != nil {
		return err
	}

	f.mu.Lock()
	f.pending = append(f.pending, s)
	shouldFlush := len(f.pending) >= f.flushThreshold
	f.mu.Unlock()

	if shouldFlush {
		return f.Flush(ctx)
	}
	return nil
}

// Query delegates to the in-memory collector.
func (f *FileCollector) Query(ctx context.Context, q SignalQuery) ([]Signal, error) {
	return f.mem.Query(ctx, q)
}

// Aggregate delegates to the in-memory collector.
func (f *FileCollector) Aggregate(ctx context.Context, q SignalQuery) (*SignalAggregate, error) {
	return f.mem.Aggregate(ctx, q)
}

// Flush persists all pending signals to JSONL files grouped by date.
func (f *FileCollector) Flush(_ context.Context) error {
	f.mu.Lock()
	if len(f.pending) == 0 {
		f.mu.Unlock()
		return nil
	}
	batch := f.pending
	f.pending = nil
	f.mu.Unlock()

	// Group signals by date.
	byDate := make(map[string][]Signal)
	for _, s := range batch {
		date := s.Timestamp.Format("2006-01-02")
		byDate[date] = append(byDate[date], s)
	}

	dir := filepath.Join(f.basePath, "signals")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("signals: mkdir %s: %w", dir, err)
	}

	for date, signals := range byDate {
		path := filepath.Join(dir, date+".jsonl")
		file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("signals: open %s: %w", path, err)
		}
		enc := json.NewEncoder(file)
		for _, s := range signals {
			if err := enc.Encode(s); err != nil {
				file.Close()
				return fmt.Errorf("signals: encode: %w", err)
			}
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("signals: close %s: %w", path, err)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helper functions
// ---------------------------------------------------------------------------

// EmitToolResult is a convenience function that emits either a tool success,
// failure, or timeout signal based on the provided error and latency.
func EmitToolResult(collector SignalCollector, sessionID, userID, toolName string, err error, latencyMs float64) error {
	sigType := SignalToolSuccess
	msg := "tool completed successfully"
	if err != nil {
		sigType = SignalToolFailure
		msg = err.Error()
		if isTimeout(err) {
			sigType = SignalToolTimeout
			msg = "tool timed out: " + err.Error()
		}
	}

	return collector.Emit(context.Background(), Signal{
		Type:      sigType,
		SessionID: sessionID,
		UserID:    userID,
		ToolName:  toolName,
		Value:     latencyMs,
		Message:   msg,
	})
}

// EmitApproval emits an approval accepted or rejected signal.
func EmitApproval(collector SignalCollector, sessionID, userID, toolName string, accepted bool) error {
	sigType := SignalApprovalAccepted
	msg := "action approved"
	if !accepted {
		sigType = SignalApprovalRejected
		msg = "action rejected"
	}

	return collector.Emit(context.Background(), Signal{
		Type:      sigType,
		SessionID: sessionID,
		UserID:    userID,
		ToolName:  toolName,
		Message:   msg,
	})
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// filterSignals applies query filters and returns matching signals in order.
func filterSignals(signals []Signal, q SignalQuery) []Signal {
	typeSet := make(map[SignalType]struct{}, len(q.Types))
	for _, t := range q.Types {
		typeSet[t] = struct{}{}
	}

	var out []Signal
	for _, s := range signals {
		if q.SessionID != "" && s.SessionID != q.SessionID {
			continue
		}
		if q.UserID != "" && s.UserID != q.UserID {
			continue
		}
		if len(typeSet) > 0 {
			if _, ok := typeSet[s.Type]; !ok {
				continue
			}
		}
		if q.ToolName != "" && s.ToolName != q.ToolName {
			continue
		}
		if !q.Since.IsZero() && s.Timestamp.Before(q.Since) {
			continue
		}
		if !q.Until.IsZero() && s.Timestamp.After(q.Until) {
			continue
		}
		out = append(out, s)
		if q.Limit > 0 && len(out) >= q.Limit {
			break
		}
	}
	return out
}

// computeAggregate derives stats from a set of already-filtered signals.
func computeAggregate(signals []Signal, q SignalQuery) *SignalAggregate {
	agg := &SignalAggregate{
		CountByType:      make(map[SignalType]int),
		ToolFailureRates: make(map[string]float64),
		MinValue:         math.MaxFloat64,
		MaxValue:         -math.MaxFloat64,
	}

	if len(signals) == 0 {
		agg.MinValue = 0
		agg.MaxValue = 0
		return agg
	}

	var sumValue float64
	// Track per-tool success/failure counts for failure rate.
	type toolCounts struct {
		total    int
		failures int
	}
	toolStats := make(map[string]*toolCounts)

	var earliest, latest time.Time

	for _, s := range signals {
		agg.TotalCount++
		agg.CountByType[s.Type]++
		sumValue += s.Value

		if s.Value > agg.MaxValue {
			agg.MaxValue = s.Value
		}
		if s.Value < agg.MinValue {
			agg.MinValue = s.Value
		}

		// Track tool failure rates for tool-related signals.
		if s.ToolName != "" {
			tc, ok := toolStats[s.ToolName]
			if !ok {
				tc = &toolCounts{}
				toolStats[s.ToolName] = tc
			}
			switch s.Type {
			case SignalToolSuccess, SignalToolFailure, SignalToolTimeout:
				tc.total++
				if s.Type == SignalToolFailure || s.Type == SignalToolTimeout {
					tc.failures++
				}
			}
		}

		if earliest.IsZero() || s.Timestamp.Before(earliest) {
			earliest = s.Timestamp
		}
		if latest.IsZero() || s.Timestamp.After(latest) {
			latest = s.Timestamp
		}
	}

	agg.AvgValue = sumValue / float64(agg.TotalCount)

	for tool, tc := range toolStats {
		if tc.total > 0 {
			agg.ToolFailureRates[tool] = float64(tc.failures) / float64(tc.total)
		}
	}

	if !earliest.IsZero() && !latest.IsZero() {
		agg.Period = latest.Sub(earliest)
	}

	// If the query specified a time range, use that for Period instead.
	if !q.Since.IsZero() && !q.Until.IsZero() {
		agg.Period = q.Until.Sub(q.Since)
	}

	return agg
}

// readJSONLFile reads signals from a JSONL file.
func readJSONLFile(path string) ([]Signal, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var signals []Signal
	scanner := bufio.NewScanner(file)
	// Allow lines up to 1 MB.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var s Signal
		if err := json.Unmarshal(line, &s); err != nil {
			continue // skip malformed lines
		}
		signals = append(signals, s)
	}
	return signals, scanner.Err()
}

// isTimeout checks if an error looks like a timeout.
func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return contains(msg, "timeout") || contains(msg, "deadline exceeded")
}

// contains checks if s contains substr without importing strings.
func contains(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
