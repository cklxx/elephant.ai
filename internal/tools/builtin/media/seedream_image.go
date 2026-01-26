package media

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/jsonx"
	"alex/internal/logging"

	arkm "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
)

type seedreamImageTool struct {
	config  SeedreamConfig
	factory *seedreamClientFactory
	logger  logging.Logger
}

// NewSeedreamImageToImage returns a tool that performs image-to-image refinement.
func NewSeedreamImageToImage(config SeedreamConfig) tools.ToolExecutor {
	return &seedreamImageTool{
		config:  config,
		factory: &seedreamClientFactory{config: config},
		logger:  logging.NewComponentLogger("SeedreamImageToImage"),
	}
}

func (t *seedreamImageTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "image_to_image",
		Version:  "2.0.0",
		Category: "design",
		Tags:     []string{"image", "generation", "seedream", "image-to-image"},
		MaterialCapabilities: ports.ToolMaterialCapabilities{
			Consumes: []string{"image/png", "image/jpeg", "image/webp", "image/gif"},
			Produces: []string{"image/png", "image/jpeg", "image/webp", "image/gif"},
		},
	}
}

func (t *seedreamImageTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "image_to_image",
		Description: "Transform or upscale reference art with Seedream image-to-image models. Provide a base64 string, HTTPS URL, or previously generated `[placeholder.png]` in `init_image` along with an optional prompt. The runtime automatically resolves placeholders into the required data URI.",
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
				"size": {
					Type:        "string",
					Description: "Output WxH string (e.g. 1920x1920). Defaults to 1920x1920.",
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
			},
			Required: []string{"init_image"},
		},
		MaterialCapabilities: ports.ToolMaterialCapabilities{
			Consumes: []string{"image/png", "image/jpeg", "image/webp", "image/gif"},
			Produces: []string{"image/png", "image/jpeg", "image/webp", "image/gif"},
		},
	}
}

func (t *seedreamImageTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	if msg := seedreamMissingConfigMessage(t.config); msg != "" {
		return &ports.ToolResult{CallID: call.ID, Content: msg}, nil
	}

	rawImageValue, _ := call.Arguments["init_image"].(string)
	imageValue := strings.TrimSpace(rawImageValue)
	if resolved, placeholder, ok := resolveSeedreamInitImagePlaceholder(ctx, imageValue); ok {
		logging.OrNop(t.logger).Debug("Resolved init_image placeholder [%s] via attachment context", placeholder)
		imageValue = resolved
	} else if name, isPlaceholder := extractPlaceholderIdentifier(imageValue); isPlaceholder {
		err := fmt.Errorf("init_image placeholder [%s] could not be resolved via attachment context", name)
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	normalizedImage, kind, err := normalizeSeedreamInitImage(imageValue)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	prompt, _ := call.Arguments["prompt"].(string)
	requestID := strings.TrimSpace(call.ID)

	req := arkm.GenerateImagesRequest{
		Model:          t.config.Model,
		Prompt:         strings.TrimSpace(prompt),
		Image:          normalizedImage,
		ResponseFormat: volcengine.String(arkm.GenerateImagesResponseFormatBase64),
		Watermark:      volcengine.Bool(false),
	}
	applyImageRequestOptions(&req, call.Arguments)
	logSeedreamRequestPayload(requestID, req)
	t.logRequestPayload(imageValue, normalizedImage, kind, req)

	client, err := t.factory.instance()
	if err != nil {
		wrapped := fmt.Errorf("seedream client init: %w", err)
		return &ports.ToolResult{CallID: call.ID, Content: wrapped.Error(), Error: wrapped}, nil
	}

	resp, err := client.GenerateImages(ctx, req)
	if err != nil {
		logSeedreamResponsePayload(requestID, map[string]any{"error": err.Error()})
		return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("Seedream request failed: %v", err), Error: err}, nil
	}
	logSeedreamResponsePayload(requestID, resp)
	if resp.Error != nil {
		apiErr := fmt.Sprintf("Seedream API error (%s): %s", resp.Error.Code, resp.Error.Message)
		return &ports.ToolResult{CallID: call.ID, Content: apiErr, Error: errors.New(apiErr)}, nil
	}

	content, metadata, attachments := formatSeedreamResponseWithContext(ctx, &resp, t.config.ModelDescriptor, prompt)
	return &ports.ToolResult{CallID: call.ID, Content: content, Metadata: metadata, Attachments: attachments}, nil
}

func (t *seedreamImageTool) logRequestPayload(rawImage, normalizedImage, kind string, req arkm.GenerateImagesRequest) {
	if logging.IsNil(t.logger) {
		return
	}
	payload := map[string]any{
		"model":               strings.TrimSpace(req.Model),
		"prompt":              strings.TrimSpace(req.Prompt),
		"init_image_raw":      summarizeSeedreamImageValue(rawImage),
		"init_image_kind":     kind,
		"init_image_resolved": summarizeSeedreamImageValue(normalizedImage),
	}

	if req.Size != nil && strings.TrimSpace(*req.Size) != "" {
		payload["size"] = strings.TrimSpace(*req.Size)
	}
	if req.ResponseFormat != nil && strings.TrimSpace(*req.ResponseFormat) != "" {
		payload["response_format"] = strings.TrimSpace(*req.ResponseFormat)
	}
	if req.Seed != nil {
		payload["seed"] = *req.Seed
	}
	if req.GuidanceScale != nil {
		payload["cfg_scale"] = *req.GuidanceScale
	}
	if req.Watermark != nil {
		payload["watermark"] = *req.Watermark
	}
	if req.OptimizePrompt != nil {
		payload["optimize_prompt"] = *req.OptimizePrompt
	}

	encoded, err := jsonx.Marshal(payload)
	if err != nil {
		t.logger.Debug("Seedream image-to-image request payload: %+v", payload)
		return
	}
	t.logger.Debug("Seedream image-to-image request payload: %s", string(encoded))
}
