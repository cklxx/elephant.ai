package kernel

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"alex/internal/shared/logging"
)

// ─────────────────────────────────────────────────────────────────────────────
// classifyReasonBucket — 0% coverage
// ─────────────────────────────────────────────────────────────────────────────

func TestClassifyReasonBucket_Build(t *testing.T) {
	cases := []struct {
		reason string
		want   string
	}{
		{"implement the new feature", "build"},
		{"fix the broken test", "build"},
		{"deploy to production", "build"},
		{"write code for the handler", "build"},
		{"release version 2.0", "build"},
	}
	for _, tc := range cases {
		got := classifyReasonBucket(tc.reason)
		if got != tc.want {
			t.Errorf("classifyReasonBucket(%q) = %q, want %q", tc.reason, got, tc.want)
		}
	}
}

func TestClassifyReasonBucket_Research(t *testing.T) {
	cases := []struct {
		reason string
		want   string
	}{
		{"research the latency issue", "research"},
		{"investigate memory leak", "research"},
		{"analyze benchmark results", "research"},
		{"compare different approaches", "research"},
	}
	for _, tc := range cases {
		got := classifyReasonBucket(tc.reason)
		if got != tc.want {
			t.Errorf("classifyReasonBucket(%q) = %q, want %q", tc.reason, got, tc.want)
		}
	}
}

func TestClassifyReasonBucket_Outreach(t *testing.T) {
	cases := []struct {
		reason string
		want   string
	}{
		{"send outreach message", "outreach"},
		{"email the team lead", "outreach"},
		{"notify stakeholders", "outreach"},
		{"contact the client", "outreach"},
		{"sync with the PM", "outreach"},
	}
	for _, tc := range cases {
		got := classifyReasonBucket(tc.reason)
		if got != tc.want {
			t.Errorf("classifyReasonBucket(%q) = %q, want %q", tc.reason, got, tc.want)
		}
	}
}

func TestClassifyReasonBucket_Data(t *testing.T) {
	cases := []struct {
		reason string
		want   string
	}{
		{"update state file", "data"},
		{"record the results", "data"},
		{"take a snapshot", "data"},
		{"write artifact log", "data"},
		{"backup data files", "data"},
	}
	for _, tc := range cases {
		got := classifyReasonBucket(tc.reason)
		if got != tc.want {
			t.Errorf("classifyReasonBucket(%q) = %q, want %q", tc.reason, got, tc.want)
		}
	}
}

func TestClassifyReasonBucket_Audit(t *testing.T) {
	cases := []struct {
		reason string
		want   string
	}{
		{"audit the codebase", "audit"},
		{"validate the schema", "audit"},
		{"verify test results", "audit"},
		{"review the PR", "audit"},
		{"check health status", "audit"},
		{"assess risk factors", "audit"},
	}
	for _, tc := range cases {
		got := classifyReasonBucket(tc.reason)
		if got != tc.want {
			t.Errorf("classifyReasonBucket(%q) = %q, want %q", tc.reason, got, tc.want)
		}
	}
}

func TestClassifyReasonBucket_Empty(t *testing.T) {
	got := classifyReasonBucket("")
	if got != "" {
		t.Errorf("classifyReasonBucket('') = %q, want empty string", got)
	}
}

func TestClassifyReasonBucket_NoMatch(t *testing.T) {
	got := classifyReasonBucket("nothing matches here at all")
	if got != "" {
		t.Errorf("classifyReasonBucket('nothing matches') = %q, want empty string", got)
	}
}

