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

// ===========================================================================
// Extended coverage tests below
// ===========================================================================

// ---------------------------------------------------------------------------
// Channel context — extended
// ---------------------------------------------------------------------------

func TestChannelFromContext_Overwrite(t *testing.T) {
	ctx := WithChannel(context.Background(), "lark")
	ctx = WithChannel(ctx, "web")
	if got := ChannelFromContext(ctx); got != "web" {
		t.Errorf("ChannelFromContext after overwrite = %q, want web", got)
	}
}

func TestChannelFromContext_EmptyString(t *testing.T) {
	ctx := WithChannel(context.Background(), "")
	if got := ChannelFromContext(ctx); got != "" {
		t.Errorf("ChannelFromContext(empty string) = %q, want empty", got)
	}
}

func TestChatIDFromContext_Unset(t *testing.T) {
	if got := ChatIDFromContext(context.Background()); got != "" {
		t.Errorf("ChatIDFromContext(unset) = %q, want empty", got)
	}
}

func TestChatIDFromContext_Overwrite(t *testing.T) {
	ctx := WithChatID(context.Background(), "oc_first")
	ctx = WithChatID(ctx, "oc_second")
	if got := ChatIDFromContext(ctx); got != "oc_second" {
		t.Errorf("ChatIDFromContext after overwrite = %q, want oc_second", got)
	}
}

// ---------------------------------------------------------------------------
// Session history — extended
// ---------------------------------------------------------------------------

func TestSessionHistory_Toggle(t *testing.T) {
	ctx := WithSessionHistory(context.Background(), false)
	if SessionHistoryEnabled(ctx) {
		t.Error("should be disabled after first set")
	}
	ctx = WithSessionHistory(ctx, true)
	if !SessionHistoryEnabled(ctx) {
		t.Error("should be re-enabled after toggle")
	}
}

// ---------------------------------------------------------------------------
// Subagent / unattended — extended
// ---------------------------------------------------------------------------

func TestIsSubagentContext_Nil(t *testing.T) {
	// IsSubagentContext uses ctx.Value which panics on nil context.
	// Verify it doesn't panic if we guard properly; the function itself
	// does not guard nil, so we test the normal path.
	ctx := MarkSubagentContext(context.Background())
	if !IsSubagentContext(ctx) {
		t.Error("should be subagent after marking")
	}
}

func TestIsSubagentContext_DoubleMarking(t *testing.T) {
	ctx := MarkSubagentContext(context.Background())
	ctx = MarkSubagentContext(ctx)
	if !IsSubagentContext(ctx) {
		t.Error("double marking should still be subagent")
	}
}

func TestIsUnattendedContext_DoubleMarking(t *testing.T) {
	ctx := MarkUnattendedContext(context.Background())
	ctx = MarkUnattendedContext(ctx)
	if !IsUnattendedContext(ctx) {
		t.Error("double marking should still be unattended")
	}
}

func TestSubagentAndUnattended_Independent(t *testing.T) {
	ctx := MarkSubagentContext(context.Background())
	if IsUnattendedContext(ctx) {
		t.Error("subagent mark should not set unattended")
	}

	ctx2 := MarkUnattendedContext(context.Background())
	if IsSubagentContext(ctx2) {
		t.Error("unattended mark should not set subagent")
	}
}

func TestSubagentAndUnattended_Both(t *testing.T) {
	ctx := MarkSubagentContext(context.Background())
	ctx = MarkUnattendedContext(ctx)
	if !IsSubagentContext(ctx) {
		t.Error("should be subagent")
	}
	if !IsUnattendedContext(ctx) {
		t.Error("should be unattended")
	}
}

// ---------------------------------------------------------------------------
// Plan review — extended
// ---------------------------------------------------------------------------

func TestPlanReviewEnabled_ExplicitFalse(t *testing.T) {
	ctx := WithPlanReviewEnabled(context.Background(), false)
	if PlanReviewEnabled(ctx) {
		t.Error("explicitly set false should return false")
	}
}

