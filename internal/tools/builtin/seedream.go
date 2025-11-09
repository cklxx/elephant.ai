package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"alex/internal/agent/ports"

	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	arkm "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
)

// SeedreamConfig captures common configuration for the Seedream tools.
type SeedreamConfig struct {
	APIKey          string
	Model           string
	ModelDescriptor string
	ModelEnvVar     string
}

type seedreamClientFactory struct {
	config SeedreamConfig
	once   sync.Once
	client *arkruntime.Client
	err    error
}

func (f *seedreamClientFactory) instance() (*arkruntime.Client, error) {
	f.once.Do(func() {
		apiKey := strings.TrimSpace(f.config.APIKey)
		if apiKey == "" {
			f.err = errors.New("seedream API key missing")
			return
		}
		f.client = arkruntime.NewClientWithApiKey(apiKey)
	})
	if f.err != nil {
		return nil, f.err
	}
	return f.client, nil
}

type seedreamTextTool struct {
	config  SeedreamConfig
	factory *seedreamClientFactory
}

type seedreamImageTool struct {
	config  SeedreamConfig
	factory *seedreamClientFactory
}

type seedreamVisionTool struct {
	config  SeedreamConfig
	factory *seedreamClientFactory
}

// NewSeedreamTextToImage returns a tool that generates imagery from prompts.
func NewSeedreamTextToImage(config SeedreamConfig) ports.ToolExecutor {
	return &seedreamTextTool{
		config:  config,
		factory: &seedreamClientFactory{config: config},
	}
}

// NewSeedreamImageToImage returns a tool that performs image-to-image refinement.
func NewSeedreamImageToImage(config SeedreamConfig) ports.ToolExecutor {
	return &seedreamImageTool{
		config:  config,
		factory: &seedreamClientFactory{config: config},
	}
}

// NewSeedreamVisionAnalyze returns a tool that analyzes images with the vision model.
func NewSeedreamVisionAnalyze(config SeedreamConfig) ports.ToolExecutor {
	return &seedreamVisionTool{
		config:  config,
		factory: &seedreamClientFactory{config: config},
	}
}

func (t *seedreamTextTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "seedream_text_to_image",
		Version:  "2.0.0",
		Category: "design",
		Tags:     []string{"image", "generation", "seedream", "text-to-image"},
	}
}

func (t *seedreamTextTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "seedream_text_to_image",
		Description: "Generate new imagery with Volcano Engine Seedream text-to-image models.",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"prompt": {
					Type:        "string",
					Description: "Creative brief describing what to render.",
				},
				"response_format": {
					Type:        "string",
					Description: "Set to `url` (default) or `b64_json`.",
				},
				"size": {
					Type:        "string",
					Description: "Optional WxH string (e.g. 1024x1024).",
				},
				"width": {
					Type:        "integer",
					Description: "Alternative way to set output width in pixels.",
				},
				"height": {
					Type:        "integer",
					Description: "Alternative way to set output height in pixels.",
				},
				"seed": {
					Type:        "integer",
					Description: "Random seed for reproducible generations.",
				},
				"cfg_scale": {
					Type:        "number",
					Description: "Classifier-free guidance / prompt strength.",
				},
				"watermark": {
					Type:        "boolean",
					Description: "Whether to embed the Seedream watermark (default true).",
				},
				"optimize_prompt": {
					Type:        "boolean",
					Description: "Let Seedream refine the prompt automatically.",
				},
			},
			Required: []string{"prompt"},
		},
	}
}

