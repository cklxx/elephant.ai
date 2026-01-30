package web

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"alex/internal/agent/ports"
	portsllm "alex/internal/agent/ports/llm"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/httpclient"
	internalllm "alex/internal/llm"
	toolartifacts "alex/internal/tools/builtin/artifacts"
	"alex/internal/tools/builtin/shared"
	id "alex/internal/utils/id"
)

type htmlEdit struct {
	shared.BaseTool
	llm portsllm.LLMClient
}

func NewHTMLEdit(client portsllm.LLMClient) tools.ToolExecutor {
	if client == nil {
		client = internalllm.NewMockClient()
	}
	return &htmlEdit{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "html_edit",
				Description: "View, edit, or validate HTML artifacts. Loads HTML from a named attachment or inline html, applies edit instructions, and returns validation results.",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"action": {
							Type:        "string",
							Description: "Action to perform: view, edit, or validate. Defaults to edit when instructions are provided, otherwise validate.",
						},
						"name": {
							Type:        "string",
							Description: "HTML artifact filename or placeholder to load (e.g. game.html or [game.html])",
						},
						"html": {
							Type:        "string",
							Description: "Raw HTML to edit/validate (optional; overrides name)",
						},
						"instructions": {
							Type:        "string",
							Description: "Edit instructions to apply when action=edit",
						},
						"output_name": {
							Type:        "string",
							Description: "Output artifact filename (default: name or edited.html)",
						},
						"validate_only": {
							Type:        "boolean",
							Description: "If true, skip edits and only validate",
						},
					},
				},
			},
			ports.ToolMetadata{
				Name:     "html_edit",
				Version:  "1.0.0",
				Category: "web",
				Tags:     []string{"html", "edit", "validate"},
				MaterialCapabilities: ports.ToolMaterialCapabilities{
					Produces: []string{"text/html"},
				},
			},
		),
		llm: client,
	}
}

func (t *htmlEdit) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	args := call.Arguments
	action := strings.ToLower(strings.TrimSpace(shared.StringArg(args, "action")))
	name := toolartifacts.UnwrapArtifactPlaceholderName(shared.StringArg(args, "name"))
	rawHTML := strings.TrimSpace(shared.StringArg(args, "html"))
	instructions := strings.TrimSpace(shared.StringArg(args, "instructions"))
	outputName := strings.TrimSpace(shared.StringArg(args, "output_name"))
	validateOnly := boolArg(args, "validate_only")

	if action == "" {
		if instructions != "" {
			action = "edit"
		} else {
			action = "validate"
		}
	}

	switch action {
	case "view", "validate":
		validateOnly = true
	case "edit":
		if instructions == "" {
			return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("instructions are required for edit")}, nil
		}
	default:
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("unsupported action: %s", action)}, nil
	}

	sourceHTML, sourceName, err := resolveHTMLInput(ctx, name, rawHTML)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}

	if outputName == "" {
		if name != "" {
			outputName = name
		} else {
			outputName = "edited.html"
		}
	}

	metadata := map[string]any{}

	editedHTML := sourceHTML
	edited := false
	if !validateOnly {
		updated, meta, err := t.applyEdits(ctx, sourceHTML, instructions)
		if err != nil {
			return &ports.ToolResult{CallID: call.ID, Error: err}, nil
		}
		editedHTML = updated
		if meta != nil {
			if meta.RequestID != "" {
				metadata["llm_request_id"] = meta.RequestID
			}
			if meta.Duration > 0 {
				metadata["llm_duration_ms"] = meta.Duration.Milliseconds()
			}
			if meta.Model != "" {
				metadata["llm_model"] = meta.Model
			}
		}
		edited = true
	}

	issues := validateHTMLSource(editedHTML)
	errors, warnings := splitValidationIssues(issues)

	attachment := buildHTMLAttachment(outputName, editedHTML, "html_edit")
	attachments := map[string]ports.Attachment{outputName: attachment}

	mutationKey := "add"
	if existing, _ := tools.GetAttachmentContext(ctx); existing != nil {
		if _, ok := existing[outputName]; ok {
			mutationKey = "update"
		}
	}

	metadata["source_name"] = sourceName
	metadata["output_name"] = outputName
	metadata["edited"] = edited
	metadata["validation"] = map[string]any{
		"error_count":   len(errors),
		"warning_count": len(warnings),
		"issues":        issues,
	}
	metadata["attachment_mutations"] = map[string]any{
		mutationKey: attachments,
	}

	content := buildHTMLResultSummary(action, outputName, len(errors), len(warnings))
	if action == "view" {
		content = editedHTML
	}

	return &ports.ToolResult{
		CallID:      call.ID,
		Content:     content,
		Attachments: attachments,
		Metadata:    metadata,
	}, nil
}

