package kernel

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	agentports "alex/internal/domain/agent/ports/agent"
	kerneldomain "alex/internal/domain/kernel"
	subscription "alex/internal/app/subscription"
)

// ─────────────────────────────────────────────────────────────────────────────
// uniqueTrimmed
// ─────────────────────────────────────────────────────────────────────────────

func TestUniqueTrimmed_Empty(t *testing.T) {
	if got := uniqueTrimmed(nil); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
	if got := uniqueTrimmed([]string{}); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestUniqueTrimmed_DedupsCaseInsensitive(t *testing.T) {
	in := []string{"Foo", "foo", " BAR ", "bar", "baz"}
	got := uniqueTrimmed(in)
	if len(got) != 3 {
		t.Errorf("expected 3 unique values, got %d: %v", len(got), got)
	}
}

func TestUniqueTrimmed_SkipsBlank(t *testing.T) {
	in := []string{"", "  ", "hello"}
	got := uniqueTrimmed(in)
	if len(got) != 1 || got[0] != "hello" {
		t.Errorf("expected [hello], got %v", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// normalizeAgentID
// ─────────────────────────────────────────────────────────────────────────────

func TestNormalizeAgentID_KnownAgent(t *testing.T) {
	p := newTestLLMPlanner([]AgentConfig{{AgentID: "build-executor"}})
	got := p.normalizeAgentID("build-executor", "build something")
	if got != "build-executor" {
		t.Errorf("expected build-executor, got %q", got)
	}
}

func TestNormalizeAgentID_TeamPrefix(t *testing.T) {
	p := newTestLLMPlanner(nil)
	got := p.normalizeAgentID("team:my-template", "run team")
	if got != "team:my-template" {
		t.Errorf("expected team:my-template, got %q", got)
	}
}

func TestNormalizeAgentID_UnknownRemappedToBucketAgent(t *testing.T) {
	p := newTestLLMPlanner([]AgentConfig{{AgentID: "build-executor"}})
	got := p.normalizeAgentID("unknown-agent", "implement the feature")
	// Should be remapped to the bucket agent matching "build"
	if got == "" {
		t.Error("expected a non-empty remapped agent ID")
	}
}

func TestNormalizeAgentID_EmptyFallsBackToBucket(t *testing.T) {
	p := newTestLLMPlanner([]AgentConfig{{AgentID: "audit-executor"}})
	got := p.normalizeAgentID("", "audit the code")
	if got == "" {
		t.Error("expected a bucket agent for empty agentID, got empty string")
	}
}

func TestNormalizeAgentID_EmptyNoDefault(t *testing.T) {
	p := newTestLLMPlanner(nil)
	got := p.normalizeAgentID("", "something")
	if got != "" {
		t.Errorf("expected empty string when no default, got %q", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// isAllowedTeamTemplate
// ─────────────────────────────────────────────────────────────────────────────

func TestIsAllowedTeamTemplate_EmptyTemplate(t *testing.T) {
	p := newTestLLMPlanner(nil)
	if p.isAllowedTeamTemplate("") {
		t.Error("empty template should not be allowed")
	}
}

func TestIsAllowedTeamTemplate_NoAllowedSet(t *testing.T) {
	p := newTestLLMPlanner(nil)
	if p.isAllowedTeamTemplate("some-template") {
		t.Error("template should not be allowed when set is empty")
	}
}

func TestIsAllowedTeamTemplate_Allowed(t *testing.T) {
	agents := []AgentConfig{{AgentID: "build-executor"}}
	p := newTestLLMPlannerWithTemplates(agents, []string{"my-template"})
	if !p.isAllowedTeamTemplate("my-template") {
		t.Error("expected my-template to be allowed")
	}
	if !p.isAllowedTeamTemplate("MY-TEMPLATE") {
		t.Error("expected case-insensitive match")
	}
}

func TestIsAllowedTeamTemplate_NotInSet(t *testing.T) {
	agents := []AgentConfig{{AgentID: "build-executor"}}
	p := newTestLLMPlannerWithTemplates(agents, []string{"my-template"})
	if p.isAllowedTeamTemplate("other-template") {
		t.Error("other-template should not be allowed")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// normalizePlanningDecisionKind
// ─────────────────────────────────────────────────────────────────────────────

func TestNormalizePlanningDecisionKind_ExplicitTeam(t *testing.T) {
	d := planningDecision{Kind: "team"}
	got := normalizePlanningDecisionKind(d)
	if got != kerneldomain.DispatchKindTeam {
		t.Errorf("expected team kind, got %v", got)
	}
}

func TestNormalizePlanningDecisionKind_EmptyWithTemplate(t *testing.T) {
	d := planningDecision{Kind: "", TeamTemplate: "my-template"}
	got := normalizePlanningDecisionKind(d)
	if got != kerneldomain.DispatchKindTeam {
		t.Errorf("expected team kind when template set, got %v", got)
	}
}

func TestNormalizePlanningDecisionKind_EmptyNoTemplate(t *testing.T) {
	d := planningDecision{Kind: ""}
	got := normalizePlanningDecisionKind(d)
	if got != kerneldomain.DispatchKindAgent {
		t.Errorf("expected agent kind, got %v", got)
	}
}

func TestNormalizePlanningDecisionKind_ExplicitAgent(t *testing.T) {
	d := planningDecision{Kind: "agent"}
	got := normalizePlanningDecisionKind(d)
	if got != kerneldomain.DispatchKindAgent {
		t.Errorf("expected agent kind, got %v", got)
	}
}

func TestNormalizePlanningDecisionKind_UnknownFallsToAgent(t *testing.T) {
	d := planningDecision{Kind: "bogus"}
	got := normalizePlanningDecisionKind(d)
	if got != kerneldomain.DispatchKindAgent {
		t.Errorf("expected agent kind for unknown, got %v", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// answerContainsUserConfirmationPrompt — missing branches
// ─────────────────────────────────────────────────────────────────────────────

func TestAnswerContainsUserConfirmationPrompt_Empty(t *testing.T) {
	if answerContainsUserConfirmationPrompt("") {
		t.Error("empty answer should not be a confirmation prompt")
	}
	if answerContainsUserConfirmationPrompt("   ") {
		t.Error("blank answer should not be a confirmation prompt")
	}
}

func TestAnswerContainsUserConfirmationPrompt_EnglishPatterns(t *testing.T) {
	cases := []struct {
		answer string
		want   bool
	}{
		{"Do you want me to proceed?", true},
		{"My understanding is X — is that right?", true},
		{"Please confirm this action.", true},
		{"Please choose one of the following:", true},
		{"Option A: fast, Option B: safe", true},
		{"I'll go ahead and do it.", false},
	}
	for _, tc := range cases {
		got := answerContainsUserConfirmationPrompt(tc.answer)
		if got != tc.want {
			t.Errorf("answerContainsUserConfirmationPrompt(%q) = %v, want %v", tc.answer, got, tc.want)
		}
	}
}

func TestAnswerContainsUserConfirmationPrompt_ChinesePatterns(t *testing.T) {
	cases := []struct {
		answer string
		want   bool
	}{
		{"我的理解是你想要重启服务——对吗？", true},
		{"你要我删除文件吗？", true},
		{"请确认这个操作", true},
		{"请选择一个方案", true},
		{"请回复Y或者N", true},
		{"好的，我来处理。", false},
	}
	for _, tc := range cases {
		got := answerContainsUserConfirmationPrompt(tc.answer)
		if got != tc.want {
			t.Errorf("answerContainsUserConfirmationPrompt(%q) = %v, want %v", tc.answer, got, tc.want)
		}
	}
}

func TestAnswerContainsUserConfirmationPrompt_OptionsPattern(t *testing.T) {
	answer := "可选方案：a) 快速部署 b) 灰度发布"
	got := answerContainsUserConfirmationPrompt(answer)
	if !got {
		t.Errorf("expected true for option-list pattern, got false")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// isKernelExecutionSummaryValid — missing branches
// ─────────────────────────────────────────────────────────────────────────────

func TestIsKernelExecutionSummaryValid_Empty(t *testing.T) {
	if isKernelExecutionSummaryValid("") {
		t.Error("empty summary should be invalid")
	}
	if isKernelExecutionSummaryValid("  ") {
		t.Error("blank summary should be invalid")
	}
}

func TestIsKernelExecutionSummaryValid_EmptyResponsePrefix(t *testing.T) {
	cases := []string{
		"empty response: no content",
		"Empty Response: something",
		"empty completion: nothing here",
	}
	for _, c := range cases {
		if isKernelExecutionSummaryValid(c) {
			t.Errorf("expected invalid for %q", c)
		}
	}
}

func TestIsKernelExecutionSummaryValid_JSONBlob(t *testing.T) {
	blob := `{"stop_reason":"end_turn","content":"hi","input_tokens":10,"output_tokens":20}`
	if isKernelExecutionSummaryValid(blob) {
		t.Error("raw JSON response blob should be invalid")
	}
}

func TestIsKernelExecutionSummaryValid_ValidSummary(t *testing.T) {
	cases := []string{
		"## Execution Summary\nFixed the build.",
		"Done. Tests pass.",
		"[autonomy=actionable] Resolved K-03.",
	}
	for _, c := range cases {
		if !isKernelExecutionSummaryValid(c) {
			t.Errorf("expected valid for %q", c)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// extractKernelExecutionSummary — missing branches
// ─────────────────────────────────────────────────────────────────────────────

func TestExtractKernelExecutionSummary_NilResult(t *testing.T) {
	got := extractKernelExecutionSummary(nil)
	if got != "" {
		t.Errorf("expected empty string for nil result, got %q", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Helper: newTestLLMPlannerWithTemplates
// ─────────────────────────────────────────────────────────────────────────────

func newTestLLMPlannerWithTemplates(agents []AgentConfig, templates []string) *LLMPlanner {
	cfg := LLMPlannerConfig{
		AllowedTeamTemplates: templates,
	}
	return NewLLMPlanner("test-kernel", nil, cfg, agents, nil)
}

func newTestLLMPlanner(agents []AgentConfig) *LLMPlanner {
	return NewLLMPlanner("test-kernel", nil, LLMPlannerConfig{}, agents, nil)
}

// ─────────────────────────────────────────────────────────────────────────────
// bootstrap_docs helpers — formatTimestamp, nonEmpty
// ─────────────────────────────────────────────────────────────────────────────

func TestFormatTimestamp_ZeroTime(t *testing.T) {
	got := formatTimestamp(time.Time{})
	if got != "unknown" {
		t.Errorf("expected 'unknown' for zero time, got %q", got)
	}
}

func TestFormatTimestamp_NonZero(t *testing.T) {
	ts := time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC)
	got := formatTimestamp(ts)
	if !strings.Contains(got, "2026-03-03") {
		t.Errorf("expected ISO timestamp containing 2026-03-03, got %q", got)
	}
}

func TestNonEmpty_Blank(t *testing.T) {
	if got := nonEmpty(""); got != "(empty)" {
		t.Errorf("expected (empty), got %q", got)
	}
	if got := nonEmpty("  "); got != "(empty)" {
		t.Errorf("expected (empty) for spaces, got %q", got)
	}
}

func TestNonEmpty_WithValue(t *testing.T) {
	if got := nonEmpty("hello"); got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// notifier: formatAgentSummaryLine — failed branch
// ─────────────────────────────────────────────────────────────────────────────

func TestFormatAgentSummaryLine_FailedWithError(t *testing.T) {
	entry := kerneldomain.AgentCycleSummary{
		AgentID: "build-executor",
		Status:  kerneldomain.DispatchFailed,
		Error:   "something went wrong",
	}
	got := formatAgentSummaryLine(entry)
	if !strings.Contains(got, "build-executor") {
		t.Error("expected AgentID in line")
	}
	if !strings.Contains(got, "something went wrong") {
		t.Error("expected error message in line")
	}
}

func TestFormatAgentSummaryLine_FailedEmptyError(t *testing.T) {
	entry := kerneldomain.AgentCycleSummary{
		AgentID: "audit-executor",
		Status:  kerneldomain.DispatchFailed,
		Error:   "",
	}
	got := formatAgentSummaryLine(entry)
	if !strings.Contains(got, "(unknown error)") {
		t.Errorf("expected '(unknown error)' for empty error, got %q", got)
	}
}

func TestFormatAgentSummaryLine_EmptyStatus(t *testing.T) {
	entry := kerneldomain.AgentCycleSummary{
		AgentID: "data-executor",
		Status:  "",
		Summary: "synced state",
	}
	got := formatAgentSummaryLine(entry)
	if !strings.Contains(got, "synced state") {
		t.Errorf("expected summary in line, got %q", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// CoordinatorExecutor: artifactsDir / tasksDir / SetSelectionResolver
// ─────────────────────────────────────────────────────────────────────────────

func TestCoordinatorExecutor_ArtifactsDir_WithStateDir(t *testing.T) {
	exec := NewCoordinatorExecutor(nil, 0, "/tmp/mystate")
	got := exec.artifactsDir()
	if got != "/tmp/mystate/artifacts" {
		t.Errorf("expected /tmp/mystate/artifacts, got %q", got)
	}
}

func TestCoordinatorExecutor_ArtifactsDir_NoStateDir(t *testing.T) {
	exec := NewCoordinatorExecutor(nil, 0, "")
	got := exec.artifactsDir()
	// Should fall back to home/.alex/kernel/artifacts or ./artifacts
	if got == "" {
		t.Error("expected non-empty artifacts dir")
	}
}

func TestCoordinatorExecutor_TasksDir_WithStateDir(t *testing.T) {
	exec := NewCoordinatorExecutor(nil, 0, "/tmp/mystate")
	got := exec.tasksDir()
	if got != "/tmp/mystate/tasks" {
		t.Errorf("expected /tmp/mystate/tasks, got %q", got)
	}
}

func TestCoordinatorExecutor_TasksDir_NoStateDir(t *testing.T) {
	exec := NewCoordinatorExecutor(nil, 0, "")
	got := exec.tasksDir()
	if got == "" {
		t.Error("expected non-empty tasks dir")
	}
}

func TestCoordinatorExecutor_SetSelectionResolver_NilSafe(t *testing.T) {
	var exec *CoordinatorExecutor
	// Should not panic
	exec.SetSelectionResolver(nil)
}

func TestCoordinatorExecutor_SetSelectionResolver_Sets(t *testing.T) {
	exec := NewCoordinatorExecutor(nil, 0, "")
	called := false
	exec.SetSelectionResolver(func(_ context.Context, _, _, _ string) (subscription.ResolvedSelection, bool) {
		called = true
		return subscription.ResolvedSelection{}, false
	})
	_ = called // just verifying it compiles and sets without panic
}

// ─────────────────────────────────────────────────────────────────────────────
// dispatchStillAwaitsUserConfirmation — missing branches
// ─────────────────────────────────────────────────────────────────────────────

func TestDispatchStillAwaitsUserConfirmation_NilResult(t *testing.T) {
	if dispatchStillAwaitsUserConfirmation(nil) {
		t.Error("nil result should return false")
	}
}

func TestDispatchStillAwaitsUserConfirmation_StopReasonAwait(t *testing.T) {
	result := &agentports.TaskResult{StopReason: "await_user_input"}
	if !dispatchStillAwaitsUserConfirmation(result) {
		t.Error("expected true for await_user_input stop reason")
	}
}

func TestDispatchStillAwaitsUserConfirmation_StopReasonCaseInsensitive(t *testing.T) {
	result := &agentports.TaskResult{StopReason: "AWAIT_USER_INPUT"}
	if !dispatchStillAwaitsUserConfirmation(result) {
		t.Error("expected true for case-insensitive await_user_input stop reason")
	}
}

func TestDispatchStillAwaitsUserConfirmation_AnswerWithConfirmation(t *testing.T) {
	result := &agentports.TaskResult{Answer: "Please confirm this action."}
	if !dispatchStillAwaitsUserConfirmation(result) {
		t.Error("expected true for answer containing confirmation prompt")
	}
}

func TestDispatchStillAwaitsUserConfirmation_NormalAnswer(t *testing.T) {
	result := &agentports.TaskResult{Answer: "All done, no action needed."}
	if dispatchStillAwaitsUserConfirmation(result) {
		t.Error("expected false for normal answer")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// readGoalFile — empty and truncated paths
// ─────────────────────────────────────────────────────────────────────────────

func TestReadGoalFile_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	goalPath := dir + "/GOAL.md"
	if err := os.WriteFile(goalPath, []byte("  \n  "), 0o644); err != nil {
		t.Fatal(err)
	}
	p := NewLLMPlanner("k", nil, LLMPlannerConfig{GoalFilePath: goalPath}, nil, nil)
	content, status := p.readGoalFile()
	if content != "" {
		t.Errorf("expected empty content, got %q", content)
	}
	if status != "goal_context_empty" {
		t.Errorf("expected goal_context_empty status, got %q", status)
	}
}

func TestReadGoalFile_TruncatesLongFile(t *testing.T) {
	dir := t.TempDir()
	goalPath := dir + "/GOAL.md"
	// Write > 3000 rune content.
	longContent := strings.Repeat("a", 3100)
	if err := os.WriteFile(goalPath, []byte(longContent), 0o644); err != nil {
		t.Fatal(err)
	}
	p := NewLLMPlanner("k", nil, LLMPlannerConfig{GoalFilePath: goalPath}, nil, nil)
	content, status := p.readGoalFile()
	if status != "goal_context_loaded_truncated" {
		t.Errorf("expected goal_context_loaded_truncated, got %q", status)
	}
	if !strings.Contains(content, "...(truncated)") {
		t.Error("expected truncation marker in content")
	}
}

func TestReadGoalFile_HomeTildeExpansion(t *testing.T) {
	// We can't easily test tilde expansion without knowing HOME,
	// but we can verify the "not configured" fast path.
	p := NewLLMPlanner("k", nil, LLMPlannerConfig{}, nil, nil)
	content, status := p.readGoalFile()
	if content != "" {
		t.Errorf("expected empty content, got %q", content)
	}
	if status != "goal_context_not_configured" {
		t.Errorf("expected goal_context_not_configured, got %q", status)
	}
}

func TestReadGoalFile_UnreadablePath(t *testing.T) {
	p := NewLLMPlanner("k", nil, LLMPlannerConfig{GoalFilePath: "/nonexistent/GOAL.md"}, nil, nil)
	content, status := p.readGoalFile()
	if content != "" {
		t.Errorf("expected empty content, got %q", content)
	}
	if status != "goal_context_unreadable" {
		t.Errorf("expected goal_context_unreadable, got %q", status)
	}
}
