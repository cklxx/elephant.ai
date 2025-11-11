package builtin

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"alex/internal/agent/ports"

	arkm "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
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
	descriptor := "Seedream 3.0 text-to-image"

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

	descriptor := "Seedream 3.0 text-to-image"

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
