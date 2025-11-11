package builtin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"alex/internal/agent/ports"
	"alex/internal/utils"

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
	logger  *utils.Logger
}

type seedreamVisionTool struct {
	config  SeedreamConfig
	factory *seedreamClientFactory
}

type seedreamVideoTool struct {
	config  SeedreamConfig
	factory *seedreamClientFactory
}

const (
	// doubao-seedance-1.0-pro documentation: https://www.volcengine.com/docs/82379/1587798
	seedanceMinDurationSeconds = 2
	seedanceMaxDurationSeconds = 12
)

var seedreamPlaceholderNonce = func() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
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
		logger:  utils.NewComponentLogger("SeedreamImageToImage"),
	}
}

// NewSeedreamVisionAnalyze returns a tool that analyzes images with the vision model.
func NewSeedreamVisionAnalyze(config SeedreamConfig) ports.ToolExecutor {
	return &seedreamVisionTool{
		config:  config,
		factory: &seedreamClientFactory{config: config},
	}
}

// NewSeedreamVideoGenerate returns a tool that creates short videos from prompts.
func NewSeedreamVideoGenerate(config SeedreamConfig) ports.ToolExecutor {
	return &seedreamVideoTool{
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
	normalizedImage, kind, err := normalizeSeedreamInitImage(imageValue)
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
	t.logRequestPayload(imageValue, normalizedImage, kind, req)

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

func (t *seedreamImageTool) logRequestPayload(rawImage, normalizedImage, kind string, req arkm.GenerateImagesRequest) {
	if t.logger == nil {
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

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.logger.Debug("Seedream image-to-image request payload: %+v", payload)
		return
	}
	t.logger.Debug("Seedream image-to-image request payload: %s", string(encoded))
}

func (t *seedreamVisionTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "seedream_vision_analyze",
		Version:  "1.0.0",
		Category: "analysis",
		Tags:     []string{"vision", "analysis", "seedream", "multimodal"},
	}
}

func (t *seedreamVideoTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "seedream_video_generate",
		Version:  "1.0.0",
		Category: "design",
		Tags:     []string{"video", "generation", "seedream", "seedance"},
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

func (t *seedreamVideoTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name: "seedream_video_generate",
		Description: fmt.Sprintf(
			"Create short cinematic videos with the Doubao Seedance model. The tool will plan for an establishing first frame, honor the requested duration, and enforces the official %d-%d second clip window.",
			seedanceMinDurationSeconds,
			seedanceMaxDurationSeconds,
		),
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"prompt": {
					Type:        "string",
					Description: "High-level creative direction for the video sequence.",
				},
				"duration_seconds": {
					Type:        "integer",
					Description: fmt.Sprintf("Target length of the generated video in seconds (%d-%d per official Seedance docs).", seedanceMinDurationSeconds, seedanceMaxDurationSeconds),
				},
				"first_frame_prompt": {
					Type:        "string",
					Description: "Optional detail describing the opening frame composition.",
				},
				"first_frame_url": {
					Type:        "string",
					Description: "Optional HTTPS or data URL to reuse as the opening frame.",
				},
				"first_frame_base64": {
					Type:        "string",
					Description: "Optional base64-encoded image (with or without data URL header) for the opening frame.",
				},
				"first_frame_mime_type": {
					Type:        "string",
					Description: "When providing base64 data, override the inferred MIME type (e.g. image/png).",
				},
				"resolution": {
					Type:        "string",
					Description: "Output resolution such as 1080p or 720p.",
				},
				"camera_fixed": {
					Type:        "boolean",
					Description: "Whether to keep the camera fixed for the entire shot (default false).",
				},
				"watermark": {
					Type:        "boolean",
					Description: "Embed the Seedance watermark (default true).",
				},
				"seed": {
					Type:        "integer",
					Description: "Optional deterministic seed for repeatable generations.",
				},
				"return_last_frame": {
					Type:        "boolean",
					Description: "Request the service to return the last frame thumbnail (default true).",
				},
				"poll_interval_seconds": {
					Type:        "integer",
					Description: "How frequently to poll for task completion (default 3).",
				},
				"max_wait_seconds": {
					Type:        "integer",
					Description: "Maximum time to wait for the task before timing out (default 300).",
				},
			},
			Required: []string{"prompt", "duration_seconds"},
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

func (t *seedreamVideoTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	if msg := seedreamMissingConfigMessage(t.config); msg != "" {
		return &ports.ToolResult{CallID: call.ID, Content: msg}, nil
	}

	prompt, _ := call.Arguments["prompt"].(string)
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		err := errors.New("prompt parameter required")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	duration, ok := readInt(call.Arguments, "duration_seconds")
	if !ok {
		err := errors.New("duration_seconds must be a positive integer")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	if duration < seedanceMinDurationSeconds || duration > seedanceMaxDurationSeconds {
		err := fmt.Errorf("duration_seconds must be between %d and %d seconds per Seedance documentation", seedanceMinDurationSeconds, seedanceMaxDurationSeconds)
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	firstFramePrompt, _ := call.Arguments["first_frame_prompt"].(string)
	firstFramePrompt = strings.TrimSpace(firstFramePrompt)
	if firstFramePrompt == "" {
		firstFramePrompt = prompt
	}

	firstFrameURLRaw := strings.TrimSpace(stringFromArgs(call.Arguments, "first_frame_url"))
	firstFrameBase64 := strings.TrimSpace(stringFromArgs(call.Arguments, "first_frame_base64"))
	firstFrameMimeType := strings.TrimSpace(stringFromArgs(call.Arguments, "first_frame_mime_type"))

	firstFrameURL, firstFrameKind, firstFrameMimeTypeResolved, err := coalesceSeedreamFirstFrameSource(firstFrameURLRaw, firstFrameBase64, firstFrameMimeType)
	if err != nil {
		wrapped := fmt.Errorf("first frame input invalid: %w", err)
		return &ports.ToolResult{CallID: call.ID, Content: wrapped.Error(), Error: wrapped}, nil
	}

	resolution := strings.TrimSpace(stringFromArgs(call.Arguments, "resolution"))
	if resolution == "" {
		resolution = "1080p"
	}

	cameraFixed := readBoolWithDefault(call.Arguments, "camera_fixed", false)
	watermark := readBoolWithDefault(call.Arguments, "watermark", true)
	returnLastFrame := readBoolWithDefault(call.Arguments, "return_last_frame", true)

	seed, seedProvided := readInt(call.Arguments, "seed")

	pollInterval := readDurationWithDefault(call.Arguments, "poll_interval_seconds", 3)
	maxWait := readDurationWithDefault(call.Arguments, "max_wait_seconds", 300)
	if maxWait <= 0 {
		maxWait = 300
	}

	payload := buildSeedreamVideoPrompt(prompt, firstFramePrompt, duration, resolution, cameraFixed, watermark, seed, seedProvided)

	request := arkm.CreateContentGenerationTaskRequest{
		Model: t.config.Model,
		Content: []*arkm.CreateContentGenerationContentItem{
			{
				Type: arkm.ContentGenerationContentItemTypeText,
				Text: volcengine.String(payload),
			},
		},
	}
	if firstFrameURL != "" {
		request.Content = append(request.Content, &arkm.CreateContentGenerationContentItem{
			Type: arkm.ContentGenerationContentItemTypeImage,
			ImageURL: &arkm.ImageURL{
				URL: firstFrameURL,
			},
		})
	}
	if returnLastFrame {
		request.ReturnLastFrame = volcengine.Bool(true)
	}

	client, err := t.factory.instance()
	if err != nil {
		wrapped := fmt.Errorf("seedream client init: %w", err)
		return &ports.ToolResult{CallID: call.ID, Content: wrapped.Error(), Error: wrapped}, nil
	}

	createResp, err := client.CreateContentGenerationTask(ctx, request)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("Seedance request failed: %v", err), Error: err}, nil
	}

	taskID := strings.TrimSpace(createResp.ID)
	if taskID == "" {
		err = errors.New("Seedance did not return a task identifier")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	deadline := time.Now().Add(maxWait)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			err := fmt.Errorf("Seedance polling cancelled: %w", ctx.Err())
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		default:
		}

		resp, err := client.GetContentGenerationTask(ctx, arkm.GetContentGenerationTaskRequest{ID: taskID})
		if err != nil {
			return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("Seedance polling failed: %v", err), Error: err}, nil
		}

		switch resp.Status {
		case arkm.StatusSucceeded:
			content, metadata, attachments := formatSeedreamVideoResponse(
				&resp,
				t.config.ModelDescriptor,
				prompt,
				duration,
				resolution,
				firstFramePrompt,
				firstFrameURL,
				firstFrameKind,
				firstFrameMimeTypeResolved,
			)
			metadata["task_id"] = taskID
			metadata["poll_interval_seconds"] = int(pollInterval / time.Second)
			metadata["max_wait_seconds"] = int(maxWait / time.Second)
			metadata["stitching_planned"] = true
			return &ports.ToolResult{CallID: call.ID, Content: content, Metadata: metadata, Attachments: attachments}, nil
		case arkm.StatusFailed:
			msg := "Seedance task failed"
			if resp.Error != nil {
				msg = fmt.Sprintf("Seedance task failed (%s): %s", resp.Error.Code, resp.Error.Message)
			}
			err := errors.New(msg)
			return &ports.ToolResult{CallID: call.ID, Content: msg, Error: err}, nil
		case arkm.StatusCancelled:
			err := errors.New("Seedance task cancelled")
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}

		if time.Now().After(deadline) {
			err := fmt.Errorf("Seedance task %s did not complete within %d seconds", taskID, int(maxWait/time.Second))
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}

		select {
		case <-ctx.Done():
			err := fmt.Errorf("Seedance polling cancelled: %w", ctx.Err())
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		case <-ticker.C:
		}
	}
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

	requestID := seedreamAttachmentPrefix(resp.Model, resp.Created)

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

func normalizeSeedreamInitImage(raw string) (string, string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", "", errors.New("init_image parameter must be provided (base64 or URL)")
	}

	if strings.HasPrefix(trimmed, "data:") {
		if _, err := extractBase64Payload(trimmed); err != nil {
			return "", "", fmt.Errorf("invalid init_image data URI: %w", err)
		}
		return trimmed, classifySeedreamInitImageKind(trimmed), nil
	}

	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return trimmed, classifySeedreamInitImageKind(trimmed), nil
	}

	if strings.Contains(trimmed, "://") {
		return "", "", fmt.Errorf("init_image must be an HTTPS URL or data URI")
	}

	// Assume bare base64 PNG blobs and wrap in a generic data URI.
	encoded := fmt.Sprintf("data:image/png;base64,%s", trimmed)
	return encoded, classifySeedreamInitImageKind(encoded), nil
}

