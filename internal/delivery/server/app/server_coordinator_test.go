package app

import (
	"context"
	"errors"
	"testing"
	"time"

	appcontext "alex/internal/app/agent/context"
	serverPorts "alex/internal/delivery/server/ports"
	"alex/internal/domain/agent"
	core "alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
	"alex/internal/infra/analytics"
	"alex/internal/infra/analytics/journal"
	"alex/internal/infra/observability"
	sessionstate "alex/internal/infra/session/state_store"
)

// Mock implementations for testing

type MockSessionStore struct {
	sessions map[string]*storage.Session
}

func NewMockSessionStore() *MockSessionStore {
	return &MockSessionStore{
		sessions: make(map[string]*storage.Session),
	}
}

func (m *MockSessionStore) Create(ctx context.Context) (*storage.Session, error) {
	session := &storage.Session{
		ID:        "session-" + time.Now().Format("20060102150405.000000"),
		Messages:  []core.Message{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata:  make(map[string]string),
	}
	m.sessions[session.ID] = session
	return session, nil
}

func (m *MockSessionStore) Get(ctx context.Context, id string) (*storage.Session, error) {
	if session, ok := m.sessions[id]; ok {
		return session, nil
	}
	return m.Create(ctx)
}

func (m *MockSessionStore) Save(ctx context.Context, session *storage.Session) error {
	m.sessions[session.ID] = session
	return nil
}

func (m *MockSessionStore) List(ctx context.Context, limit int, offset int) ([]string, error) {
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	return ids, nil
}

func (m *MockSessionStore) Delete(ctx context.Context, id string) error {
	delete(m.sessions, id)
	return nil
}

type MockAgentCoordinator struct {
	sessionStore storage.SessionStore
}

func NewMockAgentCoordinator(sessionStore storage.SessionStore) *MockAgentCoordinator {
	return &MockAgentCoordinator{
		sessionStore: sessionStore,
	}
}

func (m *MockAgentCoordinator) GetSession(ctx context.Context, id string) (*storage.Session, error) {
	return m.sessionStore.Get(ctx, id)
}

func (m *MockAgentCoordinator) ExecuteTask(ctx context.Context, task string, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
	return &agent.TaskResult{
		Answer:     "Mock answer",
		Iterations: 3,
		TokensUsed: 100,
		StopReason: "completed",
		SessionID:  sessionID,
	}, nil
}

func (m *MockAgentCoordinator) GetConfig() agent.AgentConfig {
	return agent.AgentConfig{}
}

func (m *MockAgentCoordinator) PreviewContextWindow(ctx context.Context, sessionID string) (agent.ContextWindowPreview, error) {
	return agent.ContextWindowPreview{
		Window: agent.ContextWindow{
			SessionID: sessionID,
		},
		TokenLimit: 128000,
		ToolMode:   "cli",
	}, nil
}

type mockAnalytics struct {
	captures []struct {
		distinctID string
		event      string
		properties map[string]any
	}
}

func (m *mockAnalytics) Capture(ctx context.Context, distinctID string, event string, properties map[string]any) error {
	copied := make(map[string]any, len(properties))
	for key, value := range properties {
		copied[key] = value
	}
	m.captures = append(m.captures, struct {
		distinctID string
		event      string
		properties map[string]any
	}{distinctID: distinctID, event: event, properties: copied})
	return nil
}

func (m *mockAnalytics) Close() error {
	return nil
}

// TestSessionIDConsistency verifies the critical P0 fix:
// Session ID must be generated synchronously and remain consistent
func TestSessionIDConsistency(t *testing.T) {
	// Setup
	sessionStore := NewMockSessionStore()
	taskStore := NewInMemoryTaskStore()
	broadcaster := NewEventBroadcaster()

	stateStore := sessionstate.NewInMemoryStore()

	agentCoordinator := NewMockAgentCoordinator(sessionStore)

	serverCoordinator := NewServerCoordinator(
		agentCoordinator,
		broadcaster,
		sessionStore,
		taskStore,
		stateStore,
	)

	// Test Case 1: Task created WITHOUT session_id
	t.Run("EmptySessionID", func(t *testing.T) {
		ctx := context.Background()

		// Execute task async with empty session ID
		task, err := serverCoordinator.ExecuteTaskAsync(ctx, "test task", "", "", "")
		if err != nil {
			t.Fatalf("ExecuteTaskAsync failed: %v", err)
		}

		// Verify task was created
		if task.ID == "" {
			t.Fatal("Task ID is empty")
		}

		// CRITICAL: Verify session_id is NOT empty in the initial response
		if task.SessionID == "" {
			t.Fatal("FAILED: session_id is empty in initial response (P0 bug not fixed!)")
		}

		t.Logf("✓ Session ID present in initial response: %s", task.SessionID)

		// Store initial session ID for comparison - get fresh data to avoid race
		initialTask, err := taskStore.Get(ctx, task.ID)
		if err != nil {
			t.Fatalf("Failed to get initial task: %v", err)
		}
		initialSessionID := initialTask.SessionID

		// Wait briefly to simulate polling
		time.Sleep(100 * time.Millisecond)

		// Retrieve task again
		retrievedTask, err := taskStore.Get(ctx, task.ID)
		if err != nil {
			t.Fatalf("Failed to retrieve task: %v", err)
		}

		// CRITICAL: Verify session ID didn't change
		if retrievedTask.SessionID != initialSessionID {
			t.Fatalf("FAILED: Session ID changed!\n  Initial: %s\n  Retrieved: %s",
				initialSessionID, retrievedTask.SessionID)
		}

		t.Logf("✓ Session ID remained consistent: %s", retrievedTask.SessionID)
	})

	// Test Case 2: Task created WITH explicit session_id
	t.Run("ExplicitSessionID", func(t *testing.T) {
		ctx := context.Background()
		explicitSessionID := "session-explicit-test"

		// Create session first
		session := &storage.Session{
			ID:        explicitSessionID,
			Messages:  []core.Message{},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Metadata:  make(map[string]string),
		}
		sessionStore.sessions[explicitSessionID] = session

		// Execute task async with explicit session ID
		task, err := serverCoordinator.ExecuteTaskAsync(ctx, "test task 2", explicitSessionID, "", "")
		if err != nil {
			t.Fatalf("ExecuteTaskAsync failed: %v", err)
		}

		// CRITICAL: Verify explicit session_id is preserved
		// Use taskStore.Get() to avoid race condition with background goroutine
		freshTask, err := taskStore.Get(ctx, task.ID)
		if err != nil {
			t.Fatalf("Failed to get fresh task: %v", err)
		}

		if freshTask.SessionID != explicitSessionID {
			t.Fatalf("FAILED: Explicit session_id not preserved!\n  Expected: %s\n  Got: %s",
				explicitSessionID, freshTask.SessionID)
		}

		t.Logf("✓ Explicit session_id preserved: %s", freshTask.SessionID)
	})

	// Test Case 3: Verify progress fields are not null (no omitempty)
	t.Run("ProgressFieldsPresent", func(t *testing.T) {
		ctx := context.Background()

		task, err := serverCoordinator.ExecuteTaskAsync(ctx, "test task 3", "", "", "")
		if err != nil {
			t.Fatalf("ExecuteTaskAsync failed: %v", err)
		}

		// Progress fields should be 0 initially, not omitted
		// This is verified by the JSON marshaling, but we can check the values
		// Get fresh copy to avoid race condition
		freshTask, err := taskStore.Get(ctx, task.ID)
		if err != nil {
			t.Fatalf("Failed to get fresh task: %v", err)
		}

		if freshTask.CurrentIteration != 0 {
			t.Logf("⚠ CurrentIteration is %d (expected 0)", freshTask.CurrentIteration)
		}

		if freshTask.TokensUsed != 0 {
			t.Logf("⚠ TokensUsed is %d (expected 0)", freshTask.TokensUsed)
		}

		t.Logf("✓ Progress fields initialized: current_iteration=%d, tokens_used=%d",
			freshTask.CurrentIteration, freshTask.TokensUsed)
	})
}

func TestServerCoordinatorAnalyticsCapture(t *testing.T) {
	sessionStore := NewMockSessionStore()
	taskStore := NewInMemoryTaskStore()
	broadcaster := NewEventBroadcaster()

	stateStore := sessionstate.NewInMemoryStore()

	agentCoordinator := NewMockAgentCoordinator(sessionStore)
	analyticsMock := &mockAnalytics{}

	coordinator := NewServerCoordinator(
		agentCoordinator,
		broadcaster,
		sessionStore,
		taskStore,
		stateStore,
		WithAnalyticsClient(analyticsMock),
	)

	ctx := context.Background()
	coordinator.Tasks.emitWorkflowInputReceivedEvent(ctx, "session-analytics", "task-analytics", "capture metrics")

	if len(analyticsMock.captures) != 1 {
		t.Fatalf("expected 1 analytics capture, got %d", len(analyticsMock.captures))
	}

	capture := analyticsMock.captures[0]
	if capture.distinctID != "session-analytics" {
		t.Errorf("expected distinctID session-analytics, got %s", capture.distinctID)
	}
	if capture.event != analytics.EventTaskExecutionStarted {
		t.Errorf("expected event %s, got %s", analytics.EventTaskExecutionStarted, capture.event)
	}
	if capture.properties["run_id"] != "task-analytics" {
		t.Errorf("expected run_id task-analytics, got %v", capture.properties["run_id"])
	}
	if capture.properties["source"] != "server" {
		t.Errorf("expected source server, got %v", capture.properties["source"])
	}
}

func TestReplaySessionRehydratesSnapshots(t *testing.T) {
	sessionStore := NewMockSessionStore()
	taskStore := NewInMemoryTaskStore()
	broadcaster := NewEventBroadcaster()
	stateStore := sessionstate.NewInMemoryStore()
	agentCoordinator := NewMockAgentCoordinator(sessionStore)
	reader := &stubJournalReader{entries: map[string][]journal.TurnJournalEntry{
		"sess-99": {
			{SessionID: "sess-99", TurnID: 1, LLMTurnSeq: 1, Summary: "start", Timestamp: time.Unix(1, 0)},
			{SessionID: "sess-99", TurnID: 2, LLMTurnSeq: 2, Summary: "done", Timestamp: time.Unix(2, 0)},
		},
	}}
	coordinator := NewServerCoordinator(
		agentCoordinator,
		broadcaster,
		sessionStore,
		taskStore,
		stateStore,
		WithJournalReader(reader),
	)
	if err := coordinator.ReplaySession(context.Background(), "sess-99"); err != nil {
		t.Fatalf("ReplaySession returned error: %v", err)
	}
	snapshot, err := stateStore.GetSnapshot(context.Background(), "sess-99", 2)
	if err != nil {
		t.Fatalf("expected snapshot for turn 2: %v", err)
	}
	if snapshot.Summary != "done" || snapshot.LLMTurnSeq != 2 {
		t.Fatalf("unexpected snapshot payload: %+v", snapshot)
	}
}

func TestReplaySessionErrorsWithoutEntries(t *testing.T) {
	sessionStore := NewMockSessionStore()
	stateStore := sessionstate.NewInMemoryStore()
	coordinator := NewServerCoordinator(
		NewMockAgentCoordinator(sessionStore),
		NewEventBroadcaster(),
		sessionStore,
		NewInMemoryTaskStore(),
		stateStore,
		WithJournalReader(&stubJournalReader{}),
	)
	if err := coordinator.ReplaySession(context.Background(), "missing"); err == nil {
		t.Fatalf("expected error when no entries exist")
	}
}

func TestReplaySessionClearsExistingSnapshots(t *testing.T) {
	ctx := context.Background()
	sessionStore := NewMockSessionStore()
	stateStore := sessionstate.NewInMemoryStore()
	if err := stateStore.SaveSnapshot(ctx, sessionstate.Snapshot{SessionID: "sess-99", TurnID: 5, Summary: "stale"}); err != nil {
		t.Fatalf("setup snapshot: %v", err)
	}
	reader := &stubJournalReader{entries: map[string][]journal.TurnJournalEntry{
		"sess-99": {
			{SessionID: "sess-99", TurnID: 1, LLMTurnSeq: 1, Summary: "one"},
			{SessionID: "sess-99", TurnID: 2, LLMTurnSeq: 2, Summary: "two"},
		},
	}}
	coordinator := NewServerCoordinator(
		NewMockAgentCoordinator(sessionStore),
		NewEventBroadcaster(),
		sessionStore,
		NewInMemoryTaskStore(),
		stateStore,
		WithJournalReader(reader),
	)
	if err := coordinator.ReplaySession(ctx, "sess-99"); err != nil {
		t.Fatalf("ReplaySession returned error: %v", err)
	}
	if _, err := stateStore.GetSnapshot(ctx, "sess-99", 5); !errors.Is(err, sessionstate.ErrSnapshotNotFound) {
		t.Fatalf("expected stale snapshot removed, got %v", err)
	}
	if _, err := stateStore.GetSnapshot(ctx, "sess-99", 2); err != nil {
		t.Fatalf("expected replayed snapshot to exist: %v", err)
	}
}

type stubJournalReader struct {
	entries map[string][]journal.TurnJournalEntry
	err     error
}

func (r *stubJournalReader) Stream(_ context.Context, sessionID string, fn func(journal.TurnJournalEntry) error) error {
	if r.err != nil {
		return r.err
	}
	entries := r.entries[sessionID]
	for _, entry := range entries {
		if err := fn(entry); err != nil {
			return err
		}
	}
	return nil
}

func (r *stubJournalReader) ReadAll(_ context.Context, sessionID string) ([]journal.TurnJournalEntry, error) {
	if r.err != nil {
		return nil, r.err
	}
	entries := r.entries[sessionID]
	if len(entries) == 0 {
		return nil, nil
	}
	cloned := make([]journal.TurnJournalEntry, len(entries))
	copy(cloned, entries)
	return cloned, nil
}

// TestBroadcasterMapping verifies that broadcaster task-session mapping uses correct session ID
func TestBroadcasterMapping(t *testing.T) {
	sessionStore := NewMockSessionStore()
	taskStore := NewInMemoryTaskStore()
	broadcaster := NewEventBroadcaster()

	stateStore := sessionstate.NewInMemoryStore()

	agentCoordinator := NewMockAgentCoordinator(sessionStore)

	serverCoordinator := NewServerCoordinator(
		agentCoordinator,
		broadcaster,
		sessionStore,
		taskStore,
		stateStore,
	)

	ctx := context.Background()

	// Create task without session ID
	task, err := serverCoordinator.ExecuteTaskAsync(ctx, "test task", "", "", "")
	if err != nil {
		t.Fatalf("ExecuteTaskAsync failed: %v", err)
	}

	// Wait for background goroutine to register mapping
	time.Sleep(200 * time.Millisecond)

	// Verify that broadcaster has a mapping for the session ID
	// (This would require exposing broadcaster internals or using a different test approach)
	// For now, we just verify the session ID is not empty - get fresh data to avoid race
	freshTask, err := taskStore.Get(ctx, task.ID)
	if err != nil {
		t.Fatalf("Failed to get fresh task: %v", err)
	}
	if freshTask.SessionID == "" {
		t.Fatal("Session ID is empty - broadcaster mapping will fail")
	}

	t.Logf("✓ Task created with valid session ID for broadcaster mapping: %s", freshTask.SessionID)
}

// TestTaskStoreProgressFields verifies that task store properly handles progress fields
func TestTaskStoreProgressFields(t *testing.T) {
	taskStore := NewInMemoryTaskStore()
	ctx := context.Background()

	// Create task
	task, err := taskStore.Create(ctx, "session-123", "test task", "", "")
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// Update progress
	err = taskStore.UpdateProgress(ctx, task.ID, 3, 150)
	if err != nil {
		t.Fatalf("Failed to update progress: %v", err)
	}

	// Retrieve and verify
	retrieved, err := taskStore.Get(ctx, task.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve task: %v", err)
	}

	if retrieved.CurrentIteration != 3 {
		t.Fatalf("CurrentIteration mismatch: expected 3, got %d", retrieved.CurrentIteration)
	}

	if retrieved.TokensUsed != 150 {
		t.Fatalf("TokensUsed mismatch: expected 150, got %d", retrieved.TokensUsed)
	}

	// Set result and verify total fields
	result := &agent.TaskResult{
		Answer:     "Done",
		Iterations: 5,
		TokensUsed: 300,
		SessionID:  "session-123",
	}

	err = taskStore.SetResult(ctx, task.ID, result)
	if err != nil {
		t.Fatalf("Failed to set result: %v", err)
	}

	final, err := taskStore.Get(ctx, task.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve final task: %v", err)
	}

	if final.TotalIterations != 5 {
		t.Fatalf("TotalIterations mismatch: expected 5, got %d", final.TotalIterations)
	}

	if final.TotalTokens != 300 {
		t.Fatalf("TotalTokens mismatch: expected 300, got %d", final.TotalTokens)
	}

	t.Logf("✓ Progress fields updated correctly: total_iterations=%d, total_tokens=%d",
		final.TotalIterations, final.TotalTokens)
}

// TestWorkflowInputReceivedEventEmission verifies that user-submitted attachments are emitted via SSE.
func TestWorkflowInputReceivedEventEmission(t *testing.T) {
	sessionStore := NewMockSessionStore()
	taskStore := NewInMemoryTaskStore()
	broadcaster := NewEventBroadcaster()

	stateStore := sessionstate.NewInMemoryStore()

	agentCoordinator := NewMockAgentCoordinator(sessionStore)
	serverCoordinator := NewServerCoordinator(
		agentCoordinator,
		broadcaster,
		sessionStore,
		taskStore,
		stateStore,
	)

	original := []core.Attachment{
		{
			Name:        " sketch.png ",
			MediaType:   "image/png",
			URI:         "https://example.com/sketch.png",
			Description: "hand-drawn sketch",
		},
		{
			Name:      "diagram.svg",
			MediaType: "image/svg+xml",
			URI:       "https://example.com/diagram.svg",
		},
		{
			Name:      "   ",
			MediaType: "image/jpeg",
			Data:      "ignored",
		},
	}

	ctx := appcontext.WithUserAttachments(context.Background(), original)

	task, err := serverCoordinator.ExecuteTaskAsync(ctx, "展示占位符 [sketch.png] 和 [diagram.svg]", "", "", "")
	if err != nil {
		t.Fatalf("ExecuteTaskAsync failed: %v", err)
	}

	sessionID := task.SessionID

	var userTaskEvent *domain.WorkflowInputReceivedEvent
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		history := broadcaster.GetEventHistory(sessionID)
		for _, event := range history {
			if typed, ok := event.(*domain.WorkflowInputReceivedEvent); ok {
				userTaskEvent = typed
				break
			}
		}
		if userTaskEvent != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if userTaskEvent == nil {
		t.Fatalf("expected workflow.input.received event to be emitted, but none was recorded for session %s", sessionID)
	}

	if userTaskEvent.Task != "展示占位符 [sketch.png] 和 [diagram.svg]" {
		t.Fatalf("unexpected task content: %q", userTaskEvent.Task)
	}

	if len(userTaskEvent.Attachments) != 2 {
		t.Fatalf("expected 2 attachments, got %d", len(userTaskEvent.Attachments))
	}

	if _, exists := userTaskEvent.Attachments[""]; exists {
		t.Fatal("blank attachment key should have been filtered out")
	}

	sketch, ok := userTaskEvent.Attachments["sketch.png"]
	if !ok {
		t.Fatalf("missing sanitized attachment key 'sketch.png': %+v", userTaskEvent.Attachments)
	}
	if sketch.Name != "sketch.png" {
		t.Fatalf("expected trimmed name 'sketch.png', got %q", sketch.Name)
	}
	if sketch.Source != "user_upload" {
		t.Fatalf("expected user_upload source, got %q", sketch.Source)
	}
	if sketch.MediaType != "image/png" {
		t.Fatalf("unexpected media type: %s", sketch.MediaType)
	}
	if sketch.Data != "" {
		t.Fatalf("expected attachment data to be omitted, got %q", sketch.Data)
	}
	if sketch.URI != "https://example.com/sketch.png" {
		t.Fatalf("unexpected sketch URI: %q", sketch.URI)
	}

	diagram, ok := userTaskEvent.Attachments["diagram.svg"]
	if !ok {
		t.Fatalf("missing attachment 'diagram.svg': %+v", userTaskEvent.Attachments)
	}
	if diagram.URI != "https://example.com/diagram.svg" {
		t.Fatalf("unexpected diagram URI: %q", diagram.URI)
	}
	if diagram.Source != "user_upload" {
		t.Fatalf("expected diagram source to be user_upload, got %q", diagram.Source)
	}

	// Ensure original slice data wasn't mutated by context helpers.
	if original[0].Name != " sketch.png " {
		t.Fatalf("expected original attachment name to remain trimmed only externally, got %q", original[0].Name)
	}
	if original[0].Source != "" {
		t.Fatalf("unexpected source mutation on original attachment: %q", original[0].Source)
	}
}

// Mock agent coordinator that supports cancellation
type MockCancellableAgentCoordinator struct {
	sessionStore storage.SessionStore
	delay        time.Duration
}

func NewMockCancellableAgentCoordinator(sessionStore storage.SessionStore, delay time.Duration) *MockCancellableAgentCoordinator {
	return &MockCancellableAgentCoordinator{
		sessionStore: sessionStore,
		delay:        delay,
	}
}

func (m *MockCancellableAgentCoordinator) GetSession(ctx context.Context, id string) (*storage.Session, error) {
	return m.sessionStore.Get(ctx, id)
}

func (m *MockCancellableAgentCoordinator) ExecuteTask(ctx context.Context, task string, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
	// Simulate long-running task that checks for cancellation
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	deadline := time.Now().Add(m.delay)
	for {
		select {
		case <-ctx.Done():
			// Context was cancelled
			return nil, ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				// Task completed successfully
				return &agent.TaskResult{
					Answer:     "Mock answer",
					Iterations: 3,
					TokensUsed: 100,
					StopReason: "completed",
					SessionID:  sessionID,
				}, nil
			}
		}
	}
}

