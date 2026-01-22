package builtin

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"alex/internal/agent/ports"
)

func TestResolveAttachmentBytesFromContextURI(t *testing.T) {
	payload := mustTestPNG(t, color.RGBA{R: 12, G: 34, B: 56, A: 255})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	attachments := map[string]ports.Attachment{
		"slide1.png": {
			Name:      "slide1.png",
			MediaType: "image/png",
			URI:       server.URL + "/slide1.png",
		},
	}

	ctx := ports.WithAttachmentContext(context.Background(), attachments, map[string]int{"slide1.png": 1})
	bytesOut, mimeType, err := resolveAttachmentBytes(ctx, "[slide1.png]", server.Client())
	if err != nil {
		t.Fatalf("resolveAttachmentBytes: %v", err)
	}
	if mimeType != "image/png" {
		t.Fatalf("expected image/png, got %q", mimeType)
	}
	if !bytes.Equal(bytesOut, payload) {
		t.Fatalf("expected payload to match server response")
	}
}

func TestResolveAttachmentBytesFromURL(t *testing.T) {
	payload := mustTestPNG(t, color.RGBA{R: 99, G: 88, B: 77, A: 255})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	bytesOut, mimeType, err := resolveAttachmentBytes(context.Background(), server.URL+"/frame.png", server.Client())
	if err != nil {
		t.Fatalf("resolveAttachmentBytes: %v", err)
	}
	if mimeType != "image/png" {
		t.Fatalf("expected image/png, got %q", mimeType)
	}
	if !bytes.Equal(bytesOut, payload) {
		t.Fatalf("expected payload to match server response")
	}
}

func mustTestPNG(t *testing.T, fill color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 3, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			img.SetRGBA(x, y, fill)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}