func TestClassifyReasonBucket_CaseInsensitive(t *testing.T) {
	got := classifyReasonBucket("BUILD THE FEATURE")
	if got != "build" {
		t.Errorf("classifyReasonBucket('BUILD THE FEATURE') = %q, want %q", got, "build")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// selectBucketAgent — 0% coverage
// ─────────────────────────────────────────────────────────────────────────────

func newTestLLMPlannerWithAgents(agents []AgentConfig) *LLMPlanner {
	return NewLLMPlanner(
		"test-kernel",
		nil, // factory not needed for unit-level bucket tests
		LLMPlannerConfig{},
		agents,
		logging.Nop(),
	)
}

func TestSelectBucketAgent_MatchesBuildExecutor(t *testing.T) {
	p := newTestLLMPlannerWithAgents([]AgentConfig{
		{AgentID: "build-executor"},
		{AgentID: "audit-executor"},
	})
	got := p.selectBucketAgent("implement the new feature")
	if got != "build-executor" {
		t.Errorf("selectBucketAgent('implement...') = %q, want %q", got, "build-executor")
	}
}

func TestSelectBucketAgent_MatchesAuditExecutor(t *testing.T) {
	p := newTestLLMPlannerWithAgents([]AgentConfig{
		{AgentID: "build-executor"},
		{AgentID: "audit-executor"},
	})
	got := p.selectBucketAgent("audit the codebase for quality issues")
	if got != "audit-executor" {
		t.Errorf("selectBucketAgent('audit...') = %q, want %q", got, "audit-executor")
	}
}

func TestSelectBucketAgent_FallsBackToDefaultWhenBucketNotConfigured(t *testing.T) {
	// Only build-executor configured; outreach bucket has no matching agent.
	p := newTestLLMPlannerWithAgents([]AgentConfig{
		{AgentID: "build-executor"},
	})
	got := p.selectBucketAgent("send outreach message to the client")
	// Should fall back to first *-executor found = build-executor
	if got != "build-executor" {
		t.Errorf("selectBucketAgent (fallback) = %q, want %q", got, "build-executor")
	}
}

func TestSelectBucketAgent_EmptyReason_ReturnsDefault(t *testing.T) {
	p := newTestLLMPlannerWithAgents([]AgentConfig{
		{AgentID: "build-executor"},
	})
	got := p.selectBucketAgent("")
	if got != "build-executor" {
		t.Errorf("selectBucketAgent('') = %q, want %q", got, "build-executor")
	}
}

func TestSelectBucketAgent_NoAgentsConfigured_ReturnsEmpty(t *testing.T) {
	p := newTestLLMPlannerWithAgents(nil)
	got := p.selectBucketAgent("build the feature")
	if got != "" {
		t.Errorf("selectBucketAgent (no agents) = %q, want empty", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// summarizeTeamRoles — 0% coverage
// ─────────────────────────────────────────────────────────────────────────────

func TestSummarizeTeamRoles_NoStatusFile_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	p := newTestLLMPlannerWithAgents(nil)
	p.config.StateDir = dir
	// No status file written — should silently return ""
	got := p.summarizeTeamRoles("nonexistent-template")
	if got != "" {
		t.Errorf("summarizeTeamRoles (no file) = %q, want empty", got)
	}
}

func TestSummarizeTeamRoles_AllCompleted(t *testing.T) {
	dir := t.TempDir()
	tasksDir := filepath.Join(dir, "tasks")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatal(err)
	}
	statusYAML := `plan_id: "test-plan"
tasks:
  - id: role-a
    status: completed
  - id: role-b
    status: completed
  - id: role-c
    status: completed
`
	statusPath := filepath.Join(tasksDir, "team-mytemplate.status.yaml")
	if err := os.WriteFile(statusPath, []byte(statusYAML), 0644); err != nil {
		t.Fatal(err)
	}

	p := newTestLLMPlannerWithAgents(nil)
	p.config.StateDir = dir
	got := p.summarizeTeamRoles("mytemplate")
	// Should contain "3/3 done"
	if got == "" {
		t.Fatal("expected non-empty summary, got empty")
	}
	if !containsAny(got, "3/3") {
		t.Errorf("summarizeTeamRoles (all done) = %q, expected '3/3' in output", got)
	}
}

func TestSummarizeTeamRoles_WithFailures(t *testing.T) {
	dir := t.TempDir()
	tasksDir := filepath.Join(dir, "tasks")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatal(err)
	}
	statusYAML := `plan_id: "test-plan"
tasks:
  - id: role-a
    status: completed
  - id: role-b
    status: failed
    error: "connection refused"
  - id: role-c
    status: failed
    error: "timeout"
`
	statusPath := filepath.Join(tasksDir, "team-mytemplate.status.yaml")
	if err := os.WriteFile(statusPath, []byte(statusYAML), 0644); err != nil {
		t.Fatal(err)
	}

	p := newTestLLMPlannerWithAgents(nil)
	p.config.StateDir = dir
	got := p.summarizeTeamRoles("mytemplate")
	if got == "" {
		t.Fatal("expected non-empty summary for failed roles")
	}
	// Should mention "failed"
	if !containsAny(got, "failed") {
		t.Errorf("summarizeTeamRoles (with failures) = %q, expected 'failed' in output", got)
	}
}

func TestSummarizeTeamRoles_EmptyTaskList_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	tasksDir := filepath.Join(dir, "tasks")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatal(err)
	}
	statusYAML := `plan_id: "test-plan"
tasks: []
`
	statusPath := filepath.Join(tasksDir, "team-empty.status.yaml")
	if err := os.WriteFile(statusPath, []byte(statusYAML), 0644); err != nil {
		t.Fatal(err)
	}

	p := newTestLLMPlannerWithAgents(nil)
	p.config.StateDir = dir
	got := p.summarizeTeamRoles("empty")
	if got != "" {
		t.Errorf("summarizeTeamRoles (empty tasks) = %q, want empty", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Engine.Name and Engine.Drain — 0% coverage
// ─────────────────────────────────────────────────────────────────────────────

func newMinimalEngine(t *testing.T) *Engine {
	t.Helper()
	dir := t.TempDir()
	store := newMemStore()
	sf := NewStateFile(dir)
	planner := NewStaticPlanner("test-kernel", nil)
	cfg := KernelConfig{
		KernelID: "test-kernel",
		Schedule: "*/10 * * * *",
	}
	return NewEngine(cfg, sf, store, planner, &mockExecutor{}, logging.NewComponentLogger("test"))
}

func TestEngine_Name_ReturnsKernel(t *testing.T) {
	e := newMinimalEngine(t)
	if got := e.Name(); got != "kernel" {
		t.Errorf("Engine.Name() = %q, want %q", got, "kernel")
	}
}

func TestEngine_Drain_StopsEngine(t *testing.T) {
	e := newMinimalEngine(t)

	// Drain must not block or panic; it signals Stop and waits for wg.
	// Since the engine is not running, wg is already at zero.
	ctx := context.Background()
	if err := e.Drain(ctx); err != nil {
		t.Errorf("Engine.Drain() returned error: %v", err)
	}
}

func TestEngine_Drain_IdempotentAfterStop(t *testing.T) {
	e := newMinimalEngine(t)

	e.Stop()
	ctx := context.Background()
	// Second drain after explicit Stop should be safe (sync.Once guard)
	if err := e.Drain(ctx); err != nil {
		t.Errorf("Engine.Drain() after Stop returned error: %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// StateFile.WriteInit and StateFile.SeedSystemPrompt — 0% coverage
// ─────────────────────────────────────────────────────────────────────────────

func TestStateFile_WriteInit_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	sf := NewStateFile(dir)
	content := "# INIT\nThis is the init file."
	if err := sf.WriteInit(content); err != nil {
		t.Fatalf("WriteInit: %v", err)
	}
	got, err := sf.ReadInit()
	if err != nil {
		t.Fatalf("ReadInit: %v", err)
	}
	if got != content {
		t.Errorf("ReadInit = %q, want %q", got, content)
	}
}

func TestStateFile_WriteInit_Overwrites(t *testing.T) {
	dir := t.TempDir()
	sf := NewStateFile(dir)
	if err := sf.WriteInit("first"); err != nil {
		t.Fatalf("WriteInit first: %v", err)
	}
	if err := sf.WriteInit("second"); err != nil {
		t.Fatalf("WriteInit second: %v", err)
	}
	got, err := sf.ReadInit()
	if err != nil {
		t.Fatalf("ReadInit: %v", err)
	}
	if got != "second" {
		t.Errorf("ReadInit after overwrite = %q, want %q", got, "second")
	}
}

func TestStateFile_SeedSystemPrompt_WritesWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	sf := NewStateFile(dir)
	content := "# SYSTEM PROMPT\nYou are an agent."
	if err := sf.SeedSystemPrompt(content); err != nil {
		t.Fatalf("SeedSystemPrompt: %v", err)
	}
	got, err := sf.ReadSystemPrompt()
	if err != nil {
		t.Fatalf("ReadSystemPrompt: %v", err)
	}
	if got != content {
		t.Errorf("ReadSystemPrompt = %q, want %q", got, content)
	}
}

func TestStateFile_SeedSystemPrompt_DoesNotOverwriteExisting(t *testing.T) {
	dir := t.TempDir()
	sf := NewStateFile(dir)
	original := "# ORIGINAL"
	if err := sf.WriteSystemPrompt(original); err != nil {
		t.Fatalf("WriteSystemPrompt: %v", err)
	}
	// Seed should be a no-op when file already exists.
	if err := sf.SeedSystemPrompt("# SHOULD NOT OVERWRITE"); err != nil {
		t.Fatalf("SeedSystemPrompt: %v", err)
	}
	got, err := sf.ReadSystemPrompt()
	if err != nil {
		t.Fatalf("ReadSystemPrompt: %v", err)
	}
	if got != original {
		t.Errorf("ReadSystemPrompt after Seed = %q, want original %q", got, original)
	}
}
