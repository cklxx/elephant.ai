package tools

import (
	"context"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	ports "alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"

	"github.com/prometheus/client_golang/prometheus"
)

// --- helpers ----------------------------------------------------------------

// stubToolExecutor is a minimal ToolExecutor for testing.
type stubToolExecutor struct {
	name    string
	execFn  func(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error)
	sleepMs int
}

func (s *stubToolExecutor) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	if s.sleepMs > 0 {
		time.Sleep(time.Duration(s.sleepMs) * time.Millisecond)
	}
	if s.execFn != nil {
		return s.execFn(ctx, call)
	}
	return &ports.ToolResult{CallID: call.ID, Content: "ok"}, nil
}

func (s *stubToolExecutor) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{Name: s.name, Description: "stub"}
}

func (s *stubToolExecutor) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{Name: s.name}
}

var _ tools.ToolExecutor = (*stubToolExecutor)(nil)

// newTestCollector creates an SLACollector with an isolated registry.
func newTestCollector(t *testing.T) *SLACollector {
	t.Helper()
	reg := prometheus.NewRegistry()
	return NewSLACollector(reg)
}

// --- TestSLACollector_RecordExecution ----------------------------------------

func TestSLACollector_RecordExecution(t *testing.T) {
	c := newTestCollector(t)

	// Record a success.
	c.RecordExecution("my_tool", 100*time.Millisecond, nil)
	sla := c.GetSLA("my_tool")
	if sla.CallCount != 1 {
		t.Fatalf("expected CallCount=1, got %d", sla.CallCount)
	}
	if sla.SuccessRate != 1.0 {
		t.Fatalf("expected SuccessRate=1.0, got %f", sla.SuccessRate)
	}

	// Record an error.
	c.RecordExecution("my_tool", 200*time.Millisecond, fmt.Errorf("timeout"))
	sla = c.GetSLA("my_tool")
	if sla.CallCount != 2 {
		t.Fatalf("expected CallCount=2, got %d", sla.CallCount)
	}
	if sla.SuccessRate != 0.5 {
		t.Fatalf("expected SuccessRate=0.5, got %f", sla.SuccessRate)
	}
	if sla.ErrorRate != 0.5 {
		t.Fatalf("expected ErrorRate=0.5, got %f", sla.ErrorRate)
	}
	if sla.CostUSDTotal != 0 {
		t.Fatalf("expected CostUSDTotal=0, got %f", sla.CostUSDTotal)
	}
}

func TestSLACollector_RecordExecutionWithCost(t *testing.T) {
	c := newTestCollector(t)

	c.RecordExecutionWithCost("cost_tool", 100*time.Millisecond, nil, 0.25)
	c.RecordExecutionWithCost("cost_tool", 200*time.Millisecond, fmt.Errorf("timeout"), 0.75)

	sla := c.GetSLA("cost_tool")
	if sla.CallCount != 2 {
		t.Fatalf("expected CallCount=2, got %d", sla.CallCount)
	}
	if sla.CostUSDTotal != 1.0 {
		t.Fatalf("expected CostUSDTotal=1.0, got %f", sla.CostUSDTotal)
	}
	if sla.CostUSDAvg != 0.5 {
		t.Fatalf("expected CostUSDAvg=0.5, got %f", sla.CostUSDAvg)
	}
}

// --- TestSLACollector_GetSLA ------------------------------------------------

func TestSLACollector_GetSLA(t *testing.T) {
	c := newTestCollector(t)

	// Record 100 calls with linearly increasing latencies (1ms .. 100ms)
	for i := 1; i <= 100; i++ {
		c.RecordExecution("latency_tool", time.Duration(i)*time.Millisecond, nil)
	}

	sla := c.GetSLA("latency_tool")
	if sla.CallCount != 100 {
		t.Fatalf("expected CallCount=100, got %d", sla.CallCount)
	}

	// P50 should be around 50ms
	assertLatencyNear(t, "P50", sla.P50Latency, 50*time.Millisecond, 5*time.Millisecond)
	// P95 should be around 95ms
	assertLatencyNear(t, "P95", sla.P95Latency, 95*time.Millisecond, 5*time.Millisecond)
	// P99 should be around 99ms
	assertLatencyNear(t, "P99", sla.P99Latency, 99*time.Millisecond, 5*time.Millisecond)
}

