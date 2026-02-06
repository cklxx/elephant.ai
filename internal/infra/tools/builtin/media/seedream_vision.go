package media

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/shared"

	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
)

type seedreamVisionTool struct {
	config  SeedreamConfig
	factory *seedreamClientFactory
}

type visionTool struct {
	shared.BaseTool
	seedream *seedreamVisionTool
}

func newSeedreamVisionTool(config SeedreamConfig) *seedreamVisionTool {
	return &seedreamVisionTool{
		config:  config,
		factory: &seedreamClientFactory{config: config},
	}
}

// NewSeedreamVisionAnalyze returns a tool that analyzes images with the vision model.
func NewSeedreamVisionAnalyze(config SeedreamConfig) tools.ToolExecutor {
	return NewVisionAnalyze(VisionConfig{Provider: VisionProviderSeedream, Seedream: config})
}

// NewVisionAnalyze returns a provider-agnostic vision tool (defaults to Seedream).
func NewVisionAnalyze(config VisionConfig) tools.ToolExecutor {
	provider := strings.TrimSpace(strings.ToLower(config.Provider))
	if provider == "" {
		provider = VisionProviderSeedream
	}
	base := shared.NewBaseTool(
		ports.ToolDefinition{
			Name:        "vision_analyze",
			Description: "Use the Doubao multimodal vision model to describe or answer questions about supplied image URLs.",
			Parameters: ports.ParameterSchema{
				Type: "object",
				Properties: map[string]ports.Property{
					"images": {
						Type:        "array",
						Description: "List of image URLs or data URIs to analyze.",
						Items:       &ports.Property{Type: "string"},
					},
					"prompt": {
						Type:        "string",
						Description: "Question or instruction for the vision model.",
					},
					"detail": {
						Type:        "string",
						Description: "Detail level for the vision model: low, high, or auto.",
						Enum:        []any{"low", "high", "auto"},
					},
				},
				Required: []string{"images"},
			},
			MaterialCapabilities: ports.ToolMaterialCapabilities{
				Consumes: []string{"image/png", "image/jpeg", "image/webp", "image/gif"},
			},
		},
		ports.ToolMetadata{
			Name:     "vision_analyze",
			Version:  "1.0.0",
			Category: "analysis",
			Tags:     []string{"vision", "analysis", "seedream", "multimodal"},
			MaterialCapabilities: ports.ToolMaterialCapabilities{
				Consumes: []string{"image/png", "image/jpeg", "image/webp", "image/gif"},
			},
		},
	)
	switch provider {
	case VisionProviderSeedream:
		return &visionTool{
			BaseTool: base,
			seedream: newSeedreamVisionTool(config.Seedream),
		}
	default:
		return &visionTool{
			BaseTool: base,
			seedream: newSeedreamVisionTool(config.Seedream),
		}
	}
}

func (t *seedreamVisionTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "vision_analyze",
		Version:  "1.0.0",
		Category: "analysis",
		Tags:     []string{"vision", "analysis", "seedream", "multimodal"},
		MaterialCapabilities: ports.ToolMaterialCapabilities{
			Consumes: []string{"image/png", "image/jpeg", "image/webp", "image/gif"},
		},
	}
}

func (t *seedreamVisionTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "vision_analyze",
		Description: "Use the Doubao multimodal vision model to describe or answer questions about supplied image URLs.",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"images": {
					Type:        "array",
					Description: "List of image URLs or data URIs to analyze.",
					Items:       &ports.Property{Type: "string"},
				},
				"prompt": {
					Type:        "string",
					Description: "Question or instruction for the vision model.",
				},
				"detail": {
					Type:        "string",
					Description: "Detail level for the vision model: low, high, or auto.",
					Enum:        []any{"low", "high", "auto"},
				},
			},
			Required: []string{"images"},
		},
		MaterialCapabilities: ports.ToolMaterialCapabilities{
			Consumes: []string{"image/png", "image/jpeg", "image/webp", "image/gif"},
		},
	}
}

