package context

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
	"alex/internal/infra/analytics/journal"
	sessionstate "alex/internal/infra/session/state_store"
)

func TestSelectWorldPrefersExplicitKey(t *testing.T) {
	worlds := map[string]agent.WorldProfile{
		"prod":    {ID: "prod", Environment: "production"},
		"staging": {ID: "staging", Environment: "staging"},
	}
	session := &storage.Session{Metadata: map[string]string{"world": "staging"}}

	world := selectWorld("prod", session, worlds)
	if world.ID != "prod" {
		t.Fatalf("expected explicit world key to win, got %q", world.ID)
	}

	world = selectWorld("", session, worlds)
	if world.ID != "staging" {
		t.Fatalf("expected session metadata world, got %q", world.ID)
	}

	world = selectWorld("", &storage.Session{}, map[string]agent.WorldProfile{})
	if world.ID != "default" {
		t.Fatalf("expected default world fallback, got %q", world.ID)
	}
}

func TestBuildWindowIncludesWorldProfile(t *testing.T) {
	root := buildStaticContextTree(t)
	mgr := NewManager(WithConfigRoot(root))

	session := &storage.Session{ID: "sess-1", Metadata: map[string]string{"world": "fallback"}}
	window, err := mgr.BuildWindow(context.Background(), session, agent.ContextWindowConfig{WorldKey: "prod"})
	if err != nil {
		t.Fatalf("BuildWindow returned error: %v", err)
	}

	if window.Static.World.ID != "prod" {
		t.Fatalf("expected prod world, got %q", window.Static.World.ID)
	}
	if window.Static.World.Environment != "production" {
		t.Fatalf("unexpected environment: %q", window.Static.World.Environment)
	}
	if len(window.Static.World.Capabilities) != 2 {
		t.Fatalf("expected 2 capabilities, got %d", len(window.Static.World.Capabilities))
	}
}

func TestBuildWindowPopulatesSystemPrompt(t *testing.T) {
	root := buildStaticContextTree(t)
	mgr := NewManager(WithConfigRoot(root))
	session := &storage.Session{ID: "sess-ctx", Messages: []ports.Message{{Role: "user", Content: "hi"}}}
	window, err := mgr.BuildWindow(context.Background(), session, agent.ContextWindowConfig{EnvironmentSummary: "CI lab"})
	if err != nil {
		t.Fatalf("BuildWindow returned error: %v", err)
	}
	if strings.TrimSpace(window.SystemPrompt) == "" {
		t.Fatalf("expected system prompt to be populated")
	}
	if !strings.Contains(window.SystemPrompt, "CI lab") {
		t.Fatalf("expected environment summary in system prompt, got %q", window.SystemPrompt)
	}
	if !strings.Contains(window.SystemPrompt, "Deliver value") {
		t.Fatalf("expected goal context in system prompt, got %q", window.SystemPrompt)
	}
	if !strings.Contains(window.SystemPrompt, "Identity & Persona") {
		t.Fatalf("expected persona section, got %q", window.SystemPrompt)
	}
}

func TestBuildWindowDoesNotIncludeStructuredUserPersonaCore(t *testing.T) {
	root := buildStaticContextTree(t)
	mgr := NewManager(WithConfigRoot(root))
	session := &storage.Session{
		ID:       "sess-persona",
		Messages: []ports.Message{{Role: "user", Content: "hi"}},
		UserPersona: &ports.UserPersonaProfile{
			Version:           "persona-v1",
			UpdatedAt:         time.Now(),
			InitiativeSources: []string{"curiosity", "impact"},
			TopDrives:         []string{"autonomy", "mastery"},
			DecisionStyle:     "deliberate with evidence-first bias",
		},
	}
	window, err := mgr.BuildWindow(context.Background(), session, agent.ContextWindowConfig{})
	if err != nil {
		t.Fatalf("BuildWindow returned error: %v", err)
	}
	if strings.Contains(window.SystemPrompt, "# User Persona Core (Highest Priority)") {
		t.Fatalf("did not expect structured user persona section, got %q", window.SystemPrompt)
	}
	if !strings.Contains(window.SystemPrompt, "# Identity & Persona") {
		t.Fatalf("expected identity persona section, got %q", window.SystemPrompt)
	}
}

