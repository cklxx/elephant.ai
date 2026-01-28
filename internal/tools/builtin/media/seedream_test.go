package media

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/utils"

	arkm "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model/responses"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
)

func stubSeedreamNonce(t *testing.T, value string) {
	t.Helper()
	original := seedreamPlaceholderNonce
	seedreamPlaceholderNonce = func() string {
		return value
	}
	t.Cleanup(func() {
		seedreamPlaceholderNonce = original
	})
}

func disableSeedreamUploader(t *testing.T) {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, nil, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("ALEX_CONFIG_PATH", configPath)

	seedreamUploaderOnce = sync.Once{}
	seedreamUploader = nil
	seedreamUploaderErr = nil
}

func assertContains(t *testing.T, content, needle, label string) {
	t.Helper()
	if !strings.Contains(content, needle) {
		t.Fatalf("expected %s to include %q, got %q", label, needle, content)
	}
}

func TestFormatSeedreamResponsePrefersPromptForDescriptions(t *testing.T) {
	disableSeedreamUploader(t)
	stubSeedreamNonce(t, "nonce")

	resp := &arkm.ImagesResponse{
		Model:   "doubao/seedream-3",
		Created: 123,
		Data: []*arkm.Image{
			{
				Url:     volcengine.String("https://example.com/a.png"),
				B64Json: volcengine.String("YWJjMTIz"),
				Size:    "1024x1024",
			},
		},
	}

	prompt := "超真实的猫咪坐在沙发上录音"
	descriptor := "Seedream 4.5 text-to-image"

	content, metadata, attachments := formatSeedreamResponse(resp, descriptor, prompt)

	assertContains(t, content, descriptor, "descriptor header")
	assertContains(t, content, "Prompt: "+prompt, "prompt line")
	assertContains(t, content, "[doubao_seedream-3_nonce_0.png]", "placeholder listing")

	if metadata["description"] != prompt {
		t.Fatalf("expected metadata description to equal prompt, got %#v", metadata["description"])
	}

	placeholder := "doubao_seedream-3_nonce_0.png"
	att, ok := attachments[placeholder]
	if !ok {
		t.Fatalf("expected attachment %q to exist", placeholder)
	}
	if att.Description != prompt {
		t.Fatalf("expected attachment description to be prompt, got %q", att.Description)
	}
	if att.Data != "YWJjMTIz" {
		t.Fatalf("expected attachment to preserve base64 payload, got %q", att.Data)
	}
	if !strings.HasPrefix(att.URI, "data:image/png;base64,") {
		t.Fatalf("expected data URI to be populated, got %q", att.URI)
	}
}

func TestFormatSeedreamResponseFallsBackToDescriptor(t *testing.T) {
	disableSeedreamUploader(t)
	stubSeedreamNonce(t, "nonce")

	resp := &arkm.ImagesResponse{
		Model:   "seedream",
		Created: 456,
		Data: []*arkm.Image{
			{
				Url:     volcengine.String("https://example.com/b.png"),
				B64Json: volcengine.String("ZGVm"),
				Size:    "512x512",
			},
		},
	}

	descriptor := "Seedream 4.5 text-to-image"

	content, metadata, attachments := formatSeedreamResponse(resp, descriptor, "")

	assertContains(t, content, descriptor, "descriptor title")
	assertContains(t, content, "[seedream_nonce_0.png]", "placeholder name")

	if metadata["description"] != descriptor {
		t.Fatalf("expected metadata description to equal descriptor, got %#v", metadata["description"])
	}

	placeholder := "seedream_nonce_0.png"
	att, ok := attachments[placeholder]
	if !ok {
		t.Fatalf("expected attachment %q to exist", placeholder)
	}
	if att.Description != descriptor {
		t.Fatalf("expected attachment description to fall back to descriptor, got %q", att.Description)
	}
}

func TestFormatSeedreamResponsePopulatesAttachmentURIFromBase64(t *testing.T) {
	disableSeedreamUploader(t)
	stubSeedreamNonce(t, "nonce")

	resp := &arkm.ImagesResponse{
		Model:   "seedream",
		Created: 789,
		Data: []*arkm.Image{
			{
				B64Json: volcengine.String("YWJjMTIz"),
				Size:    "256x256",
			},
		},
	}

	content, _, attachments := formatSeedreamResponse(resp, "descriptor", "prompt")
	placeholder := "seedream_nonce_0.png"
	att, ok := attachments[placeholder]
	if !ok {
		t.Fatalf("expected base64 attachment %q to be included", placeholder)
	}
	if att.URI == "" || att.Data == "" {
		t.Fatalf("expected attachment to include URI and Data fields, got %+v", att)
	}
	assertContains(t, content, "[seedream_nonce_0.png]", "placeholder listing")
}