func (t *seedreamTextTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	if msg := seedreamMissingConfigMessage(t.config); msg != "" {
		return &ports.ToolResult{CallID: call.ID, Content: msg}, nil
	}

	prompt, _ := call.Arguments["prompt"].(string)
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		err := errors.New("prompt parameter required")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	req := arkm.GenerateImagesRequest{
		Model:          t.config.Model,
		Prompt:         prompt,
		ResponseFormat: volcengine.String(arkm.GenerateImagesResponseFormatBase64),
		Watermark:      volcengine.Bool(true),
	}
	applyImageRequestOptions(&req, call.Arguments)

	client, err := t.factory.instance()
	if err != nil {
		wrapped := fmt.Errorf("seedream client init: %w", err)
		return &ports.ToolResult{CallID: call.ID, Content: wrapped.Error(), Error: wrapped}, nil
	}

	resp, err := client.GenerateImages(ctx, req)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("Seedream request failed: %v", err), Error: err}, nil
	}
	if resp.Error != nil {
		apiErr := fmt.Sprintf("Seedream API error (%s): %s", resp.Error.Code, resp.Error.Message)
		return &ports.ToolResult{CallID: call.ID, Content: apiErr, Error: errors.New(apiErr)}, nil
	}

	content, metadata, attachments := formatSeedreamResponse(&resp, t.config.ModelDescriptor, prompt)
	return &ports.ToolResult{CallID: call.ID, Content: content, Metadata: metadata, Attachments: attachments}, nil
}

func (t *seedreamImageTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "seedream_image_to_image",
		Version:  "2.0.0",
		Category: "design",
		Tags:     []string{"image", "generation", "seedream", "image-to-image"},
	}
}

func (t *seedreamImageTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "seedream_image_to_image",
		Description: "Transform or upscale reference art with Seedream image-to-image models. Provide a base64 string or URL in `init_image` along with an optional prompt.",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"init_image": {
					Type:        "string",
					Description: "Base64 data URI or URL of the source image.",
				},
				"prompt": {
					Type:        "string",
					Description: "Optional guidance describing the target adjustments.",
				},
				"response_format": {
					Type:        "string",
					Description: "Set to `url` (default) or `b64_json`.",
				},
				"size": {
					Type:        "string",
					Description: "Output WxH string (e.g. 1024x1024).",
				},
				"width": {
					Type: "integer",
				},
				"height": {
					Type: "integer",
				},
				"seed": {
					Type: "integer",
				},
				"cfg_scale": {
					Type: "number",
				},
				"watermark": {
					Type:        "boolean",
					Description: "Whether to embed the Seedream watermark (default true).",
				},
			},
			Required: []string{"init_image"},
		},
	}
}

func (t *seedreamImageTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	if msg := seedreamMissingConfigMessage(t.config); msg != "" {
		return &ports.ToolResult{CallID: call.ID, Content: msg}, nil
	}

	imageValue, _ := call.Arguments["init_image"].(string)
	normalizedImage, err := normalizeSeedreamInitImage(imageValue)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	prompt, _ := call.Arguments["prompt"].(string)

	req := arkm.GenerateImagesRequest{
		Model:          t.config.Model,
		Prompt:         strings.TrimSpace(prompt),
		Image:          normalizedImage,
		ResponseFormat: volcengine.String(arkm.GenerateImagesResponseFormatBase64),
		Watermark:      volcengine.Bool(true),
	}
	applyImageRequestOptions(&req, call.Arguments)

	client, err := t.factory.instance()
	if err != nil {
		wrapped := fmt.Errorf("seedream client init: %w", err)
		return &ports.ToolResult{CallID: call.ID, Content: wrapped.Error(), Error: wrapped}, nil
	}

	resp, err := client.GenerateImages(ctx, req)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("Seedream request failed: %v", err), Error: err}, nil
	}
	if resp.Error != nil {
		apiErr := fmt.Sprintf("Seedream API error (%s): %s", resp.Error.Code, resp.Error.Message)
		return &ports.ToolResult{CallID: call.ID, Content: apiErr, Error: errors.New(apiErr)}, nil
	}

	content, metadata, attachments := formatSeedreamResponse(&resp, t.config.ModelDescriptor, prompt)
	return &ports.ToolResult{CallID: call.ID, Content: content, Metadata: metadata, Attachments: attachments}, nil
}

func (t *seedreamVisionTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "seedream_vision_analyze",
		Version:  "1.0.0",
		Category: "analysis",
		Tags:     []string{"vision", "analysis", "seedream", "multimodal"},
	}
}