func assertLatencyNear(t *testing.T, label string, actual, expected, tolerance time.Duration) {
	t.Helper()
	diff := actual - expected
	if diff < 0 {
		diff = -diff
	}
	if diff > tolerance {
		t.Errorf("%s: expected ~%v, got %v (tolerance %v)", label, expected, actual, tolerance)
	}
}

// --- TestSLACollector_SuccessRate (sliding window) --------------------------

func TestSLACollector_SuccessRate(t *testing.T) {
	c := newTestCollector(t)

	// Fill the window with 100 successes.
	for i := 0; i < 100; i++ {
		c.RecordExecution("window_tool", time.Millisecond, nil)
	}
	sla := c.GetSLA("window_tool")
	if sla.SuccessRate != 1.0 {
		t.Fatalf("expected SuccessRate=1.0, got %f", sla.SuccessRate)
	}

	// Add 50 errors — the window should slide, keeping 50 old successes + 50 new errors.
	for i := 0; i < 50; i++ {
		c.RecordExecution("window_tool", time.Millisecond, fmt.Errorf("fail"))
	}
	sla = c.GetSLA("window_tool")
	if math.Abs(sla.SuccessRate-0.5) > 0.01 {
		t.Fatalf("expected SuccessRate~0.5, got %f", sla.SuccessRate)
	}

	// Add 50 more errors — now the window is 100 errors.
	for i := 0; i < 50; i++ {
		c.RecordExecution("window_tool", time.Millisecond, fmt.Errorf("fail"))
	}
	sla = c.GetSLA("window_tool")
	if sla.SuccessRate != 0.0 {
		t.Fatalf("expected SuccessRate=0.0, got %f", sla.SuccessRate)
	}

	// Total call count should be 200 even though the window only holds 100.
	if sla.CallCount != 200 {
		t.Fatalf("expected CallCount=200, got %d", sla.CallCount)
	}
}

// --- TestSLAExecutor_RecordsMetrics -----------------------------------------

func TestSLAExecutor_RecordsMetrics(t *testing.T) {
	c := newTestCollector(t)

	stub := &stubToolExecutor{name: "exec_test", sleepMs: 10}
	exec := NewSLAExecutor(stub, c)

	call := ports.ToolCall{ID: "call-1", Name: "exec_test"}
	result, err := exec.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "ok" {
		t.Fatalf("expected content 'ok', got %q", result.Content)
	}

	sla := c.GetSLA("exec_test")
	if sla.CallCount != 1 {
		t.Fatalf("expected CallCount=1, got %d", sla.CallCount)
	}
	if sla.SuccessRate != 1.0 {
		t.Fatalf("expected SuccessRate=1.0, got %f", sla.SuccessRate)
	}
	if sla.P50Latency < 10*time.Millisecond {
		t.Errorf("expected P50 >= 10ms, got %v", sla.P50Latency)
	}

	// Verify Definition/Metadata delegation.
	if exec.Definition().Name != "exec_test" {
		t.Fatalf("expected Definition().Name='exec_test', got %q", exec.Definition().Name)
	}
	if exec.Metadata().Name != "exec_test" {
		t.Fatalf("expected Metadata().Name='exec_test', got %q", exec.Metadata().Name)
	}
}

func TestSLAExecutor_RecordsErrors(t *testing.T) {
	c := newTestCollector(t)

	stub := &stubToolExecutor{
		name: "err_tool",
		execFn: func(_ context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
			return nil, fmt.Errorf("boom")
		},
	}
	exec := NewSLAExecutor(stub, c)

	call := ports.ToolCall{ID: "call-err", Name: "err_tool"}
	_, err := exec.Execute(context.Background(), call)
	if err == nil {
		t.Fatalf("expected error")
	}

	sla := c.GetSLA("err_tool")
	if sla.CallCount != 1 {
		t.Fatalf("expected CallCount=1, got %d", sla.CallCount)
	}
	if sla.SuccessRate != 0.0 {
		t.Fatalf("expected SuccessRate=0.0, got %f", sla.SuccessRate)
	}
}

