package context

import (
	"context"
	"testing"

	"alex/internal/app/subscription"
	"alex/internal/domain/agent/ports"
)

// ---------------------------------------------------------------------------
// Channel context
// ---------------------------------------------------------------------------

func TestChannelFromContext_Set(t *testing.T) {
	ctx := WithChannel(context.Background(), "lark")
	if got := ChannelFromContext(ctx); got != "lark" {
		t.Errorf("ChannelFromContext = %q, want lark", got)
	}
}

func TestChannelFromContext_Unset(t *testing.T) {
	if got := ChannelFromContext(context.Background()); got != "" {
		t.Errorf("ChannelFromContext(empty) = %q, want empty", got)
	}
}

func TestChannelFromContext_Nil(t *testing.T) {
	if got := ChannelFromContext(nil); got != "" {
		t.Errorf("ChannelFromContext(nil) = %q, want empty", got)
	}
}

func TestChatIDFromContext_Set(t *testing.T) {
	ctx := WithChatID(context.Background(), "oc_abc123")
	if got := ChatIDFromContext(ctx); got != "oc_abc123" {
		t.Errorf("ChatIDFromContext = %q, want oc_abc123", got)
	}
}

func TestChatIDFromContext_Nil(t *testing.T) {
	if got := ChatIDFromContext(nil); got != "" {
		t.Errorf("ChatIDFromContext(nil) = %q, want empty", got)
	}
}

func TestWithIsGroup(t *testing.T) {
	ctx := WithIsGroup(context.Background(), true)
	// WithIsGroup sets the value but has no getter — verify it doesn't panic.
	if ctx == nil {
		t.Fatal("WithIsGroup returned nil")
	}
}

// ---------------------------------------------------------------------------
// Session history
// ---------------------------------------------------------------------------

func TestSessionHistoryEnabled_Default(t *testing.T) {
	if !SessionHistoryEnabled(context.Background()) {
		t.Error("SessionHistoryEnabled should default to true")
	}
}

func TestSessionHistoryEnabled_Nil(t *testing.T) {
	if !SessionHistoryEnabled(nil) {
		t.Error("SessionHistoryEnabled(nil) should default to true")
	}
}

func TestSessionHistoryEnabled_Disabled(t *testing.T) {
	ctx := WithSessionHistory(context.Background(), false)
	if SessionHistoryEnabled(ctx) {
		t.Error("SessionHistoryEnabled should return false when disabled")
	}
}

func TestSessionHistoryEnabled_Enabled(t *testing.T) {
	ctx := WithSessionHistory(context.Background(), true)
	if !SessionHistoryEnabled(ctx) {
		t.Error("SessionHistoryEnabled should return true when enabled")
	}
}

// ---------------------------------------------------------------------------
// Subagent / unattended markers
// ---------------------------------------------------------------------------

func TestIsSubagentContext_Default(t *testing.T) {
	if IsSubagentContext(context.Background()) {
		t.Error("background context should not be subagent")
	}
}

func TestIsSubagentContext_Marked(t *testing.T) {
	ctx := MarkSubagentContext(context.Background())
	if !IsSubagentContext(ctx) {
		t.Error("marked context should be subagent")
	}
}

func TestIsUnattendedContext_Default(t *testing.T) {
	if IsUnattendedContext(context.Background()) {
		t.Error("background context should not be unattended")
	}
}

func TestIsUnattendedContext_Marked(t *testing.T) {
	ctx := MarkUnattendedContext(context.Background())
	if !IsUnattendedContext(ctx) {
		t.Error("marked context should be unattended")
	}
}

// ---------------------------------------------------------------------------
// Plan review
// ---------------------------------------------------------------------------

func TestPlanReviewEnabled_Default(t *testing.T) {
	if PlanReviewEnabled(context.Background()) {
		t.Error("PlanReviewEnabled should default to false")
	}
}

func TestPlanReviewEnabled_Nil(t *testing.T) {
	if PlanReviewEnabled(nil) {
		t.Error("PlanReviewEnabled(nil) should return false")
	}
}

