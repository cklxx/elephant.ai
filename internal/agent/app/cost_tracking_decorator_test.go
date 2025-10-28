package app

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"alex/internal/agent/ports"
)

// mockLLMClient implements ports.LLMClient for testing
type mockLLMClient struct {
	model         string
	callCount     int
	mu            sync.Mutex
	responseDelay time.Duration
}

func newMockLLMClient(model string) *mockLLMClient {
	return &mockLLMClient{
		model: model,
	}
}

func (m *mockLLMClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	m.mu.Lock()
	m.callCount++
	m.mu.Unlock()

	// Simulate API latency if configured
	if m.responseDelay > 0 {
		time.Sleep(m.responseDelay)
	}

	return &ports.CompletionResponse{
		Content:    "mock response",
		StopReason: "end_turn",
		Usage: ports.TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
	}, nil
}

func (m *mockLLMClient) Model() string {
	return m.model
}

func (m *mockLLMClient) GetCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// mockCostTracker implements ports.CostTracker with thread-safe storage
type mockCostTracker struct {
	records []ports.UsageRecord
	mu      sync.Mutex
}

func newMockCostTracker() *mockCostTracker {
	return &mockCostTracker{
		records: make([]ports.UsageRecord, 0),
	}
}

func (m *mockCostTracker) RecordUsage(ctx context.Context, usage ports.UsageRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records = append(m.records, usage)
	return nil
}

func (m *mockCostTracker) GetSessionCost(ctx context.Context, sessionID string) (*ports.CostSummary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	summary := &ports.CostSummary{
		ByModel:    make(map[string]float64),
		ByProvider: make(map[string]float64),
	}

	for _, r := range m.records {
		if r.SessionID == sessionID {
			summary.TotalCost += r.TotalCost
			summary.InputTokens += r.InputTokens
			summary.OutputTokens += r.OutputTokens
			summary.TotalTokens += r.TotalTokens
			summary.RequestCount++
		}
	}

	return summary, nil
}

func (m *mockCostTracker) GetSessionStats(ctx context.Context, sessionID string) (*ports.SessionStats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	stats := &ports.SessionStats{
		SessionID:  sessionID,
		ByModel:    make(map[string]float64),
		ByProvider: make(map[string]float64),
	}

	var firstTime, lastTime time.Time
	for _, r := range m.records {
		if r.SessionID == sessionID {
			stats.TotalCost += r.TotalCost
			stats.InputTokens += r.InputTokens
			stats.OutputTokens += r.OutputTokens
			stats.TotalTokens += r.TotalTokens
			stats.RequestCount++

			if firstTime.IsZero() || r.Timestamp.Before(firstTime) {
				firstTime = r.Timestamp
			}
			if lastTime.IsZero() || r.Timestamp.After(lastTime) {
				lastTime = r.Timestamp
			}
		}
	}

	if stats.RequestCount > 0 {
		stats.FirstRequest = firstTime
		stats.LastRequest = lastTime
		stats.Duration = lastTime.Sub(firstTime)
	}

	return stats, nil
}

func (m *mockCostTracker) GetDailyCost(ctx context.Context, date time.Time) (*ports.CostSummary, error) {
	return nil, nil
}

func (m *mockCostTracker) GetMonthlyCost(ctx context.Context, year int, month int) (*ports.CostSummary, error) {
	return nil, nil
}

func (m *mockCostTracker) GetDateRangeCost(ctx context.Context, start, end time.Time) (*ports.CostSummary, error) {
	return nil, nil
}

func (m *mockCostTracker) Export(ctx context.Context, format ports.ExportFormat, filter ports.ExportFilter) ([]byte, error) {
	return nil, nil
}

func (m *mockCostTracker) GetRecordsBySession(sessionID string) []ports.UsageRecord {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []ports.UsageRecord
	for _, r := range m.records {
		if r.SessionID == sessionID {
			result = append(result, r)
		}
	}
	return result
}

func (m *mockCostTracker) GetAllRecords() []ports.UsageRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]ports.UsageRecord{}, m.records...)
}

// mockLogger implements ports.Logger for testing
type mockLogger struct {
	messages []string
	mu       sync.Mutex
}

func newMockLogger() *mockLogger {
	return &mockLogger{
		messages: make([]string, 0),
	}
}

func (m *mockLogger) Debug(format string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, fmt.Sprintf(format, args...))
}

func (m *mockLogger) Info(format string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, fmt.Sprintf(format, args...))
}

func (m *mockLogger) Warn(format string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, fmt.Sprintf(format, args...))
}