func (t *htmlEdit) applyEdits(ctx context.Context, html, instructions string) (string, *llmCallMeta, error) {
	prompt := buildHTMLEditPrompt(html, instructions)
	meta := &llmCallMeta{}
	meta.RequestID = id.NewRequestIDWithLogID(id.LogIDFromContext(ctx))
	if t.llm != nil {
		meta.Model = strings.TrimSpace(t.llm.Model())
	}
	started := time.Now()
	resp, err := t.llm.Complete(ctx, ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "system", Content: "You are a precise HTML editor. Apply the requested changes and return ONLY the full HTML document with minimal unrelated changes."},
			{Role: "user", Content: prompt},
		},
		Temperature: 0.2,
		MaxTokens:   1500,
		Metadata: map[string]any{
			"request_id": meta.RequestID,
		},
	})
	if err != nil {
		meta.Duration = time.Since(started)
		return "", meta, fmt.Errorf("html_edit failed: %w", err)
	}
	meta.Duration = time.Since(started)
	updated := extractHTMLFromResponse(resp.Content)
	if updated == "" {
		return "", meta, fmt.Errorf("html_edit produced empty output")
	}
	if !looksLikeHTML(updated) {
		return "", meta, fmt.Errorf("html_edit did not return valid HTML")
	}
	return updated, meta, nil
}

func buildHTMLEditPrompt(html, instructions string) string {
	var builder strings.Builder
	builder.WriteString("Instructions:\n")
	builder.WriteString(instructions)
	builder.WriteString("\n\nOriginal HTML:\n<<<HTML\n")
	builder.WriteString(html)
	builder.WriteString("\nHTML\n\nReturn ONLY the updated HTML document.")
	return builder.String()
}

type htmlValidationIssue struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

func validateHTMLSource(html string) []htmlValidationIssue {
	trimmed := strings.TrimSpace(html)
	if trimmed == "" {
		return []htmlValidationIssue{{Level: "error", Message: "HTML is empty."}}
	}

	lower := strings.ToLower(trimmed)
	var issues []htmlValidationIssue

	htmlTags := strings.Count(lower, "<html")
	headTags := strings.Count(lower, "<head")
	bodyTags := strings.Count(lower, "<body")

	if !strings.Contains(lower, "<!doctype html") {
		issues = append(issues, htmlValidationIssue{Level: "warning", Message: "Missing <!DOCTYPE html>."})
	}
	if htmlTags == 0 {
		issues = append(issues, htmlValidationIssue{Level: "warning", Message: "Missing <html> tag."})
	} else if htmlTags > 1 {
		issues = append(issues, htmlValidationIssue{Level: "error", Message: "Multiple <html> tags found."})
	}
	if headTags == 0 {
		issues = append(issues, htmlValidationIssue{Level: "warning", Message: "Missing <head> tag."})
	} else if headTags > 1 {
		issues = append(issues, htmlValidationIssue{Level: "warning", Message: "Multiple <head> tags found."})
	}
	if bodyTags == 0 {
		issues = append(issues, htmlValidationIssue{Level: "warning", Message: "Missing <body> tag."})
	} else if bodyTags > 1 {
		issues = append(issues, htmlValidationIssue{Level: "warning", Message: "Multiple <body> tags found."})
	}
	if !strings.Contains(lower, "<meta charset") {
		issues = append(issues, htmlValidationIssue{Level: "warning", Message: "Missing <meta charset>."})
	}
	if !strings.Contains(lower, "name=\"viewport\"") && !strings.Contains(lower, "name='viewport'") {
		issues = append(issues, htmlValidationIssue{Level: "warning", Message: "Missing <meta name=\"viewport\">."})
	}
	if !strings.Contains(lower, "<title") {
		issues = append(issues, htmlValidationIssue{Level: "warning", Message: "Missing <title> tag."})
	}

	openScripts := strings.Count(lower, "<script")
	closeScripts := strings.Count(lower, "</script>")
	if openScripts != closeScripts {
		issues = append(issues, htmlValidationIssue{Level: "error", Message: "Mismatched <script> tags."})
	}

	openStyles := strings.Count(lower, "<style")
	closeStyles := strings.Count(lower, "</style>")
	if openStyles != closeStyles {
		issues = append(issues, htmlValidationIssue{Level: "warning", Message: "Mismatched <style> tags."})
	}

	return issues
}

func splitValidationIssues(issues []htmlValidationIssue) (errors []htmlValidationIssue, warnings []htmlValidationIssue) {
	for _, issue := range issues {
		if issue.Level == "error" {
			errors = append(errors, issue)
		} else {
			warnings = append(warnings, issue)
		}
	}
	return errors, warnings
}

func buildHTMLResultSummary(action, name string, errorCount, warningCount int) string {
	verb := "Validated"
	if action == "edit" {
		verb = "Updated"
	}
	return fmt.Sprintf("%s HTML: %s (errors: %d, warnings: %d)", verb, name, errorCount, warningCount)
}

