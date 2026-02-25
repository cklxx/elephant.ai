package signals

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// InMemoryCollector tests
// ---------------------------------------------------------------------------

func TestEmit_GeneratesIDAndTimestamp(t *testing.T) {
	c := NewInMemoryCollector(100)
	ctx := context.Background()

	err := c.Emit(ctx, Signal{
		Type:      SignalToolSuccess,
		SessionID: "sess-1",
		UserID:    "user-1",
		ToolName:  "web_search",
		Value:     42.5,
	})
	require.NoError(t, err)

	results, err := c.Query(ctx, SignalQuery{})
	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.NotEmpty(t, results[0].ID, "ID should be auto-generated")
	assert.False(t, results[0].Timestamp.IsZero(), "Timestamp should be auto-set")
	assert.Equal(t, SignalToolSuccess, results[0].Type)
	assert.Equal(t, "sess-1", results[0].SessionID)
	assert.Equal(t, 42.5, results[0].Value)
}

func TestEmit_PreservesExplicitIDAndTimestamp(t *testing.T) {
	c := NewInMemoryCollector(100)
	ctx := context.Background()
	ts := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	err := c.Emit(ctx, Signal{
		ID:        "custom-id",
		Type:      SignalUserFeedback,
		SessionID: "sess-1",
		Timestamp: ts,
	})
	require.NoError(t, err)

	results, err := c.Query(ctx, SignalQuery{})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "custom-id", results[0].ID)
	assert.Equal(t, ts, results[0].Timestamp)
}

func TestQuery_FilterByType(t *testing.T) {
	c := NewInMemoryCollector(100)
	ctx := context.Background()

	_ = c.Emit(ctx, Signal{Type: SignalToolSuccess, SessionID: "s1"})
	_ = c.Emit(ctx, Signal{Type: SignalToolFailure, SessionID: "s1"})
	_ = c.Emit(ctx, Signal{Type: SignalToolTimeout, SessionID: "s1"})
	_ = c.Emit(ctx, Signal{Type: SignalToolSuccess, SessionID: "s1"})

	results, err := c.Query(ctx, SignalQuery{Types: []SignalType{SignalToolFailure, SignalToolTimeout}})
	require.NoError(t, err)
	assert.Len(t, results, 2)
	for _, r := range results {
		assert.Contains(t, []SignalType{SignalToolFailure, SignalToolTimeout}, r.Type)
	}
}

func TestQuery_FilterBySessionAndUser(t *testing.T) {
	c := NewInMemoryCollector(100)
	ctx := context.Background()

	_ = c.Emit(ctx, Signal{Type: SignalToolSuccess, SessionID: "s1", UserID: "u1"})
	_ = c.Emit(ctx, Signal{Type: SignalToolSuccess, SessionID: "s2", UserID: "u1"})
	_ = c.Emit(ctx, Signal{Type: SignalToolSuccess, SessionID: "s1", UserID: "u2"})
	_ = c.Emit(ctx, Signal{Type: SignalToolSuccess, SessionID: "s2", UserID: "u2"})

	results, err := c.Query(ctx, SignalQuery{SessionID: "s1", UserID: "u1"})
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "s1", results[0].SessionID)
	assert.Equal(t, "u1", results[0].UserID)
}

func TestQuery_FilterByToolName(t *testing.T) {
	c := NewInMemoryCollector(100)
	ctx := context.Background()

	_ = c.Emit(ctx, Signal{Type: SignalToolSuccess, ToolName: "browser"})
	_ = c.Emit(ctx, Signal{Type: SignalToolFailure, ToolName: "web_search"})
	_ = c.Emit(ctx, Signal{Type: SignalToolSuccess, ToolName: "browser"})

	results, err := c.Query(ctx, SignalQuery{ToolName: "web_search"})
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "web_search", results[0].ToolName)
}

