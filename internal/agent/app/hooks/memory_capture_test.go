package hooks

import (
	"context"
	"errors"
	"strings"
	"testing"

	appcontext "alex/internal/agent/app/context"
	"alex/internal/memory"
)

func TestMemoryCaptureHook_Name(t *testing.T) {
	hook := NewMemoryCaptureHook(nil, nil, MemoryCaptureConfig{})
	if hook.Name() != "memory_capture" {
		t.Errorf("expected name 'memory_capture', got %q", hook.Name())
	}
}

func TestMemoryCaptureHook_OnTaskStart_Noop(t *testing.T) {
	hook := NewMemoryCaptureHook(nil, nil, MemoryCaptureConfig{})
	result := hook.OnTaskStart(context.Background(), TaskInfo{TaskInput: "test"})
	if result != nil {
		t.Errorf("expected nil from OnTaskStart, got %v", result)
	}
}

func TestMemoryCaptureHook_NilService(t *testing.T) {
	hook := NewMemoryCaptureHook(nil, nil, MemoryCaptureConfig{})
	err := hook.OnTaskCompleted(context.Background(), TaskResultInfo{
		TaskInput: "test",
		Answer:    "result",
		ToolCalls: []ToolResultInfo{{ToolName: "bash", Success: true}},
	})
	if err != nil {
		t.Errorf("expected nil error with nil service, got %v", err)
	}
}

func TestMemoryCaptureHook_SkipsNoToolCalls(t *testing.T) {
	svc := &mockMemoryService{}
	hook := NewMemoryCaptureHook(svc, nil, MemoryCaptureConfig{})

	err := hook.OnTaskCompleted(context.Background(), TaskResultInfo{
		TaskInput: "just a question",
		Answer:    "just an answer",
		ToolCalls: nil, // no tool calls
	})

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if svc.saveCalled != 0 {
		t.Errorf("expected no save for task without tool calls, got %d", svc.saveCalled)
	}
}

func TestMemoryCaptureHook_SkipsEmptyAnswer(t *testing.T) {
	svc := &mockMemoryService{}
	hook := NewMemoryCaptureHook(svc, nil, MemoryCaptureConfig{})

	err := hook.OnTaskCompleted(context.Background(), TaskResultInfo{
		TaskInput: "test",
		Answer:    "",
		ToolCalls: []ToolResultInfo{{ToolName: "bash", Success: true}},
	})

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if svc.saveCalled != 0 {
		t.Errorf("expected no save for empty answer, got %d", svc.saveCalled)
	}
}

func TestMemoryCaptureHook_SuccessfulCapture(t *testing.T) {
	svc := &mockMemoryService{
		saveResult: memory.Entry{Key: "test-key"},
	}
	hook := NewMemoryCaptureHook(svc, nil, MemoryCaptureConfig{})

	err := hook.OnTaskCompleted(context.Background(), TaskResultInfo{
		TaskInput:  "deploy the new migration",
		Answer:     "Migration deployed successfully",
		SessionID:  "sess-123",
		UserID:     "testuser",
		Iterations: 3,
		StopReason: "complete",
		ToolCalls: []ToolResultInfo{
			{ToolName: "bash", Success: true, Output: "ok"},
			{ToolName: "file_write", Success: true, Output: "written"},
			{ToolName: "bash", Success: true, Output: "deployed"},
		},
	})

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if svc.saveCalled != 2 {
		t.Fatalf("expected 2 save calls (capture + trace), got %d", svc.saveCalled)
	}

	var entry memory.Entry
	for _, candidate := range svc.entries {
		if candidate.Slots != nil && candidate.Slots["type"] == "auto_capture" {
			entry = candidate
			break
		}
	}
	if entry.Content == "" {
		t.Fatal("expected auto_capture entry to be saved")
	}

	// Check user ID
	if entry.UserID != "testuser" {
		t.Errorf("expected userID 'testuser', got %q", entry.UserID)
	}

	// Check content
	if !contains(entry.Content, "deploy") {
		t.Error("expected content to contain task input")
	}
	if !contains(entry.Content, "Migration deployed successfully") {
		t.Error("expected content to contain answer")
	}
	if !contains(entry.Content, "bash") {
		t.Error("expected content to contain tool names")
	}

	// Check keywords include tool names
	hasToolKeyword := false
	for _, kw := range entry.Keywords {
		if kw == "bash" || kw == "file_write" {
			hasToolKeyword = true
			break
		}
	}
	if !hasToolKeyword {
		t.Errorf("expected keywords to include tool names, got %v", entry.Keywords)
	}

	// Check slots
	if entry.Slots["type"] != "auto_capture" {
		t.Errorf("expected slot type='auto_capture', got %q", entry.Slots["type"])
	}
	if entry.Slots["scope"] != "user" {
		t.Errorf("expected slot scope='user', got %q", entry.Slots["scope"])
	}
	if entry.Slots["source"] != "memory_capture" {
		t.Errorf("expected slot source='memory_capture', got %q", entry.Slots["source"])
	}
	if entry.Slots["outcome"] != "complete" {
		t.Errorf("expected slot outcome='complete', got %q", entry.Slots["outcome"])
	}
	if entry.Slots["session_id"] != "sess-123" {
		t.Errorf("expected slot session_id='sess-123', got %q", entry.Slots["session_id"])
	}
	if entry.Slots["sender_id"] != "testuser" {
		t.Errorf("expected slot sender_id='testuser', got %q", entry.Slots["sender_id"])
	}

	// Check tool sequence
	toolSeq := entry.Slots["tool_sequence"]
	if toolSeq != "bash→file_write→bash" {
		t.Errorf("expected tool_sequence 'bash→file_write→bash', got %q", toolSeq)
	}
}

