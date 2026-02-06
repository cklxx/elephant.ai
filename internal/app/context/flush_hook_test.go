package context

import (
	"context"
	"errors"
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
)

func TestMemoryFlushHook_ExtractsKeyInfo(t *testing.T) {
	var savedContent string
	var savedMeta map[string]string
	hook := NewMemoryFlushHook(func(_ context.Context, content string, metadata map[string]string) error {
		savedContent = content
		savedMeta = metadata
		return nil
	})

	messages := []ports.Message{
		{Role: "user", Content: "Please analyze the logs"},
		{Role: "assistant", Content: "I found 3 errors in the logs"},
		{Role: "user", Content: "Now fix the critical one"},
		{Role: "assistant", Content: "Applied the patch successfully"},
	}

	err := hook.OnBeforeCompaction(context.Background(), messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if savedContent == "" {
		t.Fatalf("expected content to be saved")
	}
	if !strings.Contains(savedContent, "Please analyze the logs") {
		t.Fatalf("expected user message to be captured, got %q", savedContent)
	}
	if !strings.Contains(savedContent, "Now fix the critical one") {
		t.Fatalf("expected second user message to be captured, got %q", savedContent)
	}
	if !strings.Contains(savedContent, "I found 3 errors") {
		t.Fatalf("expected assistant reply to be captured, got %q", savedContent)
	}
	if !strings.Contains(savedContent, "Applied the patch") {
		t.Fatalf("expected second assistant reply to be captured, got %q", savedContent)
	}
	if !strings.Contains(savedContent, "[User messages]") {
		t.Fatalf("expected user messages section, got %q", savedContent)
	}
	if !strings.Contains(savedContent, "[Assistant replies]") {
		t.Fatalf("expected assistant replies section, got %q", savedContent)
	}
	if savedMeta["type"] != "compaction_flush" {
		t.Fatalf("expected metadata type=compaction_flush, got %q", savedMeta["type"])
	}
	if savedMeta["source"] != "context_compaction" {
		t.Fatalf("expected metadata source=context_compaction, got %q", savedMeta["source"])
	}
}

func TestMemoryFlushHook_TruncatesOutput(t *testing.T) {
	var savedContent string
	hook := NewMemoryFlushHook(func(_ context.Context, content string, _ map[string]string) error {
		savedContent = content
		return nil
	})

	// Generate messages that will exceed 2000 characters.
	var messages []ports.Message
	for i := 0; i < 30; i++ {
		messages = append(messages,
			ports.Message{Role: "user", Content: strings.Repeat("u", 150)},
			ports.Message{Role: "assistant", Content: strings.Repeat("a", 150)},
		)
	}

	err := hook.OnBeforeCompaction(context.Background(), messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len([]rune(savedContent)) > flushMaxChars+10 {
		t.Fatalf("expected content to be capped near %d chars, got %d", flushMaxChars, len([]rune(savedContent)))
	}
	if !strings.HasSuffix(savedContent, "...") {
		t.Fatalf("expected truncated content to end with '...', got suffix %q", savedContent[len(savedContent)-10:])
	}
}

func TestMemoryFlushHook_ToolResultsSummarized(t *testing.T) {
	var savedContent string
	hook := NewMemoryFlushHook(func(_ context.Context, content string, _ map[string]string) error {
		savedContent = content
		return nil
	})

	longToolOutput := strings.Repeat("x", 500)
	messages := []ports.Message{
		{Role: "user", Content: "Run the tests"},
		{
			Role:    "assistant",
			Content: "Running tests now",
			ToolCalls: []ports.ToolCall{
				{ID: "call-1", Name: "shell_exec"},
			},
		},
		{
			Role:       "tool",
			Content:    longToolOutput,
			ToolCallID: "call-1",
		},
		{
			Role:    "assistant",
			Content: "Tests completed",
			ToolResults: []ports.ToolResult{
				{CallID: "result-1", Content: strings.Repeat("y", 500)},
			},
		},
	}

	err := hook.OnBeforeCompaction(context.Background(), messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(savedContent, "[Tool results]") {
		t.Fatalf("expected tool results section, got %q", savedContent)
	}
	if !strings.Contains(savedContent, "tool_call:shell_exec") {
		t.Fatalf("expected tool call summary, got %q", savedContent)
	}
	if !strings.Contains(savedContent, "call-1 ->") {
		t.Fatalf("expected tool result with call ID, got %q", savedContent)
	}
	if !strings.Contains(savedContent, "result-1 ->") {
		t.Fatalf("expected inline tool result, got %q", savedContent)
	}
	// The tool outputs should be truncated, not the full 500 chars.
	if strings.Contains(savedContent, longToolOutput) {
		t.Fatalf("expected tool output to be truncated, but found full output")
	}
}

func TestMemoryFlushHook_SaveError(t *testing.T) {
	saveErr := errors.New("storage unavailable")
	hook := NewMemoryFlushHook(func(_ context.Context, _ string, _ map[string]string) error {
		return saveErr
	})

	messages := []ports.Message{
		{Role: "user", Content: "hello"},
	}

	err := hook.OnBeforeCompaction(context.Background(), messages)
	if err == nil {
		t.Fatalf("expected error to be returned")
	}
	if !errors.Is(err, saveErr) {
		t.Fatalf("expected save error, got %v", err)
	}
}

func TestMemoryFlushHook_NilSaveFn(t *testing.T) {
	hook := NewMemoryFlushHook(nil)
	err := hook.OnBeforeCompaction(context.Background(), []ports.Message{
		{Role: "user", Content: "test"},
	})
	if err != nil {
		t.Fatalf("expected nil saveFn to be a no-op, got %v", err)
	}
}

func TestMemoryFlushHook_EmptyMessages(t *testing.T) {
	called := false
	hook := NewMemoryFlushHook(func(_ context.Context, _ string, _ map[string]string) error {
		called = true
		return nil
	})

	err := hook.OnBeforeCompaction(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Fatalf("save should not be called for empty messages")
	}
}

func TestNoopFlushHook(t *testing.T) {
	hook := NoopFlushHook{}
	err := hook.OnBeforeCompaction(context.Background(), []ports.Message{
		{Role: "user", Content: "important data"},
		{Role: "assistant", Content: "noted"},
	})
	if err != nil {
		t.Fatalf("NoopFlushHook should never return an error, got %v", err)
	}
}

func TestAutoCompact_CallsFlushHook(t *testing.T) {
	var flushedMessages []ports.Message
	hook := NewMemoryFlushHook(func(_ context.Context, content string, _ map[string]string) error {
		// Content is already formatted; we just verify the hook was called.
		if content == "" {
			t.Errorf("expected non-empty content in flush hook")
		}
		return nil
	})

	// Use a recording hook to capture the raw messages passed to the hook.
	recorder := &recordingFlushHook{}

	mgr := &manager{flushHook: recorder}
	limit := 50
	messages := []ports.Message{
		{Role: "system", Source: ports.MessageSourceSystemPrompt, Content: "base system"},
		{Role: "user", Content: strings.Repeat("x", 400)},
		{Role: "assistant", Content: "reply to user"},
	}

	compacted, compactedFlag := mgr.AutoCompact(messages, limit)
	if !compactedFlag {
		t.Fatalf("expected auto compaction to run")
	}
	_ = compacted

	flushedMessages = recorder.messages
	if len(flushedMessages) != 2 {
		t.Fatalf("expected 2 compressible messages (user + assistant), got %d", len(flushedMessages))
	}
	// System prompt should NOT be in the flushed messages.
	for _, msg := range flushedMessages {
		if msg.Source == ports.MessageSourceSystemPrompt {
			t.Fatalf("system prompt should not be flushed, got %+v", msg)
		}
	}

	// Verify the MemoryFlushHook variant also works end-to-end.
	mgrWithMemory := &manager{flushHook: hook}
	_, ok := mgrWithMemory.AutoCompact(messages, limit)
	if !ok {
		t.Fatalf("expected compaction with MemoryFlushHook")
	}
}

func TestAutoCompact_FlushHookErrorDoesNotBlockCompression(t *testing.T) {
	hook := NewMemoryFlushHook(func(_ context.Context, _ string, _ map[string]string) error {
		return errors.New("save failed")
	})

	mgr := &manager{flushHook: hook}
	limit := 50
	messages := []ports.Message{
		{Role: "system", Source: ports.MessageSourceSystemPrompt, Content: "base system"},
		{Role: "user", Content: strings.Repeat("x", 400)},
	}

	compacted, compactedFlag := mgr.AutoCompact(messages, limit)
	if !compactedFlag {
		t.Fatalf("expected auto compaction to proceed despite hook error")
	}
	if len(compacted) != 2 {
		t.Fatalf("expected system prompt + summary, got %d entries", len(compacted))
	}
}

func TestAutoCompact_NilFlushHookIsSkipped(t *testing.T) {
	mgr := &manager{} // flushHook is nil
	limit := 50
	messages := []ports.Message{
		{Role: "system", Source: ports.MessageSourceSystemPrompt, Content: "base system"},
		{Role: "user", Content: strings.Repeat("x", 400)},
	}

	compacted, compactedFlag := mgr.AutoCompact(messages, limit)
	if !compactedFlag {
		t.Fatalf("expected auto compaction to run")
	}
	if len(compacted) != 2 {
		t.Fatalf("expected system prompt + summary, got %d entries", len(compacted))
	}
}

// recordingFlushHook captures the messages passed to OnBeforeCompaction.
type recordingFlushHook struct {
	messages []ports.Message
	err      error
}

func (r *recordingFlushHook) OnBeforeCompaction(_ context.Context, messages []ports.Message) error {
	r.messages = append(r.messages, messages...)
	return r.err
}