func TestBuildWindowSkipsEnvironmentSectionInWebMode(t *testing.T) {
	root := buildStaticContextTree(t)
	mgr := NewManager(WithConfigRoot(root))
	session := &storage.Session{ID: "web-mode", Messages: []ports.Message{{Role: "user", Content: "hi"}}}
	window, err := mgr.BuildWindow(context.Background(), session, agent.ContextWindowConfig{
		EnvironmentSummary: "should-not-appear",
		ToolMode:           "web",
	})
	if err != nil {
		t.Fatalf("BuildWindow returned error: %v", err)
	}
	if strings.Contains(window.SystemPrompt, "# Operating Environment") {
		t.Fatalf("expected operating environment section to be omitted in web mode, got %q", window.SystemPrompt)
	}
	if strings.Contains(window.SystemPrompt, "Environment summary") {
		t.Fatalf("expected environment summary to be excluded in web mode, got %q", window.SystemPrompt)
	}
	if window.Static.EnvironmentSummary != "" {
		t.Fatalf("expected environment summary to be cleared in web mode, got %q", window.Static.EnvironmentSummary)
	}
}

func TestDefaultStaticContextCarriesCoreGuidance(t *testing.T) {
	configRoot := resolveDefaultConfigRoot(t)
	mgr := NewManager(WithConfigRoot(configRoot))
	session := &storage.Session{ID: "legacy-static", Messages: []ports.Message{{Role: "user", Content: "ping"}}}

	window, err := mgr.BuildWindow(context.Background(), session, agent.ContextWindowConfig{EnvironmentSummary: "ci lab"})
	if err != nil {
		t.Fatalf("BuildWindow returned error: %v", err)
	}

	prompt := window.SystemPrompt
	expectations := []string{
		"Perfect Subordinate — System Prompt",
		"Primary Principle: Intent Over Instruction",
		"Conclusion First, Details On Demand",
		"Never execute destructive shell commands",
		"bg_dispatch(agent_type=codex)",
	}
	for _, snippet := range expectations {
		if !strings.Contains(prompt, snippet) {
			t.Fatalf("expected system prompt to include %q, got %q", snippet, prompt)
		}
	}
}

func TestDefaultContextConfigLoadsAndBuildsPrompt(t *testing.T) {
	t.Helper()

	configRoot := resolveDefaultConfigRoot(t)

	registry := newStaticRegistry(configRoot, "", time.Minute, nil, nil)
	snapshot, err := registry.currentSnapshot(context.Background())
	if err != nil {
		t.Fatalf("failed to load default static context: %v", err)
	}

	if _, ok := snapshot.Personas["default"]; !ok {
		t.Fatalf("expected default persona to be loaded")
	}
	if _, ok := snapshot.Goals["default"]; !ok {
		t.Fatalf("expected default goal to be loaded")
	}
	if _, ok := snapshot.Knowledge["default"]; !ok {
		t.Fatalf("expected default knowledge pack to be loaded")
	}
	if _, ok := snapshot.Worlds["default"]; !ok {
		t.Fatalf("expected default world profile to be loaded")
	}

	expectedPolicies := []string{
		"Core Guardrails",
	}
	for _, id := range expectedPolicies {
		if _, ok := snapshot.Policies[id]; !ok {
			t.Fatalf("expected policy %q to be loaded", id)
		}
	}

	mgr := NewManager(WithConfigRoot(configRoot))
	session := &storage.Session{ID: "default-static", Messages: []ports.Message{{Role: "user", Content: "ping"}}}
	window, err := mgr.BuildWindow(context.Background(), session, agent.ContextWindowConfig{EnvironmentSummary: "yaml smoke"})
	if err != nil {
		t.Fatalf("BuildWindow returned error: %v", err)
	}

	sections := []string{
		"# Identity & Persona",
		"# Tool Routing Guardrails",
		"# Mission Objectives",
		"# Guardrails & Policies",
		"# Knowledge & Experience",
		"# Operating Environment",
	}
	for _, section := range sections {
		if !strings.Contains(window.SystemPrompt, section) {
			t.Fatalf("expected system prompt to include section %q, got %q", section, window.SystemPrompt)
		}
	}
	// Check for decision tree + ALWAYS/NEVER tool routing rules
	for _, snippet := range []string{
		"task_has_explicit_operation",
		"read_only_inspection",
		"memory_search/memory_get",
		"user_delegates",
		"needs_human_gate",
		"ALWAYS exhaust deterministic tools",
		"ALWAYS inject runtime facts",
		"NEVER expose secrets",
		"host shell execution with any available PATH tool",
	} {
		if !strings.Contains(window.SystemPrompt, snippet) {
			t.Fatalf("expected routing guardrail snippet %q, got %q", snippet, window.SystemPrompt)
		}
	}
}