func (m *mockLogger) Error(format string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, fmt.Sprintf(format, args...))
}

func (m *mockLogger) GetMessages() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.messages...)
}

// mockClock implements ports.Clock for deterministic time in tests
type mockClock struct {
	currentTime time.Time
	mu          sync.Mutex
}

func newMockClock(t time.Time) *mockClock {
	return &mockClock{
		currentTime: t,
	}
}

func (m *mockClock) Now() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.currentTime
}

func (m *mockClock) Advance(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentTime = m.currentTime.Add(d)
}

// TestWrapReturnsIsolatedWrapper verifies that Wrap() creates independent wrappers
func TestWrapReturnsIsolatedWrapper(t *testing.T) {
	t.Parallel()

	tracker := newMockCostTracker()
	logger := newMockLogger()
	clock := newMockClock(time.Now())
	decorator := NewCostTrackingDecorator(tracker, logger, clock)

	baseClient := newMockLLMClient("gpt-4o")
	ctx := context.Background()

	// Create two wrappers from the same base client
	wrapper1 := decorator.Wrap(ctx, "session-1", baseClient)
	wrapper2 := decorator.Wrap(ctx, "session-2", baseClient)

	// Verify they are different instances
	if wrapper1 == wrapper2 {
		t.Fatal("Wrap should return different wrapper instances")
	}

	// Verify they wrap the same base client
	if wrapper1.Model() != wrapper2.Model() {
		t.Errorf("Both wrappers should have same model, got %s and %s", wrapper1.Model(), wrapper2.Model())
	}
}

// TestWrapWithNilTracker verifies that wrapping without a tracker returns the original client
func TestWrapWithNilTracker(t *testing.T) {
	t.Parallel()

	decorator := NewCostTrackingDecorator(nil, nil, nil)
	baseClient := newMockLLMClient("gpt-4o")
	ctx := context.Background()

	wrapped := decorator.Wrap(ctx, "session-1", baseClient)

	// Should return the original client when tracker is nil
	if wrapped != baseClient {
		t.Error("Wrap with nil tracker should return original client")
	}
}

// TestConcurrentSessionsIsolation tests that multiple concurrent sessions have isolated cost tracking
func TestConcurrentSessionsIsolation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		sessions     []string
		callsPerSess int
	}{
		{
			name:         "2 sessions with 5 calls each",
			sessions:     []string{"session-1", "session-2"},
			callsPerSess: 5,
		},
		{
			name:         "3 sessions with 3 calls each",
			sessions:     []string{"session-a", "session-b", "session-c"},
			callsPerSess: 3,
		},
		{
			name:         "5 sessions with 10 calls each",
			sessions:     []string{"s1", "s2", "s3", "s4", "s5"},
			callsPerSess: 10,
		},
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tracker := newMockCostTracker()
			logger := newMockLogger()
			clock := newMockClock(time.Now())
			decorator := NewCostTrackingDecorator(tracker, logger, clock)

			// Shared base client
			baseClient := newMockLLMClient("gpt-4o")

			// Create wrapped clients for each session
			clients := make(map[string]ports.LLMClient)
			for _, sessionID := range tt.sessions {
				clients[sessionID] = decorator.Wrap(context.Background(), sessionID, baseClient)
			}

			// Execute calls concurrently
			var wg sync.WaitGroup
			for sessionID, client := range clients {
				wg.Add(1)
				go func(sid string, c ports.LLMClient) {
					defer wg.Done()
					ctx := context.Background()
					req := ports.CompletionRequest{
						Messages: []ports.Message{{Role: "user", Content: "test"}},
					}

					for i := 0; i < tt.callsPerSess; i++ {
						_, err := c.Complete(ctx, req)
						if err != nil {
							t.Errorf("Complete failed for session %s: %v", sid, err)
						}
					}
				}(sessionID, client)
			}

			wg.Wait()

			// Verify cost isolation - each session should have exactly callsPerSess records
			for _, sessionID := range tt.sessions {
				records := tracker.GetRecordsBySession(sessionID)
				if len(records) != tt.callsPerSess {
					t.Errorf("Session %s: expected %d records, got %d", sessionID, tt.callsPerSess, len(records))
				}

				// Verify all records belong to the correct session
				for _, record := range records {
					if record.SessionID != sessionID {
						t.Errorf("Found record with wrong session ID: expected %s, got %s", sessionID, record.SessionID)
					}
				}
			}

			// Verify total record count
			allRecords := tracker.GetAllRecords()
			expectedTotal := len(tt.sessions) * tt.callsPerSess
			if len(allRecords) != expectedTotal {
				t.Errorf("Expected %d total records, got %d", expectedTotal, len(allRecords))
			}
		})
	}
}