func TestMemoryCaptureHook_SaveError(t *testing.T) {
	svc := &mockMemoryService{
		saveErr: errors.New("disk full"),
	}
	hook := NewMemoryCaptureHook(svc, nil, MemoryCaptureConfig{})

	err := hook.OnTaskCompleted(context.Background(), TaskResultInfo{
		TaskInput:  "test task",
		Answer:     "test answer",
		UserID:     "testuser",
		StopReason: "complete",
		ToolCalls:  []ToolResultInfo{{ToolName: "bash", Success: true}},
	})

	if err == nil {
		t.Error("expected error on save failure")
		return
	}
	if !contains(err.Error(), "memory capture") {
		t.Errorf("expected wrapped error, got %v", err)
	}
}

func TestMemoryCaptureHook_MissingUserID(t *testing.T) {
	svc := &mockMemoryService{}
	hook := NewMemoryCaptureHook(svc, nil, MemoryCaptureConfig{})

	err := hook.OnTaskCompleted(context.Background(), TaskResultInfo{
		TaskInput:  "test",
		Answer:     "answer",
		UserID:     "",
		StopReason: "complete",
		ToolCalls:  []ToolResultInfo{{ToolName: "bash", Success: true}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if svc.saveCalled != 0 {
		t.Fatalf("expected capture to be skipped when userID missing, got %d", svc.saveCalled)
	}
}

func TestBuildCaptureSummary_TruncatesLongInput(t *testing.T) {
	longInput := strings.Repeat("a", 500)
	result := TaskResultInfo{
		TaskInput:  longInput,
		Answer:     "short answer",
		StopReason: "complete",
		ToolCalls:  []ToolResultInfo{{ToolName: "test"}},
	}

	summary := buildCaptureSummary(result)

	// Should truncate at 200 chars + "..."
	if len(summary) > 1500 {
		t.Errorf("summary too long: %d chars", len(summary))
	}
	if !contains(summary, "...") {
		t.Error("expected truncation marker")
	}
}

func TestBuildCaptureSummary_TruncatesLongAnswer(t *testing.T) {
	longAnswer := strings.Repeat("b", 2000)
	result := TaskResultInfo{
		TaskInput:  "short task",
		Answer:     longAnswer,
		StopReason: "complete",
		ToolCalls:  []ToolResultInfo{{ToolName: "test"}},
	}

	summary := buildCaptureSummary(result)

	// Answer should be truncated at maxCaptureContentLen
	if !contains(summary, "...") {
		t.Error("expected truncation marker for long answer")
	}
}

func TestBuildCaptureSlots(t *testing.T) {
	ctx := context.Background()
	ctx = appcontext.WithChannel(ctx, "lark")
	ctx = appcontext.WithChatID(ctx, "oc_chat_123")
	result := TaskResultInfo{
		StopReason: "complete",
		SessionID:  "sess-456",
		UserID:     "ou_user",
		ToolCalls: []ToolResultInfo{
			{ToolName: "search"},
			{ToolName: "fetch"},
			{ToolName: "write"},
		},
	}

	slots := buildCaptureSlots(ctx, result)

	if slots["type"] != "auto_capture" {
		t.Errorf("expected type='auto_capture', got %q", slots["type"])
	}
	if slots["scope"] != "user" {
		t.Errorf("expected scope='user', got %q", slots["scope"])
	}
	if slots["source"] != "memory_capture" {
		t.Errorf("expected source='memory_capture', got %q", slots["source"])
	}
	if slots["outcome"] != "complete" {
		t.Errorf("expected outcome='complete', got %q", slots["outcome"])
	}
	if slots["session_id"] != "sess-456" {
		t.Errorf("expected session_id, got %q", slots["session_id"])
	}
	if slots["sender_id"] != "ou_user" {
		t.Errorf("expected sender_id 'ou_user', got %q", slots["sender_id"])
	}
	if slots["channel"] != "lark" {
		t.Errorf("expected channel 'lark', got %q", slots["channel"])
	}
	if slots["chat_id"] != "oc_chat_123" {
		t.Errorf("expected chat_id 'oc_chat_123', got %q", slots["chat_id"])
	}
	if slots["tool_sequence"] != "search→fetch→write" {
		t.Errorf("expected tool_sequence 'search→fetch→write', got %q", slots["tool_sequence"])
	}
}

func TestExtractCaptureKeywords_IncludesToolNames(t *testing.T) {
	result := TaskResultInfo{
		TaskInput: "search for documentation",
		ToolCalls: []ToolResultInfo{
			{ToolName: "web_search"},
			{ToolName: "web_fetch"},
		},
	}

	keywords := extractCaptureKeywords(result)

	// Should include both task keywords and tool names
	hasSearch := false
	hasWebSearch := false
	for _, kw := range keywords {
		if kw == "search" || kw == "documentation" {
			hasSearch = true
		}
		if kw == "web_search" {
			hasWebSearch = true
		}
	}
	if !hasSearch {
		t.Errorf("expected task keywords in result, got %v", keywords)
	}
	if !hasWebSearch {
		t.Errorf("expected tool name 'web_search' in keywords, got %v", keywords)
	}
}
