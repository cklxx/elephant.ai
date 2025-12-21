package builtin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"alex/internal/agent/ports"
)

// a2uiEmit implements the a2ui_emit tool for storing A2UI JSONL payloads.
type a2uiEmit struct{}

// NewA2UIEmit constructs the a2ui_emit tool executor.
func NewA2UIEmit() ports.ToolExecutor {
	return &a2uiEmit{}
}

func (t *a2uiEmit) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	content := strings.TrimSpace(stringArg(call.Arguments, "content"))
	if content == "" {
		if messages := call.Arguments["messages"]; messages != nil {
			serialized, err := serializeA2UIMessages(messages)
			if err != nil {
				return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("messages must be JSON-serializable: %w", err)}, nil
			}
			content = serialized
		}
	}

	if content == "" {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("content or messages is required")}, nil
	}

	name := strings.TrimSpace(stringArg(call.Arguments, "name"))
	if name == "" {
		name = fmt.Sprintf("a2ui-%s.jsonl", sanitizeA2UIFilename(call.ID))
	}
	if filepath.Ext(name) == "" {
		name = fmt.Sprintf("%s.jsonl", name)
	}

	mediaType := strings.TrimSpace(stringArg(call.Arguments, "media_type"))
	if mediaType == "" {
		mediaType = "application/a2ui+json"
	}

	description := strings.TrimSpace(stringArg(call.Arguments, "description"))
	format := strings.TrimSpace(stringArg(call.Arguments, "format"))
	if format == "" {
		format = "a2ui"
	}
	format = normalizeFormat(format)

	kind := strings.TrimSpace(stringArg(call.Arguments, "kind"))
	if kind == "" {
		kind = "attachment"
	}
	retention := uint64Arg(call.Arguments, "retention_ttl_seconds")

	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	attachment := ports.Attachment{
		Name:                name,
		MediaType:           mediaType,
		Data:                encoded,
		URI:                 fmt.Sprintf("data:%s;base64,%s", mediaType, encoded),
		Description:         description,
		Kind:                kind,
		Format:              format,
		RetentionTTLSeconds: retention,
		Source:              call.Name,
		PreviewProfile:      "document.a2ui",
	}

	attachments := map[string]ports.Attachment{name: attachment}
	existing, _ := ports.GetAttachmentContext(ctx)

	mutationKey := "add"
	if existing != nil {
		if _, ok := existing[name]; ok {
			mutationKey = "update"
		}
	}

	mutations := map[string]any{
		"attachment_mutations": map[string]any{
			mutationKey: attachments,
		},
	}

	result := &ports.ToolResult{
		CallID:      call.ID,
		Content:     fmt.Sprintf("A2UI payload stored as %s.", name),
		Metadata:    mutations,
		Attachments: attachments,
	}
	return result, nil
}

func (t *a2uiEmit) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "a2ui_emit",
		Description: "Store an A2UI JSONL payload as an attachment for final-result rendering.",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"content":               {Type: "string", Description: "A2UI payload as JSON or JSONL string"},
				"messages":              {Type: "array", Description: "Optional array of JSON messages to serialize as JSONL when content is omitted"},
				"name":                  {Type: "string", Description: "Optional attachment filename (defaults to a2ui-<call_id>.jsonl)"},
				"media_type":            {Type: "string", Description: "MIME type (default: application/a2ui+json)"},
				"description":           {Type: "string", Description: "Optional description for the attachment"},
				"format":                {Type: "string", Description: "Format hint (default: a2ui)"},
				"kind":                  {Type: "string", Description: "Attachment kind (attachment or artifact)"},
				"retention_ttl_seconds": {Type: "integer", Description: "Override retention TTL in seconds"},
			},
		},
		MaterialCapabilities: ports.ToolMaterialCapabilities{
			Produces: []string{"application/a2ui+json"},
		},
	}
}

func (t *a2uiEmit) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "a2ui_emit",
		Version:  "1.0.0",
		Category: "attachments",
		Tags:     []string{"a2ui", "ui", "attachments"},
		MaterialCapabilities: ports.ToolMaterialCapabilities{
			Produces: []string{"application/a2ui+json"},
		},
	}
}

func sanitizeA2UIFilename(seed string) string {
	trimmed := strings.TrimSpace(seed)
	if trimmed == "" {
		return fmt.Sprintf("payload-%d", time.Now().UnixNano())
	}
	var builder strings.Builder
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			builder.WriteRune(r)
		default:
			builder.WriteRune('-')
		}
	}
	sanitized := strings.Trim(builder.String(), "-")
	if sanitized == "" {
		return fmt.Sprintf("payload-%d", time.Now().UnixNano())
	}
	return sanitized
}

func serializeA2UIMessages(raw any) (string, error) {
	switch typed := raw.(type) {
	case []any:
		lines := make([]string, 0, len(typed))
		for _, entry := range typed {
			serialized, err := json.Marshal(entry)
			if err != nil {
				return "", err
			}
			lines = append(lines, string(serialized))
		}
		return strings.Join(lines, "\n"), nil
	case []map[string]any:
		lines := make([]string, 0, len(typed))
		for _, entry := range typed {
			serialized, err := json.Marshal(entry)
			if err != nil {
				return "", err
			}
			lines = append(lines, string(serialized))
		}
		return strings.Join(lines, "\n"), nil
	default:
		serialized, err := json.Marshal(raw)
		if err != nil {
			return "", err
		}
		return string(serialized), nil
	}
}
