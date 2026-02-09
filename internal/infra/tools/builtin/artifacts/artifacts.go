package artifacts

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/shared"
)

// artifactsWrite implements the artifacts_write tool which creates or updates
// attachments and emits mutation metadata for the attachment registry.
type artifactsWrite struct {
	shared.BaseTool
}

// artifactsList implements the artifacts_list tool to surface attachments
// available to the current task, optionally returning a specific payload.
type artifactsList struct {
	shared.BaseTool
}

// artifactsDelete implements the artifacts_delete tool that removes one or more
// attachments via mutation metadata, leaving actual persistence to the engine.
type artifactsDelete struct {
	shared.BaseTool
}

// NewArtifactsWrite constructs the artifacts_write tool executor.
func NewArtifactsWrite() tools.ToolExecutor {
	return &artifactsWrite{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "artifacts_write",
				Description: "Create/update durable artifact files for downstream handoff (reports/specs/evidence). Use for persistent deliverables, not for listing existing artifacts. For HTML outputs, set media_type=text/html and format=html.",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"name":                  {Type: "string", Description: "Filename for the artifact (e.g., note.md or page.html)"},
						"content":               {Type: "string", Description: "Raw text content to store (HTML is supported)"},
						"media_type":            {Type: "string", Description: "MIME type (default: text/markdown); use text/html for HTML"},
						"description":           {Type: "string", Description: "Optional description for context"},
						"format":                {Type: "string", Description: "Normalized format such as markdown or html"},
						"kind":                  {Type: "string", Description: "Attachment kind (attachment or artifact)"},
						"retention_ttl_seconds": {Type: "integer", Description: "Override retention TTL in seconds"},
					},
					Required: []string{"name", "content"},
				},
			},
			ports.ToolMetadata{
				Name:     "artifacts_write",
				Version:  "1.0.0",
				Category: "attachments",
				Tags:     []string{"artifact", "write", "report", "deliverable", "persist"},
			},
		),
	}
}

// NewArtifactsList constructs the artifacts_list tool executor.
func NewArtifactsList() tools.ToolExecutor {
	return &artifactsList{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "artifacts_list",
				Description: "List/index artifacts currently available to the task. Use for inventory/selection before publish; use artifacts_write to create new artifacts.",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"name": {Type: "string", Description: "Optional attachment name to return in full"},
					},
				},
			},
			ports.ToolMetadata{
				Name:     "artifacts_list",
				Version:  "1.0.0",
				Category: "attachments",
				Tags:     []string{"artifact", "list", "inventory", "manifest", "index"},
			},
		),
	}
}

// NewArtifactsDelete constructs the artifacts_delete tool executor.
func NewArtifactsDelete() tools.ToolExecutor {
	return &artifactsDelete{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "artifacts_delete",
				Description: "Remove one or more attachments from the current task",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"name":  {Type: "string", Description: "Single attachment name to remove"},
						"names": {Type: "array", Description: "List of attachment names to remove", Items: &ports.Property{Type: "string"}},
					},
				},
			},
			ports.ToolMetadata{
				Name:     "artifacts_delete",
				Version:  "1.0.0",
				Category: "attachments",
				Tags:     []string{"artifact", "delete", "cleanup", "remove"},
			},
		),
	}
}

func (t *artifactsWrite) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	name := UnwrapArtifactPlaceholderName(shared.StringArg(call.Arguments, "name"))
	if name == "" {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("name is required")}, nil
	}

	content := shared.StringArg(call.Arguments, "content")
	mediaType := strings.TrimSpace(shared.StringArg(call.Arguments, "media_type"))
	if mediaType == "" {
		mediaType = "text/markdown"
	}

	description := strings.TrimSpace(shared.StringArg(call.Arguments, "description"))
	format := strings.TrimSpace(shared.StringArg(call.Arguments, "format"))
	if format == "" {
		format = strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	}
	if format == "" && strings.Contains(strings.ToLower(mediaType), "markdown") {
		format = "markdown"
	}
	format = normalizeFormat(format)
	if description == "" {
		description = strings.TrimSpace(deriveAttachmentDescription(name, content, mediaType, format))
	}

	kind := strings.TrimSpace(shared.StringArg(call.Arguments, "kind"))
	if kind == "" {
		kind = "artifact"
	}
	retention := shared.Uint64Arg(call.Arguments, "retention_ttl_seconds")

	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	sum := sha256.Sum256([]byte(content))
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
		PreviewProfile:      shared.PreviewProfile(mediaType, format),
	}

	if snippet := strings.TrimSpace(shared.ContentSnippet(content, 240)); snippet != "" {
		attachment.PreviewAssets = []ports.AttachmentPreviewAsset{{
			AssetID:     fmt.Sprintf("%s-thumb", name),
			Label:       "Thumbnail",
			MimeType:    "text/plain",
			CDNURL:      fmt.Sprintf("data:text/plain;base64,%s", base64.StdEncoding.EncodeToString([]byte(snippet))),
			PreviewType: "thumbnail",
		}}
	}

	attachments := map[string]ports.Attachment{name: attachment}
	existing, _ := tools.GetAttachmentContext(ctx)

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
		"content_len":    len(content),
		"content_sha256": fmt.Sprintf("%x", sum),
	}

	result := &ports.ToolResult{
		CallID:      call.ID,
		Content:     fmt.Sprintf("Saved %s (%s)", name, mediaType),
		Metadata:    mutations,
		Attachments: attachments,
	}
	return result, nil
}

