package preparation

import (
	"alex/internal/shared/utils"
	"context"
	"encoding/json"
	"strings"
	"time"

	"alex/internal/app/agent/llmclient"
	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	llm "alex/internal/domain/agent/ports/llm"
	storage "alex/internal/domain/agent/ports/storage"
	"alex/internal/shared/utils/clilatency"
	id "alex/internal/shared/utils/id"
)

func (s *ExecutionPreparationService) preAnalyzeTask(ctx context.Context, session *storage.Session, task string) *agent.TaskAnalysis {
	if utils.IsBlank(task) {
		return nil
	}
	if session == nil {
		return nil
	}
	if analysis, ok := quickTriageTask(task); ok {
		clilatency.PrintfWithContext(ctx, "[latency] preanalysis=skipped reason=%s\n", analysis.Approach)
		return analysis
	}
	profile := s.config.DefaultLLMProfile()
	if utils.IsBlank(profile.Provider) || utils.IsBlank(profile.Model) {
		return nil
	}
	client, _, err := llmclient.GetIsolatedClientFromProfile(
		s.llmFactory,
		profile,
		llmclient.CredentialRefresher(s.credentialRefresher),
		true,
	)
	if err != nil {
		s.logger.Warn("Task pre-analysis skipped: %v", err)
		return nil
	}
	client = s.costDecorator.Wrap(ctx, session.ID, client)

	taskNameRule := `- task_name must be a short single-line title (<= 32 chars), suitable for a session title.` + "\n\n"
	if utils.HasContent(session.Metadata["title"]) {
		taskNameRule = `- task_name must be an empty string because the session already has a title.` + "\n\n"
	}

	requestID := id.NewRequestIDWithLogID(id.LogIDFromContext(ctx))
	req := ports.CompletionRequest{
		Messages: []ports.Message{
			{
				Role: "system",
				Content: "You are a fast task triage assistant. Analyze the user's task.\n\n" +
					"Definitions:\n" +
					`- complexity="simple": can be completed quickly with straightforward steps; no deep design/architecture; no large refactors; no ambiguous requirements; no heavy external research.\n` +
					`- complexity="complex": otherwise.\n\n` +
					"Output requirements:\n" +
					`- Respond ONLY with JSON.\n` +
					taskNameRule +
					`- react_emoji: a single Lark emoji type that best matches the user's message sentiment/topic. ` +
					`Choose from: THUMBSUP, SMILE, WAVE, THINKING, MUSCLE, HEART, APPLAUSE, DONE, Coffee, Fire, LGTM, OK, THANKS, Get, JIAYI.` + "\n\n" +
					`Schema: {"complexity":"simple|complex","task_name":"...","goal":"...","approach":"...","success_criteria":["..."],` +
					`"steps":[{"description":"...","rationale":"...","needs_external_context":false}],` +
					`"retrieval":{"should_retrieve":false,"local_queries":[],"search_queries":[],"crawl_urls":[],"knowledge_gaps":[],"notes":""},` +
					`"react_emoji":"..."}`,
			},
			{Role: "user", Content: task},
		},
		Temperature: 0.2,
		MaxTokens:   320,
		Metadata: map[string]any{
			"request_id": requestID,
			"intent":     "task_preanalysis",
		},
	}

	preanalysisStarted := time.Now()
	analysisCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	streaming, ok := llm.EnsureStreamingClient(client).(llm.StreamingLLMClient)
	if !ok {
		resp, err := client.Complete(analysisCtx, req)
		clilatency.PrintfWithContext(ctx,
			"[latency] preanalysis_ms=%.2f model=%s\n",
			float64(time.Since(preanalysisStarted))/float64(time.Millisecond),
			strings.TrimSpace(profile.Model),
		)
		if err != nil || resp == nil {
			s.logger.Warn("Task pre-analysis failed: %v", err)
			return nil
		}
		analysis := parseTaskAnalysis(resp.Content)
		if analysis == nil {
			s.logger.Warn("Task pre-analysis returned unparsable output")
			return nil
		}
		return analysis
	}
	resp, err := streaming.StreamComplete(analysisCtx, req, ports.CompletionStreamCallbacks{
		OnContentDelta: func(ports.ContentDelta) {},
	})
	clilatency.PrintfWithContext(ctx,
		"[latency] preanalysis_ms=%.2f model=%s\n",
		float64(time.Since(preanalysisStarted))/float64(time.Millisecond),
		strings.TrimSpace(profile.Model),
	)
	if err != nil || resp == nil {
		s.logger.Warn("Task pre-analysis failed: %v", err)
		return nil
	}
	analysis := parseTaskAnalysis(resp.Content)
	if analysis == nil {
		s.logger.Warn("Task pre-analysis returned unparsable output")
		return nil
	}
	return analysis
}