func TestNormalizeSeedreamInitImageDataURI(t *testing.T) {
	base64 := "YWJjMTIz"
	value := " data:image/png;base64," + base64 + "  "

	actual, kind, err := normalizeSeedreamInitImage(value)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	expected := "data:image/png;base64," + base64
	if actual != expected {
		t.Fatalf("expected payload %q, got %q", expected, actual)
	}
	if kind != "data_uri" {
		t.Fatalf("expected kind %q, got %q", "data_uri", kind)
	}
}

func TestNormalizeSeedreamInitImageHTTPURL(t *testing.T) {
	raw := "https://example.com/seed.png"
	actual, kind, err := normalizeSeedreamInitImage(raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if actual != raw {
		t.Fatalf("expected URL %q, got %q", raw, actual)
	}
	if kind != "url" {
		t.Fatalf("expected kind %q, got %q", "url", kind)
	}
}

func TestNormalizeSeedreamInitImagePlainBase64(t *testing.T) {
	payload := "ZXhhbXBsZQ=="
	actual, kind, err := normalizeSeedreamInitImage(payload)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	expected := "data:image/png;base64," + payload
	if actual != expected {
		t.Fatalf("expected payload %q, got %q", expected, actual)
	}
	if kind != "data_uri" {
		t.Fatalf("expected kind %q, got %q", "data_uri", kind)
	}
}

func TestNormalizeSeedreamInitImageRejectsBadScheme(t *testing.T) {
	if actual, kind, err := normalizeSeedreamInitImage("ftp://example.com/image.png"); err == nil {
		t.Fatalf("expected error for unsupported scheme")
	} else {
		if actual != "" {
			t.Fatalf("expected empty payload on error, got %q", actual)
		}
		if kind != "" {
			t.Fatalf("expected empty kind on error, got %q", kind)
		}
	}
}

func TestSanitizeSeedreamGuidanceScale(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    float64
		expected float64
	}{
		{
			name:     "within bounds",
			input:    5.5,
			expected: 5.5,
		},
		{
			name:     "below minimum",
			input:    0.5,
			expected: seedreamDefaultGuidanceScale,
		},
		{
			name:     "above maximum",
			input:    11.0,
			expected: seedreamDefaultGuidanceScale,
		},
		{
			name:     "nan",
			input:    math.NaN(),
			expected: seedreamDefaultGuidanceScale,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			actual := sanitizeSeedreamGuidanceScale(tt.input)
			if !floatEquals(actual, tt.expected) {
				t.Fatalf("expected %v, got %v", tt.expected, actual)
			}
		})
	}
}

func TestApplyImageRequestOptionsDefaultsSize(t *testing.T) {
	t.Parallel()

	req := arkm.GenerateImagesRequest{}
	applyImageRequestOptions(&req, map[string]any{})

	if req.Size == nil {
		t.Fatal("expected default size to be set")
	}
	if *req.Size != seedreamDefaultImageSize {
		t.Fatalf("expected default size %q, got %q", seedreamDefaultImageSize, *req.Size)
	}
}

func TestApplyImageRequestOptionsUpscalesSmallSize(t *testing.T) {
	t.Parallel()

	req := arkm.GenerateImagesRequest{}
	applyImageRequestOptions(&req, map[string]any{"size": "1024x1024"})

	if req.Size == nil {
		t.Fatal("expected size to be set")
	}

	if *req.Size != seedreamDefaultImageSize {
		t.Fatalf("expected size to be upscaled to %s, got %s", seedreamDefaultImageSize, *req.Size)
	}
}

func floatEquals(a, b float64) bool {
	if math.IsNaN(a) && math.IsNaN(b) {
		return true
	}
	return a == b
}

func TestNormalizeSeedreamInitImageRequiresPayload(t *testing.T) {
	if actual, kind, err := normalizeSeedreamInitImage("data:image/png;base64,"); err == nil {
		t.Fatalf("expected error for empty payload")
	} else {
		if actual != "" {
			t.Fatalf("expected empty payload on error, got %q", actual)
		}
		if kind != "" {
			t.Fatalf("expected empty kind on error, got %q", kind)
		}
	}
}