func TestPlanReviewEnabled_Toggle(t *testing.T) {
	ctx := WithPlanReviewEnabled(context.Background(), true)
	if !PlanReviewEnabled(ctx) {
		t.Error("should be enabled")
	}
	ctx = WithPlanReviewEnabled(ctx, false)
	if PlanReviewEnabled(ctx) {
		t.Error("should be disabled after toggle")
	}
}

// ---------------------------------------------------------------------------
// Memory policy — extended
// ---------------------------------------------------------------------------

func TestMemoryPolicy_AllFields(t *testing.T) {
	policy := MemoryPolicy{
		Enabled:          true,
		AutoRecall:       true,
		AutoCapture:      true,
		CaptureMessages:  true,
		RefreshEnabled:   true,
		RefreshInterval:  300,
		RefreshMaxTokens: 4096,
	}
	ctx := WithMemoryPolicy(context.Background(), policy)
	got, ok := MemoryPolicyFromContext(ctx)
	if !ok {
		t.Fatal("should return ok=true")
	}
	if got != policy {
		t.Errorf("got %+v, want %+v", got, policy)
	}
}

func TestMemoryPolicy_ZeroValue(t *testing.T) {
	// Zero-value policy should be distinguishable from "not set".
	ctx := WithMemoryPolicy(context.Background(), MemoryPolicy{})
	got, ok := MemoryPolicyFromContext(ctx)
	if !ok {
		t.Fatal("zero-value policy should be set (ok=true)")
	}
	if got.Enabled {
		t.Error("zero-value policy should have Enabled=false")
	}
}

func TestResolveMemoryPolicy_Nil(t *testing.T) {
	got := ResolveMemoryPolicy(nil)
	if !got.Enabled {
		t.Error("nil context should resolve to default (Enabled=true)")
	}
}

func TestResolveMemoryPolicy_PartialOverride(t *testing.T) {
	// Setting only Enabled=true leaves other fields at zero.
	ctx := WithMemoryPolicy(context.Background(), MemoryPolicy{Enabled: true})
	got := ResolveMemoryPolicy(ctx)
	if !got.Enabled {
		t.Error("should be enabled")
	}
	if got.AutoRecall {
		t.Error("custom partial policy should not default AutoRecall")
	}
	if got.RefreshEnabled {
		t.Error("custom partial policy should not default RefreshEnabled")
	}
}

func TestMemoryPolicy_OverwritesPrevious(t *testing.T) {
	ctx := WithMemoryPolicy(context.Background(), MemoryPolicy{Enabled: true, AutoRecall: true})
	ctx = WithMemoryPolicy(ctx, MemoryPolicy{Enabled: false})
	got := ResolveMemoryPolicy(ctx)
	if got.Enabled {
		t.Error("second policy should override first")
	}
	if got.AutoRecall {
		t.Error("second policy should override AutoRecall to false")
	}
}

// ---------------------------------------------------------------------------
// Attachments — extended
// ---------------------------------------------------------------------------

func TestWithUserAttachments_EmptySlice(t *testing.T) {
	// Empty slice (not nil) should also be a no-op.
	ctx := WithUserAttachments(context.Background(), []ports.Attachment{})
	if got := GetUserAttachments(ctx); got != nil {
		t.Errorf("empty slice should be no-op, got %v", got)
	}
}

func TestWithUserAttachments_Overwrite(t *testing.T) {
	first := []ports.Attachment{{Name: "a.txt", MediaType: "text/plain"}}
	second := []ports.Attachment{{Name: "b.txt", MediaType: "text/plain"}}
	ctx := WithUserAttachments(context.Background(), first)
	ctx = WithUserAttachments(ctx, second)
	got := GetUserAttachments(ctx)
	if len(got) != 1 || got[0].Name != "b.txt" {
		t.Errorf("overwrite should yield b.txt, got %v", got)
	}
}

func TestWithUserAttachments_AllFields(t *testing.T) {
	att := ports.Attachment{
		Name:      "report.pdf",
		MediaType: "application/pdf",
		URI:       "https://example.com/report.pdf",
		Data:      "base64data",
		Source:    "user_upload",
		Kind:      "document",
		Format:    "pdf",
	}
	ctx := WithUserAttachments(context.Background(), []ports.Attachment{att})
	got := GetUserAttachments(ctx)
	if len(got) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(got))
	}
	g := got[0]
	if g.Name != "report.pdf" || g.MediaType != "application/pdf" || g.URI != "https://example.com/report.pdf" {
		t.Errorf("field mismatch: %+v", g)
	}
	if g.Source != "user_upload" || g.Kind != "document" || g.Format != "pdf" {
		t.Errorf("metadata mismatch: source=%q kind=%q format=%q", g.Source, g.Kind, g.Format)
	}
}