func (t *seedreamVisionTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "seedream_vision_analyze",
		Description: "Use the Doubao multimodal vision model to describe or answer questions about supplied image URLs.",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"images": {
					Type:        "array",
					Description: "List of image URLs or data URIs to analyze.",
				},
				"prompt": {
					Type:        "string",
					Description: "Question or instruction for the vision model.",
				},
				"detail": {
					Type:        "string",
					Description: "Optional detail level: auto (default), low, or high.",
				},
			},
			Required: []string{"images"},
		},
	}
}

func (t *seedreamVisionTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	if msg := seedreamMissingConfigMessage(t.config); msg != "" {
		return &ports.ToolResult{CallID: call.ID, Content: msg}, nil
	}

	images := readStringSlice(call.Arguments["images"])
	if len(images) == 0 {
		err := errors.New("images parameter must include at least one URL or data URI")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	prompt, _ := call.Arguments["prompt"].(string)
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		prompt = "Describe the images in detail."
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

	resp, err := client.CreateResponses(ctx, req)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("Seedream vision request failed: %v", err), Error: err}, nil
	}
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

func applyImageRequestOptions(req *arkm.GenerateImagesRequest, args map[string]any) {
	if format, ok := args["response_format"].(string); ok {
		switch strings.ToLower(strings.TrimSpace(format)) {
		case "b64_json":
			req.ResponseFormat = volcengine.String(arkm.GenerateImagesResponseFormatBase64)
		case "url":
			req.ResponseFormat = volcengine.String(arkm.GenerateImagesResponseFormatURL)
		}
	}
	if size, ok := args["size"].(string); ok && strings.TrimSpace(size) != "" {
		req.Size = volcengine.String(strings.TrimSpace(size))
	} else if width, ok := readInt(args, "width"); ok {
		if height, okh := readInt(args, "height"); okh && width > 0 && height > 0 {
			req.Size = volcengine.String(fmt.Sprintf("%dx%d", width, height))
		}
	}
	if seed, ok := readInt(args, "seed"); ok {
		req.Seed = volcengine.Int64(int64(seed))
	}
	if cfgScale, ok := readFloat(args, "cfg_scale"); ok {
		req.GuidanceScale = volcengine.Float64(cfgScale)
	}
	if watermark, ok := args["watermark"].(bool); ok {
		req.Watermark = volcengine.Bool(watermark)
	}
	if optimize, ok := args["optimize_prompt"].(bool); ok {
		req.OptimizePrompt = volcengine.Bool(optimize)
	}
}

func seedreamMissingConfigMessage(config SeedreamConfig) string {
	missing := []string{}
	if strings.TrimSpace(config.APIKey) == "" {
		missing = append(missing, "ARK_API_KEY")
	}
	if strings.TrimSpace(config.Model) == "" {
		label := "Seedream model identifier"
		if config.ModelEnvVar != "" {
			label = strings.ToUpper(config.ModelEnvVar)
		}
		missing = append(missing, label)
	}
	if len(missing) == 0 {
		return ""
	}

	toolName := config.ModelDescriptor
	if toolName == "" {
		toolName = "Seedream"
	}

	builder := &strings.Builder{}
	fmt.Fprintf(builder, "%s is not configured. Missing values: %s.\n\n", toolName, strings.Join(missing, ", "))
	builder.WriteString("Provide the following settings via environment variables or ~/.alex-config.json:\n\n")
	builder.WriteString("- ARK_API_KEY from the Volcano Engine Ark console\n")
	if config.ModelEnvVar != "" {
		fmt.Fprintf(builder, "- %s to select the desired Seedream model\n", strings.ToUpper(config.ModelEnvVar))
	} else {
		builder.WriteString("- Seedream model identifier (e.g. doubao-seedream-3-0-t2i-250415)\n")
	}
	return builder.String()
}