func TestSLAExecutor_RecordsResultError(t *testing.T) {
	c := newTestCollector(t)

	stub := &stubToolExecutor{
		name: "result_err_tool",
		execFn: func(_ context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
			return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("result error")}, nil
		},
	}
	exec := NewSLAExecutor(stub, c)

	call := ports.ToolCall{ID: "call-re", Name: "result_err_tool"}
	result, err := exec.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected Go-level error: %v", err)
	}
	if result.Error == nil {
		t.Fatalf("expected result error")
	}

	sla := c.GetSLA("result_err_tool")
	if sla.SuccessRate != 0.0 {
		t.Fatalf("expected SuccessRate=0.0 for result-error, got %f", sla.SuccessRate)
	}
}

func TestSLAExecutor_RecordsCostFromMetadata(t *testing.T) {
	c := newTestCollector(t)

	stub := &stubToolExecutor{
		name: "cost_exec_tool",
		execFn: func(_ context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
			return &ports.ToolResult{
				CallID:   call.ID,
				Content:  "ok",
				Metadata: map[string]any{"cost_usd": 1.25},
			}, nil
		},
	}
	exec := NewSLAExecutor(stub, c)

	call := ports.ToolCall{ID: "call-cost", Name: "cost_exec_tool"}
	_, err := exec.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sla := c.GetSLA("cost_exec_tool")
	if sla.CostUSDTotal != 1.25 {
		t.Fatalf("expected CostUSDTotal=1.25, got %f", sla.CostUSDTotal)
	}
	if sla.CostUSDAvg != 1.25 {
		t.Fatalf("expected CostUSDAvg=1.25, got %f", sla.CostUSDAvg)
	}
}

// --- TestSLACollector_Nil ---------------------------------------------------

func TestSLACollector_Nil(t *testing.T) {
	// Nil collector: RecordExecution and GetSLA must not panic.
	var c *SLACollector
	c.RecordExecution("test", time.Second, nil)
	sla := c.GetSLA("test")
	if sla.ToolName != "test" {
		t.Fatalf("expected ToolName='test', got %q", sla.ToolName)
	}
	if sla.CallCount != 0 {
		t.Fatalf("expected CallCount=0, got %d", sla.CallCount)
	}
}

func TestSLAExecutor_NilCollector(t *testing.T) {
	// SLAExecutor with nil collector should pass-through without recording.
	stub := &stubToolExecutor{name: "passthrough"}
	exec := NewSLAExecutor(stub, nil)

	call := ports.ToolCall{ID: "call-nil", Name: "passthrough"}
	result, err := exec.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "ok" {
		t.Fatalf("expected content 'ok', got %q", result.Content)
	}
}

// --- TestSLACollector_Concurrent --------------------------------------------

func TestSLACollector_Concurrent(t *testing.T) {
	c := newTestCollector(t)

	const goroutines = 20
	const callsPerGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			toolName := fmt.Sprintf("tool_%d", id%5) // 5 distinct tools
			for i := 0; i < callsPerGoroutine; i++ {
				var err error
				if i%7 == 0 {
					err = fmt.Errorf("transient")
				}
				c.RecordExecution(toolName, time.Duration(i)*time.Millisecond, err)
			}
		}(g)
	}

	// Concurrently read SLA snapshots.
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				for i := 0; i < 5; i++ {
					_ = c.GetSLA(fmt.Sprintf("tool_%d", i))
				}
			}
		}
	}()

	wg.Wait()
	close(done)

	// Verify total call count across all 5 tools.
	var total int64
	for i := 0; i < 5; i++ {
		sla := c.GetSLA(fmt.Sprintf("tool_%d", i))
		total += sla.CallCount
	}
	expected := int64(goroutines * callsPerGoroutine)
	if total != expected {
		t.Fatalf("expected total CallCount=%d, got %d", expected, total)
	}
}