func TestPlanReviewEnabled_Set(t *testing.T) {
	ctx := WithPlanReviewEnabled(context.Background(), true)
	if !PlanReviewEnabled(ctx) {
		t.Error("PlanReviewEnabled should return true when set")
	}
}

// ---------------------------------------------------------------------------
// Memory policy
// ---------------------------------------------------------------------------

func TestMemoryPolicyFromContext_Unset(t *testing.T) {
	_, ok := MemoryPolicyFromContext(context.Background())
	if ok {
		t.Error("MemoryPolicyFromContext should return false when unset")
	}
}

func TestMemoryPolicyFromContext_Nil(t *testing.T) {
	_, ok := MemoryPolicyFromContext(nil)
	if ok {
		t.Error("MemoryPolicyFromContext(nil) should return false")
	}
}

func TestMemoryPolicyFromContext_Set(t *testing.T) {
	policy := MemoryPolicy{Enabled: true, AutoRecall: true}
	ctx := WithMemoryPolicy(context.Background(), policy)
	got, ok := MemoryPolicyFromContext(ctx)
	if !ok {
		t.Fatal("MemoryPolicyFromContext should return true when set")
	}
	if !got.Enabled || !got.AutoRecall {
		t.Errorf("policy = %+v, want Enabled=true AutoRecall=true", got)
	}
}

func TestResolveMemoryPolicy_Default(t *testing.T) {
	got := ResolveMemoryPolicy(context.Background())
	if !got.Enabled {
		t.Error("default policy should be Enabled")
	}
	if !got.AutoRecall {
		t.Error("default policy should have AutoRecall")
	}
	if !got.AutoCapture {
		t.Error("default policy should have AutoCapture")
	}
	if got.CaptureMessages {
		t.Error("default policy should not have CaptureMessages")
	}
	if !got.RefreshEnabled {
		t.Error("default policy should have RefreshEnabled")
	}
}

func TestResolveMemoryPolicy_Custom(t *testing.T) {
	policy := MemoryPolicy{Enabled: false}
	ctx := WithMemoryPolicy(context.Background(), policy)
	got := ResolveMemoryPolicy(ctx)
	if got.Enabled {
		t.Error("custom policy should override Enabled to false")
	}
}

// ---------------------------------------------------------------------------
// LLM selection
// ---------------------------------------------------------------------------

func TestGetLLMSelection_Nil(t *testing.T) {
	_, ok := GetLLMSelection(nil)
	if ok {
		t.Error("GetLLMSelection(nil) should return false")
	}
}

func TestGetLLMSelection_Unset(t *testing.T) {
	_, ok := GetLLMSelection(context.Background())
	if ok {
		t.Error("GetLLMSelection should return false when unset")
	}
}

func TestGetLLMSelection_Set(t *testing.T) {
	sel := subscription.ResolvedSelection{Provider: "openai", Model: "gpt-4"}
	ctx := WithLLMSelection(context.Background(), sel)
	got, ok := GetLLMSelection(ctx)
	if !ok {
		t.Fatal("GetLLMSelection should return true when set")
	}
	if got.Provider != "openai" || got.Model != "gpt-4" {
		t.Errorf("selection = %+v, want openai/gpt-4", got)
	}
}

func TestPropagateLLMSelection_Copies(t *testing.T) {
	sel := subscription.ResolvedSelection{Provider: "ark", Model: "doubao-pro"}
	from := WithLLMSelection(context.Background(), sel)
	to := context.Background()

	result := PropagateLLMSelection(from, to)
	got, ok := GetLLMSelection(result)
	if !ok {
		t.Fatal("selection should be propagated")
	}
	if got.Provider != "ark" {
		t.Errorf("propagated provider = %q, want ark", got.Provider)
	}
}

func TestPropagateLLMSelection_NoopWhenMissing(t *testing.T) {
	from := context.Background()
	to := context.Background()
	result := PropagateLLMSelection(from, to)
	if _, ok := GetLLMSelection(result); ok {
		t.Error("should not propagate when source has no selection")
	}
}

