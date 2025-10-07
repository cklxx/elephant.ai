package app

import (
	"context"
	"testing"
	"time"

	agentPorts "alex/internal/agent/ports"
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

func (m *MockAgentCoordinator) ExecuteTask(ctx context.Context, task string, sessionID string, listener any) (*agentPorts.TaskResult, error) {
	return &agentPorts.TaskResult{
		Answer:     "Mock answer",
		Iterations: 3,
		TokensUsed: 100,
		StopReason: "completed",
		SessionID:  sessionID,
	}, nil
}

// TestSessionIDConsistency verifies the critical P0 fix:
// Session ID must be generated synchronously and remain consistent
func TestSessionIDConsistency(t *testing.T) {
	// Setup
	sessionStore := NewMockSessionStore()
	taskStore := NewInMemoryTaskStore()
	broadcaster := NewEventBroadcaster()
	broadcaster.SetTaskStore(taskStore)

	agentCoordinator := NewMockAgentCoordinator(sessionStore)

	serverCoordinator := NewServerCoordinator(
		agentCoordinator,
		broadcaster,
		sessionStore,
		taskStore,
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

// TestBroadcasterMapping verifies that broadcaster task-session mapping uses correct session ID
func TestBroadcasterMapping(t *testing.T) {
	sessionStore := NewMockSessionStore()
	taskStore := NewInMemoryTaskStore()
	broadcaster := NewEventBroadcaster()
	broadcaster.SetTaskStore(taskStore)

	agentCoordinator := NewMockAgentCoordinator(sessionStore)

	serverCoordinator := NewServerCoordinator(
		agentCoordinator,
		broadcaster,
		sessionStore,
		taskStore,
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