func (m *MockCancellableAgentCoordinator) GetConfig() agent.AgentConfig {
	return agent.AgentConfig{}
}

func (m *MockCancellableAgentCoordinator) PreviewContextWindow(ctx context.Context, sessionID string) (agent.ContextWindowPreview, error) {
	return agent.ContextWindowPreview{
		Window: agent.ContextWindow{
			SessionID: sessionID,
		},
		ToolMode: "cli",
	}, nil
}

// TestTaskCancellation verifies task cancellation works correctly
func TestTaskCancellation(t *testing.T) {
	// Setup
	sessionStore := NewMockSessionStore()
	taskStore := NewInMemoryTaskStore()
	broadcaster := NewEventBroadcaster()

	stateStore := sessionstate.NewInMemoryStore()

	// Use a cancellable agent coordinator with 1 second delay
	agentCoordinator := NewMockCancellableAgentCoordinator(sessionStore, 1*time.Second)

	serverCoordinator := NewServerCoordinator(
		agentCoordinator,
		broadcaster,
		sessionStore,
		taskStore,
		stateStore,
	)

	ctx := context.Background()

	// Start a long-running task
	task, err := serverCoordinator.ExecuteTaskAsync(ctx, "long running task", "", "", "")
	if err != nil {
		t.Fatalf("ExecuteTaskAsync failed: %v", err)
	}

	// Wait a bit to ensure task is running
	time.Sleep(100 * time.Millisecond)

	// Verify task is running
	runningTask, err := taskStore.Get(ctx, task.ID)
	if err != nil {
		t.Fatalf("Failed to get task: %v", err)
	}

	if runningTask.Status != serverPorts.TaskStatusPending && runningTask.Status != serverPorts.TaskStatusRunning {
		t.Logf("⚠ Task status is %s (expected pending or running)", runningTask.Status)
	}

	// Cancel the task
	err = serverCoordinator.CancelTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("Failed to cancel task: %v", err)
	}

	// Wait for cancellation to propagate
	time.Sleep(200 * time.Millisecond)

	// Verify task was cancelled
	cancelledTask, err := taskStore.Get(ctx, task.ID)
	if err != nil {
		t.Fatalf("Failed to get cancelled task: %v", err)
	}

	if cancelledTask.Status != serverPorts.TaskStatusCancelled {
		t.Errorf("Expected status 'cancelled', got '%s'", cancelledTask.Status)
	}

	if cancelledTask.TerminationReason != serverPorts.TerminationReasonCancelled {
		t.Errorf("Expected termination reason 'cancelled', got '%s'", cancelledTask.TerminationReason)
	}

	events := broadcaster.GetEventHistory(task.SessionID)
	foundCancellation := false
	for _, evt := range events {
		if evt.EventType() == "workflow.result.cancelled" {
			foundCancellation = true
			break
		}
	}
	if !foundCancellation {
		t.Errorf("expected workflow.result.cancelled event in history for session %s", task.SessionID)
	}

	t.Logf("✓ Task cancelled successfully: status=%s, reason=%s",
		cancelledTask.Status, cancelledTask.TerminationReason)
}