func coalesceSeedreamFirstFrameSource(urlValue, base64Value, mimeType string) (string, string, string, error) {
	trimmedURL := strings.TrimSpace(urlValue)
	trimmedBase := strings.TrimSpace(base64Value)
	normalizedMime := normalizeSeedreamMimeType(mimeType)

	if trimmedBase == "" {
		if trimmedURL == "" {
			return "", "", "", nil
		}
		if !strings.HasPrefix(trimmedURL, "http://") && !strings.HasPrefix(trimmedURL, "https://") && !strings.HasPrefix(trimmedURL, "data:") {
			return "", "", "", errors.New("first_frame_url must be an HTTPS URL or data URI")
		}
		return trimmedURL, classifySeedreamInitImageKind(trimmedURL), normalizeSeedreamMimeType(extractSeedreamDataURIMime(trimmedURL)), nil
	}

	if strings.HasPrefix(trimmedBase, "http://") || strings.HasPrefix(trimmedBase, "https://") {
		// Allow callers to pass a URL via the base64 field for convenience.
		return trimmedBase, classifySeedreamInitImageKind(trimmedBase), normalizeSeedreamMimeType(extractSeedreamDataURIMime(trimmedBase)), nil
	}

	if strings.HasPrefix(trimmedBase, "data:") {
		payload, err := extractBase64Payload(trimmedBase)
		if err != nil {
			return "", "", "", fmt.Errorf("invalid base64 data URI: %w", err)
		}
		canonical, err := canonicalizeSeedreamBase64(payload)
		if err != nil {
			return "", "", "", fmt.Errorf("invalid base64 payload: %w", err)
		}
		if normalizedMime == "" {
			normalizedMime = normalizeSeedreamMimeType(extractSeedreamDataURIMime(trimmedBase))
		}
		if normalizedMime == "" {
			normalizedMime = "image/png"
		}
		uri := buildSeedreamDataURI(normalizedMime, canonical)
		return uri, "data_uri", normalizedMime, nil
	}

	canonical, err := canonicalizeSeedreamBase64(trimmedBase)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid base64 payload: %w", err)
	}
	if normalizedMime == "" {
		normalizedMime = "image/png"
	}
	uri := buildSeedreamDataURI(normalizedMime, canonical)
	return uri, "data_uri", normalizedMime, nil
}

