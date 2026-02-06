package toolregistry

import (
	"context"
	"fmt"
	"testing"

	ports "alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
)

// ---------------------------------------------------------------------------
// Mock executor
// ---------------------------------------------------------------------------

type mockExecutor struct {
	result *ports.ToolResult
	err    error
	def    ports.ToolDefinition
	meta   ports.ToolMetadata
	// callCount tracks how many times Execute has been invoked.
	callCount int
}

func (m *mockExecutor) Execute(_ context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	m.callCount++
	if m.result != nil && m.result.CallID == "" {
		m.result.CallID = call.ID
	}
	return m.result, m.err
}

func (m *mockExecutor) Definition() ports.ToolDefinition { return m.def }
func (m *mockExecutor) Metadata() ports.ToolMetadata     { return m.meta }

var _ tools.ToolExecutor = (*mockExecutor)(nil)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func makeLookup(executors map[string]tools.ToolExecutor) ToolLookup {
	return func(name string) (tools.ToolExecutor, bool) {
		e, ok := executors[name]
		return e, ok
	}
}

func baseCall() ports.ToolCall {
	return ports.ToolCall{
		ID:   "call-1",
		Name: "primary_tool",
		Arguments: map[string]any{
			"query": "hello",
		},
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestDegradation_PrimarySuccess(t *testing.T) {
	primary := &mockExecutor{
		result: &ports.ToolResult{Content: "ok"},
		meta:   ports.ToolMetadata{Name: "primary_tool"},
	}
	config := DefaultDegradationConfig()
	config.FallbackMap["primary_tool"] = []string{"fb1"}

	exec := NewDegradationExecutor(primary, makeLookup(nil), config)
	res, err := exec.Execute(context.Background(), baseCall())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Content != "ok" {
		t.Fatalf("expected content 'ok', got %q", res.Content)
	}
	// No degradation metadata should be present.
	if res.Metadata != nil {
		if _, found := res.Metadata["degraded_from"]; found {
			t.Fatal("degraded_from should not be set on primary success")
		}
	}
}

func TestDegradation_PrimaryFail_FirstFallbackSucceeds(t *testing.T) {
	primary := &mockExecutor{
		err:  fmt.Errorf("primary failure"),
		meta: ports.ToolMetadata{Name: "primary_tool"},
	}
	fb1 := &mockExecutor{
		result: &ports.ToolResult{Content: "fb1-result"},
		meta:   ports.ToolMetadata{Name: "fb1"},
	}
	config := DefaultDegradationConfig()
	config.FallbackMap["primary_tool"] = []string{"fb1", "fb2"}

	lookup := makeLookup(map[string]tools.ToolExecutor{"fb1": fb1})
	exec := NewDegradationExecutor(primary, lookup, config)

	res, err := exec.Execute(context.Background(), baseCall())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Content != "fb1-result" {
		t.Fatalf("expected 'fb1-result', got %q", res.Content)
	}
	if res.Metadata["degraded_from"] != "primary_tool" {
		t.Fatalf("expected degraded_from=primary_tool, got %v", res.Metadata["degraded_from"])
	}
	if res.Metadata["degraded_to"] != "fb1" {
		t.Fatalf("expected degraded_to=fb1, got %v", res.Metadata["degraded_to"])
	}
}

func TestDegradation_PrimaryFail_FirstFallbackFails_SecondSucceeds(t *testing.T) {
	primary := &mockExecutor{
		err:  fmt.Errorf("primary failure"),
		meta: ports.ToolMetadata{Name: "primary_tool"},
	}
	fb1 := &mockExecutor{
		err:  fmt.Errorf("fb1 failure"),
		meta: ports.ToolMetadata{Name: "fb1"},
	}
	fb2 := &mockExecutor{
		result: &ports.ToolResult{Content: "fb2-result"},
		meta:   ports.ToolMetadata{Name: "fb2"},
	}
	config := DefaultDegradationConfig()
	config.FallbackMap["primary_tool"] = []string{"fb1", "fb2"}

	lookup := makeLookup(map[string]tools.ToolExecutor{"fb1": fb1, "fb2": fb2})
	exec := NewDegradationExecutor(primary, lookup, config)

	res, err := exec.Execute(context.Background(), baseCall())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Content != "fb2-result" {
		t.Fatalf("expected 'fb2-result', got %q", res.Content)
	}
	if res.Metadata["degraded_from"] != "primary_tool" {
		t.Fatalf("expected degraded_from=primary_tool, got %v", res.Metadata["degraded_from"])
	}
	if res.Metadata["degraded_to"] != "fb2" {
		t.Fatalf("expected degraded_to=fb2, got %v", res.Metadata["degraded_to"])
	}
}

func TestDegradation_AllFallbacksFail_UserPrompt(t *testing.T) {
	primary := &mockExecutor{
		err:  fmt.Errorf("primary failure"),
		meta: ports.ToolMetadata{Name: "primary_tool"},
	}
	fb1 := &mockExecutor{
		err:  fmt.Errorf("fb1 failure"),
		meta: ports.ToolMetadata{Name: "fb1"},
	}
	config := DefaultDegradationConfig()
	config.FallbackMap["primary_tool"] = []string{"fb1"}
	config.EnableUserPrompt = true

	lookup := makeLookup(map[string]tools.ToolExecutor{"fb1": fb1})
	exec := NewDegradationExecutor(primary, lookup, config)

	res, err := exec.Execute(context.Background(), baseCall())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Error != nil {
		t.Fatalf("user-prompt result should not carry an error, got %v", res.Error)
	}
	if res.Metadata["user_prompt"] != true {
		t.Fatal("expected user_prompt=true in metadata")
	}
	if res.Metadata["degraded_from"] != "primary_tool" {
		t.Fatalf("expected degraded_from=primary_tool, got %v", res.Metadata["degraded_from"])
	}
	if res.Content == "" {
		t.Fatal("expected non-empty content in user-prompt result")
	}
}

func TestDegradation_AllFallbacksFail_NoUserPrompt_OriginalError(t *testing.T) {
	primaryErr := fmt.Errorf("primary failure")
	primary := &mockExecutor{
		err:  primaryErr,
		meta: ports.ToolMetadata{Name: "primary_tool"},
	}
	fb1 := &mockExecutor{
		err:  fmt.Errorf("fb1 failure"),
		meta: ports.ToolMetadata{Name: "fb1"},
	}
	config := DefaultDegradationConfig()
	config.FallbackMap["primary_tool"] = []string{"fb1"}
	config.EnableUserPrompt = false

	lookup := makeLookup(map[string]tools.ToolExecutor{"fb1": fb1})
	exec := NewDegradationExecutor(primary, lookup, config)

	_, err := exec.Execute(context.Background(), baseCall())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if err.Error() != primaryErr.Error() {
		t.Fatalf("expected original error %q, got %q", primaryErr, err)
	}
}

func TestDegradation_MaxFallbackAttempts(t *testing.T) {
	primary := &mockExecutor{
		err:  fmt.Errorf("primary failure"),
		meta: ports.ToolMetadata{Name: "primary_tool"},
	}
	fb1 := &mockExecutor{
		err:  fmt.Errorf("fb1 failure"),
		meta: ports.ToolMetadata{Name: "fb1"},
	}
	fb2 := &mockExecutor{
		err:  fmt.Errorf("fb2 failure"),
		meta: ports.ToolMetadata{Name: "fb2"},
	}
	fb3 := &mockExecutor{
		result: &ports.ToolResult{Content: "fb3-result"},
		meta:   ports.ToolMetadata{Name: "fb3"},
	}

	config := DefaultDegradationConfig()
	config.FallbackMap["primary_tool"] = []string{"fb1", "fb2", "fb3"}
	config.MaxFallbackAttempts = 2 // should only try fb1 and fb2

	lookup := makeLookup(map[string]tools.ToolExecutor{
		"fb1": fb1, "fb2": fb2, "fb3": fb3,
	})
	exec := NewDegradationExecutor(primary, lookup, config)

	_, err := exec.Execute(context.Background(), baseCall())
	if err == nil {
		t.Fatal("expected error because MaxFallbackAttempts=2 should skip fb3")
	}
	if fb3.callCount != 0 {
		t.Fatalf("fb3 should not have been called, callCount=%d", fb3.callCount)
	}
}

func TestDegradation_NoFallbackMap_ReturnsErrorDirectly(t *testing.T) {
	primaryErr := fmt.Errorf("primary failure")
	primary := &mockExecutor{
		err:  primaryErr,
		meta: ports.ToolMetadata{Name: "unknown_tool"},
	}
	config := DefaultDegradationConfig()
	// No entry in FallbackMap for "unknown_tool"

	exec := NewDegradationExecutor(primary, makeLookup(nil), config)

	_, err := exec.Execute(context.Background(), baseCall())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != primaryErr.Error() {
		t.Fatalf("expected %q, got %q", primaryErr, err)
	}
}

func TestDegradation_MetadataSetCorrectly(t *testing.T) {
	primary := &mockExecutor{
		err:  fmt.Errorf("fail"),
		meta: ports.ToolMetadata{Name: "primary_tool"},
	}
	fb := &mockExecutor{
		result: &ports.ToolResult{Content: "fallback-ok"},
		meta:   ports.ToolMetadata{Name: "alt_tool"},
	}
	config := DefaultDegradationConfig()
	config.FallbackMap["primary_tool"] = []string{"alt_tool"}

	lookup := makeLookup(map[string]tools.ToolExecutor{"alt_tool": fb})
	exec := NewDegradationExecutor(primary, lookup, config)

	res, err := exec.Execute(context.Background(), baseCall())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	from, ok := res.Metadata["degraded_from"].(string)
	if !ok || from != "primary_tool" {
		t.Fatalf("degraded_from: expected 'primary_tool', got %v", res.Metadata["degraded_from"])
	}
	to, ok := res.Metadata["degraded_to"].(string)
	if !ok || to != "alt_tool" {
		t.Fatalf("degraded_to: expected 'alt_tool', got %v", res.Metadata["degraded_to"])
	}
}

func TestDegradation_UserPromptResultFormat(t *testing.T) {
	primary := &mockExecutor{
		err:  fmt.Errorf("fail"),
		meta: ports.ToolMetadata{Name: "my_tool"},
	}
	config := DefaultDegradationConfig()
	config.EnableUserPrompt = true
	// No fallbacks configured.

	exec := NewDegradationExecutor(primary, makeLookup(nil), config)
	res, err := exec.Execute(context.Background(), baseCall())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Error != nil {
		t.Fatalf("expected nil error in user-prompt result, got %v", res.Error)
	}
	if res.CallID != "call-1" {
		t.Fatalf("expected CallID 'call-1', got %q", res.CallID)
	}

	// Must contain the tool name so the LLM/user knows which tool failed.
	if res.Content == "" {
		t.Fatal("user prompt content must not be empty")
	}
	from, _ := res.Metadata["degraded_from"].(string)
	if from != "my_tool" {
		t.Fatalf("degraded_from: expected 'my_tool', got %q", from)
	}
	up, _ := res.Metadata["user_prompt"].(bool)
	if !up {
		t.Fatal("user_prompt metadata should be true")
	}
}

func TestDegradation_PrimaryResultError_FallbackSucceeds(t *testing.T) {
	// Primary returns a result whose Error field is non-nil (no Go error).
	primary := &mockExecutor{
		result: &ports.ToolResult{
			Content: "",
			Error:   fmt.Errorf("result-level failure"),
		},
		meta: ports.ToolMetadata{Name: "primary_tool"},
	}
	fb := &mockExecutor{
		result: &ports.ToolResult{Content: "recovered"},
		meta:   ports.ToolMetadata{Name: "fb1"},
	}
	config := DefaultDegradationConfig()
	config.FallbackMap["primary_tool"] = []string{"fb1"}

	lookup := makeLookup(map[string]tools.ToolExecutor{"fb1": fb})
	exec := NewDegradationExecutor(primary, lookup, config)

	res, err := exec.Execute(context.Background(), baseCall())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Content != "recovered" {
		t.Fatalf("expected 'recovered', got %q", res.Content)
	}
	if res.Metadata["degraded_from"] != "primary_tool" {
		t.Fatalf("expected degraded_from=primary_tool, got %v", res.Metadata["degraded_from"])
	}
}

func TestDegradation_FallbackNotFound_Skipped(t *testing.T) {
	primary := &mockExecutor{
		err:  fmt.Errorf("fail"),
		meta: ports.ToolMetadata{Name: "primary_tool"},
	}
	fb2 := &mockExecutor{
		result: &ports.ToolResult{Content: "fb2-ok"},
		meta:   ports.ToolMetadata{Name: "fb2"},
	}
	config := DefaultDegradationConfig()
	// "fb_missing" is not in the lookup, should be skipped.
	config.FallbackMap["primary_tool"] = []string{"fb_missing", "fb2"}

	lookup := makeLookup(map[string]tools.ToolExecutor{"fb2": fb2})
	exec := NewDegradationExecutor(primary, lookup, config)

	res, err := exec.Execute(context.Background(), baseCall())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Content != "fb2-ok" {
		t.Fatalf("expected 'fb2-ok', got %q", res.Content)
	}
}

func TestDegradation_ContextCanceled_StopsFallback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already canceled

	primary := &mockExecutor{
		err:  fmt.Errorf("fail"),
		meta: ports.ToolMetadata{Name: "primary_tool"},
	}
	fb1 := &mockExecutor{
		result: &ports.ToolResult{Content: "should-not-reach"},
		meta:   ports.ToolMetadata{Name: "fb1"},
	}
	config := DefaultDegradationConfig()
	config.FallbackMap["primary_tool"] = []string{"fb1"}

	lookup := makeLookup(map[string]tools.ToolExecutor{"fb1": fb1})
	exec := NewDegradationExecutor(primary, lookup, config)

	_, err := exec.Execute(ctx, baseCall())
	if err == nil {
		t.Fatal("expected error on canceled context")
	}
	if fb1.callCount != 0 {
		t.Fatalf("fallback should not be called on canceled context, callCount=%d", fb1.callCount)
	}
}

func TestDegradation_DefinitionAndMetadata(t *testing.T) {
	def := ports.ToolDefinition{Name: "my_tool", Description: "desc"}
	meta := ports.ToolMetadata{Name: "my_tool", Category: "test"}
	primary := &mockExecutor{def: def, meta: meta}

	exec := NewDegradationExecutor(primary, makeLookup(nil), DefaultDegradationConfig())
	if got := exec.Definition(); got.Name != def.Name || got.Description != def.Description {
		t.Fatalf("Definition mismatch: %+v", got)
	}
	if got := exec.Metadata(); got.Name != meta.Name || got.Category != meta.Category {
		t.Fatalf("Metadata mismatch: %+v", got)
	}
}

func TestDegradation_DefaultConfig(t *testing.T) {
	cfg := DefaultDegradationConfig()
	if cfg.FallbackMap == nil {
		t.Fatal("FallbackMap should be non-nil")
	}
	if cfg.EnableUserPrompt {
		t.Fatal("EnableUserPrompt should default to false")
	}
	if cfg.MaxFallbackAttempts != defaultMaxFallbackAttempts {
		t.Fatalf("MaxFallbackAttempts: expected %d, got %d", defaultMaxFallbackAttempts, cfg.MaxFallbackAttempts)
	}
}

func TestDegradation_ZeroMaxFallbackAttempts_UsesDefault(t *testing.T) {
	primary := &mockExecutor{
		err:  fmt.Errorf("fail"),
		meta: ports.ToolMetadata{Name: "primary_tool"},
	}
	fb1 := &mockExecutor{
		result: &ports.ToolResult{Content: "fb1-ok"},
		meta:   ports.ToolMetadata{Name: "fb1"},
	}
	config := DegradationConfig{
		FallbackMap:         map[string][]string{"primary_tool": {"fb1"}},
		MaxFallbackAttempts: 0, // should be normalized to default
	}
	lookup := makeLookup(map[string]tools.ToolExecutor{"fb1": fb1})
	exec := NewDegradationExecutor(primary, lookup, config)

	res, err := exec.Execute(context.Background(), baseCall())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Content != "fb1-ok" {
		t.Fatalf("expected 'fb1-ok', got %q", res.Content)
	}
}