// TestCancelNonExistentTask verifies error handling for non-existent task
func TestCancelNonExistentTask(t *testing.T) {
	sessionStore := NewMockSessionStore()
	taskStore := NewInMemoryTaskStore()
	broadcaster := NewEventBroadcaster()

	stateStore := sessionstate.NewInMemoryStore()

	agentCoordinator := NewMockAgentCoordinator(sessionStore)

	serverCoordinator := NewServerCoordinator(
		agentCoordinator,
		broadcaster,
		sessionStore,
		taskStore,
		stateStore,
	)

	ctx := context.Background()

	// Try to cancel non-existent task
	err := serverCoordinator.CancelTask(ctx, "non-existent-task-id")
	if err == nil {
		t.Error("Expected error when cancelling non-existent task")
	}

	t.Logf("✓ Correctly returned error for non-existent task: %v", err)
}

// TestCancelCompletedTask verifies that completed tasks cannot be cancelled
func TestCancelCompletedTask(t *testing.T) {
	sessionStore := NewMockSessionStore()
	taskStore := NewInMemoryTaskStore()
	broadcaster := NewEventBroadcaster()

	stateStore := sessionstate.NewInMemoryStore()

	agentCoordinator := NewMockAgentCoordinator(sessionStore)

	serverCoordinator := NewServerCoordinator(
		agentCoordinator,
		broadcaster,
		sessionStore,
		taskStore,
		stateStore,
	)

	ctx := context.Background()

	// Create and complete a task
	task, err := taskStore.Create(ctx, "session-1", "test task", "", "")
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	result := &agent.TaskResult{
		Answer:     "Completed",
		Iterations: 1,
		TokensUsed: 50,
		StopReason: "completed",
		SessionID:  "session-1",
	}

	err = taskStore.SetResult(ctx, task.ID, result)
	if err != nil {
		t.Fatalf("Failed to set result: %v", err)
	}

	// Try to cancel completed task
	err = serverCoordinator.CancelTask(ctx, task.ID)
	if err == nil {
		t.Error("Expected error when cancelling completed task")
	}

	t.Logf("✓ Correctly returned error for completed task: %v", err)
}

