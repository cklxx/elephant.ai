package builtin

import (
	"strings"
	"testing"

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
