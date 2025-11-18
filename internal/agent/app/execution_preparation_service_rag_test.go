package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	id "alex/internal/utils/id"
)

type recordingGate struct {
	directives ports.RAGDirectives
	signals    ports.RAGSignals
}

func (g *recordingGate) Evaluate(ctx context.Context, signals ports.RAGSignals) ports.RAGDirectives {
	g.signals = signals
	return g.directives
}

type recordingEventListener struct {
	events []ports.AgentEvent
}

func (l *recordingEventListener) OnEvent(event ports.AgentEvent) {
	l.events = append(l.events, event)
}

type fakeLLMFactory struct {
	client ports.LLMClient
}

func (f *fakeLLMFactory) GetClient(provider, model string, config ports.LLMConfig) (ports.LLMClient, error) {
	if f.client == nil {
		return nil, errors.New("no client")
	}
	return f.client, nil
}

func (f *fakeLLMFactory) GetIsolatedClient(provider, model string, config ports.LLMConfig) (ports.LLMClient, error) {
	if f.client == nil {
		return nil, errors.New("no client")
	}
	return f.client, nil
}

func (f *fakeLLMFactory) DisableRetry() {}

type fakeLLMClient struct{}

func (fakeLLMClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	if req.Metadata != nil {
		if intent, ok := req.Metadata["intent"].(string); ok && intent == historySummaryIntent {
			return &ports.CompletionResponse{Content: historySummaryResponse()}, nil
		}
	}
	return &ports.CompletionResponse{Content: `<task_analysis>
  <action>Researching marketing landscape</action>
  <goal>Summarize current marketing trends</goal>
  <approach>Review internal assets then synthesize web insights</approach>
  <success_criteria>
    <criterion>List at least three trend themes</criterion>
    <criterion>Cite internal and external sources</criterion>
  </success_criteria>
  <task_breakdown>
    <step requires_external_research="false" requires_retrieval="true">
      <description>Review recent marketing briefs in the repo</description>
      <reason>Need latest internal positioning</reason>
    </step>
    <step requires_external_research="true" requires_retrieval="true">
      <description>Collect public reports on 2024 marketing trends</description>
      <reason>Fresh data lives outside the workspace</reason>
    </step>
  </task_breakdown>
  <retrieval_plan should_retrieve="true">
    <local_queries>
      <query>marketing brief</query>
      <query>campaign roadmap</query>
    </local_queries>
    <search_queries>
      <query>2024 marketing trends</query>
      <query>consumer engagement benchmarks</query>
    </search_queries>
    <crawl_urls>
      <url>https://example.com/report</url>
    </crawl_urls>
    <knowledge_gaps>
      <gap>Latest consumer engagement statistics</gap>
    </knowledge_gaps>
    <notes>Prioritize sources updated within the last quarter</notes>
  </retrieval_plan>
</task_analysis>`}, nil
}

func (fakeLLMClient) Model() string { return "stub-model" }

type registryWithList struct {
	defs []ports.ToolDefinition
}

func (r *registryWithList) Register(tool ports.ToolExecutor) error { return nil }

func (r *registryWithList) Get(name string) (ports.ToolExecutor, error) {
	return nil, errors.New("not implemented")
}

func (r *registryWithList) List() []ports.ToolDefinition {
	return append([]ports.ToolDefinition(nil), r.defs...)
}

func (r *registryWithList) Unregister(name string) error { return nil }

type stubCostTracker struct {
	stats *ports.SessionStats
}

func (s *stubCostTracker) RecordUsage(ctx context.Context, usage ports.UsageRecord) error { return nil }

func (s *stubCostTracker) GetSessionCost(ctx context.Context, sessionID string) (*ports.CostSummary, error) {
	return nil, errors.New("not implemented")
}

func (s *stubCostTracker) GetSessionStats(ctx context.Context, sessionID string) (*ports.SessionStats, error) {
	if s.stats == nil {
		return &ports.SessionStats{}, nil
	}
	stats := *s.stats
	stats.SessionID = sessionID
	return &stats, nil
}

func (s *stubCostTracker) GetDailyCost(ctx context.Context, date time.Time) (*ports.CostSummary, error) {
	return nil, errors.New("not implemented")
}

func (s *stubCostTracker) GetMonthlyCost(ctx context.Context, year int, month int) (*ports.CostSummary, error) {
	return nil, errors.New("not implemented")
}