func deriveAttachmentDescription(name, content, mediaType, format string) string {
	if strings.TrimSpace(content) == "" {
		return ""
	}

	normalizedMedia := strings.ToLower(strings.TrimSpace(mediaType))
	normalizedFormat := strings.ToLower(strings.TrimSpace(format))
	isMarkdown := strings.Contains(normalizedMedia, "markdown") || normalizedFormat == "markdown" || normalizedFormat == "md"
	if !isMarkdown {
		return ""
	}

	title := deriveMarkdownTitle(content)
	if title == "" {
		return ""
	}

	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}

	// Avoid echoing the filename as the description.
	if strings.EqualFold(title, strings.TrimSpace(name)) {
		return ""
	}

	const maxTitleRunes = 80
	runes := []rune(title)
	if len(runes) > maxTitleRunes {
		title = string(runes[:maxTitleRunes])
	}

	return title
}

func deriveMarkdownTitle(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if i >= 60 {
			break
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if strings.HasPrefix(trimmed, "#") {
			idx := 0
			for idx < len(trimmed) && trimmed[idx] == '#' {
				idx += 1
			}
			candidate := strings.TrimSpace(trimmed[idx:])
			candidate = strings.TrimSpace(strings.TrimRight(candidate, "#"))
			if candidate != "" {
				return candidate
			}
		}
	}
	return ""
}

func (t *artifactsList) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	attachments, _ := tools.GetAttachmentContext(ctx)
	if len(attachments) == 0 {
		return &ports.ToolResult{CallID: call.ID, Content: "No attachments available"}, nil
	}

	target := UnwrapArtifactPlaceholderName(shared.StringArg(call.Arguments, "name"))
	var builder strings.Builder
	builder.WriteString("Attachments on record:\n")

	resultAttachments := make(map[string]ports.Attachment)
	for key, att := range attachments {
		builder.WriteString(fmt.Sprintf("- %s (%s)\n", att.Name, att.MediaType))
		if target != "" && (key == target || att.Name == target || strings.EqualFold(key, target) || strings.EqualFold(att.Name, target)) {
			resultAttachments[key] = att
		}
	}

	if target != "" && len(resultAttachments) == 0 {
		for key, att := range attachments {
			uri := strings.TrimSpace(att.URI)
			if uri == "" {
				continue
			}
			if uri == target || strings.EqualFold(uri, target) {
				resultAttachments[key] = att
				break
			}
		}
	}

	if target != "" && len(resultAttachments) == 0 {
		if payload, ok := extractDataURIBase64(target); ok {
			for key, att := range attachments {
				if strings.TrimSpace(att.Data) == payload {
					resultAttachments[key] = att
					break
				}
			}
		}
	}

	// If a specific attachment was requested but not found, return an error
	if target != "" && len(resultAttachments) == 0 {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("attachment not found: %s", target)}, nil
	}

	// When a target is provided, return only that attachment payload for rendering
	if len(resultAttachments) > 0 {
		return &ports.ToolResult{CallID: call.ID, Content: builder.String(), Attachments: resultAttachments}, nil
	}

	return &ports.ToolResult{CallID: call.ID, Content: builder.String()}, nil
}

func (t *artifactsDelete) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	names := shared.StringSliceArg(call.Arguments, "names")
	if len(names) == 0 {
		if single := UnwrapArtifactPlaceholderName(shared.StringArg(call.Arguments, "name")); single != "" {
			names = append(names, single)
		}
	}
	if len(names) == 0 {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("at least one name is required")}, nil
	}

	for i := range names {
		names[i] = UnwrapArtifactPlaceholderName(names[i])
	}

	mutations := map[string]any{
		"attachment_mutations": map[string]any{
			"remove": names,
		},
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  fmt.Sprintf("Removed %d attachment(s): %s", len(names), strings.Join(names, ", ")),
		Metadata: mutations,
	}, nil
}

func normalizeFormat(format string) string {
	normalized := strings.ToLower(strings.TrimSpace(format))
	switch normalized {
	case "md", "markdown", "mkd", "mdown":
		return "markdown"
	case "htm":
		return "html"
	}
	return normalized
}

// UnwrapArtifactPlaceholderName strips placeholder brackets from artifact names.
func UnwrapArtifactPlaceholderName(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) < 3 {
		return trimmed
	}
	if !strings.HasPrefix(trimmed, "[") || !strings.HasSuffix(trimmed, "]") {
		return trimmed
	}
	name := strings.TrimSpace(trimmed[1 : len(trimmed)-1])
	if name == "" {
		return trimmed
	}
	return name
}

func extractDataURIBase64(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", false
	}
	lower := strings.ToLower(trimmed)
	if !strings.HasPrefix(lower, "data:") {
		return "", false
	}
	comma := strings.Index(trimmed, ",")
	if comma < 0 {
		return "", false
	}
	meta := strings.ToLower(trimmed[:comma])
	if !strings.Contains(meta, ";base64") {
		return "", false
	}
	payload := strings.TrimSpace(trimmed[comma+1:])
	if payload == "" {
		return "", false
	}
	return payload, true
}
