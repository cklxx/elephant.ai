package react

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/ports/mocks"
)

// --- helpers ---

type msgSpec struct {
	role    string
	content string
	source  ports.MessageSource
}

func msg(role, content string, source ports.MessageSource) msgSpec {
	return msgSpec{role: role, content: content, source: source}
}

func buildMsgs(specs ...msgSpec) []ports.Message {
	msgs := make([]ports.Message, len(specs))
	for i, s := range specs {
		msgs[i] = ports.Message{Role: s.role, Content: s.content, Source: s.source}
	}
	return msgs
}

func makeCheckpointEngine(store CheckpointStore) *ReactEngine {
	return NewReactEngine(ReactEngineConfig{
		MaxIterations:   10,
		Logger:          agent.NoopLogger{},
		Clock:           agent.SystemClock{},
		CheckpointStore: store,
	})
}

func makeCheckpointServices() Services {
	return Services{
		Context: &mocks.MockContextManager{
			EstimateTokensFunc: func(messages []ports.Message) int {
				total := 0
				for _, m := range messages {
					total += len(m.Content)
				}
				return total
			},
		},
	}
}

// --- tests ---

func TestApplyContextCheckpoint_Basic(t *testing.T) {
	engine := makeCheckpointEngine(nil)
	services := makeCheckpointServices()
	checkpointCallID := "call-cp"

	state := &TaskState{
		SessionID: "session-1",
		Messages: buildMsgs(
			msg("system", "system prompt", ports.MessageSourceSystemPrompt),
			msg("user", "research topic X", ports.MessageSourceUserInput),
			msg("assistant", "searching web", ports.MessageSourceAssistantReply),
			msg("user", "web search result: long data...", ports.MessageSourceToolResult),
			msg("assistant", "found: X is Y because Z", ports.MessageSourceAssistantReply),
			msg("user", "more research data...", ports.MessageSourceToolResult),
			// Current iteration: assistant calls context_checkpoint
			msg("assistant", "checkpoint call", ports.MessageSourceAssistantReply),
			msg("user", "Context checkpoint accepted. Phase: research", ports.MessageSourceToolResult),
		),
	}
	// Inject tool call reference in the assistant message.
	state.Messages[6].ToolCalls = []ports.ToolCall{{ID: checkpointCallID, Name: "context_checkpoint"}}

	summary := strings.Repeat("This is a comprehensive summary of the research phase. ", 3)
	toolCalls := []ToolCall{{ID: checkpointCallID, Name: "context_checkpoint", Arguments: map[string]any{
		"summary":     summary,
		"phase_label": "research",
	}}}
	toolResults := []ToolResult{{CallID: checkpointCallID, Content: "ok"}}

	applied := engine.applyContextCheckpoint(context.Background(), state, services, toolResults, toolCalls)
	if !applied {
		t.Fatal("expected context checkpoint to be applied")
	}

	// Check that prunable messages were removed.
	foundCheckpoint := false
	for _, m := range state.Messages {
		if m.Source == ports.MessageSourceCheckpoint {
			foundCheckpoint = true
			if !strings.Contains(m.Content, "[Phase Complete: research]") {
				t.Fatalf("checkpoint message missing phase label, got: %s", m.Content)
			}
			if !strings.Contains(m.Content, "comprehensive summary") {
				t.Fatalf("checkpoint message missing summary content, got: %s", m.Content)
			}
		}
	}
	if !foundCheckpoint {
		t.Fatal("expected checkpoint message in state.Messages")
	}

	// System prompt should survive.
	if state.Messages[0].Source != ports.MessageSourceSystemPrompt {
		t.Fatal("system prompt should be preserved")
	}

	// Current iteration messages should survive.
	last := state.Messages[len(state.Messages)-1]
	if last.Source != ports.MessageSourceToolResult {
		t.Fatalf("expected current iteration tool result to survive, got source=%s", last.Source)
	}
}