func canonicalizeSeedreamBase64(raw string) (string, error) {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.ReplaceAll(cleaned, "\n", "")
	cleaned = strings.ReplaceAll(cleaned, "\r", "")
	if cleaned == "" {
		return "", errors.New("base64 data must not be empty")
	}

	decoded, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(cleaned)
		if err != nil {
			return "", err
		}
	}
	return base64.StdEncoding.EncodeToString(decoded), nil
}

func buildSeedreamDataURI(mimeType, payload string) string {
	if strings.TrimSpace(mimeType) == "" {
		mimeType = "image/png"
	}
	return fmt.Sprintf("data:%s;base64,%s", mimeType, payload)
}

func normalizeSeedreamMimeType(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	switch trimmed {
	case "", "default":
		return ""
	case ".png", "png":
		return "image/png"
	case ".jpg", "jpg", ".jpeg", "jpeg":
		return "image/jpeg"
	case ".webp", "webp":
		return "image/webp"
	case ".gif", "gif":
		return "image/gif"
	default:
		return trimmed
	}
}

func extractSeedreamDataURIMime(dataURI string) string {
	if !strings.HasPrefix(dataURI, "data:") {
		return ""
	}
	comma := strings.Index(dataURI, ",")
	if comma == -1 {
		return ""
	}
	header := dataURI[len("data:"):comma]
	if header == "" {
		return ""
	}
	parts := strings.Split(header, ";")
	if len(parts) == 0 {
		return ""
	}
	if strings.Contains(parts[0], "/") {
		return parts[0]
	}
	return ""
}