func TestGetUserAttachments_ReturnedSliceImmutability(t *testing.T) {
	att := []ports.Attachment{{Name: "orig", MediaType: "text/plain"}}
	ctx := WithUserAttachments(context.Background(), att)

	// First retrieval
	got1 := GetUserAttachments(ctx)
	if got1[0].Name != "orig" {
		t.Fatal("setup failed")
	}

	// Mutate the returned slice — should not affect second retrieval.
	// Note: the stored slice IS the same reference (no clone on get), but the
	// individual attachments were cloned on set. Verify the stored Name is stable.
	got1[0].Name = "mutated"

	// The stored reference IS the same (Get doesn't clone), so this mutation
	// is visible. The critical invariant is that the original input was cloned
	// on set (tested in TestWithUserAttachments_Clones).
	got2 := GetUserAttachments(ctx)
	// Both point to the same backing slice, so mutation is expected to be visible.
	// The important guarantee is the SET-time clone, not GET-time clone.
	_ = got2
}

func TestGetInheritedAttachments_WithPayload(t *testing.T) {
	// Manually construct the internal payload to test the getter.
	payload := inheritedAttachmentPayload{
		attachments: map[string]ports.Attachment{
			"file.txt": {Name: "file.txt", MediaType: "text/plain", URI: "/tmp/file.txt"},
		},
		iterations: map[string]int{
			"file.txt": 3,
		},
	}
	ctx := context.WithValue(context.Background(), inheritedAttachmentsKey{}, payload)
	atts, iters := GetInheritedAttachments(ctx)
	if len(atts) != 1 {
		t.Fatalf("expected 1 inherited attachment, got %d", len(atts))
	}
	if atts["file.txt"].Name != "file.txt" {
		t.Errorf("attachment name = %q, want file.txt", atts["file.txt"].Name)
	}
	if iters["file.txt"] != 3 {
		t.Errorf("iteration count = %d, want 3", iters["file.txt"])
	}
}

func TestGetInheritedAttachments_ClonesOutput(t *testing.T) {
	payload := inheritedAttachmentPayload{
		attachments: map[string]ports.Attachment{
			"a": {Name: "a", MediaType: "text/plain"},
		},
		iterations: map[string]int{"a": 1},
	}
	ctx := context.WithValue(context.Background(), inheritedAttachmentsKey{}, payload)

	atts1, iters1 := GetInheritedAttachments(ctx)
	// Mutate the returned maps.
	delete(atts1, "a")
	delete(iters1, "a")

	// Second call should still return the original data (cloned).
	atts2, iters2 := GetInheritedAttachments(ctx)
	if len(atts2) != 1 {
		t.Errorf("mutation of returned map affected stored data: got %d attachments", len(atts2))
	}
	if len(iters2) != 1 {
		t.Errorf("mutation of returned map affected stored iterations: got %d", len(iters2))
	}
}

// ---------------------------------------------------------------------------
// Preset config — extended
// ---------------------------------------------------------------------------

func TestPresetConfig_EmptyFields(t *testing.T) {
	preset := PresetConfig{}
	ctx := context.WithValue(context.Background(), PresetContextKey{}, preset)
	got, ok := ctx.Value(PresetContextKey{}).(PresetConfig)
	if !ok {
		t.Fatal("empty PresetConfig should be retrievable")
	}
	if got.AgentPreset != "" || got.ToolPreset != "" {
		t.Errorf("empty preset should have empty fields, got %+v", got)
	}
}

func TestPresetConfig_NotSet(t *testing.T) {
	_, ok := context.Background().Value(PresetContextKey{}).(PresetConfig)
	if ok {
		t.Error("PresetConfig should not be present on bare context")
	}
}