func TestApplyContextCheckpoint_TooFewMessages(t *testing.T) {
	engine := makeCheckpointEngine(nil)
	services := makeCheckpointServices()
	checkpointCallID := "call-cp"

	state := &TaskState{
		SessionID: "session-few",
		Messages: buildMsgs(
			msg("system", "system prompt", ports.MessageSourceSystemPrompt),
			msg("user", "short task", ports.MessageSourceUserInput),
			// Current iteration: calls context_checkpoint
			msg("assistant", "checkpoint call", ports.MessageSourceAssistantReply),
			msg("user", "ok", ports.MessageSourceToolResult),
		),
	}
	state.Messages[2].ToolCalls = []ports.ToolCall{{ID: checkpointCallID, Name: "context_checkpoint"}}

	summary := strings.Repeat("x", 60)
	toolCalls := []ToolCall{{ID: checkpointCallID, Name: "context_checkpoint", Arguments: map[string]any{
		"summary": summary,
	}}}
	toolResults := []ToolResult{{CallID: checkpointCallID, Content: "ok"}}

	applied := engine.applyContextCheckpoint(context.Background(), state, services, toolResults, toolCalls)
	if applied {
		t.Fatal("expected checkpoint to be skipped with too few messages")
	}
}

func TestApplyContextCheckpoint_PreservesSystemAndImportant(t *testing.T) {
	engine := makeCheckpointEngine(nil)
	services := makeCheckpointServices()
	checkpointCallID := "call-cp"

	state := &TaskState{
		SessionID: "session-preserve",
		Messages: buildMsgs(
			msg("system", "system prompt", ports.MessageSourceSystemPrompt),
			msg("system", "important note", ports.MessageSourceImportant),
			msg("user", "research X", ports.MessageSourceUserInput),
			msg("assistant", "found A", ports.MessageSourceAssistantReply),
			msg("user", "tool result 1", ports.MessageSourceToolResult),
			msg("assistant", "found B", ports.MessageSourceAssistantReply),
			msg("user", "tool result 2", ports.MessageSourceToolResult),
			// Current iteration
			msg("assistant", "checkpoint call", ports.MessageSourceAssistantReply),
			msg("user", "ok", ports.MessageSourceToolResult),
		),
	}
	state.Messages[7].ToolCalls = []ports.ToolCall{{ID: checkpointCallID, Name: "context_checkpoint"}}

	summary := strings.Repeat("summary content here padding padding padding ", 2)
	toolCalls := []ToolCall{{ID: checkpointCallID, Name: "context_checkpoint", Arguments: map[string]any{
		"summary":     summary,
		"phase_label": "analysis",
	}}}
	toolResults := []ToolResult{{CallID: checkpointCallID, Content: "ok"}}

	applied := engine.applyContextCheckpoint(context.Background(), state, services, toolResults, toolCalls)
	if !applied {
		t.Fatal("expected context checkpoint to be applied")
	}

	// Verify system and important messages survived.
	var sources []ports.MessageSource
	for _, m := range state.Messages {
		sources = append(sources, m.Source)
	}

	hasSystem := false
	hasImportant := false
	hasCheckpoint := false
	for _, src := range sources {
		switch src {
		case ports.MessageSourceSystemPrompt:
			hasSystem = true
		case ports.MessageSourceImportant:
			hasImportant = true
		case ports.MessageSourceCheckpoint:
			hasCheckpoint = true
		}
	}
	if !hasSystem {
		t.Fatal("system prompt should survive")
	}
	if !hasImportant {
		t.Fatal("important note should survive")
	}
	if !hasCheckpoint {
		t.Fatal("checkpoint message should be inserted")
	}
}

