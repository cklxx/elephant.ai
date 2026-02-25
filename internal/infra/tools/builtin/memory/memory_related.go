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
	"alex/internal/shared/utils"
)

type memoryRelated struct {
	shared.BaseTool
	engine memory.Engine
}

const relatedDefaultLimit = 6

// NewMemoryRelated constructs a tool for graph-style related-memory traversal.
func NewMemoryRelated(engine memory.Engine) tools.ToolExecutor {
	return &memoryRelated{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "memory_related",
				Description: "Find memory notes related to a known memory path (and optional line span) discovered via memory_search/memory_get. Use this to expand recall across linked memory entries before opening exact lines with memory_get. Not for repository/workspace source files.",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"path": {
							Type:        "string",
							Description: "Memory path returned by memory_search.",
						},
						"from": {
							Type:        "integer",
							Description: "Optional start line (1-based) for local link extraction.",
						},
						"to": {
							Type:        "integer",
							Description: "Optional end line (1-based) for local link extraction.",
						},
						"maxResults": {
							Type:        "integer",
							Description: "Maximum related entries to return (default 6).",
						},
					},
					Required: []string{"path"},
				},
			},
			ports.ToolMetadata{
				Name:        "memory_related",
				Version:     "0.1.0",
				Category:    "memory",
				Tags:        []string{"memory", "graph", "related", "recall", "continuity"},
				SafetyLevel: ports.SafetyLevelReadOnly,
			},
		),
		engine: engine,
	}
}

func (t *memoryRelated) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	if t.engine == nil {
		err := fmt.Errorf("memory engine not configured")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	userID := id.UserIDFromContext(ctx)
	if utils.IsBlank(userID) {
		err := fmt.Errorf("user_id required for memory_related")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	path, errResult := shared.RequireStringArg(call.Arguments, call.ID, "path")
	if errResult != nil {
		return errResult, nil
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return shared.ToolError(call.ID, "path cannot be empty")
	}

	fromLine := 0
	if raw, ok := call.Arguments["from"]; ok {
		switch v := raw.(type) {
		case float64:
			if v > 0 {
				fromLine = int(v)
			}
		case int:
			if v > 0 {
				fromLine = v
			}
		}
	}
	toLine := 0
	if raw, ok := call.Arguments["to"]; ok {
		switch v := raw.(type) {
		case float64:
			if v > 0 {
				toLine = int(v)
			}
		case int:
			if v > 0 {
				toLine = v
			}
		}
	}
	maxResults := relatedDefaultLimit
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

	related, err := t.engine.Related(ctx, userID, path, fromLine, toLine, maxResults)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	metadata := map[string]any{
		"path":       path,
		"from":       fromLine,
		"to":         toLine,
		"maxResults": maxResults,
		"results":    related,
	}
	content := "No related memories found."
	if len(related) > 0 {
		content = formatRelatedResults(related)
	}
	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  content,
		Metadata: metadata,
	}, nil
}

func formatRelatedResults(results []memory.RelatedHit) string {
	var sb strings.Builder
	for _, hit := range results {
		relation := strings.TrimSpace(hit.RelationType)
		if relation == "" {
			relation = "related"
		}
		sb.WriteString(fmt.Sprintf("%s:%d-%d [%s] (score=%.2f)\n", hit.Path, hit.StartLine, hit.EndLine, relation, hit.Score))
		if snippet := strings.TrimSpace(hit.Snippet); snippet != "" {
			sb.WriteString(snippet)
			sb.WriteString("\n")
		}
	}
	return strings.TrimSpace(sb.String())
}