func quickTriageTask(task string) (*agent.TaskAnalysis, bool) {
	trimmed := strings.TrimSpace(task)
	if trimmed == "" {
		return nil, false
	}
	if strings.Contains(trimmed, "\n") || strings.Contains(trimmed, "\r") {
		return nil, false
	}
	runes := []rune(trimmed)
	if len(runes) > 24 {
		return nil, false
	}

	switch strings.ToLower(trimmed) {
	case "hi", "hello", "hey", "yo", "nihao", "你好", "您好", "嗨", "在吗", "ping", "pings":
		return &agent.TaskAnalysis{
			Complexity: "simple",
			ActionName: "Greeting",
			Goal:       "",
			Approach:   "greeting",
			ReactEmoji: "WAVE",
		}, true
	case "thanks", "thank you", "thx", "谢谢", "多谢", "感谢", "ok", "okay", "好的", "收到":
		return &agent.TaskAnalysis{
			Complexity: "simple",
			ActionName: "Acknowledge",
			Goal:       "",
			Approach:   "ack",
			ReactEmoji: "THUMBSUP",
		}, true
	default:
		return nil, false
	}
}

func shouldSkipContextWindow(task string, session *storage.Session) (bool, string) {
	if session == nil {
		return false, ""
	}
	if len(session.Messages) > 0 {
		return false, ""
	}
	analysis, ok := quickTriageTask(task)
	if !ok || analysis == nil {
		return false, ""
	}
	switch strings.TrimSpace(strings.ToLower(analysis.Approach)) {
	case "greeting":
		return true, "greeting"
	case "ack":
		return true, "ack"
	default:
		return false, ""
	}
}

type taskAnalysisPayload struct {
	Complexity string `json:"complexity"`
	TaskName   string `json:"task_name"`
	ActionName       string                     `json:"action_name"`
	Goal             string                     `json:"goal"`
	Approach         string                     `json:"approach"`
	SuccessCriteria  []string                   `json:"success_criteria"`
	Steps            []taskAnalysisStepPayload  `json:"steps"`
	Retrieval        taskAnalysisRetrievalHints `json:"retrieval"`
	ReactEmoji       string                     `json:"react_emoji"`
}

type taskAnalysisStepPayload struct {
	Description          string `json:"description"`
	Rationale            string `json:"rationale"`
	NeedsExternalContext bool   `json:"needs_external_context"`
}

type taskAnalysisRetrievalHints struct {
	ShouldRetrieve bool     `json:"should_retrieve"`
	LocalQueries   []string `json:"local_queries"`
	SearchQueries  []string `json:"search_queries"`
	CrawlURLs      []string `json:"crawl_urls"`
	KnowledgeGaps  []string `json:"knowledge_gaps"`
	Notes          string   `json:"notes"`
}

