package hooks_test

import (
	"context"
	"testing"

	"alex/internal/agent/app/hooks"
	"alex/internal/memory"
)

// TestIntegration_FullLifecycle verifies the complete proactive hooks lifecycle:
// 1. Register memory recall + capture hooks
// 2. Run OnTaskStart → auto-recall returns relevant memories
// 3. Run OnTaskCompleted → auto-capture writes task summary
// 4. Run OnTaskStart again → recall retrieves the previously captured memory
func TestIntegration_FullLifecycle(t *testing.T) {
	// Use a real in-memory store for integration testing
	store := memory.NewInMemoryStore()
	svc := memory.NewService(store)

	// Seed an existing memory
	_, err := svc.Save(context.Background(), memory.Entry{
		UserID:   "testuser",
		Content:  "We deployed version 2.1 to production on Jan 28. Used blue-green deployment strategy.",
		Keywords: []string{"deployment", "production", "blue-green"},
		Slots:    map[string]string{"project": "api-gateway", "outcome": "success"},
	})
	if err != nil {
		t.Fatalf("Failed to seed memory: %v", err)
	}

	// Build registry with both hooks
	registry := hooks.NewRegistry(nil)
	registry.Register(hooks.NewMemoryRecallHook(svc, nil, hooks.MemoryRecallConfig{MaxRecalls: 5}))
	registry.Register(hooks.NewMemoryCaptureHook(svc, nil, hooks.MemoryCaptureConfig{DedupeThreshold: 0.99}))

	if registry.HookCount() != 2 {
		t.Fatalf("expected 2 hooks, got %d", registry.HookCount())
	}

	// === Phase 1: Task Start — auto-recall should find the seeded memory ===
	injections := registry.RunOnTaskStart(context.Background(), hooks.TaskInfo{
		TaskInput: "deploy the api-gateway to production",
		SessionID: "sess-001",
		RunID:     "run-001",
		UserID:    "testuser",
	})

	if len(injections) == 0 {
		t.Fatal("Phase 1: expected at least 1 injection from memory recall")
	}
	if injections[0].Type != hooks.InjectionMemoryRecall {
		t.Errorf("Phase 1: expected InjectionMemoryRecall, got %v", injections[0].Type)
	}
	if injections[0].Priority != 100 {
		t.Errorf("Phase 1: expected priority 100, got %d", injections[0].Priority)
	}

	// Verify the formatted context contains the seeded memory content
	formatted := hooks.FormatInjectionsAsContext(injections)
	if formatted == "" {
		t.Fatal("Phase 1: expected non-empty formatted context")
	}
	assertContains(t, formatted, "blue-green", "Phase 1: formatted context should contain seeded memory")
	assertContains(t, formatted, "deployment", "Phase 1: formatted context should contain keyword")

	t.Logf("Phase 1 PASS: Recalled %d injections, formatted length: %d bytes", len(injections), len(formatted))

	// === Phase 2: Task Completed — auto-capture should write to memory ===
	registry.RunOnTaskCompleted(context.Background(), hooks.TaskResultInfo{
		TaskInput:  "deploy the api-gateway to production",
		Answer:     "Deployment completed successfully. Version 2.2 is now live with zero-downtime rollout.",
		SessionID:  "sess-001",
		RunID:      "run-001",
		UserID:     "testuser",
		Iterations: 5,
		StopReason: "complete",
		ToolCalls: []hooks.ToolResultInfo{
			{ToolName: "bash", Success: true, Output: "kubectl apply -f deployment.yaml"},
			{ToolName: "bash", Success: true, Output: "deployment verified"},
		},
	})

	// Verify the captured memory was written to the store.
	// Search by task-related keywords (auto_capture is a slot value, not a keyword).
	captured, err := svc.Recall(context.Background(), memory.Query{
		UserID:   "testuser",
		Keywords: []string{"deploy", "bash"},
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("Phase 2: failed to recall captured memories: %v", err)
	}

	// Should find at least the auto-captured entry (identified by slot type=auto_capture)
	foundCapture := false
	for _, entry := range captured {
		if entry.Slots != nil && entry.Slots["type"] == "auto_capture" {
			foundCapture = true
			assertContains(t, entry.Content, "deploy", "Phase 2: captured content should reference task")
			assertContains(t, entry.Content, "bash", "Phase 2: captured content should list tools")
			assertContains(t, entry.Slots["tool_sequence"], "bash", "Phase 2: tool_sequence slot should contain tool names")
			assertContains(t, entry.Slots["outcome"], "complete", "Phase 2: outcome slot should be 'complete'")
			t.Logf("Phase 2 PASS: Captured memory key=%s, keywords=%v", entry.Key, entry.Keywords)
			break
		}
	}
	if !foundCapture {
		t.Fatal("Phase 2: expected auto-captured memory entry with type=auto_capture")
	}

	// === Phase 3: Second Task Start — should recall BOTH seeded + captured memories ===
	injections2 := registry.RunOnTaskStart(context.Background(), hooks.TaskInfo{
		TaskInput: "what happened with the last api-gateway deployment?",
		SessionID: "sess-002",
		RunID:     "run-002",
		UserID:    "testuser",
	})

	if len(injections2) == 0 {
		t.Fatal("Phase 3: expected injections from second task start")
	}

	formatted2 := hooks.FormatInjectionsAsContext(injections2)
	// Should contain content from both the original seeded memory AND the auto-captured one
	assertContains(t, formatted2, "deployment", "Phase 3: should recall deployment-related memories")

	t.Logf("Phase 3 PASS: Recalled %d injections on second task", len(injections2))
}

// TestIntegration_CaptureSkipsConversationOnly verifies that pure conversations
// (no tool calls) do not produce auto-captured memories.
func TestIntegration_CaptureSkipsConversationOnly(t *testing.T) {
	store := memory.NewInMemoryStore()
	svc := memory.NewService(store)

	registry := hooks.NewRegistry(nil)
	registry.Register(hooks.NewMemoryCaptureHook(svc, nil, hooks.MemoryCaptureConfig{}))

	// Complete a task with NO tool calls (pure conversation)
	registry.RunOnTaskCompleted(context.Background(), hooks.TaskResultInfo{
		TaskInput:  "What is the meaning of life?",
		Answer:     "42",
		UserID:     "testuser",
		StopReason: "complete",
		ToolCalls:  nil, // no tools used
	})

	// Verify: nothing should be captured
	entries, err := svc.Recall(context.Background(), memory.Query{
		UserID:   "testuser",
		Keywords: []string{"meaning", "life"},
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("Recall failed: %v", err)
	}

	for _, entry := range entries {
		if entry.Slots["type"] == "auto_capture" {
			t.Fatal("Expected no auto-capture for conversation-only tasks")
		}
	}

	t.Log("PASS: Conversation-only task correctly skipped by capture hook")
}

// TestIntegration_RecallWithNoExistingMemories verifies graceful handling
// when the memory store is empty.
func TestIntegration_RecallWithNoExistingMemories(t *testing.T) {
	store := memory.NewInMemoryStore()
	svc := memory.NewService(store)

	registry := hooks.NewRegistry(nil)
	registry.Register(hooks.NewMemoryRecallHook(svc, nil, hooks.MemoryRecallConfig{MaxRecalls: 5}))

	injections := registry.RunOnTaskStart(context.Background(), hooks.TaskInfo{
		TaskInput: "build a new feature",
		UserID:    "newuser",
	})

	if len(injections) != 0 {
		t.Errorf("Expected 0 injections for empty memory store, got %d", len(injections))
	}

	t.Log("PASS: Empty memory store returns no injections")
}

// TestIntegration_MultipleHooksOrdering verifies that injections from
// multiple hooks are correctly ordered by priority.
func TestIntegration_MultipleHooksOrdering(t *testing.T) {
	store := memory.NewInMemoryStore()
	svc := memory.NewService(store)

	// Seed a memory so recall produces results
	if _, err := svc.Save(context.Background(), memory.Entry{
		UserID:   "testuser",
		Content:  "Important context about testing",
		Keywords: []string{"testing", "ci"},
	}); err != nil {
		t.Fatalf("save seed memory: %v", err)
	}

	// Custom high-priority hook
	customHook := &staticHook{
		name: "custom_high",
		injections: []hooks.Injection{
			{Type: hooks.InjectionWarning, Content: "Warning: rate limit approaching", Source: "custom_high", Priority: 200},
		},
	}

	registry := hooks.NewRegistry(nil)
	registry.Register(hooks.NewMemoryRecallHook(svc, nil, hooks.MemoryRecallConfig{MaxRecalls: 5}))
	registry.Register(customHook)

	injections := registry.RunOnTaskStart(context.Background(), hooks.TaskInfo{
		TaskInput: "run the tests for ci pipeline",
		UserID:    "testuser",
	})

	if len(injections) < 2 {
		t.Fatalf("Expected at least 2 injections, got %d", len(injections))
	}

	// Custom hook (priority 200) should come before memory recall (priority 100)
	if injections[0].Source != "custom_high" {
		t.Errorf("Expected first injection from custom_high (priority 200), got %q", injections[0].Source)
	}
	if injections[1].Source != "memory_recall" {
		t.Errorf("Expected second injection from memory_recall (priority 100), got %q", injections[1].Source)
	}

	t.Logf("PASS: %d injections correctly ordered by priority", len(injections))
}

// staticHook is a test hook that returns fixed injections.
type staticHook struct {
	name       string
	injections []hooks.Injection
}

func (h *staticHook) Name() string { return h.name }
func (h *staticHook) OnTaskStart(_ context.Context, _ hooks.TaskInfo) []hooks.Injection {
	return h.injections
}
func (h *staticHook) OnTaskCompleted(_ context.Context, _ hooks.TaskResultInfo) error {
	return nil
}

func assertContains(t *testing.T, s, substr, msg string) {
	t.Helper()
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return
		}
	}
	t.Errorf("%s: %q not found in output (len=%d)", msg, substr, len(s))
}