func TestDeriveHistoryAwareMetaBuildsTimelineFromSessionHistory(t *testing.T) {
	messages := []ports.Message{
		{Role: "system", Content: "Static context", Source: ports.MessageSourceSystemPrompt},
		{Role: "user", Content: "第一轮：分析日志", Source: ports.MessageSourceUserInput},
		{Role: "tool", Content: "shell[logs-1]: grep found 2 errors", Source: ports.MessageSourceToolResult, ToolCallID: "logs-1"},
		{Role: "assistant", Content: "我会根据错误代码修复", Source: ports.MessageSourceAssistantReply},
		{Role: "user", Content: "第二轮：生成修复计划", Source: ports.MessageSourceUserInput},
		{Role: "tool", Content: "shell[plan-1]: patched failing test", Source: ports.MessageSourceToolResult, ToolCallID: "plan-1"},
		{Role: "assistant", Content: "已完成计划", Source: ports.MessageSourceAssistantReply},
	}

	meta := deriveHistoryAwareMeta(messages, "persona-v1")
	if meta.PersonaVersion != "persona-v1" {
		t.Fatalf("expected persona version to be carried into meta context, got %q", meta.PersonaVersion)
	}

	var timeline *agent.MemoryFragment
	for i := range meta.Memories {
		if meta.Memories[i].Key == "recent_session_timeline" {
			timeline = &meta.Memories[i]
			break
		}
	}
	if timeline == nil {
		t.Fatalf("expected recent session timeline memory to be recorded, got %+v", meta.Memories)
	}
	for _, expected := range []string{"01. system", "02. user", "03. tool[logs-1]", "05. user", "07. assistant"} {
		if !strings.Contains(timeline.Content, expected) {
			t.Fatalf("expected timeline to include %q, got %q", expected, timeline.Content)
		}
	}

	var hasUser, hasAssistant, hasTool bool
	for _, rec := range meta.Recommendations {
		if strings.Contains(rec, "Latest user request") {
			hasUser = true
		}
		if strings.Contains(rec, "Previous assistant response") {
			hasAssistant = true
		}
		if strings.Contains(rec, "Latest tool insight") {
			hasTool = true
		}
	}
	if !hasUser || !hasAssistant || !hasTool {
		t.Fatalf("expected meta recommendations to include user/assistant/tool snippets, got %#v", meta.Recommendations)
	}
}