func TestCoalesceSeedreamFirstFrameSourcePlainBase64(t *testing.T) {
	uri, kind, mime, err := coalesceSeedreamFirstFrameSource("", "aGVsbG8=", "image/jpeg")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if uri != "data:image/jpeg;base64,aGVsbG8=" {
		t.Fatalf("expected jpeg data URI, got %q", uri)
	}
	if kind != "data_uri" {
		t.Fatalf("expected kind data_uri, got %q", kind)
	}
	if mime != "image/jpeg" {
		t.Fatalf("expected resolved mime image/jpeg, got %q", mime)
	}
}

func TestCoalesceSeedreamFirstFrameSourceDataURIOverride(t *testing.T) {
	input := " data:image/png;base64,aGVsbG8= "
	uri, kind, mime, err := coalesceSeedreamFirstFrameSource("", input, ".webp")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if uri != "data:image/webp;base64,aGVsbG8=" {
		t.Fatalf("expected webp data URI, got %q", uri)
	}
	if kind != "data_uri" {
		t.Fatalf("expected kind data_uri, got %q", kind)
	}
	if mime != "image/webp" {
		t.Fatalf("expected resolved mime image/webp, got %q", mime)
	}
}

func TestCoalesceSeedreamFirstFrameSourceURL(t *testing.T) {
	uri, kind, mime, err := coalesceSeedreamFirstFrameSource("https://example.com/frame.png", "", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if uri != "https://example.com/frame.png" {
		t.Fatalf("expected URL passthrough, got %q", uri)
	}
	if kind != "url" {
		t.Fatalf("expected kind url, got %q", kind)
	}
	if mime != "" {
		t.Fatalf("expected empty mime for url, got %q", mime)
	}
}

func TestCoalesceSeedreamFirstFrameSourceRejectsInvalidBase64(t *testing.T) {
	if _, _, _, err := coalesceSeedreamFirstFrameSource("", "@@notbase64@@", ""); err == nil {
		t.Fatalf("expected error for invalid base64 input")
	}
}

func TestBuildSeedreamVideoPromptIncludesParameters(t *testing.T) {
	result := buildSeedreamVideoPrompt(
		"无人机穿越峡谷",
		"以山谷日出为开场",
		8,
		"4k",
		true,
		false,
		123,
		true,
	)

	if !strings.Contains(result, "无人机穿越峡谷") {
		t.Fatalf("expected prompt to include base instructions, got %q", result)
	}
	if !strings.Contains(result, "以山谷日出为开场") {
		t.Fatalf("expected prompt to mention first frame, got %q", result)
	}
	if !strings.Contains(result, "--duration 8") {
		t.Fatalf("expected duration flag, got %q", result)
	}
	if !strings.Contains(result, "--camerafixed true") {
		t.Fatalf("expected camera flag, got %q", result)
	}
	if strings.Contains(result, "--watermark true") {
		t.Fatalf("expected watermark false, got %q", result)
	}
	if !strings.Contains(result, "--watermark false") {
		t.Fatalf("expected watermark flag, got %q", result)
	}
	if !strings.Contains(result, "--seed 123") {
		t.Fatalf("expected seed flag, got %q", result)
	}
}

func TestFormatSeedreamVideoResponseCreatesAttachments(t *testing.T) {
	stubSeedreamNonce(t, "nonce")

	frames := int64(200)
	fps := int64(24)
	fileFormat := "mp4"
	revised := "Updated prompt"

	resp := &arkm.GetContentGenerationTaskResponse{
		ID:      "task-xyz",
		Model:   "doubao/seedance-pro",
		Status:  arkm.StatusSucceeded,
		Content: arkm.Content{VideoURL: "https://cdn.example.com/video.mp4", LastFrameURL: "https://cdn.example.com/last.png"},
		Usage:   arkm.Usage{CompletionTokens: 32},
	}
	resp.CreatedAt = 1700000000
	resp.UpdatedAt = 1700000010
	resp.Frames = &frames
	resp.FramesPerSecond = &fps
	resp.FileFormat = &fileFormat
	resp.RevisedPrompt = &revised

	content, metadata, attachments := formatSeedreamVideoResponse(resp, "Seedance pro", "山谷探险", 8, "1080p", "以山谷日出为开场", "https://example.com/frame.png", "url", "")

	if len(attachments) != 2 {
		t.Fatalf("expected two attachments, got %d", len(attachments))
	}

	videoKey := "doubao_seedance-pro_nonce_video.mp4"
	att, ok := attachments[videoKey]
	if !ok {
		t.Fatalf("expected video attachment %q", videoKey)
	}
	if att.MediaType != "video/mp4" {
		t.Fatalf("expected video media type, got %q", att.MediaType)
	}
	if att.URI != resp.Content.VideoURL {
		t.Fatalf("expected video URI %q, got %q", resp.Content.VideoURL, att.URI)
	}

	lastFrameKey := "doubao_seedance-pro_nonce_last_frame.png"
	if _, ok := attachments[lastFrameKey]; !ok {
		t.Fatalf("expected last frame attachment %q", lastFrameKey)
	}

	if !strings.Contains(content, "Video: ["+videoKey+"]") {
		t.Fatalf("expected content to reference video placeholder, got %q", content)
	}
	if !strings.Contains(content, "stitching") {
		t.Fatalf("expected content to encourage stitching workflow, got %q", content)
	}

	if metadata["video_placeholder"] != videoKey {
		t.Fatalf("expected metadata video placeholder %q, got %#v", videoKey, metadata["video_placeholder"])
	}
	if metadata["first_frame_kind"] != "url" {
		t.Fatalf("expected first_frame_kind metadata to be url, got %#v", metadata["first_frame_kind"])
	}
	if metadata["first_frame_mime"] != "" {
		t.Fatalf("expected empty first_frame_mime metadata, got %#v", metadata["first_frame_mime"])
	}

	capabilities, ok := metadata["capabilities"].(map[string]any)
	if !ok {
		t.Fatalf("expected capabilities metadata to be present")
	}
	if capabilities["stitching"] != "planned" {
		t.Fatalf("expected capabilities metadata to mention stitching")
	}
}

func TestSeedreamVideoDefinitionMentionsDurationRange(t *testing.T) {
	config := SeedreamConfig{APIKey: "k", Model: "m", ModelDescriptor: "Seedance"}
	tool := NewSeedreamVideoGenerate(config)

	def := tool.Definition()
	rangeToken := fmt.Sprintf("%d-%d", seedanceMinDurationSeconds, seedanceMaxDurationSeconds)
	if !strings.Contains(def.Description, rangeToken) {
		t.Fatalf("expected description %q to mention range token %q", def.Description, rangeToken)
	}

	durationDesc := def.Parameters.Properties["duration_seconds"].Description
	if !strings.Contains(durationDesc, rangeToken) {
		t.Fatalf("expected duration description %q to include range token %q", durationDesc, rangeToken)
	}
}

func TestSeedreamTextToolLogsRequestAndResponse(t *testing.T) {
	logDir := t.TempDir()
	t.Setenv("ALEX_REQUEST_LOG_DIR", logDir)

	cfg := SeedreamConfig{APIKey: "key", Model: "seedream", ModelDescriptor: "Seedream"}
	client := &stubSeedreamClient{
		imagesResp: arkm.ImagesResponse{
			Model:   "seedream",
			Created: 123,
			Data: []*arkm.Image{{
				B64Json: volcengine.String("YQ=="),
				Size:    "1x1",
			}},
		},
	}

	tool := &seedreamTextTool{config: cfg, factory: &seedreamClientFactory{config: cfg, client: client}}

	if _, err := tool.Execute(context.Background(), ports.ToolCall{ID: "seedream-call", Arguments: map[string]any{"prompt": "hi"}}); err != nil {
		t.Fatalf("expected tool to succeed, got error: %v", err)
	}

	logPath := filepath.Join(logDir, "llm.jsonl")
	if !utils.WaitForRequestLogQueueDrain(2 * time.Second) {
		t.Fatalf("timed out waiting for request log queue to drain")
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read request log: %v", err)
	}

	entries := parseRequestLogEntries(t, string(data))
	if !hasRequestLogEntry(entries, "seedream-call", "request") {
		t.Fatalf("expected request payload entry in log, got: %#v", entries)
	}
	if !hasRequestLogEntry(entries, "seedream-call", "response") {
		t.Fatalf("expected response payload entry in log, got: %#v", entries)
	}
}

type stubSeedreamClient struct {
	imagesResp    arkm.ImagesResponse
	imagesErr     error
	responsesResp *responses.ResponseObject
	responsesErr  error
	createResp    *arkm.CreateContentGenerationTaskResponse
	createErr     error
	getResp       *arkm.GetContentGenerationTaskResponse
	getErr        error
}

func (s *stubSeedreamClient) GenerateImages(context.Context, arkm.GenerateImagesRequest) (arkm.ImagesResponse, error) {
	return s.imagesResp, s.imagesErr
}

func (s *stubSeedreamClient) CreateResponses(context.Context, *responses.ResponsesRequest) (*responses.ResponseObject, error) {
	if s.responsesResp != nil || s.responsesErr != nil {
		return s.responsesResp, s.responsesErr
	}
	return nil, errors.New("unexpected CreateResponses call")
}

func (s *stubSeedreamClient) CreateContentGenerationTask(context.Context, arkm.CreateContentGenerationTaskRequest) (*arkm.CreateContentGenerationTaskResponse, error) {
	if s.createResp != nil || s.createErr != nil {
		return s.createResp, s.createErr
	}
	return nil, errors.New("unexpected CreateContentGenerationTask call")
}

func (s *stubSeedreamClient) GetContentGenerationTask(context.Context, arkm.GetContentGenerationTaskRequest) (*arkm.GetContentGenerationTaskResponse, error) {
	if s.getResp != nil || s.getErr != nil {
		return s.getResp, s.getErr
	}
	return nil, errors.New("unexpected GetContentGenerationTask call")
}

type requestLogEntry struct {
	RequestID string `json:"request_id"`
	EntryType string `json:"entry_type"`
}

func parseRequestLogEntries(t *testing.T, content string) []requestLogEntry {
	t.Helper()
	var entries []requestLogEntry
	for _, line := range strings.Split(strings.TrimSpace(content), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry requestLogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("failed to parse request log entry: %v", err)
		}
		entries = append(entries, entry)
	}
	return entries
}