// TestConcurrentCompleteCallsTrackCostsCorrectly verifies concurrent Complete() calls
func TestConcurrentCompleteCallsTrackCostsCorrectly(t *testing.T) {
	t.Parallel()

	tracker := newMockCostTracker()
	logger := newMockLogger()
	clock := newMockClock(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC))
	decorator := NewCostTrackingDecorator(tracker, logger, clock)

	baseClient := newMockLLMClient("gpt-4o")
	ctx := context.Background()

	session1Client := decorator.Wrap(ctx, "session-1", baseClient)
	session2Client := decorator.Wrap(ctx, "session-2", baseClient)

	// Execute 50 calls from each session concurrently
	numCalls := 50
	var wg sync.WaitGroup

	for i := 0; i < numCalls; i++ {
		wg.Add(2)

		// Session 1 calls
		go func() {
			defer wg.Done()
			req := ports.CompletionRequest{
				Messages: []ports.Message{{Role: "user", Content: "test"}},
			}
			_, _ = session1Client.Complete(ctx, req)
		}()

		// Session 2 calls
		go func() {
			defer wg.Done()
			req := ports.CompletionRequest{
				Messages: []ports.Message{{Role: "user", Content: "test"}},
			}
			_, _ = session2Client.Complete(ctx, req)
		}()
	}

	wg.Wait()

	// Verify session 1
	summary1, err := tracker.GetSessionCost(ctx, "session-1")
	if err != nil {
		t.Fatalf("Failed to get session-1 cost: %v", err)
	}
	if summary1.RequestCount != numCalls {
		t.Errorf("Session 1: expected %d requests, got %d", numCalls, summary1.RequestCount)
	}
	if summary1.TotalTokens != numCalls*150 { // 150 tokens per call
		t.Errorf("Session 1: expected %d tokens, got %d", numCalls*150, summary1.TotalTokens)
	}

	// Verify session 2
	summary2, err := tracker.GetSessionCost(ctx, "session-2")
	if err != nil {
		t.Fatalf("Failed to get session-2 cost: %v", err)
	}
	if summary2.RequestCount != numCalls {
		t.Errorf("Session 2: expected %d requests, got %d", numCalls, summary2.RequestCount)
	}
	if summary2.TotalTokens != numCalls*150 {
		t.Errorf("Session 2: expected %d tokens, got %d", numCalls*150, summary2.TotalTokens)
	}

	// Verify no cross-contamination
	records1 := tracker.GetRecordsBySession("session-1")
	records2 := tracker.GetRecordsBySession("session-2")

	if len(records1) != numCalls {
		t.Errorf("Session 1: expected %d records, got %d", numCalls, len(records1))
	}
	if len(records2) != numCalls {
		t.Errorf("Session 2: expected %d records, got %d", numCalls, len(records2))
	}

	// Ensure all records have the correct session ID
	for _, r := range records1 {
		if r.SessionID != "session-1" {
			t.Error("Found session-2 record in session-1 records")
		}
	}
	for _, r := range records2 {
		if r.SessionID != "session-2" {
			t.Error("Found session-1 record in session-2 records")
		}
	}
}