func TestQuery_FilterByTimeRange(t *testing.T) {
	c := NewInMemoryCollector(100)
	ctx := context.Background()

	t1 := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	t3 := time.Date(2025, 1, 1, 14, 0, 0, 0, time.UTC)

	_ = c.Emit(ctx, Signal{Type: SignalToolSuccess, Timestamp: t1, Message: "morning"})
	_ = c.Emit(ctx, Signal{Type: SignalToolSuccess, Timestamp: t2, Message: "noon"})
	_ = c.Emit(ctx, Signal{Type: SignalToolSuccess, Timestamp: t3, Message: "afternoon"})

	since := time.Date(2025, 1, 1, 11, 0, 0, 0, time.UTC)
	until := time.Date(2025, 1, 1, 13, 0, 0, 0, time.UTC)

	results, err := c.Query(ctx, SignalQuery{Since: since, Until: until})
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "noon", results[0].Message)
}

func TestQuery_RespectsLimit(t *testing.T) {
	c := NewInMemoryCollector(100)
	ctx := context.Background()

	for i := 0; i < 20; i++ {
		_ = c.Emit(ctx, Signal{Type: SignalToolSuccess, SessionID: "s1"})
	}

	results, err := c.Query(ctx, SignalQuery{Limit: 5})
	require.NoError(t, err)
	assert.Len(t, results, 5)
}

func TestQuery_EmptyQueryReturnsAll(t *testing.T) {
	c := NewInMemoryCollector(100)
	ctx := context.Background()

	_ = c.Emit(ctx, Signal{Type: SignalToolSuccess})
	_ = c.Emit(ctx, Signal{Type: SignalToolFailure})
	_ = c.Emit(ctx, Signal{Type: SignalRetry})

	results, err := c.Query(ctx, SignalQuery{})
	require.NoError(t, err)
	assert.Len(t, results, 3)
}

func TestAggregate_CountsAndAverages(t *testing.T) {
	c := NewInMemoryCollector(100)
	ctx := context.Background()

	_ = c.Emit(ctx, Signal{Type: SignalToolSuccess, Value: 100})
	_ = c.Emit(ctx, Signal{Type: SignalToolSuccess, Value: 200})
	_ = c.Emit(ctx, Signal{Type: SignalToolFailure, Value: 300})

	agg, err := c.Aggregate(ctx, SignalQuery{})
	require.NoError(t, err)

	assert.Equal(t, 3, agg.TotalCount)
	assert.Equal(t, 2, agg.CountByType[SignalToolSuccess])
	assert.Equal(t, 1, agg.CountByType[SignalToolFailure])
	assert.InDelta(t, 200.0, agg.AvgValue, 0.01)
	assert.Equal(t, 300.0, agg.MaxValue)
	assert.Equal(t, 100.0, agg.MinValue)
}

func TestAggregate_ToolFailureRates(t *testing.T) {
	c := NewInMemoryCollector(100)
	ctx := context.Background()

	// browser: 3 success, 1 failure => 25% failure rate
	_ = c.Emit(ctx, Signal{Type: SignalToolSuccess, ToolName: "browser"})
	_ = c.Emit(ctx, Signal{Type: SignalToolSuccess, ToolName: "browser"})
	_ = c.Emit(ctx, Signal{Type: SignalToolSuccess, ToolName: "browser"})
	_ = c.Emit(ctx, Signal{Type: SignalToolFailure, ToolName: "browser"})

	// web_search: 1 success, 1 timeout => 50% failure rate
	_ = c.Emit(ctx, Signal{Type: SignalToolSuccess, ToolName: "web_search"})
	_ = c.Emit(ctx, Signal{Type: SignalToolTimeout, ToolName: "web_search"})

	agg, err := c.Aggregate(ctx, SignalQuery{})
	require.NoError(t, err)

	assert.InDelta(t, 0.25, agg.ToolFailureRates["browser"], 0.01)
	assert.InDelta(t, 0.50, agg.ToolFailureRates["web_search"], 0.01)
}