func formatSeedreamResponse(resp *arkm.ImagesResponse, descriptor, prompt string) (string, map[string]any, map[string]ports.Attachment) {
	if resp == nil {
		return "Seedream returned an empty response.", nil, nil
	}

	images := make([]map[string]any, 0, len(resp.Data))
	attachments := make(map[string]ports.Attachment)

	requestID := fmt.Sprintf("seedream_%d", time.Now().Unix())
	if resp.Model != "" {
		requestID = strings.ReplaceAll(resp.Model, "/", "_")
	}

	trimmedPrompt := strings.TrimSpace(prompt)
	attachmentDescription := trimmedPrompt
	if attachmentDescription == "" {
		attachmentDescription = strings.TrimSpace(descriptor)
	}

	for idx, item := range resp.Data {
		if item == nil {
			continue
		}
		entry := map[string]any{"index": idx}
		var urlStr string
		if item.Url != nil && *item.Url != "" {
			urlStr = *item.Url
			entry["url"] = urlStr
		}
		var encoded string
		if item.B64Json != nil && *item.B64Json != "" {
			encoded = *item.B64Json
		}
		placeholder := fmt.Sprintf("%s_%d.png", requestID, idx)
		entry["placeholder"] = placeholder
		attachments[placeholder] = ports.Attachment{
			Name:        placeholder,
			MediaType:   "image/png",
			Data:        encoded,
			URI:         urlStr,
			Source:      "seedream",
			Description: attachmentDescription,
		}
		images = append(images, entry)
	}

	metadata := map[string]any{
		"model":   resp.Model,
		"created": resp.Created,
		"images":  images,
	}
	if resp.Usage != nil {
		metadata["usage"] = resp.Usage
	}
	if descriptor != "" {
		metadata["model_descriptor"] = descriptor
	}
	if trimmedPrompt != "" {
		metadata["prompt"] = trimmedPrompt
		metadata["description"] = trimmedPrompt
	} else if descriptor != "" {
		metadata["description"] = descriptor
	}

	var builder strings.Builder
	title := strings.TrimSpace(descriptor)
	if title == "" {
		title = "Seedream"
	}
	if title != "" {
		fmt.Fprintf(&builder, "%s response\n", title)
	}
	if trimmedPrompt != "" {
		fmt.Fprintf(&builder, "Prompt:\n%s\n\n", trimmedPrompt)
	}
	if len(images) > 0 {
		fmt.Fprintf(&builder, "Generated %d image(s). Use these placeholders for follow-up steps:\n", len(images))
		for idx, img := range images {
			placeholder, _ := img["placeholder"].(string)
			url, _ := img["url"].(string)
			fmt.Fprintf(&builder, "%d. [%s]", idx+1, placeholder)
			if url != "" {
				fmt.Fprintf(&builder, " (url: %s)", url)
			}
			builder.WriteString("\n")
		}
	}
	content := strings.TrimSpace(builder.String())
	if content == "" {
		return "Seedream image generation complete.", metadata, attachments
	}
	return content, metadata, attachments
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

func normalizeSeedreamInitImage(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("init_image parameter must be provided (base64 or URL)")
	}

	if strings.HasPrefix(trimmed, "data:") {
		payload, err := extractBase64Payload(trimmed)
		if err != nil {
			return "", fmt.Errorf("invalid init_image data URI: %w", err)
		}
		return payload, nil
	}

	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return trimmed, nil
	}

	if strings.Contains(trimmed, "://") {
		return "", fmt.Errorf("init_image must be an HTTPS URL or data URI")
	}

	return trimmed, nil
}

func extractBase64Payload(dataURI string) (string, error) {
	comma := strings.Index(dataURI, ",")
	if !strings.HasPrefix(dataURI, "data:") || comma == -1 {
		return "", errors.New("invalid data URI format")
	}
	payload := dataURI[comma+1:]
	if payload == "" {
		return "", errors.New("missing data payload")
	}
	return payload, nil
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

func readInt(args map[string]any, key string) (int, bool) {
	value, ok := args[key]
	if !ok {
		return 0, false
	}
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return int(i), true
		}
	}
	return 0, false
}

func readFloat(args map[string]any, key string) (float64, bool) {
	value, ok := args[key]
	if !ok {
		return 0, false
	}
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return f, true
		}
	}
	return 0, false
}

func readStringSlice(value any) []string {
	switch v := value.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				out = append(out, str)
			}
		}
		return out
	default:
		return nil
	}
}
