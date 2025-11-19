package app

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"alex/internal/agent/ports"
	id "alex/internal/utils/id"
)

// ragPreloader orchestrates pre-task retrieval based on the selected directives.
type ragPreloader struct {
	logger ports.Logger
}

func newRAGPreloader(logger ports.Logger) *ragPreloader {
	if logger == nil {
		logger = ports.NoopLogger{}
	}
	return &ragPreloader{logger: logger}
}

// apply embeds pre-task context into the execution environment based on the
// directives stored on the environment.
func (p *ragPreloader) apply(ctx context.Context, env *ports.ExecutionEnvironment) error {
	if env == nil || env.RAGDirectives == nil {
		return nil
	}
	directives := env.RAGDirectives
	if !directives.UseRetrieval && !directives.UseSearch && !directives.UseCrawl {
		p.logger.Debug("RAG directives indicate no preloading")
		p.annotateSessionMetadata(env)
		return nil
	}

	if env.Services.ToolExecutor == nil {
		return fmt.Errorf("tool registry unavailable for RAG preloading")
	}

	p.annotateSessionMetadata(env)

	var execErr error
	executedRetrieval := false
	executedSearch := false
	executedCrawl := false
	notes := make([]string, 0, 1)

	if directives.UseRetrieval {
		if _, err := env.Services.ToolExecutor.Get("code_search"); err != nil {
			p.logger.Debug("Skipping RAG retrieval preload: code_search unavailable: %v", err)
			notes = append(notes, "Retrieval skipped because code_search tool is unavailable.")
		} else if err := p.executeTool(ctx, env, "code_search", map[string]any{"query": directives.Query}); err != nil {
			execErr = err
		} else {
			executedRetrieval = true
		}
	}

	if directives.UseSearch {
		baseQuery := strings.TrimSpace(directives.Query)
		seedText := strings.TrimSpace(strings.Join(directives.SearchSeeds, " "))

		fallbackQuery := baseQuery
		if fallbackQuery == "" {
			fallbackQuery = seedText
		} else if seedText != "" {
			fallbackQuery = strings.TrimSpace(baseQuery + " " + seedText)
		}

		finalQuery := fallbackQuery
		if finalQuery == "" {
			finalQuery = baseQuery
		}

		if env != nil && env.Services.LLM != nil {
			generated, err := p.generateSearchQuery(ctx, env.Services.LLM, baseQuery, directives.SearchSeeds)
			if err != nil {
				p.logger.Warn("RAG preloader search query generation failed: %v", err)
				if fallbackQuery != "" {
					finalQuery = fallbackQuery
				} else {
					finalQuery = baseQuery
				}
			} else if strings.TrimSpace(generated) != "" {
				finalQuery = generated
			} else if fallbackQuery != "" {
				finalQuery = fallbackQuery
			}
		}

		if finalQuery == "" {
			finalQuery = directives.Query
		}

		searchArgs := map[string]any{"query": finalQuery}
		if err := p.executeTool(ctx, env, "web_search", searchArgs); err != nil {
			execErr = err
		} else {
			executedSearch = true
		}
	}

	if directives.UseCrawl && len(directives.CrawlSeeds) > 0 {
		maxFetches := 3
		if maxFetches > len(directives.CrawlSeeds) {
			maxFetches = len(directives.CrawlSeeds)
		}
		for _, seed := range directives.CrawlSeeds[:maxFetches] {
			url := strings.TrimSpace(seed)
			if url == "" {
				continue
			}
			if err := p.executeTool(ctx, env, "web_fetch", map[string]any{"url": url}); err != nil {
				execErr = err
			} else {
				executedCrawl = true
			}
		}
	}

	p.appendDirectiveSummary(env, executedRetrieval, executedSearch, executedCrawl, notes)

	return execErr
}

func (p *ragPreloader) annotateSessionMetadata(env *ports.ExecutionEnvironment) {
	if env == nil || env.Session == nil || env.RAGDirectives == nil {
		return
	}
	if env.Session.Metadata == nil {
		env.Session.Metadata = make(map[string]string)
	}
	env.Session.Metadata["rag_last_directives"] = encodeDirectiveSummary(*env.RAGDirectives)
	if len(env.RAGDirectives.SearchSeeds) > 0 {
		env.Session.Metadata["rag_plan_search_seeds"] = strings.Join(env.RAGDirectives.SearchSeeds, ",")
	} else {
		delete(env.Session.Metadata, "rag_plan_search_seeds")
	}
	if len(env.RAGDirectives.CrawlSeeds) > 0 {
		env.Session.Metadata["rag_plan_crawl_seeds"] = strings.Join(env.RAGDirectives.CrawlSeeds, ",")
	} else {
		delete(env.Session.Metadata, "rag_plan_crawl_seeds")
	}
}

func (p *ragPreloader) appendDirectiveSummary(env *ports.ExecutionEnvironment, executedRetrieval, executedSearch, executedCrawl bool, notes []string) {
	if env == nil || env.State == nil || env.RAGDirectives == nil {
		return
	}
	directives := env.RAGDirectives
	actions := describeDirectiveActions(ports.RAGDirectives{
		UseRetrieval: executedRetrieval,
		UseSearch:    executedSearch,
		UseCrawl:     executedCrawl,
	})
	summary := fmt.Sprintf("Automated context loader executed actions: %s.", actions)
	if len(directives.SearchSeeds) > 0 {
		summary += " Focus domains: " + strings.Join(directives.SearchSeeds, ", ") + "."
	}
	if len(directives.CrawlSeeds) > 0 {
		summary += " Crawl seeds queued: " + strings.Join(directives.CrawlSeeds, ", ") + "."
	}
	if directives.Justification != nil {
		summary += " Signals:" + formatJustification(directives.Justification)
	}
	if len(notes) > 0 {
		summary += " " + strings.Join(notes, " ")
	}
	env.State.Messages = append(env.State.Messages, ports.Message{
		Role:     "system",
		Content:  summary,
		Source:   ports.MessageSourceDebug,
		Metadata: map[string]any{"rag_preload": true},
	})
}

