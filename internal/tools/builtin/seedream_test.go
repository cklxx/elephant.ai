package builtin

import (
	"strings"
	"testing"

	arkm "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
)

func TestFormatSeedreamResponsePrefersPromptForDescriptions(t *testing.T) {
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

	if !strings.Contains(content, prompt) {
		t.Fatalf("expected content to include prompt, got %q", content)
	}
	if !strings.Contains(content, "[doubao_seedream-3_0.png]") {
		t.Fatalf("expected content to include placeholder listing, got %q", content)
	}

	if metadata["description"] != prompt {
		t.Fatalf("expected metadata description to equal prompt, got %#v", metadata["description"])
	}

	placeholder := "doubao_seedream-3_0.png"
	att, ok := attachments[placeholder]
	if !ok {
		t.Fatalf("expected attachment %q to exist", placeholder)
	}
	if att.Description != prompt {
		t.Fatalf("expected attachment description to be prompt, got %q", att.Description)
	}
}

func TestFormatSeedreamResponseFallsBackToDescriptor(t *testing.T) {
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
	if !strings.Contains(content, "[seedream_0.png]") {
		t.Fatalf("expected content to list placeholder names, got %q", content)
	}

	if metadata["description"] != descriptor {
		t.Fatalf("expected metadata description to equal descriptor, got %#v", metadata["description"])
	}

	placeholder := "seedream_0.png"
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

	actual, err := normalizeSeedreamInitImage(value)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	img, ok := actual.(*arkm.Image)
	if !ok {
		t.Fatalf("expected arkm.Image, got %T", actual)
	}
	if img.B64Json == nil || *img.B64Json != base64 {
		t.Fatalf("expected payload %q, got %+v", base64, img.B64Json)
	}
	if img.Url != nil {
		t.Fatalf("expected URL to be nil for data URI")
	}
}

func TestNormalizeSeedreamInitImageHTTPURL(t *testing.T) {
	raw := "https://example.com/seed.png"
	actual, err := normalizeSeedreamInitImage(raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	img, ok := actual.(*arkm.Image)
	if !ok {
		t.Fatalf("expected arkm.Image, got %T", actual)
	}
	if img.Url == nil || *img.Url != raw {
		t.Fatalf("expected URL %q, got %+v", raw, img.Url)
	}
	if img.B64Json != nil {
		t.Fatalf("expected B64Json to be nil for URL input")
	}
}

func TestNormalizeSeedreamInitImagePlainBase64(t *testing.T) {
	payload := "ZXhhbXBsZQ=="
	actual, err := normalizeSeedreamInitImage(payload)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	img, ok := actual.(*arkm.Image)
	if !ok {
		t.Fatalf("expected arkm.Image, got %T", actual)
	}
	if img.B64Json == nil || *img.B64Json != payload {
		t.Fatalf("expected payload %q, got %+v", payload, img.B64Json)
	}
}

func TestNormalizeSeedreamInitImageRejectsBadScheme(t *testing.T) {
	if _, err := normalizeSeedreamInitImage("ftp://example.com/image.png"); err == nil {
		t.Fatalf("expected error for unsupported scheme")
	}
}

func TestNormalizeSeedreamInitImageRequiresPayload(t *testing.T) {
	if _, err := normalizeSeedreamInitImage("data:image/png;base64,"); err == nil {
		t.Fatalf("expected error for empty payload")
	}
}