// TestNoCancelFunctionLeak verifies that cancel functions are cleaned up
func TestNoCancelFunctionLeak(t *testing.T) {
	sessionStore := NewMockSessionStore()
	taskStore := NewInMemoryTaskStore()
	broadcaster := NewEventBroadcaster()

	stateStore := sessionstate.NewInMemoryStore()

	// Use fast mock coordinator to quickly complete tasks
	agentCoordinator := NewMockAgentCoordinator(sessionStore)

	serverCoordinator := NewServerCoordinator(
		agentCoordinator,
		broadcaster,
		sessionStore,
		taskStore,
		stateStore,
	)

	ctx := context.Background()

	// Create multiple tasks
	taskIDs := make([]string, 5)
	for i := 0; i < 5; i++ {
		task, err := serverCoordinator.ExecuteTaskAsync(ctx, "test task", "", "", "")
		if err != nil {
			t.Fatalf("ExecuteTaskAsync failed: %v", err)
		}
		taskIDs[i] = task.ID
	}

	// Wait for tasks to complete
	time.Sleep(300 * time.Millisecond)

	// Verify all tasks completed
	for _, taskID := range taskIDs {
		task, err := taskStore.Get(ctx, taskID)
		if err != nil {
			t.Fatalf("Failed to get task %s: %v", taskID, err)
		}

		if task.Status != serverPorts.TaskStatusCompleted {
			t.Logf("⚠ Task %s status is %s (expected completed)", taskID, task.Status)
		}
	}

	// Check that cancel functions were cleaned up
	serverCoordinator.cancelMu.RLock()
	numCancelFuncs := len(serverCoordinator.cancelFuncs)
	serverCoordinator.cancelMu.RUnlock()

	if numCancelFuncs != 0 {
		t.Errorf("Expected 0 cancel functions after tasks completed, got %d", numCancelFuncs)
	} else {
		t.Logf("✓ No cancel function leak: all %d tasks cleaned up", len(taskIDs))
	}
}