func summarizeSeedreamImageValue(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "(empty)"
	}

	const previewLen = 32

	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		prefix := trimmed
		if len(prefix) > previewLen {
			prefix = prefix[:previewLen] + "..."
		}
		return fmt.Sprintf("url(len=%d,prefix=%q)", len(trimmed), prefix)
	}

	if strings.HasPrefix(trimmed, "data:") {
		meta := trimmed
		dataIdx := strings.Index(meta, ",")
		if dataIdx != -1 {
			header := meta[:dataIdx]
			payload := meta[dataIdx+1:]
			if len(payload) > previewLen {
				payload = payload[:previewLen] + "..."
			}
			return fmt.Sprintf("data_uri(header=%q,len=%d,payload_prefix=%q)", header, len(trimmed), payload)
		}
		if len(meta) > previewLen {
			meta = meta[:previewLen] + "..."
		}
		return fmt.Sprintf("data_uri(len=%d,prefix=%q)", len(trimmed), meta)
	}

	payload := trimmed
	if len(payload) > previewLen {
		payload = payload[:previewLen] + "..."
	}
	return fmt.Sprintf("base64(len=%d,prefix=%q)", len(trimmed), payload)
}

func classifySeedreamInitImageKind(value string) string {
	switch {
	case strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://"):
		return "url"
	case strings.HasPrefix(value, "data:"):
		return "data_uri"
	default:
		return "base64"
	}
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

func stringFromArgs(args map[string]any, key string) string {
	if value, ok := args[key]; ok {
		switch v := value.(type) {
		case string:
			return v
		case json.Number:
			return v.String()
		case fmt.Stringer:
			return v.String()
		}
	}
	return ""
}

func readBoolWithDefault(args map[string]any, key string, def bool) bool {
	value, ok := args[key]
	if !ok {
		return def
	}
	switch v := value.(type) {
	case bool:
		return v
	case string:
		trimmed := strings.ToLower(strings.TrimSpace(v))
		switch trimmed {
		case "", "default":
			return def
		case "true", "1", "yes", "y", "on":
			return true
		case "false", "0", "no", "n", "off":
			return false
		}
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return i != 0
		}
	}
	return def
}