func TestAggregate_EmptySignals(t *testing.T) {
	c := NewInMemoryCollector(100)
	ctx := context.Background()

	agg, err := c.Aggregate(ctx, SignalQuery{})
	require.NoError(t, err)
	assert.Equal(t, 0, agg.TotalCount)
	assert.Equal(t, 0.0, agg.MinValue)
	assert.Equal(t, 0.0, agg.MaxValue)
	assert.Equal(t, 0.0, agg.AvgValue)
}

func TestConcurrentEmitSafety(t *testing.T) {
	c := NewInMemoryCollector(1000)
	ctx := context.Background()
	const goroutines = 50
	const perGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				_ = c.Emit(ctx, Signal{
					Type:      SignalToolSuccess,
					SessionID: "concurrent",
				})
			}
		}()
	}
	wg.Wait()

	results, err := c.Query(ctx, SignalQuery{SessionID: "concurrent"})
	require.NoError(t, err)
	// Ring buffer is 1000, total emits = 5000 so buffer is full.
	assert.Equal(t, 1000, len(results))
}

func TestRingBuffer_EvictsOldest(t *testing.T) {
	c := NewInMemoryCollector(3)
	ctx := context.Background()

	_ = c.Emit(ctx, Signal{Type: SignalToolSuccess, Message: "first"})
	_ = c.Emit(ctx, Signal{Type: SignalToolSuccess, Message: "second"})
	_ = c.Emit(ctx, Signal{Type: SignalToolSuccess, Message: "third"})
	_ = c.Emit(ctx, Signal{Type: SignalToolSuccess, Message: "fourth"})

	results, err := c.Query(ctx, SignalQuery{})
	require.NoError(t, err)
	assert.Len(t, results, 3)

	// "first" should have been evicted.
	messages := make([]string, len(results))
	for i, r := range results {
		messages[i] = r.Message
	}
	assert.NotContains(t, messages, "first")
	assert.Contains(t, messages, "second")
	assert.Contains(t, messages, "third")
	assert.Contains(t, messages, "fourth")
}

func TestRingBuffer_OrderOldestFirst(t *testing.T) {
	c := NewInMemoryCollector(3)
	ctx := context.Background()

	_ = c.Emit(ctx, Signal{Type: SignalToolSuccess, Message: "a"})
	_ = c.Emit(ctx, Signal{Type: SignalToolSuccess, Message: "b"})
	_ = c.Emit(ctx, Signal{Type: SignalToolSuccess, Message: "c"})
	// Buffer is full, next write overwrites position 0.
	_ = c.Emit(ctx, Signal{Type: SignalToolSuccess, Message: "d"})

	results, err := c.Query(ctx, SignalQuery{})
	require.NoError(t, err)
	require.Len(t, results, 3)
	// Oldest first: b, c, d
	assert.Equal(t, "b", results[0].Message)
	assert.Equal(t, "c", results[1].Message)
	assert.Equal(t, "d", results[2].Message)
}

func TestDefaultMaxSize(t *testing.T) {
	c := NewInMemoryCollector(0)
	assert.Equal(t, defaultMaxSize, c.maxSize)

	c2 := NewInMemoryCollector(-5)
	assert.Equal(t, defaultMaxSize, c2.maxSize)
}

// ---------------------------------------------------------------------------
// FileCollector tests
// ---------------------------------------------------------------------------

