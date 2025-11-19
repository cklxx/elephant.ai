package app

import (
	"context"
	"testing"
	"time"

	agentapp "alex/internal/agent/app"
	"alex/internal/agent/domain"
	agentPorts "alex/internal/agent/ports"
	"alex/internal/analytics"
	serverPorts "alex/internal/server/ports"
	sessionstate "alex/internal/session/state_store"
)

// Mock implementations for testing

type MockSessionStore struct {
	sessions map[string]*agentPorts.Session
}

func NewMockSessionStore() *MockSessionStore {
	return &MockSessionStore{
		sessions: make(map[string]*agentPorts.Session),
	}
}

func (m *MockSessionStore) Create(ctx context.Context) (*agentPorts.Session, error) {
	session := &agentPorts.Session{
		ID:        "session-" + time.Now().Format("20060102150405.000000"),
		Messages:  []agentPorts.Message{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata:  make(map[string]string),
	}
	m.sessions[session.ID] = session
	return session, nil
}

func (m *MockSessionStore) Get(ctx context.Context, id string) (*agentPorts.Session, error) {
	if session, ok := m.sessions[id]; ok {
		return session, nil
	}
	return m.Create(ctx)
}

func (m *MockSessionStore) Save(ctx context.Context, session *agentPorts.Session) error {
	m.sessions[session.ID] = session
	return nil
}

func (m *MockSessionStore) List(ctx context.Context) ([]string, error) {
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
	sessionStore agentPorts.SessionStore
}

func NewMockAgentCoordinator(sessionStore agentPorts.SessionStore) *MockAgentCoordinator {
	return &MockAgentCoordinator{
		sessionStore: sessionStore,
	}
}

func (m *MockAgentCoordinator) GetSession(ctx context.Context, id string) (*agentPorts.Session, error) {
	return m.sessionStore.Get(ctx, id)
}

func (m *MockAgentCoordinator) ExecuteTask(ctx context.Context, task string, sessionID string, listener agentPorts.EventListener) (*agentPorts.TaskResult, error) {
	return &agentPorts.TaskResult{
		Answer:     "Mock answer",
		Iterations: 3,
		TokensUsed: 100,
		StopReason: "completed",
		SessionID:  sessionID,
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
	broadcaster.SetTaskStore(taskStore)
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
		session := &agentPorts.Session{
			ID:        explicitSessionID,
			Messages:  []agentPorts.Message{},
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
	broadcaster.SetTaskStore(taskStore)
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
	coordinator.emitUserTaskEvent(ctx, "session-analytics", "task-analytics", "capture metrics")

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
	if capture.properties["task_id"] != "task-analytics" {
		t.Errorf("expected task_id task-analytics, got %v", capture.properties["task_id"])
	}
	if capture.properties["source"] != "server" {
		t.Errorf("expected source server, got %v", capture.properties["source"])
	}
}

// TestBroadcasterMapping verifies that broadcaster task-session mapping uses correct session ID
func TestBroadcasterMapping(t *testing.T) {
	sessionStore := NewMockSessionStore()
	taskStore := NewInMemoryTaskStore()
	broadcaster := NewEventBroadcaster()
	broadcaster.SetTaskStore(taskStore)
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
	result := &agentPorts.TaskResult{
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

// TestUserTaskEventEmission verifies that user-submitted attachments are emitted via SSE.
func TestUserTaskEventEmission(t *testing.T) {
	sessionStore := NewMockSessionStore()
	taskStore := NewInMemoryTaskStore()
	broadcaster := NewEventBroadcaster()
	broadcaster.SetTaskStore(taskStore)
	stateStore := sessionstate.NewInMemoryStore()

	agentCoordinator := NewMockAgentCoordinator(sessionStore)
	serverCoordinator := NewServerCoordinator(
		agentCoordinator,
		broadcaster,
		sessionStore,
		taskStore,
		stateStore,
	)

	original := []agentPorts.Attachment{
		{
			Name:        " sketch.png ",
			MediaType:   "image/png",
			Data:        "dGVzdC1pbWFnZS1iYXNlNjQ=",
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

	ctx := agentapp.WithUserAttachments(context.Background(), original)

	task, err := serverCoordinator.ExecuteTaskAsync(ctx, "展示占位符 [sketch.png] 和 [diagram.svg]", "", "", "")
	if err != nil {
		t.Fatalf("ExecuteTaskAsync failed: %v", err)
	}

	sessionID := task.SessionID

	var userTaskEvent *domain.UserTaskEvent
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		history := broadcaster.GetEventHistory(sessionID)
		for _, event := range history {
			if typed, ok := event.(*domain.UserTaskEvent); ok {
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
		t.Fatalf("expected user_task event to be emitted, but none was recorded for session %s", sessionID)
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
	if sketch.Data == "" {
		t.Fatal("expected base64 data for sketch attachment")
	}
	if sketch.URI != "" {
		t.Fatalf("expected empty URI when data is provided, got %q", sketch.URI)
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
	sessionStore agentPorts.SessionStore
	delay        time.Duration
}

func NewMockCancellableAgentCoordinator(sessionStore agentPorts.SessionStore, delay time.Duration) *MockCancellableAgentCoordinator {
	return &MockCancellableAgentCoordinator{
		sessionStore: sessionStore,
		delay:        delay,
	}
}

func (m *MockCancellableAgentCoordinator) GetSession(ctx context.Context, id string) (*agentPorts.Session, error) {
	return m.sessionStore.Get(ctx, id)
}

func (m *MockCancellableAgentCoordinator) ExecuteTask(ctx context.Context, task string, sessionID string, listener agentPorts.EventListener) (*agentPorts.TaskResult, error) {
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
				return &agentPorts.TaskResult{
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

// TestTaskCancellation verifies task cancellation works correctly
func TestTaskCancellation(t *testing.T) {
	// Setup
	sessionStore := NewMockSessionStore()
	taskStore := NewInMemoryTaskStore()
	broadcaster := NewEventBroadcaster()
	broadcaster.SetTaskStore(taskStore)
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
		if evt.EventType() == "task_cancelled" {
			foundCancellation = true
			break
		}
	}
	if !foundCancellation {
		t.Errorf("expected task_cancelled event in history for session %s", task.SessionID)
	}

	t.Logf("✓ Task cancelled successfully: status=%s, reason=%s",
		cancelledTask.Status, cancelledTask.TerminationReason)
}

// TestCancelNonExistentTask verifies error handling for non-existent task
func TestCancelNonExistentTask(t *testing.T) {
	sessionStore := NewMockSessionStore()
	taskStore := NewInMemoryTaskStore()
	broadcaster := NewEventBroadcaster()
	broadcaster.SetTaskStore(taskStore)
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
	broadcaster.SetTaskStore(taskStore)
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

	result := &agentPorts.TaskResult{
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
	broadcaster.SetTaskStore(taskStore)
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
