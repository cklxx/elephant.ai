package builtin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"reflect"
	"strings"
	"time"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/httpclient"
	"alex/internal/logging"

	arkm "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
)

type seedreamVideoTool struct {
	config     SeedreamConfig
	factory    *seedreamClientFactory
	httpClient *http.Client
	logger     logging.Logger
}

// NewSeedreamVideoGenerate returns a tool that creates short videos from prompts.
func NewSeedreamVideoGenerate(config SeedreamConfig) tools.ToolExecutor {
	logger := logging.NewComponentLogger("SeedreamVideo")
	return &seedreamVideoTool{
		config:     config,
		factory:    &seedreamClientFactory{config: config},
		httpClient: httpclient.New(seedreamAssetHTTPTimeout, logger),
		logger:     logger,
	}
}

func (t *seedreamVideoTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "video_generate",
		Version:  "1.0.0",
		Category: "design",
		Tags:     []string{"video", "generation", "seedream", "seedance"},
		MaterialCapabilities: ports.ToolMaterialCapabilities{
			Consumes: []string{"image/png", "image/jpeg", "image/webp", "image/gif"},
			Produces: []string{"video/mp4", "image/png", "application/octet-stream"},
		},
	}
}

func (t *seedreamVideoTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name: "video_generate",
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
					Description: "Optional prompt for the establishing first frame (defaults to prompt).",
				},
				"first_frame_url": {
					Type:        "string",
					Description: "Optional URL or data URI for the first frame reference.",
				},
				"first_frame_base64": {
					Type:        "string",
					Description: "Optional base64 payload for the first frame image.",
				},
				"first_frame_mime_type": {
					Type:        "string",
					Description: "Optional MIME type for first_frame_base64.",
				},
				"resolution": {
					Type:        "string",
					Description: "Requested output resolution (e.g. 1080p, 720p).",
				},
				"camera_fixed": {
					Type:        "boolean",
					Description: "Whether the camera should remain fixed during generation.",
				},
				"return_last_frame": {
					Type:        "boolean",
					Description: "Whether to include the last frame image in the response.",
				},
				"seed": {
					Type:        "integer",
					Description: "Random seed for reproducible generation.",
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
		MaterialCapabilities: ports.ToolMaterialCapabilities{
			Consumes: []string{"image/png", "image/jpeg", "image/webp", "image/gif"},
			Produces: []string{"video/mp4", "image/png", "application/octet-stream"},
		},
	}
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
	watermark := false
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

	requestID := strings.TrimSpace(call.ID)

	client, err := t.factory.instance()
	if err != nil {
		wrapped := fmt.Errorf("seedream client init: %w", err)
		return &ports.ToolResult{CallID: call.ID, Content: wrapped.Error(), Error: wrapped}, nil
	}

	logSeedreamRequestPayload(requestID, request)

	createResp, err := client.CreateContentGenerationTask(ctx, request)
	if err != nil {
		logSeedreamResponsePayload(requestID, map[string]any{"error": err.Error(), "stage": "create_task"})
		return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("Seedance request failed: %v", err), Error: err}, nil
	}
	logSeedreamResponsePayload(requestID, createResp)

	taskID := strings.TrimSpace(createResp.ID)
	if taskID == "" {
		err = errors.New("seedance did not return a task identifier")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	deadline := time.Now().Add(maxWait)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			err := fmt.Errorf("seedance polling cancelled: %w", ctx.Err())
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		default:
		}

		resp, err := client.GetContentGenerationTask(ctx, arkm.GetContentGenerationTaskRequest{ID: taskID})
		if err != nil {
			logSeedreamResponsePayload(requestID, map[string]any{"error": err.Error(), "stage": "poll", "task_id": taskID})
			return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("Seedance polling failed: %v", err), Error: err}, nil
		}

		if resp.Status != arkm.StatusRunning {
			logSeedreamResponsePayload(requestID, resp)
		}

		switch resp.Status {
		case arkm.StatusSucceeded:
			content, metadata, attachments := formatSeedreamVideoResponse(
				resp,
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
			err := errors.New("seedance task cancelled")
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}

		if time.Now().After(deadline) {
			err := fmt.Errorf("seedance task %s did not complete within %d seconds", taskID, int(maxWait/time.Second))
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}

		select {
		case <-ctx.Done():
			err := fmt.Errorf("seedance polling cancelled: %w", ctx.Err())
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		case <-ticker.C:
		}
	}
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
	if usage := seedreamVideoUsage(resp); usage != nil && usage.CompletionTokens != 0 {
		metadata["usage"] = *usage
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

func inlineLimitForMedia(mediaType string) int64 {
	lower := strings.ToLower(strings.TrimSpace(mediaType))
	switch {
	case strings.HasPrefix(lower, "video/"):
		return seedreamMaxInlineVideoBytes
	case strings.HasPrefix(lower, "image/"):
		return seedreamMaxInlineImageBytes
	default:
		return seedreamMaxInlineBinaryBytes
	}
}

func (t *seedreamVideoTool) downloadAsset(ctx context.Context, assetURL string, maxBytes int64) ([]byte, string, error) {
	if maxBytes <= 0 {
		maxBytes = seedreamMaxInlineBinaryBytes
	}
	client := t.httpClient
	if client == nil {
		client = httpclient.New(seedreamAssetHTTPTimeout, t.logger)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, assetURL, nil)
	if err != nil {
		return nil, "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("http %d received while fetching %s", resp.StatusCode, assetURL)
	}

	limited := io.LimitReader(resp.Body, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, "", err
	}
	if int64(len(data)) > maxBytes {
		return nil, "", fmt.Errorf("asset exceeds inline limit (%d bytes)", maxBytes)
	}

	mediaType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if mediaType == "" {
		if ext := path.Ext(assetURL); ext != "" {
			if resolved := mime.TypeByExtension(ext); resolved != "" {
				mediaType = resolved
			}
		}
	}

	return data, mediaType, nil
}

// seedreamVideoUsage returns a non-nil usage pointer when the Seedance response
// includes usage metrics. The underlying SDK currently models Usage as a
// struct, but some responses may omit it entirely (and future SDK versions may
// switch to a pointer), so we use reflection to safely support both layouts.
func seedreamVideoUsage(resp *arkm.GetContentGenerationTaskResponse) *arkm.Usage {
	if resp == nil {
		return nil
	}

	field := reflect.ValueOf(resp).Elem().FieldByName("Usage")
	if !field.IsValid() {
		return nil
	}

	switch field.Kind() {
	case reflect.Pointer:
		if field.IsNil() {
			return nil
		}
		usage, ok := field.Interface().(*arkm.Usage)
		if !ok || usage == nil {
			return nil
		}
		if reflect.ValueOf(usage).Elem().IsZero() {
			return nil
		}
		return usage
	case reflect.Struct:
		if field.IsZero() {
			return nil
		}
		if _, ok := field.Interface().(arkm.Usage); !ok {
			return nil
		}
		return &resp.Usage
	default:
		return nil
	}
}