func (s *stubCostTracker) GetDateRangeCost(ctx context.Context, start, end time.Time) (*ports.CostSummary, error) {
	return nil, errors.New("not implemented")
}

func (s *stubCostTracker) Export(ctx context.Context, format ports.ExportFormat, filter ports.ExportFilter) ([]byte, error) {
	return nil, errors.New("not implemented")
}

func TestExecutionPreparationServiceEmitsRAGDirectives(t *testing.T) {
	session := &ports.Session{
		ID: "session-rag-1",
		Messages: []ports.Message{
			{
				Source: ports.MessageSourceToolResult,
				ToolResults: []ports.ToolResult{{
					Metadata: map[string]any{"repo_path": "/repo", "result_count": 2},
					Content:  "matched snippet",
				}},
			},
		},
		Metadata: map[string]string{
			"rag_search_seeds":  "example.com, example.org",
			"rag_budget_target": "3.5",
		},
	}
	sessionStore := &stubSessionStore{session: session}

	toolRegistry := &registryWithList{defs: []ports.ToolDefinition{
		{Name: "web_search"},
		{Name: "web_fetch"},
		{Name: "code_search"},
	}}

	tracker := &stubCostTracker{stats: &ports.SessionStats{TotalCost: 1.2}}
	gate := &recordingGate{directives: ports.RAGDirectives{
		UseRetrieval:  true,
		UseSearch:     true,
		UseCrawl:      false,
		SearchSeeds:   []string{"seed-from-plan"},
		CrawlSeeds:    []string{"https://example.com"},
		Justification: map[string]float64{"total_score": 0.6},
	}}

	now := time.Date(2024, time.May, 10, 15, 4, 5, 0, time.UTC)
	listener := &recordingEventListener{}

	deps := ExecutionPreparationDeps{
		LLMFactory:    &fakeLLMFactory{client: fakeLLMClient{}},
		ToolRegistry:  toolRegistry,
		SessionStore:  sessionStore,
		ContextMgr:    stubContextManager{},
		Parser:        stubParser{},
		Config:        Config{LLMProvider: "mock", LLMModel: "test", MaxIterations: 5},
		Logger:        ports.NoopLogger{},
		Clock:         ports.ClockFunc(func() time.Time { return now }),
		CostDecorator: NewCostTrackingDecorator(tracker, ports.NoopLogger{}, ports.ClockFunc(func() time.Time { return now })),
		CostTracker:   tracker,
		RAGGate:       gate,
		EventEmitter:  listener,
	}

	service := NewExecutionPreparationService(deps)

	ctx := id.WithTaskID(context.Background(), "task-rag-event")
	env, err := service.Prepare(ctx, "latest marketing trends", session.ID)
	if err != nil {
		t.Fatalf("prepare returned error: %v", err)
	}
	if env == nil {
		t.Fatal("expected execution environment")
	}
	if env.RAGDirectives == nil {
		t.Fatal("expected RAG directives to be populated")
	}
	if !env.RAGDirectives.UseRetrieval || !env.RAGDirectives.UseSearch || env.RAGDirectives.UseCrawl {
		t.Fatalf("unexpected directives: %+v", env.RAGDirectives)
	}
	if env.RAGDirectives.Query != "marketing brief" {
		t.Fatalf("expected directives query to prefer structured plan, got %q", env.RAGDirectives.Query)
	}
	if env.RAGDirectives.Justification["total_score"] != 0.6 {
		t.Fatalf("expected justification to be copied, got %#v", env.RAGDirectives.Justification)
	}
	if !containsString(env.RAGDirectives.SearchSeeds, "seed-from-plan") || !containsString(env.RAGDirectives.SearchSeeds, "2024 marketing trends") {
		t.Fatalf("expected directives search seeds to merge plan suggestions, got %#v", env.RAGDirectives.SearchSeeds)
	}
	if !containsString(env.RAGDirectives.CrawlSeeds, "https://example.com") || !containsString(env.RAGDirectives.CrawlSeeds, "https://example.com/report") {
		t.Fatalf("expected directives crawl seeds to merge plan suggestions, got %#v", env.RAGDirectives.CrawlSeeds)
	}
	if gate.signals.Query != "marketing brief" {
		t.Fatalf("expected signals query to align with local retrieval plan, got %q", gate.signals.Query)
	}
	if !containsString(gate.signals.SearchSeeds, "2024 marketing trends") || !containsString(gate.signals.SearchSeeds, "consumer engagement benchmarks") {
		t.Fatalf("expected signals search seeds to include plan guidance, got %#v", gate.signals.SearchSeeds)
	}
	if !containsString(gate.signals.CrawlSeeds, "https://example.com/report") {
		t.Fatalf("expected signals crawl seeds to include plan guidance, got %#v", gate.signals.CrawlSeeds)
	}

	metadata := env.Session.Metadata
	if metadata["rag_last_directives"] != "retrieve+search" {
		t.Fatalf("expected rag_last_directives to be retrieve+search, got %q", metadata["rag_last_directives"])
	}
	if metadata["rag_last_plan_score"] != "0.6000" {
		t.Fatalf("expected rag_last_plan_score 0.6000, got %q", metadata["rag_last_plan_score"])
	}
	if metadata["rag_last_hit_rate"] != "1.0000" {
		t.Fatalf("expected rag_last_hit_rate 1.0000, got %q", metadata["rag_last_hit_rate"])
	}
	if metadata["rag_budget_remaining"] != "2.3000" {
		t.Fatalf("expected rag_budget_remaining 2.3000, got %q", metadata["rag_budget_remaining"])
	}
	if metadata["rag_budget_target"] != "3.5000" {
		t.Fatalf("expected rag_budget_target 3.5000, got %q", metadata["rag_budget_target"])
	}
	if metadata["rag_search_seeds"] == "" {
		t.Fatal("expected rag_search_seeds metadata to remain populated")
	}

	if len(listener.events) != 1 {
		t.Fatalf("expected one event emitted, got %d", len(listener.events))
	}
	evt, ok := listener.events[0].(*domain.RAGDirectivesEvaluatedEvent)
	if !ok {
		t.Fatalf("expected RAGDirectivesEvaluatedEvent, got %T", listener.events[0])
	}
	if !evt.Directives.UseRetrieval || !evt.Directives.UseSearch || evt.Directives.UseCrawl {
		t.Fatalf("event directives mismatch: %+v", evt.Directives)
	}
	if evt.Directives.Justification["total_score"] != 0.6 {
		t.Fatalf("event justification missing score: %#v", evt.Directives.Justification)
	}
	if evt.Signals.BudgetRemaining != 3.5-tracker.stats.TotalCost {
		t.Fatalf("event budget remaining mismatch, got %.2f", evt.Signals.BudgetRemaining)
	}
	if evt.Timestamp() != now {
		t.Fatalf("expected event timestamp %v, got %v", now, evt.Timestamp())
	}
	if evt.GetSessionID() != session.ID {
		t.Fatalf("expected event session ID %s, got %s", session.ID, evt.GetSessionID())
	}
	if evt.GetTaskID() != "task-rag-event" {
		t.Fatalf("expected event task ID, got %s", evt.GetTaskID())
	}

	signals := gate.signals
	if !signals.AllowSearch {
		t.Fatalf("expected gate signals to permit search")
	}
	if !signals.AllowCrawl {
		t.Fatalf("expected gate signals to permit crawl")
	}
	if signals.RetrievalHitRate <= 0.5 {
		t.Fatalf("expected retrieval hit rate to reflect prior success, got %.2f", signals.RetrievalHitRate)
	}
	if !containsString(signals.SearchSeeds, "example.com") || !containsString(signals.SearchSeeds, "example.org") {
		t.Fatalf("expected metadata seeds to persist, got %#v", signals.SearchSeeds)
	}
	if !containsString(signals.SearchSeeds, "2024 marketing trends") || !containsString(signals.SearchSeeds, "consumer engagement benchmarks") {
		t.Fatalf("expected signals to include plan seeds, got %#v", signals.SearchSeeds)
	}
	expectedRemaining := 3.5 - tracker.stats.TotalCost
	if diff := signals.BudgetRemaining - expectedRemaining; diff < -0.001 || diff > 0.001 {
		t.Fatalf("expected budget remaining %.2f, got %.2f", expectedRemaining, signals.BudgetRemaining)
	}
	if signals.BudgetTarget != 3.5 {
		t.Fatalf("expected budget target 3.5, got %.2f", signals.BudgetTarget)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