func TestApplyContextCheckpoint_SequentialCheckpoints(t *testing.T) {
	engine := makeCheckpointEngine(nil)
	services := makeCheckpointServices()

	// Simulate state after a first checkpoint was already applied.
	state := &TaskState{
		SessionID: "session-multi",
		Messages: buildMsgs(
			msg("system", "system prompt", ports.MessageSourceSystemPrompt),
			msg("user", "[Phase Complete: research]\n\nPrior summary", ports.MessageSourceCheckpoint),
			// New phase work
			msg("user", "implement feature", ports.MessageSourceUserInput),
			msg("assistant", "writing code", ports.MessageSourceAssistantReply),
			msg("user", "code output 1", ports.MessageSourceToolResult),
			msg("assistant", "writing tests", ports.MessageSourceAssistantReply),
			msg("user", "test output", ports.MessageSourceToolResult),
			// Current iteration
			msg("assistant", "checkpoint call", ports.MessageSourceAssistantReply),
			msg("user", "ok", ports.MessageSourceToolResult),
		),
	}

	checkpointCallID := "call-cp2"
	state.Messages[7].ToolCalls = []ports.ToolCall{{ID: checkpointCallID, Name: "context_checkpoint"}}

	summary := strings.Repeat("implementation phase summary with all details ", 2)
	toolCalls := []ToolCall{{ID: checkpointCallID, Name: "context_checkpoint", Arguments: map[string]any{
		"summary":     summary,
		"phase_label": "implementation",
	}}}
	toolResults := []ToolResult{{CallID: checkpointCallID, Content: "ok"}}

	applied := engine.applyContextCheckpoint(context.Background(), state, services, toolResults, toolCalls)
	if !applied {
		t.Fatal("expected second context checkpoint to be applied")
	}

	// Should have both checkpoint messages.
	checkpointCount := 0
	for _, m := range state.Messages {
		if m.Source == ports.MessageSourceCheckpoint {
			checkpointCount++
		}
	}
	if checkpointCount != 2 {
		t.Fatalf("expected 2 checkpoint messages, got %d", checkpointCount)
	}

	// Original checkpoint should be preserved (index 0=system, 1=old checkpoint).
	if state.Messages[1].Source != ports.MessageSourceCheckpoint {
		t.Fatal("original checkpoint message should be preserved")
	}
	if !strings.Contains(state.Messages[1].Content, "Prior summary") {
		t.Fatal("original checkpoint content should be intact")
	}
}

func TestApplyContextCheckpoint_NoToolCall(t *testing.T) {
	engine := makeCheckpointEngine(nil)
	services := makeCheckpointServices()

	state := &TaskState{SessionID: "session-none"}
	toolCalls := []ToolCall{{ID: "call-1", Name: "web_search"}}
	toolResults := []ToolResult{{CallID: "call-1", Content: "result"}}

	applied := engine.applyContextCheckpoint(context.Background(), state, services, toolResults, toolCalls)
	if applied {
		t.Fatal("expected no checkpoint to be applied when no context_checkpoint call")
	}
}

func TestApplyContextCheckpoint_TokenRecalculation(t *testing.T) {
	engine := makeCheckpointEngine(nil)
	services := makeCheckpointServices()
	checkpointCallID := "call-cp"

	state := &TaskState{
		SessionID:  "session-tokens",
		TokenCount: 9999, // will be recalculated
		Messages: buildMsgs(
			msg("system", "system prompt", ports.MessageSourceSystemPrompt),
			msg("user", "research data one", ports.MessageSourceUserInput),
			msg("assistant", "found something", ports.MessageSourceAssistantReply),
			msg("user", "tool result data", ports.MessageSourceToolResult),
			msg("assistant", "more findings", ports.MessageSourceAssistantReply),
			msg("user", "more tool data", ports.MessageSourceToolResult),
			// Current iteration
			msg("assistant", "checkpoint call", ports.MessageSourceAssistantReply),
			msg("user", "ok", ports.MessageSourceToolResult),
		),
	}
	state.Messages[6].ToolCalls = []ports.ToolCall{{ID: checkpointCallID, Name: "context_checkpoint"}}

	summary := strings.Repeat("token recalculation test summary with content ", 2)
	toolCalls := []ToolCall{{ID: checkpointCallID, Name: "context_checkpoint", Arguments: map[string]any{
		"summary": summary,
	}}}
	toolResults := []ToolResult{{CallID: checkpointCallID, Content: "ok"}}

	engine.applyContextCheckpoint(context.Background(), state, services, toolResults, toolCalls)

	// Token count should be recalculated (mock uses len(Content) as token count).
	expectedTokens := 0
	for _, m := range state.Messages {
		expectedTokens += len(m.Content)
	}
	if state.TokenCount != expectedTokens {
		t.Fatalf("expected token count %d, got %d", expectedTokens, state.TokenCount)
	}
}