func (t *seedreamVisionTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	if msg := seedreamMissingConfigMessage(t.config); msg != "" {
		return &ports.ToolResult{CallID: call.ID, Content: msg}, nil
	}

	rawImages := readStringSlice(call.Arguments["images"])
	if len(rawImages) == 0 {
		err := errors.New("images parameter must include at least one URL or data URI")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	images, sourceNote, err := resolveSeedreamVisionImages(ctx, rawImages)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	prompt, _ := call.Arguments["prompt"].(string)
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		prompt = "Describe the images in detail."
	}
	if sourceNote != "" {
		if prompt != "" {
			prompt = prompt + "\n\n" + sourceNote
		} else {
			prompt = sourceNote
		}
	}

	detail := responses.ContentItemImageDetail_auto.Enum()
	if detailStr, ok := call.Arguments["detail"].(string); ok {
		switch strings.ToLower(strings.TrimSpace(detailStr)) {
		case "low":
			detail = responses.ContentItemImageDetail_low.Enum()
		case "high":
			detail = responses.ContentItemImageDetail_high.Enum()
		}
	}

	contentItems, err := buildVisionContent(images, prompt, detail)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	requestID := strings.TrimSpace(call.ID)

	req := &responses.ResponsesRequest{
		Model: t.config.Model,
		Input: &responses.ResponsesInput{
			Union: &responses.ResponsesInput_ListValue{
				ListValue: &responses.InputItemList{
					ListValue: []*responses.InputItem{
						{
							Union: &responses.InputItem_InputMessage{
								InputMessage: &responses.ItemInputMessage{
									Role:    responses.MessageRole_user,
									Content: contentItems,
								},
							},
						},
					},
				},
			},
		},
	}

	client, err := t.factory.instance()
	if err != nil {
		wrapped := fmt.Errorf("seedream client init: %w", err)
		return &ports.ToolResult{CallID: call.ID, Content: wrapped.Error(), Error: wrapped}, nil
	}

	logSeedreamRequestPayload(requestID, req)

	resp, err := client.CreateResponses(ctx, req)
	if err != nil {
		logSeedreamResponsePayload(requestID, map[string]any{"error": err.Error()})
		return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("Seedream vision request failed: %v", err), Error: err}, nil
	}
	logSeedreamResponsePayload(requestID, resp)
	if resp.Error != nil {
		apiErr := fmt.Sprintf("Seedream API error (%s): %s", resp.Error.GetCode(), resp.Error.GetMessage())
		return &ports.ToolResult{CallID: call.ID, Content: apiErr, Error: errors.New(apiErr)}, nil
	}

	answer := collectVisionText(resp)
	if answer == "" {
		answer = "Seedream vision model returned no textual output."
	}

	metadata := map[string]any{
		"model":        resp.GetModel(),
		"response_id":  resp.GetId(),
		"status":       resp.GetStatus().String(),
		"created_at":   resp.GetCreatedAt(),
		"usage":        resp.GetUsage(),
		"description":  t.config.ModelDescriptor,
		"image_count":  len(images),
		"prompt":       prompt,
		"detail_level": detail.String(),
	}

	return &ports.ToolResult{CallID: call.ID, Content: answer, Metadata: metadata}, nil
}

func (t *visionTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	if t.seedream != nil {
		return t.seedream.Execute(ctx, call)
	}
	err := errors.New("vision provider not configured")
	return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
}

func buildVisionContent(images []string, prompt string, detail *responses.ContentItemImageDetail_Enum) ([]*responses.ContentItem, error) {
	content := make([]*responses.ContentItem, 0, len(images)+1)
	for _, raw := range images {
		item, err := buildVisionImageItem(strings.TrimSpace(raw), detail)
		if err != nil {
			return nil, err
		}
		if item != nil {
			content = append(content, item)
		}
	}
	if len(content) == 0 {
		return nil, errors.New("images parameter must include at least one non-empty value")
	}
	content = append(content, &responses.ContentItem{
		Union: &responses.ContentItem_Text{
			Text: &responses.ContentItemText{
				Type: responses.ContentItemType_input_text,
				Text: prompt,
			},
		},
	})
	return content, nil
}