func hasRequestLogEntry(entries []requestLogEntry, requestID, entryType string) bool {
	for _, entry := range entries {
		if matchesRequestID(entry.RequestID, requestID) && entry.EntryType == entryType {
			return true
		}
	}
	return false
}

func matchesRequestID(actual, expected string) bool {
	if actual == expected {
		return true
	}
	return strings.HasSuffix(actual, ":"+expected)
}

func TestSeedreamVideoRejectsDurationsOutsideRange(t *testing.T) {
	config := SeedreamConfig{APIKey: "k", Model: "m", ModelDescriptor: "Seedance"}
	tool := &seedreamVideoTool{config: config, factory: &seedreamClientFactory{config: config}}

	call := ports.ToolCall{
		ID: "test",
		Arguments: map[string]any{
			"prompt":           "无人机穿越峡谷",
			"duration_seconds": seedanceMaxDurationSeconds + 1,
		},
	}

	result, err := tool.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("expected tool error to be captured in result, got %v", err)
	}
	if result.Error == nil {
		t.Fatalf("expected result error to be populated")
	}
	expected := fmt.Sprintf("between %d and %d seconds", seedanceMinDurationSeconds, seedanceMaxDurationSeconds)
	if !strings.Contains(result.Content, expected) {
		t.Fatalf("expected result content %q to mention range %q", result.Content, expected)
	}

	belowCall := ports.ToolCall{
		ID: "test-below",
		Arguments: map[string]any{
			"prompt":           "无人机穿越峡谷",
			"duration_seconds": seedanceMinDurationSeconds - 1,
		},
	}

	belowResult, err := tool.Execute(context.Background(), belowCall)
	if err != nil {
		t.Fatalf("expected below-range duration to surface as result error, got %v", err)
	}
	if belowResult.Error == nil {
		t.Fatalf("expected below-range duration to produce an error result")
	}
	if !strings.Contains(belowResult.Content, expected) {
		t.Fatalf("expected below-range content %q to mention range %q", belowResult.Content, expected)
	}
}