// ---------------------------------------------------------------------------
// Attachments
// ---------------------------------------------------------------------------

func TestGetUserAttachments_Nil(t *testing.T) {
	if got := GetUserAttachments(nil); got != nil {
		t.Errorf("GetUserAttachments(nil) = %v, want nil", got)
	}
}

func TestGetUserAttachments_Unset(t *testing.T) {
	if got := GetUserAttachments(context.Background()); got != nil {
		t.Errorf("GetUserAttachments(empty) = %v, want nil", got)
	}
}

func TestWithUserAttachments_Empty(t *testing.T) {
	ctx := WithUserAttachments(context.Background(), nil)
	// Should return the same context (no-op for empty)
	if got := GetUserAttachments(ctx); got != nil {
		t.Errorf("GetUserAttachments after empty set = %v, want nil", got)
	}
}

func TestWithUserAttachments_RoundTrip(t *testing.T) {
	attachments := []ports.Attachment{
		{Name: "file.txt", MediaType: "text/plain"},
		{Name: "image.png", MediaType: "image/png"},
	}
	ctx := WithUserAttachments(context.Background(), attachments)
	got := GetUserAttachments(ctx)
	if len(got) != 2 {
		t.Fatalf("got %d attachments, want 2", len(got))
	}
	if got[0].Name != "file.txt" || got[1].Name != "image.png" {
		t.Errorf("attachment names = %q, %q, want file.txt, image.png", got[0].Name, got[1].Name)
	}
}

func TestWithUserAttachments_Clones(t *testing.T) {
	attachments := []ports.Attachment{
		{Name: "original", MediaType: "text/plain"},
	}
	ctx := WithUserAttachments(context.Background(), attachments)

	// Mutate original — stored copy should be unaffected.
	attachments[0].Name = "mutated"

	got := GetUserAttachments(ctx)
	if got[0].Name != "original" {
		t.Errorf("stored attachment name = %q, want original (should be cloned)", got[0].Name)
	}
}

func TestGetInheritedAttachments_Nil(t *testing.T) {
	a, i := GetInheritedAttachments(nil)
	if a != nil || i != nil {
		t.Error("GetInheritedAttachments(nil) should return nil, nil")
	}
}

func TestGetInheritedAttachments_Unset(t *testing.T) {
	a, i := GetInheritedAttachments(context.Background())
	if a != nil || i != nil {
		t.Error("GetInheritedAttachments(empty) should return nil, nil")
	}
}

// ---------------------------------------------------------------------------
// Preset config
// ---------------------------------------------------------------------------

func TestPresetConfig_StoredViaContext(t *testing.T) {
	preset := PresetConfig{AgentPreset: "leader", ToolPreset: "full"}
	ctx := context.WithValue(context.Background(), PresetContextKey{}, preset)
	got, ok := ctx.Value(PresetContextKey{}).(PresetConfig)
	if !ok {
		t.Fatal("PresetConfig should be retrievable from context")
	}
	if got.AgentPreset != "leader" || got.ToolPreset != "full" {
		t.Errorf("preset = %+v, want leader/full", got)
	}
}

// ---------------------------------------------------------------------------
// Combined context stacking
// ---------------------------------------------------------------------------

func TestContextStacking(t *testing.T) {
	ctx := context.Background()
	ctx = WithChannel(ctx, "lark")
	ctx = WithChatID(ctx, "oc_test")
	ctx = WithSessionHistory(ctx, false)
	ctx = MarkSubagentContext(ctx)
	ctx = WithPlanReviewEnabled(ctx, true)

	if ChannelFromContext(ctx) != "lark" {
		t.Error("channel lost after stacking")
	}
	if ChatIDFromContext(ctx) != "oc_test" {
		t.Error("chatID lost after stacking")
	}
	if SessionHistoryEnabled(ctx) {
		t.Error("session history should be disabled")
	}
	if !IsSubagentContext(ctx) {
		t.Error("subagent marker lost after stacking")
	}
	if !PlanReviewEnabled(ctx) {
		t.Error("plan review lost after stacking")
	}
}