func buildVisionImageItem(raw string, detail *responses.ContentItemImageDetail_Enum) (*responses.ContentItem, error) {
	if raw == "" {
		return nil, nil
	}

	if strings.HasPrefix(raw, "data:") {
		if _, err := extractBase64Payload(raw); err != nil {
			return nil, fmt.Errorf("invalid data URI: %w", err)
		}
		return &responses.ContentItem{
			Union: &responses.ContentItem_Image{
				Image: &responses.ContentItemImage{
					Type:     responses.ContentItemType_input_image,
					Detail:   detail,
					ImageUrl: volcengine.String(raw),
				},
			},
		}, nil
	}

	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return &responses.ContentItem{
			Union: &responses.ContentItem_Image{
				Image: &responses.ContentItemImage{
					Type:   responses.ContentItemType_input_image,
					Detail: detail,
					ImageUrl: func() *string {
						return volcengine.String(raw)
					}(),
				},
			},
		}, nil
	}

	return nil, fmt.Errorf("image value must be an HTTPS URL or data URI")
}

func collectVisionText(resp *responses.ResponseObject) string {
	if resp == nil {
		return ""
	}
	var parts []string
	for _, item := range resp.GetOutput() {
		msg := item.GetOutputMessage()
		if msg == nil {
			continue
		}
		for _, content := range msg.GetContent() {
			if text := content.GetText(); text != nil && strings.TrimSpace(text.GetText()) != "" {
				parts = append(parts, text.GetText())
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

type seedreamVisionImageSourceEntry struct {
	name   string
	source string
}

func resolveSeedreamVisionImages(ctx context.Context, rawImages []string) ([]string, string, error) {
	resolved := make([]string, 0, len(rawImages))
	seen := make(map[string]bool)
	var sources []seedreamVisionImageSourceEntry

	for _, raw := range rawImages {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if isSeedreamVisionImageReference(trimmed) {
			resolved = append(resolved, trimmed)
			continue
		}

		if name, ok := extractPlaceholderIdentifier(trimmed); ok {
			value, canonical, source, ok := resolveSeedreamAttachmentByName(ctx, name)
			if !ok {
				return nil, "", fmt.Errorf("image placeholder [%s] could not be resolved via attachment context", name)
			}
			resolved = append(resolved, value)
			sources = appendSeedreamVisionSourceEntry(sources, seen, canonical, source)
			continue
		}

		value, canonical, source, ok := resolveSeedreamAttachmentByName(ctx, trimmed)
		if ok {
			resolved = append(resolved, value)
			sources = appendSeedreamVisionSourceEntry(sources, seen, canonical, source)
			continue
		}

		return nil, "", fmt.Errorf("image value %q must be an HTTPS URL, data URI, or attachment placeholder", trimmed)
	}

	return resolved, buildSeedreamVisionImageSourceNote(sources), nil
}

func isSeedreamVisionImageReference(value string) bool {
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "data:")
}

func resolveSeedreamAttachmentByName(ctx context.Context, name string) (string, string, string, bool) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", "", "", false
	}

	attachments, _ := tools.GetAttachmentContext(ctx)
	if len(attachments) == 0 {
		return "", "", "", false
	}

	if att, exists := attachments[trimmed]; exists {
		if resolved := attachmentReferenceValueForTool(att); resolved != "" {
			return resolved, trimmed, strings.TrimSpace(att.Source), true
		}
	}

	lowerName := strings.ToLower(trimmed)
	for key, att := range attachments {
		if strings.ToLower(strings.TrimSpace(key)) != lowerName {
			continue
		}
		if resolved := attachmentReferenceValueForTool(att); resolved != "" {
			canonical := strings.TrimSpace(key)
			if canonical == "" {
				canonical = strings.TrimSpace(att.Name)
			}
			return resolved, canonical, strings.TrimSpace(att.Source), true
		}
	}

	return "", "", "", false
}

func appendSeedreamVisionSourceEntry(entries []seedreamVisionImageSourceEntry, seen map[string]bool, name, source string) []seedreamVisionImageSourceEntry {
	trimmedName := strings.TrimSpace(name)
	trimmedSource := strings.TrimSpace(source)
	if trimmedName == "" || trimmedSource == "" {
		return entries
	}
	if seen[trimmedName] {
		return entries
	}
	seen[trimmedName] = true
	return append(entries, seedreamVisionImageSourceEntry{name: trimmedName, source: trimmedSource})
}

func buildSeedreamVisionImageSourceNote(entries []seedreamVisionImageSourceEntry) string {
	if len(entries) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("Image sources:\n")
	for i, entry := range entries {
		builder.WriteString(fmt.Sprintf("- %s: %s", entry.name, entry.source))
		if i < len(entries)-1 {
			builder.WriteString("\n")
		}
	}
	return builder.String()
}
