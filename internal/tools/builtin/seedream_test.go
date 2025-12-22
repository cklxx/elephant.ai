package builtin

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"alex/internal/agent/ports"

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

func TestFormatSeedreamResponsePrefersPromptForDescriptions(t *testing.T) {
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

	if !strings.Contains(content, descriptor) {
		t.Fatalf("expected content to include descriptor header, got %q", content)
	}
	if strings.Contains(content, prompt) {
		t.Fatalf("expected prompt to be omitted from content, got %q", content)
	}
	if !strings.Contains(content, "[doubao_seedream-3_nonce_0.png]") {
		t.Fatalf("expected content to include placeholder listing, got %q", content)
	}

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
}

func TestFormatSeedreamResponseFallsBackToDescriptor(t *testing.T) {
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

	if !strings.Contains(content, descriptor) {
		t.Fatalf("expected content to include descriptor title, got %q", content)
	}
	if !strings.Contains(content, "[seedream_nonce_0.png]") {
		t.Fatalf("expected content to list placeholder names, got %q", content)
	}

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

	_, _, attachments := formatSeedreamResponse(resp, "descriptor", "prompt")
	placeholder := "seedream_nonce_0.png"
	att, ok := attachments[placeholder]
	if !ok {
		t.Fatalf("expected attachment %q to exist", placeholder)
	}
	if att.URI == "" {
		t.Fatalf("expected attachment URI to be populated for %q", placeholder)
	}
	if !strings.HasPrefix(att.URI, "data:image/png;base64,") {
		t.Fatalf("expected attachment URI to be data URI, got %q", att.URI)
	}
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

func TestSeedreamVideoToolEmbedRemoteAttachmentDataInlinesVideo(t *testing.T) {
	payload := bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 512)

	tool := &seedreamVideoTool{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				resp := &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(bytes.NewReader(payload)),
				}
				resp.Header.Set("Content-Type", "video/mp4")
				return resp, nil
			}),
		},
	}
	attachments := map[string]ports.Attachment{
		"demo.mp4": {
			Name:      "demo.mp4",
			MediaType: "video/mp4",
			URI:       "https://example.com/demo.mp4",
		},
	}

	tool.embedRemoteAttachmentData(context.Background(), attachments)

	att := attachments["demo.mp4"]
	if att.Data == "" {
		t.Fatalf("expected video attachment to include inline data")
	}
	decoded, err := base64.StdEncoding.DecodeString(att.Data)
	if err != nil {
		t.Fatalf("failed to decode attachment data: %v", err)
	}
	if !bytes.Equal(decoded, payload) {
		t.Fatalf("expected attachment payload to match source bytes")
	}
	if att.MediaType != "video/mp4" {
		t.Fatalf("expected media type to remain video/mp4, got %s", att.MediaType)
	}
}

func TestSeedreamVideoToolEmbedRemoteAttachmentDataSkipsLargeAssets(t *testing.T) {
	payload := bytes.Repeat([]byte{0xff}, int(seedreamMaxInlineVideoBytes)+16)
	tool := &seedreamVideoTool{
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				resp := &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(bytes.NewReader(payload)),
				}
				resp.Header.Set("Content-Type", "video/mp4")
				return resp, nil
			}),
		},
	}
	attachments := map[string]ports.Attachment{
		"large.mp4": {
			Name:      "large.mp4",
			MediaType: "video/mp4",
			URI:       "https://example.com/large.mp4",
		},
	}

	tool.embedRemoteAttachmentData(context.Background(), attachments)

	if attachments["large.mp4"].Data != "" {
		t.Fatalf("expected large asset to skip inlining due to size limit")
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

	logPath := filepath.Join(logDir, "streaming.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read request log: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "[req:seedream-call] [request]") {
		t.Fatalf("expected request payload entry in log, got: %s", content)
	}
	if !strings.Contains(content, "[req:seedream-call] [response]") {
		t.Fatalf("expected response payload entry in log, got: %s", content)
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
	ctx := ports.WithAttachmentContext(context.Background(), map[string]ports.Attachment{
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
	ctx := ports.WithAttachmentContext(context.Background(), map[string]ports.Attachment{
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