func TestInferMediaTypeFromURL(t *testing.T) {
	cases := map[string]string{
		"https://example.com/demo.webm":   "video/webm",
		"https://example.com/cover.jpeg":  "image/jpeg",
		"https://example.com/archive.zip": "application/zip",
		"":                                "video/mp4",
	}

	for input, expected := range cases {
		if got := inferMediaTypeFromURL(input, "video/mp4"); got != expected {
			t.Fatalf("expected media type %q for %q, got %q", expected, input, got)
		}
	}
}

func TestResolveSeedreamInitImagePlaceholder(t *testing.T) {
	ctx := tools.WithAttachmentContext(context.Background(), map[string]ports.Attachment{
		"seed.png": {
			Name:      "seed.png",
			MediaType: "image/png",
			Data:      "YmFzZTY0",
		},
	}, nil)

	resolved, placeholder, ok := resolveSeedreamInitImagePlaceholder(ctx, " [seed.png] ")
	if !ok {
		t.Fatalf("expected placeholder to resolve via attachment context")
	}
	if placeholder != "seed.png" {
		t.Fatalf("expected placeholder name to be preserved, got %q", placeholder)
	}
	expected := "data:image/png;base64,YmFzZTY0"
	if resolved != expected {
		t.Fatalf("expected resolved data URI %q, got %q", expected, resolved)
	}
}