func resolveHTMLInput(ctx context.Context, name, rawHTML string) (string, string, error) {
	if rawHTML != "" {
		if decoded, ok := decodeHTMLDataURI(rawHTML); ok {
			return string(decoded), name, nil
		}
		return rawHTML, name, nil
	}
	if name == "" {
		return "", "", fmt.Errorf("name or html is required")
	}

	attachments, _ := tools.GetAttachmentContext(ctx)
	if len(attachments) == 0 {
		return "", name, fmt.Errorf("no attachments available")
	}

	att, canonical, ok := lookupAttachmentByName(attachments, name)
	if !ok {
		return "", name, fmt.Errorf("attachment not found: %s", name)
	}

	html, err := decodeHTMLAttachment(ctx, att)
	if err != nil {
		return "", canonical, err
	}

	return html, canonical, nil
}

func lookupAttachmentByName(attachments map[string]ports.Attachment, name string) (ports.Attachment, string, bool) {
	if att, ok := attachments[name]; ok {
		return att, name, true
	}
	for key, att := range attachments {
		if strings.EqualFold(key, name) || strings.EqualFold(att.Name, name) {
			return att, key, true
		}
	}
	return ports.Attachment{}, "", false
}

func decodeHTMLAttachment(ctx context.Context, att ports.Attachment) (string, error) {
	if data := strings.TrimSpace(att.Data); data != "" {
		if decoded, ok := decodeHTMLDataURI(data); ok {
			return string(decoded), nil
		}
		decoded, err := base64.StdEncoding.DecodeString(data)
		if err == nil {
			return string(decoded), nil
		}
	}

	if uri := strings.TrimSpace(att.URI); uri != "" {
		if decoded, ok := decodeHTMLDataURI(uri); ok {
			return string(decoded), nil
		}
		if strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://") {
			return fetchHTML(ctx, uri)
		}
	}

	return "", fmt.Errorf("no inline HTML payload available for %s", att.Name)
}

func fetchHTML(ctx context.Context, uri string) (string, error) {
	opts := httpclient.DefaultURLValidationOptions()
	if shared.AllowLocalFetch(ctx) {
		opts.AllowLocalhost = true
	}
	parsed, err := httpclient.ValidateOutboundURL(uri, opts)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	// URL is validated by ValidateOutboundURL before request construction.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("fetch failed with status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func decodeHTMLDataURI(value string) ([]byte, bool) {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(strings.ToLower(trimmed), "data:") {
		return nil, false
	}
	parts := strings.SplitN(trimmed, ",", 2)
	if len(parts) != 2 {
		return nil, false
	}
	meta := strings.ToLower(parts[0])
	payload := parts[1]
	if strings.Contains(meta, ";base64") {
		decoded, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return nil, false
		}
		return decoded, true
	}
	unescaped, err := url.PathUnescape(payload)
	if err != nil {
		return nil, false
	}
	return []byte(unescaped), true
}

func extractHTMLFromResponse(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}
	parts := strings.Split(trimmed, "```")
	if len(parts) < 3 {
		return trimmed
	}
	block := strings.TrimSpace(parts[1])
	if block == "" {
		return trimmed
	}
	lines := strings.SplitN(block, "\n", 2)
	lang := strings.ToLower(strings.TrimSpace(lines[0]))
	if lang == "html" && len(lines) == 2 {
		return strings.TrimSpace(lines[1])
	}
	return strings.TrimSpace(block)
}

func looksLikeHTML(value string) bool {
	lower := strings.ToLower(value)
	return strings.Contains(lower, "<html") || strings.Contains(lower, "<!doctype") || strings.Contains(lower, "<body")
}

func buildHTMLAttachment(name, html, source string) ports.Attachment {
	encoded := base64.StdEncoding.EncodeToString([]byte(html))
	return ports.Attachment{
		Name:           name,
		MediaType:      "text/html",
		Data:           encoded,
		URI:            fmt.Sprintf("data:text/html;base64,%s", encoded),
		Source:         source,
		Description:    "HTML output",
		Kind:           "artifact",
		Format:         "html",
		PreviewProfile: "document.html",
		PreviewAssets: []ports.AttachmentPreviewAsset{
			{
				AssetID:     "html-preview",
				Label:       "HTML preview",
				MimeType:    "text/html",
				CDNURL:      fmt.Sprintf("data:text/html;base64,%s", encoded),
				PreviewType: "iframe",
			},
		},
	}
}

func boolArg(args map[string]any, key string) bool {
	if args == nil {
		return false
	}
	value, ok := args[key]
	if !ok || value == nil {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		trimmed := strings.ToLower(strings.TrimSpace(typed))
		return trimmed == "true" || trimmed == "1" || trimmed == "yes"
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case float64:
		return typed != 0
	}
	return false
}
