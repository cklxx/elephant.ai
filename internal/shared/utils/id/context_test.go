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
		SessionID:     "session-test",
		RunID:         "run-test",
		ParentRunID:   "run-parent",
		LogID:         "log-test",
		CorrelationID: "run-root",
		CausationID:   "call-trigger",
	}

	ctx = WithIDs(ctx, ids)

	got := IDsFromContext(ctx)
	if got.SessionID != ids.SessionID {
		t.Fatalf("expected session %s, got %s", ids.SessionID, got.SessionID)
	}
	if got.RunID != ids.RunID {
		t.Fatalf("expected run %s, got %s", ids.RunID, got.RunID)
	}
	if got.ParentRunID != ids.ParentRunID {
		t.Fatalf("expected parent %s, got %s", ids.ParentRunID, got.ParentRunID)
	}
	if got.LogID != ids.LogID {
		t.Fatalf("expected log id %s, got %s", ids.LogID, got.LogID)
	}
	if got.CorrelationID != ids.CorrelationID {
		t.Fatalf("expected correlation %s, got %s", ids.CorrelationID, got.CorrelationID)
	}
	if got.CausationID != ids.CausationID {
		t.Fatalf("expected causation %s, got %s", ids.CausationID, got.CausationID)
	}

	// Ensure compatibility with SessionContextKey lookup
	if compat := SessionIDFromContext(ctx); compat != ids.SessionID {
		t.Fatalf("expected compat session %s, got %s", ids.SessionID, compat)
	}
}

func TestEnsureRunID(t *testing.T) {
	ctx := context.Background()
	ctx, generated := EnsureRunID(ctx, func() string { return "run-123" })
	if generated != "run-123" {
		t.Fatalf("expected generated id run-123, got %s", generated)
	}

	// Should reuse existing value on subsequent calls
	ctx = WithRunID(ctx, "run-existing")
	ctx, generated = EnsureRunID(ctx, func() string { return "run-new" })
	if generated != "run-existing" {
		t.Fatalf("expected to reuse existing id, got %s", generated)
	}

	if RunIDFromContext(ctx) != "run-existing" {
		t.Fatalf("expected stored run id run-existing, got %s", RunIDFromContext(ctx))
	}
}

func TestEnsureLogID(t *testing.T) {
	ctx := context.Background()
	ctx, generated := EnsureLogID(ctx, func() string { return "log-123" })
	if generated != "log-123" {
		t.Fatalf("expected generated id log-123, got %s", generated)
	}

	ctx = WithLogID(ctx, "log-existing")
	ctx, generated = EnsureLogID(ctx, func() string { return "log-new" })
	if generated != "log-existing" {
		t.Fatalf("expected to reuse existing id, got %s", generated)
	}

	if LogIDFromContext(ctx) != "log-existing" {
		t.Fatalf("expected stored log id log-existing, got %s", LogIDFromContext(ctx))
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

	runID := NewRunID()
	if !strings.HasPrefix(runID, "run-") || len(runID) != len("run-")+runIDSuffixLength {
		t.Fatalf("unexpected run id format: %s", runID)
	}

	eventID := NewEventID()
	if !strings.HasPrefix(eventID, "evt-") || len(eventID) <= len("evt-") {
		t.Fatalf("unexpected event id format: %s", eventID)
	}

	SetStrategy(StrategyUUIDv7)
	sessionUUID := NewSessionID()
	if !strings.HasPrefix(sessionUUID, "session-") || len(sessionUUID) <= len("session-") {
		t.Fatalf("unexpected uuidv7 session id format: %s", sessionUUID)
	}

	runUUID := NewRunID()
	if !strings.HasPrefix(runUUID, "run-") || len(runUUID) != len("run-")+runIDSuffixLength {
		t.Fatalf("unexpected uuidv7 run id format: %s", runUUID)
	}

	if raw := NewKSUID(); raw == "" {
		t.Fatalf("expected raw ksuid to be non-empty")
	}

	if rawUUID := NewUUIDv7(); rawUUID == "" {
		t.Fatalf("expected raw uuidv7 to be non-empty")
	}

	logID := NewLogID()
	if !strings.HasPrefix(logID, "log-") || len(logID) <= len("log-") {
		t.Fatalf("unexpected log id format: %s", logID)
	}
}

func TestGeneratedIdentifiersAreUnique(t *testing.T) {
	t.Cleanup(func() {
		SetStrategy(StrategyKSUID)
	})

	const total = 1024

	sessionSeen := make(map[string]struct{}, total)
	runSeen := make(map[string]struct{}, total)

	for i := 0; i < total; i++ {
		sessionID := NewSessionID()
		if _, exists := sessionSeen[sessionID]; exists {
			t.Fatalf("duplicate session id generated: %s", sessionID)
		}
		sessionSeen[sessionID] = struct{}{}

		runID := NewRunID()
		if _, exists := runSeen[runID]; exists {
			t.Fatalf("duplicate run id generated: %s", runID)
		}
		runSeen[runID] = struct{}{}
	}

	if len(sessionSeen) != total {
		t.Fatalf("expected %d unique session ids, got %d", total, len(sessionSeen))
	}

	if len(runSeen) != total {
		t.Fatalf("expected %d unique run ids, got %d", total, len(runSeen))
	}
}

func TestIDsRoundTripJSONCompatibility(t *testing.T) {
	ctx := context.Background()
	ctx = WithIDs(ctx, IDs{SessionID: "session-123", RunID: "run-456", ParentRunID: "run-parent"})

	encoded, err := json.Marshal(IDsFromContext(ctx))
	if err != nil {
		t.Fatalf("failed to marshal ids: %v", err)
	}

	var decoded IDs
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("failed to unmarshal ids: %v", err)
	}

	if decoded != (IDs{SessionID: "session-123", RunID: "run-456", ParentRunID: "run-parent", LogID: ""}) {
		t.Fatalf("unexpected ids round trip result: %+v", decoded)
	}
}

func TestCorrelationCausationContext(t *testing.T) {
	ctx := context.Background()

	ctx = WithCorrelationID(ctx, "run-root")
	ctx = WithCausationID(ctx, "call-trigger")

	if got := CorrelationIDFromContext(ctx); got != "run-root" {
		t.Fatalf("expected correlation run-root, got %s", got)
	}
	if got := CausationIDFromContext(ctx); got != "call-trigger" {
		t.Fatalf("expected causation call-trigger, got %s", got)
	}

	// Empty context returns empty
	if got := CorrelationIDFromContext(context.Background()); got != "" {
		t.Fatalf("expected empty correlation, got %s", got)
	}
	//nolint:staticcheck // nil context is intentional to verify nil-safe accessors.
	if got := CausationIDFromContext(nil); got != "" {
		t.Fatalf("expected empty causation for nil ctx, got %s", got)
	}
}