func (p *ragPreloader) generateSearchQuery(ctx context.Context, llm ports.LLMClient, baseQuery string, seeds []string) (string, error) {
	if llm == nil {
		return "", fmt.Errorf("llm client unavailable")
	}

	baseQuery = strings.TrimSpace(baseQuery)
	seedText := strings.TrimSpace(strings.Join(seeds, ", "))
	if baseQuery == "" && seedText == "" {
		return "", fmt.Errorf("no query or seeds provided")
	}

	var userPrompt strings.Builder
	if baseQuery != "" {
		userPrompt.WriteString("Task or question: ")
		userPrompt.WriteString(baseQuery)
	} else {
		userPrompt.WriteString("Focus area requested via seeds only.")
	}
	if seedText != "" {
		userPrompt.WriteString("\nFocus phrases: ")
		userPrompt.WriteString(seedText)
	}

	req := ports.CompletionRequest{
		Messages: []ports.Message{
			{
				Role:    "system",
				Content: "You craft precise web search queries for research. When focus phrases are provided, incorporate them naturally. Respond with only the final search query, without quotes or commentary.",
			},
			{
				Role:    "user",
				Content: userPrompt.String(),
			},
		},
		Temperature: 0.2,
		MaxTokens:   48,
		Metadata: map[string]any{
			"feature": "rag_preload_search",
		},
	}

	resp, err := llm.Complete(ctx, req)
	if err != nil {
		return "", err
	}

	query := sanitizeSearchQuery(resp.Content)
	if query == "" {
		return "", fmt.Errorf("empty query from llm")
	}
	return query, nil
}

func sanitizeSearchQuery(raw string) string {
	query := strings.TrimSpace(raw)
	if query == "" {
		return ""
	}

	if idx := strings.IndexAny(query, "\r\n"); idx >= 0 {
		query = query[:idx]
	}
	query = strings.TrimSpace(query)

	lower := strings.ToLower(query)
	prefixes := []string{
		"search query:",
		"query:",
		"optimized query:",
		"final query:",
		"use this query:",
		"recommended query:",
		"search:",
		"try:",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(lower, prefix) {
			query = strings.TrimSpace(query[len(prefix):])
			break
		}
	}

	query = strings.Trim(query, "\"'`")
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}

	query = strings.Join(strings.Fields(query), " ")
	return query
}

func formatJustification(values map[string]float64) string {
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf(" %s=%.2f", key, values[key]))
	}
	return strings.Join(parts, "")
}

func (p *ragPreloader) executeTool(ctx context.Context, env *ports.ExecutionEnvironment, name string, args map[string]any) error {
	tool, err := env.Services.ToolExecutor.Get(name)
	if err != nil {
		p.logger.Warn("RAG preloader could not acquire %s: %v", name, err)
		return err
	}

	call := ports.ToolCall{
		ID:           id.NewKSUID(),
		Name:         name,
		Arguments:    args,
		SessionID:    env.Session.ID,
		TaskID:       id.TaskIDFromContext(ctx),
		ParentTaskID: id.ParentTaskIDFromContext(ctx),
	}

	result, execErr := tool.Execute(ctx, call)
	if execErr != nil {
		p.logger.Warn("RAG preloader tool %s failed: %v", name, execErr)
		return execErr
	}
	if result == nil {
		p.logger.Warn("RAG preloader tool %s returned no result", name)
		return fmt.Errorf("tool %s returned nil result", name)
	}
	if result.Error != nil {
		p.logger.Warn("RAG preloader tool %s reported error: %v", name, result.Error)
	}
	result.CallID = call.ID
	if result.Metadata == nil {
		result.Metadata = make(map[string]any)
	}
	result.Metadata["rag_preload"] = true

	p.appendResult(env, *result)
	return nil
}

func (p *ragPreloader) appendResult(env *ports.ExecutionEnvironment, result ports.ToolResult) {
	if env == nil || env.State == nil {
		return
	}

	if result.Attachments != nil {
		normalized := make(map[string]ports.Attachment, len(result.Attachments))
		for key, att := range result.Attachments {
			normalized[key] = att
		}
		result.Attachments = normalized
	}

	env.State.ToolResults = append(env.State.ToolResults, result)
	env.State.Messages = append(env.State.Messages, ports.Message{
		Role:        "assistant",
		Content:     formatToolResultContent(result),
		ToolResults: []ports.ToolResult{result},
		Attachments: result.Attachments,
		Source:      ports.MessageSourceToolResult,
		Metadata:    map[string]any{"rag_preload": true},
	})
}

func describeDirectiveActions(directives ports.RAGDirectives) string {
	actions := make([]string, 0, 3)
	if directives.UseRetrieval {
		actions = append(actions, "RETRIEVE")
	}
	if directives.UseSearch {
		actions = append(actions, "SEARCH")
	}
	if directives.UseCrawl {
		actions = append(actions, "CRAWL")
	}
	if len(actions) == 0 {
		return "SKIP"
	}
	return strings.Join(actions, "+")
}

func formatToolResultContent(result ports.ToolResult) string {
	if strings.TrimSpace(result.Content) != "" {
		return result.Content
	}
	if result.Error != nil {
		return result.Error.Error()
	}
	return ""
}