// TestCostRecordFields verifies that cost records contain correct data
func TestCostRecordFields(t *testing.T) {
	t.Parallel()

	tracker := newMockCostTracker()
	logger := newMockLogger()
	fixedTime := time.Date(2025, 1, 15, 14, 30, 0, 0, time.UTC)
	clock := newMockClock(fixedTime)
	decorator := NewCostTrackingDecorator(tracker, logger, clock)

	baseClient := newMockLLMClient("gpt-4o")
	ctx := context.Background()

	wrappedClient := decorator.Wrap(ctx, "test-session", baseClient)

	// Execute a single call
	req := ports.CompletionRequest{
		Messages: []ports.Message{{Role: "user", Content: "test"}},
	}
	_, err := wrappedClient.Complete(ctx, req)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	// Get the recorded usage
	records := tracker.GetRecordsBySession("test-session")
	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	record := records[0]

	// Verify fields
	if record.SessionID != "test-session" {
		t.Errorf("SessionID: expected 'test-session', got '%s'", record.SessionID)
	}
	if record.Model != "gpt-4o" {
		t.Errorf("Model: expected 'gpt-4o', got '%s'", record.Model)
	}
	if record.Provider != "openai" { // inferred from model name
		t.Errorf("Provider: expected 'openai', got '%s'", record.Provider)
	}
	if record.InputTokens != 100 {
		t.Errorf("InputTokens: expected 100, got %d", record.InputTokens)
	}
	if record.OutputTokens != 50 {
		t.Errorf("OutputTokens: expected 50, got %d", record.OutputTokens)
	}
	if record.TotalTokens != 150 {
		t.Errorf("TotalTokens: expected 150, got %d", record.TotalTokens)
	}
	if record.TotalCost == 0 {
		t.Error("TotalCost should be calculated and non-zero")
	}
	if !record.Timestamp.Equal(fixedTime) {
		t.Errorf("Timestamp: expected %v, got %v", fixedTime, record.Timestamp)
	}

	// Verify cost calculation for gpt-4o
	expectedInputCost := float64(100) / 1000.0 * 0.005 // $0.005 per 1K input tokens
	expectedOutputCost := float64(50) / 1000.0 * 0.015 // $0.015 per 1K output tokens
	expectedTotalCost := expectedInputCost + expectedOutputCost

	tolerance := 0.000001
	if absFloat(record.InputCost-expectedInputCost) > tolerance {
		t.Errorf("InputCost: expected %f, got %f", expectedInputCost, record.InputCost)
	}
	if absFloat(record.OutputCost-expectedOutputCost) > tolerance {
		t.Errorf("OutputCost: expected %f, got %f", expectedOutputCost, record.OutputCost)
	}
	if absFloat(record.TotalCost-expectedTotalCost) > tolerance {
		t.Errorf("TotalCost: expected %f, got %f", expectedTotalCost, record.TotalCost)
	}
}

// absFloat returns the absolute value of a float64
func absFloat(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// TestWrapperDelegatesModelCorrectly verifies Model() delegation
func TestWrapperDelegatesModelCorrectly(t *testing.T) {
	t.Parallel()

	models := []string{"gpt-4o", "gpt-4o-mini", "claude-3-5-sonnet", "deepseek-chat"}

	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			tracker := newMockCostTracker()
			decorator := NewCostTrackingDecorator(tracker, nil, nil)

			baseClient := newMockLLMClient(model)
			wrapped := decorator.Wrap(context.Background(), "session", baseClient)

			if wrapped.Model() != model {
				t.Errorf("Expected model %s, got %s", model, wrapped.Model())
			}
		})
	}
}

// TestNoRaceConditionsUnderLoad is a stress test for race conditions
func TestNoRaceConditionsUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	t.Parallel()

	tracker := newMockCostTracker()
	logger := newMockLogger()
	clock := newMockClock(time.Now())
	decorator := NewCostTrackingDecorator(tracker, logger, clock)

	baseClient := newMockLLMClient("gpt-4o")

	// Create 10 sessions
	numSessions := 10
	sessions := make([]string, numSessions)
	clients := make([]ports.LLMClient, numSessions)
	for i := 0; i < numSessions; i++ {
		sessionID := fmt.Sprintf("stress-session-%d", i)
		sessions[i] = sessionID
		clients[i] = decorator.Wrap(context.Background(), sessionID, baseClient)
	}

	// Each session makes 100 concurrent calls
	callsPerSession := 100
	var wg sync.WaitGroup

	for i := 0; i < numSessions; i++ {
		for j := 0; j < callsPerSession; j++ {
			wg.Add(1)
			go func(client ports.LLMClient) {
				defer wg.Done()
				req := ports.CompletionRequest{
					Messages: []ports.Message{{Role: "user", Content: "stress test"}},
				}
				_, _ = client.Complete(context.Background(), req)
			}(clients[i])
		}
	}

	wg.Wait()

	// Verify each session has exactly the right number of records
	for i, sessionID := range sessions {
		records := tracker.GetRecordsBySession(sessionID)
		if len(records) != callsPerSession {
			t.Errorf("Session %d (%s): expected %d records, got %d",
				i, sessionID, callsPerSession, len(records))
		}
	}

	// Verify total
	allRecords := tracker.GetAllRecords()
	expectedTotal := numSessions * callsPerSession
	if len(allRecords) != expectedTotal {
		t.Errorf("Expected %d total records, got %d", expectedTotal, len(allRecords))
	}
}