func TestApplyContextCheckpoint_ArchivePersistence(t *testing.T) {
	dir := t.TempDir()
	store := NewFileCheckpointStore(dir)
	engine := makeCheckpointEngine(store)
	services := makeCheckpointServices()
	checkpointCallID := "call-cp"

	state := &TaskState{
		SessionID: "session-archive",
		Messages: buildMsgs(
			msg("system", "system prompt", ports.MessageSourceSystemPrompt),
			msg("user", "research data", ports.MessageSourceUserInput),
			msg("assistant", "found A", ports.MessageSourceAssistantReply),
			msg("user", "result A", ports.MessageSourceToolResult),
			msg("assistant", "found B", ports.MessageSourceAssistantReply),
			msg("user", "result B", ports.MessageSourceToolResult),
			// Current iteration
			msg("assistant", "checkpoint call", ports.MessageSourceAssistantReply),
			msg("user", "ok", ports.MessageSourceToolResult),
		),
	}
	state.Messages[6].ToolCalls = []ports.ToolCall{{ID: checkpointCallID, Name: "context_checkpoint"}}

	summary := strings.Repeat("archive test summary content for persistence ", 2)
	toolCalls := []ToolCall{{ID: checkpointCallID, Name: "context_checkpoint", Arguments: map[string]any{
		"summary":     summary,
		"phase_label": "research",
	}}}
	toolResults := []ToolResult{{CallID: checkpointCallID, Content: "ok"}}

	applied := engine.applyContextCheckpoint(context.Background(), state, services, toolResults, toolCalls)
	if !applied {
		t.Fatal("expected context checkpoint to be applied")
	}

	// Verify archive file was created.
	archivePath := filepath.Join(dir, "session-archive", "archive", "0.json")
	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("expected archive file to exist: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected archive file to have content")
	}
	if !strings.Contains(string(data), "research") {
		t.Fatal("expected archive to contain phase label")
	}
}

func TestApplyContextCheckpoint_ErrorResult(t *testing.T) {
	engine := makeCheckpointEngine(nil)
	services := makeCheckpointServices()
	checkpointCallID := "call-cp"

	state := &TaskState{
		SessionID: "session-err",
		Messages: buildMsgs(
			msg("system", "system prompt", ports.MessageSourceSystemPrompt),
			msg("user", "data", ports.MessageSourceUserInput),
			msg("assistant", "checkpoint call", ports.MessageSourceAssistantReply),
			msg("user", "error result", ports.MessageSourceToolResult),
		),
	}

	toolCalls := []ToolCall{{ID: checkpointCallID, Name: "context_checkpoint", Arguments: map[string]any{
		"summary": strings.Repeat("x", 60),
	}}}
	toolResults := []ToolResult{{
		CallID:  checkpointCallID,
		Content: "error",
		Error:   context.Canceled, // simulate error
	}}

	applied := engine.applyContextCheckpoint(context.Background(), state, services, toolResults, toolCalls)
	if applied {
		t.Fatal("expected checkpoint to be skipped when tool result has error")
	}
}

func TestFileCheckpointStore_SaveArchive(t *testing.T) {
	dir := t.TempDir()
	store := NewFileCheckpointStore(dir)

	archive := &CheckpointArchive{
		SessionID:  "session-1",
		Seq:        0,
		PhaseLabel: "research",
		Messages: []MessageState{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "world"},
		},
		TokenCount: 42,
	}

	if err := store.SaveArchive(context.Background(), archive); err != nil {
		t.Fatalf("SaveArchive failed: %v", err)
	}

	archivePath := filepath.Join(dir, "session-1", "archive", "0.json")
	if _, err := os.Stat(archivePath); err != nil {
		t.Fatalf("expected archive file to exist: %v", err)
	}

	// Second archive.
	archive2 := &CheckpointArchive{
		SessionID:  "session-1",
		Seq:        1,
		PhaseLabel: "implementation",
		Messages:   []MessageState{{Role: "user", Content: "code"}},
		TokenCount: 10,
	}
	if err := store.SaveArchive(context.Background(), archive2); err != nil {
		t.Fatalf("SaveArchive(2) failed: %v", err)
	}

	archivePath2 := filepath.Join(dir, "session-1", "archive", "1.json")
	if _, err := os.Stat(archivePath2); err != nil {
		t.Fatalf("expected second archive file to exist: %v", err)
	}
}

func TestFileCheckpointStore_SaveArchiveValidation(t *testing.T) {
	dir := t.TempDir()
	store := NewFileCheckpointStore(dir)

	if err := store.SaveArchive(context.Background(), nil); err == nil {
		t.Fatal("expected error for nil archive")
	}

	if err := store.SaveArchive(context.Background(), &CheckpointArchive{}); err == nil {
		t.Fatal("expected error for empty session_id")
	}
}