// --- TestSLACollector_UnknownTool -------------------------------------------

func TestSLACollector_UnknownTool(t *testing.T) {
	c := newTestCollector(t)

	sla := c.GetSLA("nonexistent")
	if sla.ToolName != "nonexistent" {
		t.Fatalf("expected ToolName='nonexistent', got %q", sla.ToolName)
	}
	if sla.CallCount != 0 {
		t.Fatalf("expected CallCount=0 for unknown tool, got %d", sla.CallCount)
	}
	if sla.SuccessRate != 0 {
		t.Fatalf("expected SuccessRate=0 for unknown tool, got %f", sla.SuccessRate)
	}
}

// --- TestSLACollector_ErrorClassification -----------------------------------

func TestSLACollector_ErrorClassification(t *testing.T) {
	tests := []struct {
		err      error
		expected string
	}{
		{fmt.Errorf("request timeout"), "timeout"},
		{fmt.Errorf("context deadline exceeded"), "timeout"},
		{fmt.Errorf("context canceled"), "canceled"},
		{fmt.Errorf("permission denied"), "permission"},
		{fmt.Errorf("operation rejected"), "permission"},
		{fmt.Errorf("resource not found"), "not_found"},
		{fmt.Errorf("something random"), "unknown"},
		{nil, "none"},
	}

	for _, tt := range tests {
		got := classifyError(tt.err)
		if got != tt.expected {
			errMsg := "<nil>"
			if tt.err != nil {
				errMsg = tt.err.Error()
			}
			t.Errorf("classifyError(%q) = %q, want %q", errMsg, got, tt.expected)
		}
	}
}

// --- TestSLAExecutor_Delegate -----------------------------------------------

func TestSLAExecutor_Delegate(t *testing.T) {
	stub := &stubToolExecutor{name: "inner"}
	c := newTestCollector(t)
	exec := NewSLAExecutor(stub, c)

	if exec.Delegate() != stub {
		t.Fatalf("expected Delegate() to return the inner executor")
	}
}

// --- TestSLAExecutor_UsesCallNameOverMetadataName ---------------------------

func TestSLAExecutor_UsesCallNameOverMetadataName(t *testing.T) {
	c := newTestCollector(t)
	stub := &stubToolExecutor{name: "meta_name"}
	exec := NewSLAExecutor(stub, c)

	// call.Name takes precedence over metadata name.
	call := ports.ToolCall{ID: "c1", Name: "call_name"}
	_, _ = exec.Execute(context.Background(), call)

	sla := c.GetSLA("call_name")
	if sla.CallCount != 1 {
		t.Fatalf("expected metric recorded under 'call_name', got CallCount=%d", sla.CallCount)
	}

	slaMeta := c.GetSLA("meta_name")
	if slaMeta.CallCount != 0 {
		t.Fatalf("expected no metric under 'meta_name', got CallCount=%d", slaMeta.CallCount)
	}
}

func TestSLAExecutor_FallsBackToMetadataName(t *testing.T) {
	c := newTestCollector(t)
	stub := &stubToolExecutor{name: "meta_tool"}
	exec := NewSLAExecutor(stub, c)

	// Empty call name should fall back to metadata name.
	call := ports.ToolCall{ID: "c2", Name: ""}
	_, _ = exec.Execute(context.Background(), call)

	sla := c.GetSLA("meta_tool")
	if sla.CallCount != 1 {
		t.Fatalf("expected metric recorded under 'meta_tool', got CallCount=%d", sla.CallCount)
	}
}

func TestNewSLACollector_RepeatedRegisterSameRegistryDoesNotPanic(t *testing.T) {
	reg := prometheus.NewRegistry()
	first := NewSLACollector(reg)
	if first == nil {
		t.Fatal("expected first collector to be created")
	}

	second := NewSLACollector(reg)
	if second == nil {
		t.Fatal("expected second collector to be created")
	}
}