func TestFileCollector_FlushPersistsToDisk(t *testing.T) {
	dir := t.TempDir()
	fc := NewFileCollector(dir, 100)
	ctx := context.Background()

	ts := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	_ = fc.Emit(ctx, Signal{
		Type:      SignalToolSuccess,
		SessionID: "s1",
		ToolName:  "browser",
		Value:     150.0,
		Timestamp: ts,
	})

	err := fc.Flush(ctx)
	require.NoError(t, err)

	// Check that the JSONL file was created.
	path := filepath.Join(dir, "signals", "2025-06-15.jsonl")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"tool_name":"browser"`)
	assert.Contains(t, string(data), `"session_id":"s1"`)
}

func TestFileCollector_ReloadReadsPersisted(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	// Phase 1: emit and flush.
	fc1 := NewFileCollector(dir, 100)
	now := time.Now()
	_ = fc1.Emit(ctx, Signal{
		Type:      SignalToolFailure,
		SessionID: "s-reload",
		ToolName:  "shell",
		Value:     500.0,
		Message:   "command failed",
		Timestamp: now,
	})
	err := fc1.Flush(ctx)
	require.NoError(t, err)

	// Phase 2: new collector loads from disk.
	fc2 := NewFileCollector(dir, 100)
	fc2.SetLoadDays(7)
	err = fc2.Load(ctx)
	require.NoError(t, err)

	results, err := fc2.Query(ctx, SignalQuery{SessionID: "s-reload"})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, SignalToolFailure, results[0].Type)
	assert.Equal(t, "shell", results[0].ToolName)
	assert.Equal(t, "command failed", results[0].Message)
}

func TestFileCollector_AutoFlushOnThreshold(t *testing.T) {
	dir := t.TempDir()
	fc := NewFileCollector(dir, 1000)
	fc.SetFlushThreshold(3)
	ctx := context.Background()

	now := time.Now()
	// Emit 3 signals to trigger auto-flush.
	for i := 0; i < 3; i++ {
		_ = fc.Emit(ctx, Signal{
			Type:      SignalToolSuccess,
			SessionID: "auto-flush",
			Timestamp: now,
		})
	}

	// File should exist after auto-flush.
	date := now.Format("2006-01-02")
	path := filepath.Join(dir, "signals", date+".jsonl")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "auto-flush")
}

func TestFileCollector_FlushEmptyPendingIsNoop(t *testing.T) {
	dir := t.TempDir()
	fc := NewFileCollector(dir, 100)
	ctx := context.Background()

	err := fc.Flush(ctx)
	require.NoError(t, err)

	// signals directory should not be created.
	_, err = os.Stat(filepath.Join(dir, "signals"))
	assert.True(t, os.IsNotExist(err))
}

// ---------------------------------------------------------------------------
// Helper function tests
// ---------------------------------------------------------------------------

func TestEmitToolResult_Success(t *testing.T) {
	c := NewInMemoryCollector(100)
	ctx := context.Background()

	err := EmitToolResult(c, "s1", "u1", "browser", nil, 120.5)
	require.NoError(t, err)

	results, err := c.Query(ctx, SignalQuery{})
	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.Equal(t, SignalToolSuccess, results[0].Type)
	assert.Equal(t, "browser", results[0].ToolName)
	assert.Equal(t, 120.5, results[0].Value)
	assert.Equal(t, "s1", results[0].SessionID)
	assert.Equal(t, "u1", results[0].UserID)
}

func TestEmitToolResult_Failure(t *testing.T) {
	c := NewInMemoryCollector(100)
	ctx := context.Background()

	err := EmitToolResult(c, "s1", "u1", "web_search", errors.New("connection refused"), 5000.0)
	require.NoError(t, err)

	results, err := c.Query(ctx, SignalQuery{})
	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.Equal(t, SignalToolFailure, results[0].Type)
	assert.Equal(t, "web_search", results[0].ToolName)
	assert.Contains(t, results[0].Message, "connection refused")
}

func TestEmitToolResult_Timeout(t *testing.T) {
	c := NewInMemoryCollector(100)
	ctx := context.Background()

	err := EmitToolResult(c, "s1", "u1", "shell", errors.New("context deadline exceeded"), 30000.0)
	require.NoError(t, err)

	results, err := c.Query(ctx, SignalQuery{})
	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.Equal(t, SignalToolTimeout, results[0].Type)
}

func TestEmitApproval_Accepted(t *testing.T) {
	c := NewInMemoryCollector(100)
	ctx := context.Background()

	err := EmitApproval(c, "s1", "u1", "dangerous_tool", true)
	require.NoError(t, err)

	results, err := c.Query(ctx, SignalQuery{})
	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.Equal(t, SignalApprovalAccepted, results[0].Type)
	assert.Equal(t, "dangerous_tool", results[0].ToolName)
	assert.Equal(t, "s1", results[0].SessionID)
	assert.Equal(t, "u1", results[0].UserID)
}

func TestEmitApproval_Rejected(t *testing.T) {
	c := NewInMemoryCollector(100)
	ctx := context.Background()

	err := EmitApproval(c, "s1", "u1", "dangerous_tool", false)
	require.NoError(t, err)

	results, err := c.Query(ctx, SignalQuery{})
	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.Equal(t, SignalApprovalRejected, results[0].Type)
}

// ---------------------------------------------------------------------------
// Query with multiple filter combinations
// ---------------------------------------------------------------------------

func TestQuery_CombinedFilters(t *testing.T) {
	c := NewInMemoryCollector(100)
	ctx := context.Background()

	ts := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	_ = c.Emit(ctx, Signal{Type: SignalToolSuccess, SessionID: "s1", UserID: "u1", ToolName: "browser", Timestamp: ts})
	_ = c.Emit(ctx, Signal{Type: SignalToolFailure, SessionID: "s1", UserID: "u1", ToolName: "browser", Timestamp: ts})
	_ = c.Emit(ctx, Signal{Type: SignalToolSuccess, SessionID: "s1", UserID: "u1", ToolName: "shell", Timestamp: ts})
	_ = c.Emit(ctx, Signal{Type: SignalToolSuccess, SessionID: "s2", UserID: "u1", ToolName: "browser", Timestamp: ts})

	results, err := c.Query(ctx, SignalQuery{
		SessionID: "s1",
		UserID:    "u1",
		ToolName:  "browser",
		Types:     []SignalType{SignalToolSuccess},
	})
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, SignalToolSuccess, results[0].Type)
	assert.Equal(t, "browser", results[0].ToolName)
}

func TestAggregate_WithQueryTimeRange(t *testing.T) {
	c := NewInMemoryCollector(100)
	ctx := context.Background()

	since := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	_ = c.Emit(ctx, Signal{Type: SignalToolSuccess, Value: 100, Timestamp: since.Add(time.Hour)})

	agg, err := c.Aggregate(ctx, SignalQuery{Since: since, Until: until})
	require.NoError(t, err)
	assert.Equal(t, 24*time.Hour, agg.Period)
}

// ---------------------------------------------------------------------------
// Signal type coverage
// ---------------------------------------------------------------------------

func TestAllSignalTypes(t *testing.T) {
	c := NewInMemoryCollector(100)
	ctx := context.Background()

	types := []SignalType{
		SignalToolSuccess,
		SignalToolFailure,
		SignalToolTimeout,
		SignalApprovalAccepted,
		SignalApprovalRejected,
		SignalUserFeedback,
		SignalRetry,
		SignalModelSwitch,
		SignalSessionAbandoned,
		SignalLatencyOutlier,
	}

	for _, st := range types {
		err := c.Emit(ctx, Signal{Type: st, SessionID: "all-types"})
		require.NoError(t, err)
	}

	results, err := c.Query(ctx, SignalQuery{})
	require.NoError(t, err)
	assert.Len(t, results, len(types))
}

func TestSignal_MetadataMap(t *testing.T) {
	c := NewInMemoryCollector(100)
	ctx := context.Background()

	_ = c.Emit(ctx, Signal{
		Type: SignalModelSwitch,
		Metadata: map[string]string{
			"from_model": "gpt-4",
			"to_model":   "claude-3",
			"reason":     "cost",
		},
	})

	results, err := c.Query(ctx, SignalQuery{Types: []SignalType{SignalModelSwitch}})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "gpt-4", results[0].Metadata["from_model"])
	assert.Equal(t, "claude-3", results[0].Metadata["to_model"])
}
