package id

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestWithIDsAndFromContext(t *testing.T) {
	ctx := context.Background()

	ids := IDs{
		SessionID:    "session-test",
		TaskID:       "task-test",
		ParentTaskID: "parent-task",
	}

	ctx = WithIDs(ctx, ids)

	got := IDsFromContext(ctx)
	if got.SessionID != ids.SessionID {
		t.Fatalf("expected session %s, got %s", ids.SessionID, got.SessionID)
	}
	if got.TaskID != ids.TaskID {
		t.Fatalf("expected task %s, got %s", ids.TaskID, got.TaskID)
	}
	if got.ParentTaskID != ids.ParentTaskID {
		t.Fatalf("expected parent %s, got %s", ids.ParentTaskID, got.ParentTaskID)
	}

	// Ensure compatibility with ports.SessionContextKey lookup
	if compat := SessionIDFromContext(ctx); compat != ids.SessionID {
		t.Fatalf("expected compat session %s, got %s", ids.SessionID, compat)
	}
}

func TestWithUserID(t *testing.T) {
	ctx := context.Background()
	ctx = WithUserID(ctx, "user-123")
	if got := UserIDFromContext(ctx); got != "user-123" {
		t.Fatalf("expected user-123, got %s", got)
	}
	// empty user should be ignored
	ctx = WithUserID(ctx, "")
	if got := UserIDFromContext(ctx); got != "user-123" {
		t.Fatalf("expected stored user to remain user-123, got %s", got)
	}
}

func TestEnsureTaskID(t *testing.T) {
	ctx := context.Background()
	ctx, generated := EnsureTaskID(ctx, func() string { return "task-123" })
	if generated != "task-123" {
		t.Fatalf("expected generated id task-123, got %s", generated)
	}

	// Should reuse existing value on subsequent calls
	ctx = WithTaskID(ctx, "task-existing")
	ctx, generated = EnsureTaskID(ctx, func() string { return "task-new" })
	if generated != "task-existing" {
		t.Fatalf("expected to reuse existing id, got %s", generated)
	}

	if TaskIDFromContext(ctx) != "task-existing" {
		t.Fatalf("expected stored task id task-existing, got %s", TaskIDFromContext(ctx))
	}
}

func TestNewGenerators(t *testing.T) {
	t.Cleanup(func() {
		SetStrategy(StrategyKSUID)
	})

	sessionID := NewSessionID()
	if !strings.HasPrefix(sessionID, "session-") || len(sessionID) <= len("session-") {
		t.Fatalf("unexpected session id format: %s", sessionID)
	}

	taskID := NewTaskID()
	if !strings.HasPrefix(taskID, "task-") || len(taskID) <= len("task-") {
		t.Fatalf("unexpected task id format: %s", taskID)
	}

	SetStrategy(StrategyUUIDv7)
	sessionUUID := NewSessionID()
	if !strings.HasPrefix(sessionUUID, "session-") || len(sessionUUID) <= len("session-") {
		t.Fatalf("unexpected uuidv7 session id format: %s", sessionUUID)
	}

	taskUUID := NewTaskID()
	if !strings.HasPrefix(taskUUID, "task-") || len(taskUUID) <= len("task-") {
		t.Fatalf("unexpected uuidv7 task id format: %s", taskUUID)
	}

	if raw := NewKSUID(); raw == "" {
		t.Fatalf("expected raw ksuid to be non-empty")
	}

	if rawUUID := NewUUIDv7(); rawUUID == "" {
		t.Fatalf("expected raw uuidv7 to be non-empty")
	}
}

func TestGeneratedIdentifiersAreUnique(t *testing.T) {
	t.Cleanup(func() {
		SetStrategy(StrategyKSUID)
	})

	const total = 1024

	sessionSeen := make(map[string]struct{}, total)
	taskSeen := make(map[string]struct{}, total)
	artifactSeen := make(map[string]struct{}, total)

	for i := 0; i < total; i++ {
		sessionID := NewSessionID()
		if _, exists := sessionSeen[sessionID]; exists {
			t.Fatalf("duplicate session id generated: %s", sessionID)
		}
		sessionSeen[sessionID] = struct{}{}

		taskID := NewTaskID()
		if _, exists := taskSeen[taskID]; exists {
			t.Fatalf("duplicate task id generated: %s", taskID)
		}
		taskSeen[taskID] = struct{}{}

		artifactID := NewArtifactID()
		if _, exists := artifactSeen[artifactID]; exists {
			t.Fatalf("duplicate artifact id generated: %s", artifactID)
		}
		artifactSeen[artifactID] = struct{}{}
	}

	if len(sessionSeen) != total {
		t.Fatalf("expected %d unique session ids, got %d", total, len(sessionSeen))
	}

	if len(taskSeen) != total {
		t.Fatalf("expected %d unique task ids, got %d", total, len(taskSeen))
	}

	if len(artifactSeen) != total {
		t.Fatalf("expected %d unique artifact ids, got %d", total, len(artifactSeen))
	}
}

func TestIDsRoundTripJSONCompatibility(t *testing.T) {
	ctx := context.Background()
	ctx = WithIDs(ctx, IDs{SessionID: "session-123", TaskID: "task-456", ParentTaskID: "task-parent"})

	encoded, err := json.Marshal(IDsFromContext(ctx))
	if err != nil {
		t.Fatalf("failed to marshal ids: %v", err)
	}

	var decoded IDs
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("failed to unmarshal ids: %v", err)
	}

	if decoded != (IDs{SessionID: "session-123", TaskID: "task-456", ParentTaskID: "task-parent"}) {
		t.Fatalf("unexpected ids round trip result: %+v", decoded)
	}
}