func TestBuildWindowEmbedsDynamicStateIntoSystemPrompt(t *testing.T) {
	root := buildStaticContextTree(t)
	store := sessionstate.NewInMemoryStore()
	mgr := NewManager(WithConfigRoot(root), WithStateStore(store))

	snapshotTime := time.Date(2024, time.April, 1, 12, 30, 0, 0, time.UTC)
	snapshot := sessionstate.Snapshot{
		SessionID:  "sess-live",
		TurnID:     4,
		LLMTurnSeq: 9,
		CreatedAt:  snapshotTime,
		Plans: []agent.PlanNode{{
			ID:     "plan-root",
			Title:  "Ship feature",
			Status: "in_progress",
			Children: []agent.PlanNode{{
				ID:          "plan-tests",
				Title:       "Write tests",
				Description: "Add regression coverage",
				Status:      "pending",
			}},
		}},
		Beliefs: []agent.Belief{{
			Statement:  "Tests fail on CI",
			Confidence: 0.8,
		}},
		World: map[string]any{"deploy": "blocked"},
		Feedback: []agent.FeedbackSignal{{
			Kind:    "reward",
			Message: "Stabilize pipeline",
			Value:   0.6,
		}},
	}
	if err := store.SaveSnapshot(context.Background(), snapshot); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}

	session := &storage.Session{ID: snapshot.SessionID, Messages: []ports.Message{{Role: "user", Content: "status?"}}}
	window, err := mgr.BuildWindow(context.Background(), session, agent.ContextWindowConfig{})
	if err != nil {
		t.Fatalf("BuildWindow returned error: %v", err)
	}
	if !strings.Contains(window.SystemPrompt, "Live Session State") {
		t.Fatalf("expected dynamic section in system prompt, got %q", window.SystemPrompt)
	}
	if !strings.Contains(window.SystemPrompt, "Ship feature [in_progress]") {
		t.Fatalf("expected plan entry in prompt, got %q", window.SystemPrompt)
	}
	if !strings.Contains(window.SystemPrompt, "Tests fail on CI") {
		t.Fatalf("expected beliefs section, got %q", window.SystemPrompt)
	}
	// Note: WorldState, Feedback, and Snapshot timestamp removed as debug info
}

func TestBuildWindowMetaContextReflectsHistory(t *testing.T) {
	root := buildStaticContextTree(t)
	mgr := NewManager(WithConfigRoot(root))

	session := &storage.Session{
		ID: "sess-history",
		Messages: []ports.Message{
			{Role: "system", Content: "Legacy persona", Source: ports.MessageSourceSystemPrompt},
			{Role: "user", Content: "请继续昨天的代码重构"},
			{Role: "assistant", Content: "我已经完成 parser 的一半, 接下来修复测试"},
		},
	}

	window, err := mgr.BuildWindow(context.Background(), session, agent.ContextWindowConfig{})
	if err != nil {
		t.Fatalf("BuildWindow returned error: %v", err)
	}

	if window.Meta.PersonaVersion == "" {
		t.Fatalf("expected persona version to be populated in meta context")
	}
	if len(window.Meta.Memories) == 0 {
		t.Fatalf("expected meta context to capture historical memories")
	}
	// Note: session_system_prompt removed to avoid SOUL.md duplication
	// Only checking for timeline presence now
	hasTimeline := false
	for _, memory := range window.Meta.Memories {
		if memory.Key == "recent_session_timeline" {
			hasTimeline = true
			break
		}
	}
	if !hasTimeline {
		t.Fatalf("expected meta context to include session timeline, got %#v", window.Meta.Memories)
	}
	if len(window.Meta.Recommendations) < 2 {
		t.Fatalf("expected meta context to include history-driven recommendations, got %v", window.Meta.Recommendations)
	}
	historyHints := strings.Join(window.Meta.Recommendations, " ")
	if !strings.Contains(historyHints, "user request") {
		t.Fatalf("expected user request hint in recommendations, got %q", historyHints)
	}
	if !strings.Contains(historyHints, "Previous assistant response") {
		t.Fatalf("expected assistant response hint in recommendations, got %q", historyHints)
	}
}