func TestResolveSeedreamInitImagePlaceholderMissing(t *testing.T) {
	ctx := tools.WithAttachmentContext(context.Background(), map[string]ports.Attachment{
		"seed.png": {
			Name:      "seed.png",
			MediaType: "image/png",
			Data:      "YmFzZTY0",
		},
	}, nil)

	if _, _, ok := resolveSeedreamInitImagePlaceholder(ctx, "[unknown.png]"); ok {
		t.Fatalf("expected unknown placeholder to remain unresolved")
	}

	if _, _, ok := resolveSeedreamInitImagePlaceholder(context.Background(), "[seed.png]"); ok {
		t.Fatalf("expected resolution to fail without attachment context")
	}

	if _, _, ok := resolveSeedreamInitImagePlaceholder(ctx, "seed.png"); ok {
		t.Fatalf("expected bare filenames to be ignored")
	}
}

func TestResolveSeedreamVisionImagesResolvesAttachments(t *testing.T) {
	ctx := tools.WithAttachmentContext(context.Background(), map[string]ports.Attachment{
		"scene.png": {
			Name:      "scene.png",
			MediaType: "image/png",
			Data:      "YmFzZTY0",
			Source:    "camera_upload",
		},
		"Asset.JPG": {
			Name:      "Asset.JPG",
			MediaType: "image/jpeg",
			URI:       "https://example.com/asset.jpg",
			Source:    "user_upload",
		},
	}, nil)

	resolved, note, err := resolveSeedreamVisionImages(ctx, []string{
		"[scene.png]",
		"asset.jpg",
		"https://example.com/remote.png",
	})
	if err != nil {
		t.Fatalf("expected vision image resolution to succeed, got %v", err)
	}
	if len(resolved) != 3 {
		t.Fatalf("expected 3 resolved images, got %d", len(resolved))
	}
	if resolved[0] != "data:image/png;base64,YmFzZTY0" {
		t.Fatalf("expected data URI for scene.png, got %q", resolved[0])
	}
	if resolved[1] != "https://example.com/asset.jpg" {
		t.Fatalf("expected attachment URL for asset.jpg, got %q", resolved[1])
	}
	if resolved[2] != "https://example.com/remote.png" {
		t.Fatalf("expected remote URL to pass through, got %q", resolved[2])
	}
	if !strings.Contains(note, "Image sources:") {
		t.Fatalf("expected source note header, got %q", note)
	}
	if !strings.Contains(note, "scene.png: camera_upload") {
		t.Fatalf("expected scene source in note, got %q", note)
	}
	if !strings.Contains(note, "Asset.JPG: user_upload") {
		t.Fatalf("expected asset source in note, got %q", note)
	}
}

func TestResolveSeedreamVisionImagesRejectsMissingPlaceholder(t *testing.T) {
	ctx := tools.WithAttachmentContext(context.Background(), map[string]ports.Attachment{
		"seed.png": {
			Name:      "seed.png",
			MediaType: "image/png",
			Data:      "YmFzZTY0",
		},
	}, nil)

	_, _, err := resolveSeedreamVisionImages(ctx, []string{"[missing.png]"})
	if err == nil {
		t.Fatalf("expected missing placeholder to return an error")
	}
	if !strings.Contains(err.Error(), "image placeholder [missing.png] could not be resolved") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestSeedreamImageToImageRejectsUnresolvedPlaceholder(t *testing.T) {
	tool := &seedreamImageTool{
		config: SeedreamConfig{
			APIKey: "test-key",
			Model:  "test-model",
		},
		factory: &seedreamClientFactory{},
	}

	call := ports.ToolCall{
		ID: "placeholder-missing",
		Arguments: map[string]any{
			"init_image": " [missing.png] ",
		},
	}

	result, err := tool.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("expected error to be captured in result, got %v", err)
	}
	if result.Error == nil {
		t.Fatalf("expected unresolved placeholder to surface as error result")
	}
	expected := "init_image placeholder [missing.png] could not be resolved"
	if !strings.Contains(result.Content, expected) {
		t.Fatalf("expected error message to mention unresolved placeholder, got %q", result.Content)
	}
}
