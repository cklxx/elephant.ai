package preparation

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	llm "alex/internal/domain/agent/ports/llm"
	storage "alex/internal/domain/agent/ports/storage"
	"alex/internal/shared/utils/clilatency"
	id "alex/internal/shared/utils/id"
)

func (s *ExecutionPreparationService) preAnalyzeTask(ctx context.Context, session *storage.Session, task string) (*agent.TaskAnalysis, bool) {
	if strings.TrimSpace(task) == "" {
		return nil, false
	}
	provider, model, ok := s.resolveSmallModelConfig()
	if !ok || session == nil {
		return nil, false
	}
	if analysis, preferSmall, ok := quickTriageTask(task); ok {
		clilatency.PrintfWithContext(ctx, "[latency] preanalysis=skipped reason=%s\n", analysis.Approach)
		return analysis, preferSmall
	}
	llmConfig := llm.LLMConfig{
		APIKey:  s.config.APIKey,
		BaseURL: s.config.BaseURL,
	}
	if s.credentialRefresher != nil {
		if apiKey, baseURL, ok := s.credentialRefresher(provider); ok {
			llmConfig.APIKey = apiKey
			if baseURL != "" {
				llmConfig.BaseURL = baseURL
			}
		}
	}
	client, err := s.llmFactory.GetIsolatedClient(provider, model, llmConfig)
	if err != nil {
		s.logger.Warn("Task pre-analysis skipped: %v", err)
		return nil, false
	}
	client = s.costDecorator.Wrap(ctx, session.ID, client)

	taskNameRule := `- task_name must be a short single-line title (<= 32 chars), suitable for a session title.` + "\n\n"
	if strings.TrimSpace(session.Metadata["title"]) != "" {
		taskNameRule = `- task_name must be an empty string because the session already has a title.` + "\n\n"
	}

	requestID := id.NewRequestIDWithLogID(id.LogIDFromContext(ctx))
	req := ports.CompletionRequest{
		Messages: []ports.Message{
			{
				Role: "system",
				Content: "You are a fast task triage assistant. Analyze the user's task and decide which model tier is sufficient.\n\n" +
					"Definitions:\n" +
					`- complexity="simple": can be completed quickly with straightforward steps; no deep design/architecture; no large refactors; no ambiguous requirements; no heavy external research.\n` +
					`- complexity="complex": otherwise.\n` +
					`- recommended_model="small": a smaller/cheaper model should handle the entire task reliably.\n` +
					`- recommended_model="default": use the default (stronger) model.\n\n` +
					"Output requirements:\n" +
					`- Respond ONLY with JSON.\n` +
					taskNameRule +
					`- react_emoji: a single Lark emoji type that best matches the user's message sentiment/topic. ` +
					`Choose from: THUMBSUP, SMILE, WAVE, THINKING, MUSCLE, HEART, APPLAUSE, DONE, Coffee, Fire, LGTM, OK, THANKS, Get, JIAYI.` + "\n\n" +
					`Schema: {"complexity":"simple|complex","recommended_model":"small|default","task_name":"...","goal":"...","approach":"...","success_criteria":["..."],` +
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
			strings.TrimSpace(model),
		)
		if err != nil || resp == nil {
			s.logger.Warn("Task pre-analysis failed: %v", err)
			return nil, false
		}
		analysis, recommendedModel := parseTaskAnalysis(resp.Content)
		if analysis == nil {
			s.logger.Warn("Task pre-analysis returned unparsable output")
			return nil, false
		}
		preferSmallModel := false
		if strings.EqualFold(recommendedModel, "small") {
			preferSmallModel = true
		} else if strings.EqualFold(recommendedModel, "default") {
			preferSmallModel = false
		} else {
			preferSmallModel = strings.EqualFold(analysis.Complexity, "simple")
		}
		return analysis, preferSmallModel
	}
	resp, err := streaming.StreamComplete(analysisCtx, req, ports.CompletionStreamCallbacks{
		OnContentDelta: func(ports.ContentDelta) {},
	})
	clilatency.PrintfWithContext(ctx,
		"[latency] preanalysis_ms=%.2f model=%s\n",
		float64(time.Since(preanalysisStarted))/float64(time.Millisecond),
		strings.TrimSpace(model),
	)
	if err != nil || resp == nil {
		s.logger.Warn("Task pre-analysis failed: %v", err)
		return nil, false
	}
	analysis, recommendedModel := parseTaskAnalysis(resp.Content)
	if analysis == nil {
		s.logger.Warn("Task pre-analysis returned unparsable output")
		return nil, false
	}
	preferSmallModel := false
	if strings.EqualFold(recommendedModel, "small") {
		preferSmallModel = true
	} else if strings.EqualFold(recommendedModel, "default") {
		preferSmallModel = false
	} else {
		preferSmallModel = strings.EqualFold(analysis.Complexity, "simple")
	}
	return analysis, preferSmallModel
}

func quickTriageTask(task string) (*agent.TaskAnalysis, bool, bool) {
	trimmed := strings.TrimSpace(task)
	if trimmed == "" {
		return nil, false, false
	}
	if strings.Contains(trimmed, "\n") || strings.Contains(trimmed, "\r") {
		return nil, false, false
	}
	runes := []rune(trimmed)
	if len(runes) > 24 {
		return nil, false, false
	}

	switch strings.ToLower(trimmed) {
	case "hi", "hello", "hey", "yo", "nihao", "你好", "您好", "嗨", "在吗", "ping", "pings":
		return &agent.TaskAnalysis{
			Complexity: "simple",
			ActionName: "Greeting",
			Goal:       "",
			Approach:   "greeting",
			ReactEmoji: "WAVE",
		}, true, true
	case "thanks", "thank you", "thx", "谢谢", "多谢", "感谢", "ok", "okay", "好的", "收到":
		return &agent.TaskAnalysis{
			Complexity: "simple",
			ActionName: "Acknowledge",
			Goal:       "",
			Approach:   "ack",
			ReactEmoji: "THUMBSUP",
		}, true, true
	default:
		return nil, false, false
	}
}

func shouldSkipContextWindow(task string, session *storage.Session) (bool, string) {
	if session == nil {
		return false, ""
	}
	if len(session.Messages) > 0 {
		return false, ""
	}
	analysis, _, ok := quickTriageTask(task)
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

func (s *ExecutionPreparationService) resolveSmallModelConfig() (string, string, bool) {
	model := strings.TrimSpace(s.config.LLMSmallModel)
	if model == "" {
		return "", "", false
	}
	provider := strings.TrimSpace(s.config.LLMSmallProvider)
	if provider == "" {
		provider = strings.TrimSpace(s.config.LLMProvider)
	}
	if provider == "" {
		return "", "", false
	}
	return provider, model, true
}

type taskAnalysisPayload struct {
	Complexity       string                     `json:"complexity"`
	RecommendedModel string                     `json:"recommended_model"`
	TaskName         string                     `json:"task_name"`
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

func parseTaskAnalysis(raw string) (*agent.TaskAnalysis, string) {
	body := strings.TrimSpace(raw)
	start := strings.Index(body, "{")
	end := strings.LastIndex(body, "}")
	if start < 0 || end <= start {
		return nil, ""
	}
	body = body[start : end+1]

	var payload taskAnalysisPayload
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return nil, ""
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
			if strings.TrimSpace(step.Description) == "" {
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

	return analysis, normalizeRecommendedModel(payload.RecommendedModel)
}

func normalizeComplexity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "simple", "easy":
		return "simple"
	case "complex", "hard":
		return "complex"
	default:
		return ""
	}
}

func normalizeRecommendedModel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "small", "mini":
		return "small"
	case "default", "large":
		return "default"
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

func buildWorldStateFromWindow(window agent.ContextWindow) (map[string]any, map[string]any) {
	profile := window.Static.World
	envSummary := strings.TrimSpace(window.Static.EnvironmentSummary)
	hasProfile := profile.ID != "" || profile.Environment != "" || len(profile.Capabilities) > 0 || len(profile.Limits) > 0 || len(profile.CostModel) > 0
	if !hasProfile && envSummary == "" {
		return nil, nil
	}
	state := make(map[string]any)
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
		state["profile"] = profileMap
	}
	if envSummary != "" {
		state["environment_summary"] = envSummary
	}
	var diff map[string]any
	if len(state) > 0 {
		diff = make(map[string]any)
		if profile.ID != "" {
			diff["profile_loaded"] = profile.ID
		}
		if envSummary != "" {
			diff["environment_summary"] = envSummary
		}
		if len(profile.Capabilities) > 0 {
			diff["capabilities"] = append([]string(nil), profile.Capabilities...)
		}
	}
	return state, diff
}