func TestBuildWindowMetaContextIncludesHistoryTimeline(t *testing.T) {
	root := buildStaticContextTree(t)
	mgr := NewManager(WithConfigRoot(root))

	session := &storage.Session{
		ID: "sess-history-timeline",
		Messages: []ports.Message{
			{Role: "system", Content: "Legacy persona", Source: ports.MessageSourceSystemPrompt},
			{Role: "user", Content: "第一轮：帮我准备代码审查要点", Source: ports.MessageSourceUserInput},
			{Role: "assistant", Content: "我会先拉取最新提交再列出审查清单", Source: ports.MessageSourceAssistantReply},
			{Role: "tool", Content: "git_fetch: updated 3 files", Source: ports.MessageSourceToolResult, ToolCallID: "git-1"},
			{Role: "assistant", Content: "工具输出显示了 3 个新增文件", Source: ports.MessageSourceAssistantReply},
			{Role: "user", Content: "第二轮：再生成发布计划", Source: ports.MessageSourceUserInput},
			{Role: "tool", Content: "todo_update: added release checklist", Source: ports.MessageSourceToolResult, ToolCallID: "todo-2"},
			{Role: "assistant", Content: "发布计划包括 smoke 测试和回滚步骤", Source: ports.MessageSourceAssistantReply},
		},
	}

	window, err := mgr.BuildWindow(context.Background(), session, agent.ContextWindowConfig{})
	if err != nil {
		t.Fatalf("BuildWindow returned error: %v", err)
	}

	var timeline agent.MemoryFragment
	found := false
	for _, memory := range window.Meta.Memories {
		if memory.Key == "recent_session_timeline" {
			timeline = memory
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected meta context to include recent session timeline, got %#v", window.Meta.Memories)
	}

	if !strings.Contains(timeline.Content, "user") || !strings.Contains(timeline.Content, "assistant") {
		t.Fatalf("expected timeline to mention user and assistant snippets, got %q", timeline.Content)
	}
	if !strings.Contains(timeline.Content, "tool[git-1]") {
		t.Fatalf("expected timeline to include tool call identifier, got %q", timeline.Content)
	}
	if !strings.Contains(timeline.Content, "第二轮") {
		t.Fatalf("expected timeline to include later user request, got %q", timeline.Content)
	}
}

func TestComposeSystemPromptIncludesMetaLayer(t *testing.T) {
	static := agent.StaticContext{
		Persona: agent.PersonaProfile{
			Voice: "Operate like ALEX.",
			Tone:  "direct",
		},
		Goal:               agent.GoalProfile{LongTerm: []string{"Ship value"}},
		EnvironmentSummary: "CI lab",
	}
	dynamic := agent.DynamicContext{
		TurnID:     1,
		LLMTurnSeq: 2,
	}
	memoTime := time.Date(2024, time.January, 2, 0, 0, 0, 0, time.UTC)
	meta := agent.MetaContext{
		PersonaVersion: "persona-v2",
		Memories: []agent.MemoryFragment{{
			Key:       "user-pref",
			Content:   "Prefers Go",
			CreatedAt: memoTime,
			Source:    "memory",
		}},
		Recommendations: []string{"Prioritize secure defaults"},
	}

	prompt := composeSystemPrompt(systemPromptInput{
		Static:  static,
		Dynamic: dynamic,
		Meta:    meta,
	})
	if !strings.Contains(prompt, "Persona version: persona-v2") {
		t.Fatalf("expected persona version in prompt, got %q", prompt)
	}
	if !strings.Contains(prompt, "Prefers Go — user-pref") {
		t.Fatalf("expected memory snippet in prompt, got %q", prompt)
	}
	if !strings.Contains(prompt, "Prioritize secure defaults") {
		t.Fatalf("expected recommendation in prompt, got %q", prompt)
	}
	if !strings.Contains(prompt, memoTime.Format("2006-01-02")) {
		t.Fatalf("expected formatted memory date, got %q", prompt)
	}
}

func TestCompressInjectsStructuredSummary(t *testing.T) {
	mgr := &manager{}
	messages := []ports.Message{{
		Role:    "system",
		Source:  ports.MessageSourceSystemPrompt,
		Content: "base system",
	}}
	for i := 0; i < 12; i++ {
		messages = append(messages, ports.Message{Role: "user", Content: fmt.Sprintf("Need help with feature %d", i)})
		messages = append(messages, ports.Message{Role: "assistant", Content: fmt.Sprintf("Working on feature %d", i)})
	}
	target := mgr.EstimateTokens(messages) - 1
	compressed, err := mgr.Compress(messages, target)
	if err != nil {
		t.Fatalf("compress returned error: %v", err)
	}
	if len(compressed) != 2 {
		t.Fatalf("expected 2 messages (system prompt + summary), got %d", len(compressed))
	}
	summary := compressed[1]
	if summary.Source != ports.MessageSourceSystemPrompt {
		t.Fatalf("expected summary to be marked as system prompt, got %v", summary.Source)
	}
	if summary.Role != "system" {
		t.Fatalf("expected summary role system, got %s", summary.Role)
	}
	if !strings.Contains(summary.Content, "Earlier conversation had") {
		t.Fatalf("expected structured summary content, got %q", summary.Content)
	}
	if strings.Contains(summary.Content, "Previous conversation compressed") {
		t.Fatalf("legacy placeholder should be removed, got %q", summary.Content)
	}
}

func TestAutoCompactTriggersCompression(t *testing.T) {
	mgr := &manager{}
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
		t.Fatalf("expected system prompt and summary, got %d entries", len(compacted))
	}
	if compacted[0].Content != messages[0].Content {
		t.Fatalf("system prompt should be preserved")
	}
	summary := compacted[1]
	if summary.Source != ports.MessageSourceSystemPrompt || summary.Role != "system" {
		t.Fatalf("summary should be injected as a system prompt, got %+v", summary)
	}
	if !strings.Contains(summary.Content, "Earlier conversation had 1 user message(s)") {
		t.Fatalf("unexpected summary content: %q", summary.Content)
	}
}