func readDurationWithDefault(args map[string]any, key string, def int) time.Duration {
	seconds, ok := readInt(args, key)
	if !ok {
		return time.Duration(def) * time.Second
	}
	if seconds <= 0 {
		return time.Duration(def) * time.Second
	}
	return time.Duration(seconds) * time.Second
}

func buildSeedreamVideoPrompt(prompt, firstFramePrompt string, duration int, resolution string, cameraFixed, watermark bool, seed int, seedProvided bool) string {
	builder := &strings.Builder{}
	trimmedPrompt := strings.TrimSpace(prompt)
	if trimmedPrompt != "" {
		builder.WriteString(trimmedPrompt)
	}

	builder.WriteString("\n\n")
	fmt.Fprintf(builder, "Plan the sequence by establishing a striking first frame: %s\n", strings.TrimSpace(firstFramePrompt))
	fmt.Fprintf(builder, "Target video length: %d seconds.\n", duration)
	builder.WriteString("Ensure motion unfolds naturally after the opening frame and maintain visual coherence throughout.\n")

	fmt.Fprintf(
		builder,
		"--resolution %s --duration %d --camerafixed %t --watermark %t",
		strings.TrimSpace(resolution),
		duration,
		cameraFixed,
		watermark,
	)
	if seedProvided {
		fmt.Fprintf(builder, " --seed %d", seed)
	}

	return strings.TrimSpace(builder.String())
}

