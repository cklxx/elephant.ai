package memory

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/memory"
	"alex/internal/infra/tools/builtin/shared"
	id "alex/internal/shared/utils/id"
)

type memorySearch struct {
	shared.BaseTool
	engine memory.Engine
}

const (
	searchDefaultLimit = 6
	searchDefaultMin   = 0.35
)

// NewMemorySearch constructs a tool for searching user memories.
func NewMemorySearch(engine memory.Engine) tools.ToolExecutor {
	return &memorySearch{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "memory_search",
				Description: "Search user memories stored as Markdown files.",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"query": {
							Type:        "string",
							Description: "Free-form query text for memory search.",
						},
						"maxResults": {
							Type:        "integer",
							Description: "Maximum number of results to return (default 6).",
						},
						"minScore": {
							Type:        "number",
							Description: "Minimum score threshold (default 0.35).",
						},
					},
					Required: []string{"query"},
				},
			},
			ports.ToolMetadata{
				Name:        "memory_search",
				Version:     "0.1.0",
				Category:    "memory",
				Tags:        []string{"memory", "retrieval"},
				SafetyLevel: ports.SafetyLevelReadOnly,
			},
		),
		engine: engine,
	}
}

func (t *memorySearch) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	if t.engine == nil {
		err := fmt.Errorf("memory engine not configured")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	userID := id.UserIDFromContext(ctx)
	if strings.TrimSpace(userID) == "" {
		err := fmt.Errorf("user_id required for memory_search")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	query, errResult := shared.RequireStringArg(call.Arguments, call.ID, "query")
	if errResult != nil {
		return errResult, nil
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return shared.ToolError(call.ID, "query cannot be empty")
	}

	maxResults := searchDefaultLimit
	if raw, ok := call.Arguments["maxResults"]; ok {
		switch v := raw.(type) {
		case float64:
			if v > 0 {
				maxResults = int(v)
			}
		case int:
			if v > 0 {
				maxResults = v
			}
		}
	}

	minScore := searchDefaultMin
	if raw, ok := call.Arguments["minScore"]; ok {
		switch v := raw.(type) {
		case float64:
			if v > 0 {
				minScore = v
			}
		case int:
			if v > 0 {
				minScore = float64(v)
			}
		}
	}

	results, err := t.engine.Search(ctx, userID, query, maxResults, minScore)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	metadata := map[string]any{
		"results": results,
	}
	content := "No memories found."
	if len(results) > 0 {
		content = formatSearchResults(results)
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  content,
		Metadata: metadata,
	}, nil
}

func formatSearchResults(results []memory.SearchHit) string {
	var sb strings.Builder
	for _, hit := range results {
		sb.WriteString(fmt.Sprintf("%s:%d-%d (score=%.2f)\n", hit.Path, hit.StartLine, hit.EndLine, hit.Score))
		if hit.Snippet != "" {
			sb.WriteString(hit.Snippet)
			sb.WriteString("\n")
		}
	}
	return strings.TrimSpace(sb.String())
}