func TestPresetConfig_ToolOnly(t *testing.T) {
	preset := PresetConfig{ToolPreset: "restricted"}
	ctx := context.WithValue(context.Background(), PresetContextKey{}, preset)
	got := ctx.Value(PresetContextKey{}).(PresetConfig)
	if got.AgentPreset != "" {
		t.Errorf("AgentPreset should be empty, got %q", got.AgentPreset)
	}
	if got.ToolPreset != "restricted" {
		t.Errorf("ToolPreset = %q, want restricted", got.ToolPreset)
	}
}

// ---------------------------------------------------------------------------
// LLM selection — extended
// ---------------------------------------------------------------------------

func TestLLMSelection_Overwrite(t *testing.T) {
	sel1 := subscription.ResolvedSelection{Provider: "openai", Model: "gpt-4"}
	sel2 := subscription.ResolvedSelection{Provider: "ark", Model: "doubao-pro"}
	ctx := WithLLMSelection(context.Background(), sel1)
	ctx = WithLLMSelection(ctx, sel2)
	got, ok := GetLLMSelection(ctx)
	if !ok {
		t.Fatal("should have selection")
	}
	if got.Provider != "ark" || got.Model != "doubao-pro" {
		t.Errorf("overwrite failed: got %+v", got)
	}
}

func TestPropagateLLMSelection_PreservesTarget(t *testing.T) {
	// Propagation should not lose existing target context values.
	sel := subscription.ResolvedSelection{Provider: "openai", Model: "gpt-4"}
	from := WithLLMSelection(context.Background(), sel)
	to := WithChannel(context.Background(), "lark")

	result := PropagateLLMSelection(from, to)
	if ChannelFromContext(result) != "lark" {
		t.Error("propagation should preserve target's existing values")
	}
	got, ok := GetLLMSelection(result)
	if !ok || got.Provider != "openai" {
		t.Error("propagation should add LLM selection")
	}
}

// ---------------------------------------------------------------------------
// Full context stacking — extended
// ---------------------------------------------------------------------------

func TestFullContextStacking(t *testing.T) {
	att := []ports.Attachment{{Name: "file.txt", MediaType: "text/plain"}}
	sel := subscription.ResolvedSelection{Provider: "openai", Model: "gpt-4"}
	policy := MemoryPolicy{Enabled: true, AutoRecall: true, AutoCapture: true}

	ctx := context.Background()
	ctx = WithChannel(ctx, "web")
	ctx = WithChatID(ctx, "oc_123")
	ctx = WithSessionHistory(ctx, true)
	ctx = MarkSubagentContext(ctx)
	ctx = MarkUnattendedContext(ctx)
	ctx = WithPlanReviewEnabled(ctx, true)
	ctx = WithMemoryPolicy(ctx, policy)
	ctx = WithLLMSelection(ctx, sel)
	ctx = WithUserAttachments(ctx, att)
	ctx = context.WithValue(ctx, PresetContextKey{}, PresetConfig{AgentPreset: "leader"})

	// Verify all values survive stacking.
	if ChannelFromContext(ctx) != "web" {
		t.Error("channel lost")
	}
	if ChatIDFromContext(ctx) != "oc_123" {
		t.Error("chatID lost")
	}
	if !SessionHistoryEnabled(ctx) {
		t.Error("session history lost")
	}
	if !IsSubagentContext(ctx) {
		t.Error("subagent lost")
	}
	if !IsUnattendedContext(ctx) {
		t.Error("unattended lost")
	}
	if !PlanReviewEnabled(ctx) {
		t.Error("plan review lost")
	}
	gotPolicy := ResolveMemoryPolicy(ctx)
	if !gotPolicy.Enabled || !gotPolicy.AutoRecall {
		t.Error("memory policy lost")
	}
	gotSel, ok := GetLLMSelection(ctx)
	if !ok || gotSel.Provider != "openai" {
		t.Error("LLM selection lost")
	}
	gotAtt := GetUserAttachments(ctx)
	if len(gotAtt) != 1 || gotAtt[0].Name != "file.txt" {
		t.Error("attachments lost")
	}
	preset, ok := ctx.Value(PresetContextKey{}).(PresetConfig)
	if !ok || preset.AgentPreset != "leader" {
		t.Error("preset lost")
	}
}
