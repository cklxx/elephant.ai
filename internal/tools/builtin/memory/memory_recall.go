package memory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/memory"
	"alex/internal/tools/builtin/shared"
	id "alex/internal/utils/id"
)

type memoryRecall struct {
	service memory.Service
}

const recallDefaultLimit = 5

// NewMemoryRecall constructs a tool for recalling user memories.
func NewMemoryRecall(service memory.Service) tools.ToolExecutor {
	return &memoryRecall{service: service}
}

func (t *memoryRecall) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "memory_recall",
		Version:  "0.1.0",
		Category: "memory",
		Tags:     []string{"memory", "retrieval"},
	}
}

func (t *memoryRecall) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "memory_recall",
		Description: "Recall user memories using keywords and intent slots.",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"keywords": {
					Type:        "array",
					Description: "Keywords to match memories.",
					Items:       &ports.Property{Type: "string"},
				},
				"slots": {
					Type:        "object",
					Description: "Intent slots (key/value) to filter memories.",
				},
				"limit": {
					Type:        "integer",
					Description: "Maximum number of memories to return (default 5).",
				},
			},
		},
	}
}

func (t *memoryRecall) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	if t.service == nil {
		err := fmt.Errorf("memory service not configured")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	userID := id.UserIDFromContext(ctx)
	if strings.TrimSpace(userID) == "" {
		err := fmt.Errorf("user_id required for memory_recall")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	keywords := shared.StringSliceArg(call.Arguments, "keywords")
	slots := shared.StringMapArg(call.Arguments, "slots")
	if len(keywords) == 0 && len(slots) == 0 {
		err := fmt.Errorf("provide keywords or slots for recall")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	limit := recallDefaultLimit
	if rawLimit, ok := call.Arguments["limit"]; ok {
		switch v := rawLimit.(type) {
		case float64:
			if v > 0 {
				limit = int(v)
			}
		case int:
			if v > 0 {
				limit = v
			}
		}
	}

	entries, err := t.service.Recall(ctx, memory.Query{
		UserID:   userID,
		Keywords: keywords,
		Slots:    slots,
		Limit:    limit,
	})
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	metadata := map[string]any{
		"memories": SerializeMemories(entries),
	}
	content := "No memories found."
	if len(entries) > 0 {
		content = formatMemories(entries)
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  content,
		Metadata: metadata,
	}, nil
}

func formatMemories(entries []memory.Entry) string {
	var builder strings.Builder
	for _, entry := range entries {
		builder.WriteString(fmt.Sprintf("[%s] %s\n", entry.Key, entry.Content))
		if len(entry.Keywords) > 0 {
			builder.WriteString(fmt.Sprintf("  - keywords: %s\n", strings.Join(entry.Keywords, ", ")))
		}
		if len(entry.Slots) > 0 {
			builder.WriteString(fmt.Sprintf("  - slots: %v\n", entry.Slots))
		}
		builder.WriteString(fmt.Sprintf("  - created_at: %s\n", entry.CreatedAt.Format(time.RFC3339)))
	}
	return strings.TrimSpace(builder.String())
}

// SerializeMemories converts memory entries to serializable maps for tool metadata.
func SerializeMemories(entries []memory.Entry) []map[string]any {
	memories := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		memories = append(memories, map[string]any{
			"key":        entry.Key,
			"content":    entry.Content,
			"keywords":   append([]string(nil), entry.Keywords...),
			"slots":      entry.Slots,
			"created_at": entry.CreatedAt.Format(time.RFC3339),
		})
	}
	return memories
}