// TestInferProvider verifies provider inference logic
func TestInferProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		model    string
		expected string
	}{
		{"gpt-4", "openai"},
		{"gpt-4o", "openai"},
		{"gpt-4o-mini", "openai"},
		{"claude-3-5-sonnet", "anthropic"},
		{"claude-3-opus", "anthropic"},
		{"deepseek-chat", "deepseek"},
		{"deepseek-reasoner", "deepseek"},
		{"unknown-model", "openrouter"}, // defaults to openrouter for unknown models
		{"ab", "openrouter"},            // defaults to openrouter
		{"", "openrouter"},              // defaults to openrouter
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := inferProvider(tt.model)
			if result != tt.expected {
				t.Errorf("inferProvider(%q) = %q, want %q", tt.model, result, tt.expected)
			}
		})
	}
}

// TestWrapperHandlesClientErrors verifies error propagation
func TestWrapperHandlesClientErrors(t *testing.T) {
	t.Parallel()

	tracker := newMockCostTracker()
	decorator := NewCostTrackingDecorator(tracker, nil, nil)

	// Create a client that returns errors
	errorClient := &errorLLMClient{model: "gpt-4o"}
	wrapped := decorator.Wrap(context.Background(), "error-session", errorClient)

	req := ports.CompletionRequest{
		Messages: []ports.Message{{Role: "user", Content: "test"}},
	}
	_, err := wrapped.Complete(context.Background(), req)

	// Should propagate error
	if err == nil {
		t.Error("Expected error from client, got nil")
	}

	// Should not record usage on error
	records := tracker.GetRecordsBySession("error-session")
	if len(records) != 0 {
		t.Errorf("Should not record usage on error, got %d records", len(records))
	}
}

// errorLLMClient always returns errors
type errorLLMClient struct {
	model string
}

func (e *errorLLMClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	return nil, fmt.Errorf("simulated error")
}

func (e *errorLLMClient) Model() string {
	return e.model
}

// TestWrapperWithNilResponse verifies handling of nil responses
func TestWrapperWithNilResponse(t *testing.T) {
	t.Parallel()

	tracker := newMockCostTracker()
	decorator := NewCostTrackingDecorator(tracker, nil, nil)

	nilClient := &nilResponseClient{model: "gpt-4o"}
	wrapped := decorator.Wrap(context.Background(), "nil-session", nilClient)

	req := ports.CompletionRequest{
		Messages: []ports.Message{{Role: "user", Content: "test"}},
	}
	_, err := wrapped.Complete(context.Background(), req)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Should not record usage when response is nil
	records := tracker.GetRecordsBySession("nil-session")
	if len(records) != 0 {
		t.Errorf("Should not record usage for nil response, got %d records", len(records))
	}
}

// nilResponseClient returns nil response without error
type nilResponseClient struct {
	model string
}

func (n *nilResponseClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	return nil, nil
}

func (n *nilResponseClient) Model() string {
	return n.model
}

// TestNewCostTrackingDecoratorDefaults verifies default values
func TestNewCostTrackingDecoratorDefaults(t *testing.T) {
	t.Parallel()

	tracker := newMockCostTracker()

	// Create decorator with nil logger and clock
	decorator := NewCostTrackingDecorator(tracker, nil, nil)

	if decorator.tracker != tracker {
		t.Error("Tracker should be set")
	}
	if decorator.logger == nil {
		t.Error("Logger should be set to NoopLogger when nil")
	}
	if decorator.clock == nil {
		t.Error("Clock should be set to SystemClock when nil")
	}
}

// Benchmark for concurrent wrapper creation
func BenchmarkWrapperCreation(b *testing.B) {
	tracker := newMockCostTracker()
	decorator := NewCostTrackingDecorator(tracker, nil, nil)
	baseClient := newMockLLMClient("gpt-4o")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = decorator.Wrap(context.Background(), fmt.Sprintf("session-%d", i), baseClient)
	}
}

// Benchmark for concurrent Complete calls
func BenchmarkConcurrentComplete(b *testing.B) {
	tracker := newMockCostTracker()
	decorator := NewCostTrackingDecorator(tracker, nil, nil)
	baseClient := newMockLLMClient("gpt-4o")

	client1 := decorator.Wrap(context.Background(), "session-1", baseClient)
	client2 := decorator.Wrap(context.Background(), "session-2", baseClient)

	req := ports.CompletionRequest{
		Messages: []ports.Message{{Role: "user", Content: "benchmark"}},
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := context.Background()
		i := 0
		for pb.Next() {
			client := client1
			if i%2 == 0 {
				client = client2
			}
			_, _ = client.Complete(ctx, req)
			i++
		}
	})
}