func TestAutoCompactNoopBelowThreshold(t *testing.T) {
	mgr := &manager{}
	messages := []ports.Message{
		{Role: "system", Source: ports.MessageSourceSystemPrompt, Content: "base system"},
		{Role: "user", Content: "short"},
	}

	compacted, compactedFlag := mgr.AutoCompact(messages, 5_000)
	if compactedFlag {
		t.Fatalf("auto compaction should not trigger below threshold")
	}
	if len(compacted) != len(messages) {
		t.Fatalf("messages should remain untouched when no compaction occurs")
	}
}

func TestDefaultCompressionThreshold(t *testing.T) {
	mgr := &manager{}
	if got := mgr.compressionThreshold(); got != 0.7 {
		t.Fatalf("expected default compression threshold 0.7, got %.2f", got)
	}
}

func TestCompressPreservesAllSystemPrompts(t *testing.T) {
	mgr := &manager{}
	messages := []ports.Message{
		{Role: "system", Source: ports.MessageSourceSystemPrompt, Content: "primary system"},
		{Role: "user", Content: "first"},
		{Role: "system", Source: ports.MessageSourceSystemPrompt, Content: "second system"},
		{Role: "assistant", Content: "second reply"},
	}

	target := mgr.EstimateTokens(messages) - 1
	compressed, err := mgr.Compress(messages, target)
	if err != nil {
		t.Fatalf("compress returned error: %v", err)
	}

	if len(compressed) != 3 {
		t.Fatalf("expected 3 messages (two system prompts + summary), got %d", len(compressed))
	}

	if compressed[0].Content != "primary system" || compressed[2].Content != "second system" {
		t.Fatalf("system prompts should be preserved in order, got %+v", compressed)
	}

	summary := compressed[1]
	if summary.Source != ports.MessageSourceSystemPrompt || summary.Role != "system" {
		t.Fatalf("summary should be a system prompt, got %+v", summary)
	}
	if !strings.Contains(summary.Content, "Earlier conversation had 1 user message(s) and 1 assistant response(s)") {
		t.Fatalf("unexpected summary content: %q", summary.Content)
	}
}

