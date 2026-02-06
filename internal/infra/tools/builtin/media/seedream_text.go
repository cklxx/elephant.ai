package media

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/tools/builtin/shared"

	arkm "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
)

type seedreamTextTool struct {
	shared.BaseTool
	config  SeedreamConfig
	factory *seedreamClientFactory
}

// NewSeedreamTextToImage returns a tool that generates imagery from prompts.
func NewSeedreamTextToImage(config SeedreamConfig) tools.ToolExecutor {
	return &seedreamTextTool{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "text_to_image",
				Description: "Generate new imagery with Volcano Engine Seedream text-to-image models.",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"prompt": {
							Type:        "string",
							Description: "Creative brief describing what to render.",
						},
						"size": {
							Type:        "string",
							Description: "Optional WxH string (e.g. 1920x1920). Defaults to 1920x1920.",
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
						"optimize_prompt": {
							Type:        "boolean",
							Description: "Let Seedream refine the prompt automatically.",
						},
					},
					Required: []string{"prompt"},
				},
				MaterialCapabilities: ports.ToolMaterialCapabilities{
					Produces: []string{"image/png", "image/jpeg", "image/webp", "image/gif"},
				},
			},
			ports.ToolMetadata{
				Name:     "text_to_image",
				Version:  "2.0.0",
				Category: "design",
				Tags:     []string{"image", "generation", "seedream", "text-to-image"},
				MaterialCapabilities: ports.ToolMaterialCapabilities{
					Produces: []string{"image/png", "image/jpeg", "image/webp", "image/gif"},
				},
			},
		),
		config:  config,
		factory: &seedreamClientFactory{config: config},
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

	requestID := strings.TrimSpace(call.ID)

	req := arkm.GenerateImagesRequest{
		Model:          t.config.Model,
		Prompt:         prompt,
		ResponseFormat: volcengine.String(arkm.GenerateImagesResponseFormatBase64),
		Watermark:      volcengine.Bool(false),
	}
	applyImageRequestOptions(&req, call.Arguments)

	client, err := t.factory.instance()
	if err != nil {
		wrapped := fmt.Errorf("seedream client init: %w", err)
		return &ports.ToolResult{CallID: call.ID, Content: wrapped.Error(), Error: wrapped}, nil
	}

	logSeedreamRequestPayload(requestID, req)

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
