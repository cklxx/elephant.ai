package builtin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"alex/internal/agent/ports"

	maasapi "github.com/volcengine/volc-sdk-golang/service/maas/models/api/v2"
	maasv2 "github.com/volcengine/volc-sdk-golang/service/maas/v2"
)

const (
	defaultSeedreamHost   = "maas-api.ml-platform-cn-beijing.volces.com"
	defaultSeedreamRegion = "cn-beijing"
)

// SeedreamConfig holds shared configuration for Seedream tools.
type SeedreamConfig struct {
	AccessKey       string
	SecretKey       string
	EndpointID      string
	Host            string
	Region          string
	ModelDescriptor string
	EndpointEnvVar  string
}

type seedreamClientFactory struct {
	config SeedreamConfig
	once   sync.Once
	client *maasv2.MaaS
	err    error
}

func (f *seedreamClientFactory) instance() (*maasv2.MaaS, error) {
	f.once.Do(func() {
		host := strings.TrimSpace(f.config.Host)
		if host == "" {
			host = defaultSeedreamHost
		}
		region := strings.TrimSpace(f.config.Region)
		if region == "" {
			region = defaultSeedreamRegion
		}

		f.client = maasv2.NewInstance(host, region)
		if f.client == nil {
			f.err = fmt.Errorf("failed to initialize Seedream client")
			return
		}
		if f.config.AccessKey != "" {
			f.client.SetAccessKey(f.config.AccessKey)
		}
		if f.config.SecretKey != "" {
			f.client.SetSecretKey(f.config.SecretKey)
		}
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

// NewSeedreamTextToImage creates a Seedream text-to-image tool backed by Seedream 3.0.
func NewSeedreamTextToImage(config SeedreamConfig) ports.ToolExecutor {
	factory := &seedreamClientFactory{config: config}
	return &seedreamTextTool{config: config, factory: factory}
}

// NewSeedreamImageToImage creates a Seedream image-to-image tool backed by Seedream 4.0.
func NewSeedreamImageToImage(config SeedreamConfig) ports.ToolExecutor {
	factory := &seedreamClientFactory{config: config}
	return &seedreamImageTool{config: config, factory: factory}
}

func (t *seedreamTextTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "seedream_text_to_image",
		Version:  "1.0.0",
		Category: "design",
		Tags:     []string{"image", "generation", "seedream", "text-to-image"},
	}
}

func (t *seedreamTextTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name: "seedream_text_to_image",
		Description: "Generate brand-new visuals with Volcano Engine Seedream 3.0. " +
			"Provide a descriptive prompt and optionally include negative prompts or generation controls. " +
			"The tool returns download URLs when available and base64-encoded image payloads in metadata.",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"prompt": {
					Type:        "string",
					Description: "Creative brief describing the desired image.",
				},
				"negative_prompt": {
					Type:        "string",
					Description: "Optional negative prompt to avoid specific traits.",
				},
				"seed": {
					Type:        "integer",
					Description: "Random seed for reproducible outputs.",
				},
				"width": {
					Type:        "integer",
					Description: "Output width in pixels (default decided by endpoint).",
				},
				"height": {
					Type:        "integer",
					Description: "Output height in pixels (default decided by endpoint).",
				},
				"num_inference_steps": {
					Type:        "integer",
					Description: "Number of diffusion steps to run.",
				},
				"cfg_scale": {
					Type:        "number",
					Description: "Classifier-free guidance scale controlling prompt strength.",
				},
				"sampler_name": {
					Type:        "string",
					Description: "Sampler to use when the endpoint exposes multiple choices.",
				},
				"scheduler": {
					Type:        "string",
					Description: "Scheduler strategy supported by the endpoint.",
				},
				"control_images": {
					Type:        "array",
					Description: "Optional list of base64-encoded control images to steer composition.",
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

	req := &maasapi.ImagesQuickGenReq{Prompt: prompt}
	if negative, ok := call.Arguments["negative_prompt"].(string); ok {
		req.NegativePrompt = strings.TrimSpace(negative)
	}

	params := &maasapi.ImagesParameters{}
	var hasParams bool

	if seed, ok := readInt(call.Arguments, "seed"); ok {
		params.Seed = seed
		hasParams = true
	}
	if width, ok := readInt(call.Arguments, "width"); ok {
		params.Width = width
		hasParams = true
	}
	if height, ok := readInt(call.Arguments, "height"); ok {
		params.Height = height
		hasParams = true
	}
	if steps, ok := readInt(call.Arguments, "num_inference_steps"); ok {
		params.NumInferenceSteps = steps
		hasParams = true
	}
	if scale, ok := readFloat(call.Arguments, "cfg_scale"); ok {
		params.CfgScale = float32(scale)
		hasParams = true
	}
	if sampler, ok := call.Arguments["sampler_name"].(string); ok && strings.TrimSpace(sampler) != "" {
		params.SamplerName = strings.TrimSpace(sampler)
		hasParams = true
	}
	if scheduler, ok := call.Arguments["scheduler"].(string); ok && strings.TrimSpace(scheduler) != "" {
		params.Scheduler = strings.TrimSpace(scheduler)
		hasParams = true
	}
	if strength, ok := readFloat(call.Arguments, "strength"); ok {
		params.Strength = float32(strength)
		hasParams = true
	}
	if hasParams {
		req.Parameters = params
	}

	if controlImages, ok := decodeImageList(call.Arguments["control_images"]); ok {
		req.ControlImageList = controlImages
	}
	if initImg, ok := decodeImage(call.Arguments["init_image"]); ok {
		req.InitImage = initImg
	}

	client, err := t.factory.instance()
	if err != nil {
		wrapped := fmt.Errorf("seedream client init: %w", err)
		return &ports.ToolResult{CallID: call.ID, Content: wrapped.Error(), Error: wrapped}, nil
	}

	resp, status, err := client.Images().ImagesQuickGen(t.config.EndpointID, req)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: describeSeedreamError(err, status), Error: err}, nil
	}
	if resp == nil {
		err = fmt.Errorf("empty response from Seedream")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	content, metadata := formatSeedreamResponse(resp.ReqId, status, t.config.ModelDescriptor, resp.Data)
	return &ports.ToolResult{CallID: call.ID, Content: content, Metadata: metadata}, nil
}

func (t *seedreamImageTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "seedream_image_to_image",
		Version:  "1.0.0",
		Category: "design",
		Tags:     []string{"image", "generation", "seedream", "image-to-image"},
	}
}

func (t *seedreamImageTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name: "seedream_image_to_image",
		Description: "Transform or upscale reference art with Volcano Engine Seedream 4.0. " +
			"Supply a base64-encoded init_image along with the desired prompt and optional controls.",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"prompt": {
					Type:        "string",
					Description: "Reinforcement prompt describing target adjustments.",
				},
				"negative_prompt": {
					Type:        "string",
					Description: "Optional negative prompt to suppress traits.",
				},
				"init_image": {
					Type:        "string",
					Description: "Base64-encoded source image (PNG or JPEG).",
				},
				"control_images": {
					Type:        "array",
					Description: "Optional list of additional control images encoded in base64.",
				},
				"seed": {
					Type:        "integer",
					Description: "Random seed for reproducible refinements.",
				},
				"strength": {
					Type:        "number",
					Description: "Blend strength between the init image and the prompt (0-1).",
				},
				"width": {
					Type:        "integer",
					Description: "Output width in pixels (default controlled by endpoint).",
				},
				"height": {
					Type:        "integer",
					Description: "Output height in pixels (default controlled by endpoint).",
				},
				"num_inference_steps": {
					Type:        "integer",
					Description: "Number of diffusion steps to run.",
				},
				"cfg_scale": {
					Type:        "number",
					Description: "Classifier-free guidance scale controlling prompt strength.",
				},
				"sampler_name": {
					Type:        "string",
					Description: "Sampler to use when available on the endpoint.",
				},
				"scheduler": {
					Type:        "string",
					Description: "Scheduler strategy supported by the endpoint.",
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

	initImage, ok := decodeImage(call.Arguments["init_image"])
	if !ok || len(initImage) == 0 {
		err := errors.New("init_image parameter must be a base64-encoded image")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	req := &maasapi.ImagesReq{InitImage: initImage}
	if prompt, ok := call.Arguments["prompt"].(string); ok {
		req.Prompt = strings.TrimSpace(prompt)
	}
	if negative, ok := call.Arguments["negative_prompt"].(string); ok {
		req.NegativePrompt = strings.TrimSpace(negative)
	}
	if controlImages, ok := decodeImageList(call.Arguments["control_images"]); ok {
		req.ControlImageList = controlImages
	}
	if seed, ok := readInt(call.Arguments, "seed"); ok {
		req.Seed = seed
	}
	if strength, ok := readFloat(call.Arguments, "strength"); ok {
		req.Strength = float32(strength)
	}
	if width, ok := readInt(call.Arguments, "width"); ok {
		req.Width = width
	}
	if height, ok := readInt(call.Arguments, "height"); ok {
		req.Height = height
	}
	if steps, ok := readInt(call.Arguments, "num_inference_steps"); ok {
		req.NumInferenceSteps = steps
	}
	if scale, ok := readFloat(call.Arguments, "cfg_scale"); ok {
		req.CfgScale = float32(scale)
	}
	if sampler, ok := call.Arguments["sampler_name"].(string); ok && strings.TrimSpace(sampler) != "" {
		req.SamplerName = strings.TrimSpace(sampler)
	}
	if scheduler, ok := call.Arguments["scheduler"].(string); ok && strings.TrimSpace(scheduler) != "" {
		req.Scheduler = strings.TrimSpace(scheduler)
	}

	client, err := t.factory.instance()
	if err != nil {
		wrapped := fmt.Errorf("seedream client init: %w", err)
		return &ports.ToolResult{CallID: call.ID, Content: wrapped.Error(), Error: wrapped}, nil
	}

	resp, status, err := client.Images().ImagesFlexGen(t.config.EndpointID, req)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: describeSeedreamError(err, status), Error: err}, nil
	}
	if resp == nil {
		err = fmt.Errorf("empty response from Seedream")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	content, metadata := formatSeedreamResponse(resp.ReqId, status, t.config.ModelDescriptor, resp.Data)
	return &ports.ToolResult{CallID: call.ID, Content: content, Metadata: metadata}, nil
}

func seedreamMissingConfigMessage(config SeedreamConfig) string {
	missing := []string{}
	if strings.TrimSpace(config.AccessKey) == "" {
		missing = append(missing, "VOLC_ACCESSKEY")
	}
	if strings.TrimSpace(config.SecretKey) == "" {
		missing = append(missing, "VOLC_SECRETKEY")
	}
	if strings.TrimSpace(config.EndpointID) == "" {
		label := "Seedream endpoint ID"
		if config.EndpointEnvVar != "" {
			label = fmt.Sprintf("%s", strings.ToUpper(config.EndpointEnvVar))
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
	builder.WriteString(fmt.Sprintf("%s is not configured. Missing values: %s.\n\n", toolName, strings.Join(missing, ", ")))
	builder.WriteString("Provide the following settings via environment variables or ~/.alex-config.json:\n\n")
	builder.WriteString("- VOLC_ACCESSKEY and VOLC_SECRETKEY from the Volcano Engine console\n")
	if config.EndpointEnvVar != "" {
		builder.WriteString(fmt.Sprintf("- %s for the desired endpoint\n", strings.ToUpper(config.EndpointEnvVar)))
	} else {
		builder.WriteString("- Seedream endpoint identifier for the selected model\n")
	}
	builder.WriteString("Optional overrides: SEEDREAM_HOST, SEEDREAM_REGION")
	return builder.String()
}

func describeSeedreamError(err error, status int) string {
	var apiErr *maasapi.Error
	if errors.As(err, &apiErr) {
		if status != 0 {
			return fmt.Sprintf("Seedream API error (status %d): %s", status, apiErr.Error())
		}
		return fmt.Sprintf("Seedream API error: %s", apiErr.Error())
	}
	if status != 0 {
		return fmt.Sprintf("Seedream request failed with status %d: %v", status, err)
	}
	return fmt.Sprintf("Seedream request failed: %v", err)
}

func formatSeedreamResponse(reqID string, status int, model string, data []*maasapi.ImageUrl) (string, map[string]any) {
	images := make([]map[string]any, 0, len(data))
	for idx, item := range data {
		entry := map[string]any{"index": idx}
		if item.Url != "" {
			entry["url"] = item.Url
		}
		if len(item.ImageBytes) > 0 {
			encoded := base64.StdEncoding.EncodeToString(item.ImageBytes)
			entry["base64"] = encoded
			entry["data_uri"] = "data:image/png;base64," + encoded
		}
		if item.Detail != "" {
			entry["detail"] = item.Detail
		}
		images = append(images, entry)
	}

	metadata := map[string]any{
		"req_id": reqID,
		"images": images,
	}
	if status != 0 {
		metadata["status"] = status
	}
	if model != "" {
		metadata["model"] = model
	}

	var sb strings.Builder
	if model != "" {
		sb.WriteString(fmt.Sprintf("Seedream %s response", model))
	} else {
		sb.WriteString("Seedream response")
	}
	if reqID != "" {
		sb.WriteString(fmt.Sprintf(" (req_id: %s)", reqID))
	}
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("Generated %d image(s). Base64 payloads are available in metadata.\n", len(images)))
	if len(images) > 0 {
		for i, img := range images {
			if url, ok := img["url"]; ok && url != "" {
				sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, url))
			}
		}
	}
	return sb.String(), metadata
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
	case float32:
		return float64(v), true
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

// decodeImage decodes a base64 value if present.
func decodeImage(value any) ([]byte, bool) {
	str, ok := value.(string)
	if !ok {
		return nil, false
	}
	str = strings.TrimSpace(str)
	if str == "" {
		return nil, false
	}
	data, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return nil, false
	}
	return data, true
}

func decodeImageList(value any) ([][]byte, bool) {
	if value == nil {
		return nil, false
	}
	list, ok := value.([]any)
	if !ok {
		if str, ok := value.(string); ok {
			if data, ok := decodeImage(str); ok {
				return [][]byte{data}, true
			}
		}
		return nil, false
	}
	var result [][]byte
	for _, item := range list {
		if data, ok := decodeImage(item); ok {
			result = append(result, data)
		}
	}
	if len(result) == 0 {
		return nil, false
	}
	return result, true
}