func TestRecordTurnEmitsJournalEntry(t *testing.T) {
	store := sessionstate.NewInMemoryStore()
	jr := &recordingJournal{}
	mgr := NewManager(WithStateStore(store), WithJournalWriter(jr))
	record := agent.ContextTurnRecord{
		SessionID:  "sess-99",
		TurnID:     7,
		LLMTurnSeq: 3,
		Timestamp:  time.Unix(1710000000, 0),
		Summary:    "completed step",
		Plans:      []agent.PlanNode{{ID: "p1"}},
	}
	if err := mgr.RecordTurn(context.Background(), record); err != nil {
		t.Fatalf("RecordTurn returned error: %v", err)
	}
	if len(jr.entries) != 1 {
		t.Fatalf("expected 1 journal entry, got %d", len(jr.entries))
	}
	entry := jr.entries[0]
	if entry.SessionID != record.SessionID || entry.TurnID != record.TurnID {
		t.Fatalf("unexpected journal entry: %+v", entry)
	}
	if entry.Timestamp != record.Timestamp {
		t.Fatalf("expected timestamp to match, got %v", entry.Timestamp)
	}
}

type recordingJournal struct {
	entries []journal.TurnJournalEntry
}

func (r *recordingJournal) Write(_ context.Context, entry journal.TurnJournalEntry) error {
	r.entries = append(r.entries, entry)
	return nil
}

func buildStaticContextTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeContextFile(t, root, "personas", "default.yaml", `id: default
tone: balanced
risk_profile: moderate
decision_style: deliberate
voice: neutral`)
	writeContextFile(t, root, "goals", "default.yaml", `id: default
long_term:
  - Deliver value`)
	writeContextFile(t, root, "policies", "default.yaml", `id: default
hard_constraints:
  - Always follow company policies`)
	writeContextFile(t, root, "knowledge", "default.yaml", `id: default
description: base knowledge`)
	writeContextFile(t, root, "worlds", "prod.yaml", `id: prod
environment: production
capabilities:
  - deploy
  - monitor
limits:
  - No destructive actions without approval
cost_model:
  - Standard token budget`)
	return root
}

func resolveDefaultConfigRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Clean(filepath.Join("..", "..", "..", "configs", "context"))
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("default context root missing: %v", err)
	}
	return root
}

func TestSearchContextRootFromDirFindsConfigs(t *testing.T) {
	root := t.TempDir()
	contextDir := filepath.Join(root, "configs", "context")
	if err := os.MkdirAll(contextDir, 0o755); err != nil {
		t.Fatalf("create context dir: %v", err)
	}
	deployDir := filepath.Join(root, "deploy", "server", "bin")
	if err := os.MkdirAll(deployDir, 0o755); err != nil {
		t.Fatalf("create deploy dir: %v", err)
	}
	if resolved := searchContextRootFromDir(deployDir); resolved != contextDir {
		t.Fatalf("expected search to resolve %q, got %q", contextDir, resolved)
	}
}

func TestResolveContextConfigRootPrefersEnvOverride(t *testing.T) {
	custom := filepath.Join(t.TempDir(), "custom-root")
	t.Setenv(contextConfigEnvVar, custom)
	if resolved := resolveContextConfigRoot(); resolved != custom {
		t.Fatalf("expected env override to win, got %q", resolved)
	}
}

func writeContextFile(t *testing.T, root, subdir, name, body string) {
	t.Helper()
	dir := filepath.Join(root, subdir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create dir: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}
