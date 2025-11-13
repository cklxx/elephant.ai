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
		searchArgs := map[string]any{"query": directives.Query}
		if len(directives.SearchSeeds) > 0 {
			seeds := strings.Join(directives.SearchSeeds, " ")
			searchArgs["query"] = strings.TrimSpace(directives.Query + " " + seeds)
		}
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
		Role:    "system",
		Content: summary,
		Source:  ports.MessageSourceDebug,
	})
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
		Role:        "tool",
		Content:     formatToolResultContent(result),
		ToolCallID:  result.CallID,
		ToolResults: []ports.ToolResult{result},
		Attachments: result.Attachments,
		Source:      ports.MessageSourceToolResult,
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
