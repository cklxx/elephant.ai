package builtin

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/memory"
	id "alex/internal/utils/id"
)

type memoryWrite struct {
	service memory.Service
}

// NewMemoryWrite constructs a tool for persisting user-scoped memories.
func NewMemoryWrite(service memory.Service) tools.ToolExecutor {
	return &memoryWrite{service: service}
}

func (t *memoryWrite) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "memory_write",
		Version:  "0.1.0",
		Category: "memory",
		Tags:     []string{"memory", "state"},
	}
}

func (t *memoryWrite) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "memory_write",
		Description: "Persist a user-scoped memory entry using keywords and intent slots.",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"content": {
					Type:        "string",
					Description: "Full memory content to store.",
				},
				"keywords": {
					Type:        "array",
					Description: "Keywords describing the memory.",
					Items:       &ports.Property{Type: "string"},
				},
				"slots": {
					Type:        "object",
					Description: "Intent slots (key/value) to label the memory.",
				},
			},
			Required: []string{"content"},
		},
	}
}

func (t *memoryWrite) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	if t.service == nil {
		err := fmt.Errorf("memory service not configured")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	userID := id.UserIDFromContext(ctx)
	if strings.TrimSpace(userID) == "" {
		err := fmt.Errorf("user_id required for memory_write")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	content, ok := call.Arguments["content"].(string)
	if !ok {
		err := fmt.Errorf("missing 'content'")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	content = strings.TrimSpace(content)
	if content == "" {
		err := fmt.Errorf("content cannot be empty")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	keywords := parseKeywordArray(call.Arguments["keywords"])
	slots := parseSlotObject(call.Arguments["slots"])
	if len(keywords) == 0 && len(slots) == 0 {
		err := fmt.Errorf("provide keywords or slots to capture personalized memory")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	entry, err := t.service.Save(ctx, memory.Entry{
		UserID:   userID,
		Content:  content,
		Keywords: keywords,
		Slots:    slots,
	})
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	metadata := map[string]any{
		"key":       entry.Key,
		"keywords":  entry.Keywords,
		"slots":     entry.Slots,
		"createdAt": entry.CreatedAt.Format(time.RFC3339),
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  fmt.Sprintf("Memory stored (%s)", entry.Key),
		Metadata: metadata,
	}, nil
}

func parseKeywordArray(raw any) []string {
	values, ok := raw.([]any)
	if !ok {
		return nil
	}
	var keywords []string
	for _, item := range values {
		text, ok := item.(string)
		if !ok {
			continue
		}
		if trimmed := strings.TrimSpace(text); trimmed != "" {
			keywords = append(keywords, trimmed)
		}
	}
	return keywords
}

func parseSlotObject(raw any) map[string]string {
	obj, ok := raw.(map[string]any)
	if !ok || len(obj) == 0 {
		return nil
	}
	slots := make(map[string]string, len(obj))
	for key, value := range obj {
		text, ok := value.(string)
		if !ok {
			continue
		}
		keyTrimmed := strings.TrimSpace(key)
		valTrimmed := strings.TrimSpace(text)
		if keyTrimmed == "" || valTrimmed == "" {
			continue
		}
		slots[keyTrimmed] = valTrimmed
	}
	if len(slots) == 0 {
		return nil
	}
	return slots
}