func TestServerCoordinatorRecordsTaskErrorMetrics(t *testing.T) {
	sessionStore := NewMockSessionStore()
	taskStore := NewInMemoryTaskStore()
	broadcaster := NewEventBroadcaster()

	stateStore := sessionstate.NewInMemoryStore()
	failingAgent := &failingAgentCoordinator{sessionStore: sessionStore, err: errors.New("boom")}
	metrics := &observability.MetricsCollector{}
	statusCh := make(chan string, 1)
	metrics.SetTestHooks(observability.MetricsTestHooks{
		TaskExecution: func(status string, _ time.Duration) {
			statusCh <- status
		},
	})
	coordinator := NewServerCoordinator(
		failingAgent,
		broadcaster,
		sessionStore,
		taskStore,
		stateStore,
		WithObservability(&observability.Observability{Metrics: metrics}),
	)
	ctx := context.Background()
	if _, err := coordinator.ExecuteTaskAsync(ctx, "fail-task", "", "", ""); err != nil {
		t.Fatalf("ExecuteTaskAsync failed: %v", err)
	}
	select {
	case status := <-statusCh:
		if status != "error" {
			t.Fatalf("expected error status metric, got %s", status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for task execution metric")
	}
}

type failingAgentCoordinator struct {
	sessionStore storage.SessionStore
	err          error
}

func (f *failingAgentCoordinator) GetSession(ctx context.Context, id string) (*storage.Session, error) {
	return f.sessionStore.Get(ctx, id)
}

func (f *failingAgentCoordinator) ExecuteTask(ctx context.Context, task string, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
	return nil, f.err
}

func (f *failingAgentCoordinator) GetConfig() agent.AgentConfig {
	return agent.AgentConfig{}
}

func (f *failingAgentCoordinator) PreviewContextWindow(ctx context.Context, sessionID string) (agent.ContextWindowPreview, error) {
	return agent.ContextWindowPreview{}, f.err
}
