package builtin

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
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

	ctx := WithAllowLocalFetch(context.Background())
	ctx = ports.WithAttachmentContext(ctx, attachments, map[string]int{"slide1.png": 1})
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

	bytesOut, mimeType, err := resolveAttachmentBytes(WithAllowLocalFetch(context.Background()), server.URL+"/frame.png", server.Client())
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

func TestResolveAttachmentBytesPrefersResponseContentType(t *testing.T) {
	payload := mustTestJPEG(t, color.RGBA{R: 201, G: 18, B: 77, A: 255})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	attachments := map[string]ports.Attachment{
		"frame.png": {
			Name:      "frame.png",
			MediaType: "image/png",
			URI:       server.URL + "/frame.jpg",
		},
	}

	ctx := WithAllowLocalFetch(context.Background())
	ctx = ports.WithAttachmentContext(ctx, attachments, map[string]int{"frame.png": 1})
	bytesOut, mimeType, err := resolveAttachmentBytes(ctx, "[frame.png]", server.Client())
	if err != nil {
		t.Fatalf("resolveAttachmentBytes: %v", err)
	}
	if mimeType != "image/jpeg" {
		t.Fatalf("expected image/jpeg, got %q", mimeType)
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

func mustTestJPEG(t *testing.T, fill color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 3, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			img.SetRGBA(x, y, fill)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}