func formatSeedreamVideoResponse(resp *arkm.GetContentGenerationTaskResponse, descriptor, prompt string, duration int, resolution, firstFramePrompt, firstFrameURL, firstFrameKind, firstFrameMime string) (string, map[string]any, map[string]ports.Attachment) {
	description := strings.TrimSpace(descriptor)
	if description == "" {
		description = strings.TrimSpace(prompt)
	}

	metadata := map[string]any{
		"model":              resp.Model,
		"status":             resp.Status,
		"created_at":         resp.CreatedAt,
		"updated_at":         resp.UpdatedAt,
		"prompt":             strings.TrimSpace(prompt),
		"first_frame_prompt": strings.TrimSpace(firstFramePrompt),
		"duration_seconds":   duration,
		"resolution":         strings.TrimSpace(resolution),
		"first_frame_url":    strings.TrimSpace(firstFrameURL),
		"first_frame_kind":   strings.TrimSpace(firstFrameKind),
		"first_frame_mime":   strings.TrimSpace(firstFrameMime),
		"description":        description,
		"capabilities": map[string]any{
			"stitching": "planned",
		},
	}
	if resp.Usage.CompletionTokens != 0 {
		metadata["usage"] = resp.Usage
	}
	if resp.Seed != nil {
		metadata["seed"] = *resp.Seed
	}
	if resp.Frames != nil {
		metadata["frames"] = *resp.Frames
	}
	if resp.FramesPerSecond != nil {
		metadata["fps"] = *resp.FramesPerSecond
	}
	if resp.SubdivisionLevel != nil {
		metadata["subdivision_level"] = *resp.SubdivisionLevel
	}
	if resp.FileFormat != nil {
		metadata["file_format"] = *resp.FileFormat
	}
	if resp.RevisedPrompt != nil {
		metadata["revised_prompt"] = strings.TrimSpace(*resp.RevisedPrompt)
	}

	attachments := make(map[string]ports.Attachment)
	placeholderPrefix := seedreamAttachmentPrefix(resp.Model, resp.CreatedAt)

	if strings.TrimSpace(resp.Content.VideoURL) != "" {
		extension := "mp4"
		if resp.FileFormat != nil && strings.TrimSpace(*resp.FileFormat) != "" {
			extension = strings.TrimSpace(*resp.FileFormat)
		}
		placeholder := fmt.Sprintf("%s_video.%s", placeholderPrefix, extension)
		attachments[placeholder] = ports.Attachment{
			Name:        placeholder,
			MediaType:   inferMediaTypeFromURL(resp.Content.VideoURL, "video/mp4"),
			URI:         resp.Content.VideoURL,
			Source:      "seedream",
			Description: strings.TrimSpace(firstFramePrompt),
		}
		metadata["video_placeholder"] = placeholder
	}

	if strings.TrimSpace(resp.Content.LastFrameURL) != "" {
		placeholder := fmt.Sprintf("%s_last_frame.png", placeholderPrefix)
		attachments[placeholder] = ports.Attachment{
			Name:        placeholder,
			MediaType:   inferMediaTypeFromURL(resp.Content.LastFrameURL, "image/png"),
			URI:         resp.Content.LastFrameURL,
			Source:      "seedream",
			Description: "Last frame preview",
		}
		metadata["last_frame_placeholder"] = placeholder
	}

	if strings.TrimSpace(resp.Content.FileURL) != "" && metadata["video_placeholder"] == nil {
		placeholder := fmt.Sprintf("%s_asset.bin", placeholderPrefix)
		attachments[placeholder] = ports.Attachment{
			Name:        placeholder,
			MediaType:   inferMediaTypeFromURL(resp.Content.FileURL, "application/octet-stream"),
			URI:         resp.Content.FileURL,
			Source:      "seedream",
			Description: "Downloadable asset",
		}
		metadata["file_placeholder"] = placeholder
	}

	builder := &strings.Builder{}
	title := strings.TrimSpace(description)
	if title == "" {
		title = "Seedance video generation"
	}
	fmt.Fprintf(builder, "%s complete.\n", title)
	fmt.Fprintf(builder, "Task: %s\n", strings.TrimSpace(resp.ID))
	fmt.Fprintf(builder, "Duration: %d seconds at %s.\n", duration, strings.TrimSpace(resolution))
	if placeholder, ok := metadata["video_placeholder"].(string); ok && strings.TrimSpace(placeholder) != "" {
		fmt.Fprintf(builder, "Video: [%s]\n", strings.TrimSpace(placeholder))
	}
	if placeholder, ok := metadata["last_frame_placeholder"].(string); ok && strings.TrimSpace(placeholder) != "" {
		fmt.Fprintf(builder, "Last frame preview: [%s]\n", strings.TrimSpace(placeholder))
	}
	if placeholder, ok := metadata["file_placeholder"].(string); ok && strings.TrimSpace(placeholder) != "" {
		fmt.Fprintf(builder, "Download bundle: [%s]\n", strings.TrimSpace(placeholder))
	}
	builder.WriteString("Plan follow-up edits for stitching or compositing as needed.")

	return builder.String(), metadata, attachments
}

func seedreamAttachmentPrefix(model string, created int64) string {
	prefix := strings.TrimSpace(model)
	if prefix != "" {
		prefix = strings.ReplaceAll(prefix, "/", "_")
	} else {
		prefix = "seedream"
	}
	if suffix := strings.TrimSpace(seedreamPlaceholderNonce()); suffix != "" {
		return fmt.Sprintf("%s_%s", prefix, suffix)
	}
	if created > 0 {
		return fmt.Sprintf("%s_%d", prefix, created)
	}
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

func inferMediaTypeFromURL(rawURL, defaultType string) string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return defaultType
	}
	lower := strings.ToLower(trimmed)
	switch {
	case strings.HasSuffix(lower, ".mp4"):
		return "video/mp4"
	case strings.HasSuffix(lower, ".webm"):
		return "video/webm"
	case strings.HasSuffix(lower, ".mov"):
		return "video/quicktime"
	case strings.HasSuffix(lower, ".gif"):
		return "image/gif"
	case strings.HasSuffix(lower, ".png"):
		return "image/png"
	case strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(lower, ".zip"):
		return "application/zip"
	case strings.HasSuffix(lower, ".tar") || strings.HasSuffix(lower, ".tar.gz"):
		return "application/x-tar"
	}
	return defaultType
}