func parseTaskAnalysis(raw string) *agent.TaskAnalysis {
	body := strings.TrimSpace(raw)
	start := strings.Index(body, "{")
	end := strings.LastIndex(body, "}")
	if start < 0 || end <= start {
		return nil
	}
	body = body[start : end+1]

	var payload taskAnalysisPayload
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return nil
	}

	analysis := &agent.TaskAnalysis{
		Complexity:      normalizeComplexity(payload.Complexity),
		ActionName:      coalesce(payload.TaskName, payload.ActionName),
		Goal:            strings.TrimSpace(payload.Goal),
		Approach:        strings.TrimSpace(payload.Approach),
		SuccessCriteria: compactStrings(payload.SuccessCriteria),
		ReactEmoji:      strings.TrimSpace(payload.ReactEmoji),
	}

	if len(payload.Steps) > 0 {
		analysis.TaskBreakdown = make([]agent.TaskAnalysisStep, 0, len(payload.Steps))
		for _, step := range payload.Steps {
			if utils.IsBlank(step.Description) {
				continue
			}
			analysis.TaskBreakdown = append(analysis.TaskBreakdown, agent.TaskAnalysisStep{
				Description:          strings.TrimSpace(step.Description),
				NeedsExternalContext: step.NeedsExternalContext,
				Rationale:            strings.TrimSpace(step.Rationale),
			})
		}
	}

	if analysis.Retrieval.ShouldRetrieve || payload.Retrieval.ShouldRetrieve {
		analysis.Retrieval.ShouldRetrieve = true
	}
	if len(payload.Retrieval.LocalQueries) > 0 {
		analysis.Retrieval.LocalQueries = compactStrings(payload.Retrieval.LocalQueries)
	}
	if len(payload.Retrieval.SearchQueries) > 0 {
		analysis.Retrieval.SearchQueries = compactStrings(payload.Retrieval.SearchQueries)
	}
	if len(payload.Retrieval.CrawlURLs) > 0 {
		analysis.Retrieval.CrawlURLs = compactStrings(payload.Retrieval.CrawlURLs)
	}
	if len(payload.Retrieval.KnowledgeGaps) > 0 {
		analysis.Retrieval.KnowledgeGaps = compactStrings(payload.Retrieval.KnowledgeGaps)
	}
	if note := strings.TrimSpace(payload.Retrieval.Notes); note != "" {
		analysis.Retrieval.Notes = note
	}
	if !analysis.Retrieval.ShouldRetrieve {
		for _, step := range payload.Steps {
			if step.NeedsExternalContext {
				analysis.Retrieval.ShouldRetrieve = true
				break
			}
		}
	}

	return analysis
}

func normalizeComplexity(value string) string {
	switch utils.TrimLower(value) {
	case "simple", "easy":
		return "simple"
	case "complex", "hard":
		return "complex"
	default:
		return ""
	}
}

func compactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func coalesce(values ...string) string {
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func buildCognitiveFromWindow(window agent.ContextWindow) *agent.CognitiveExtension {
	profile := window.Static.World
	envSummary := strings.TrimSpace(window.Static.EnvironmentSummary)
	hasProfile := profile.ID != "" || profile.Environment != "" || len(profile.Capabilities) > 0 || len(profile.Limits) > 0 || len(profile.CostModel) > 0
	if !hasProfile && envSummary == "" {
		return nil
	}
	worldState := make(map[string]any)
	if hasProfile {
		profileMap := map[string]any{"id": profile.ID}
		if profile.Environment != "" {
			profileMap["environment"] = profile.Environment
		}
		if len(profile.Capabilities) > 0 {
			profileMap["capabilities"] = append([]string(nil), profile.Capabilities...)
		}
		if len(profile.Limits) > 0 {
			profileMap["limits"] = append([]string(nil), profile.Limits...)
		}
		if len(profile.CostModel) > 0 {
			profileMap["cost_model"] = append([]string(nil), profile.CostModel...)
		}
		worldState["profile"] = profileMap
	}
	if envSummary != "" {
		worldState["environment_summary"] = envSummary
	}
	var worldDiff map[string]any
	if len(worldState) > 0 {
		worldDiff = make(map[string]any)
		if profile.ID != "" {
			worldDiff["profile_loaded"] = profile.ID
		}
		if envSummary != "" {
			worldDiff["environment_summary"] = envSummary
		}
		if len(profile.Capabilities) > 0 {
			worldDiff["capabilities"] = append([]string(nil), profile.Capabilities...)
		}
	}
	return &agent.CognitiveExtension{
		WorldState: worldState,
		WorldDiff:  worldDiff,
	}
}
