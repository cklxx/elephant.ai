package builtin

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"io"
	"strings"
	"testing"

	"alex/internal/agent/ports"
)

func TestPPTXFromImagesBuildsDeckFromAttachmentPlaceholders(t *testing.T) {
	img1 := mustPNG(t, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	img2 := mustPNG(t, color.RGBA{R: 200, G: 10, B: 50, A: 255})

	attachments := map[string]ports.Attachment{
		"slide1.png": {
			Name:      "slide1.png",
			MediaType: "image/png",
			Data:      base64.StdEncoding.EncodeToString(img1),
		},
		"slide2.png": {
			Name:      "slide2.png",
			MediaType: "image/png",
			Data:      base64.StdEncoding.EncodeToString(img2),
		},
	}

	ctx := ports.WithAttachmentContext(context.Background(), attachments, map[string]int{
		"slide1.png": 1,
		"slide2.png": 1,
	})

	tool := NewPPTXFromImages()
	result, err := tool.Execute(ctx, ports.ToolCall{
		ID:   "call-1",
		Name: "pptx_from_images",
		Arguments: map[string]any{
			"images": []string{"[slide1.png]", "[slide2.png]"},
		},
	})
	if err != nil {
		t.Fatalf("expected no execution error, got %v", err)
	}
	if result.Error != nil {
		t.Fatalf("expected no tool result error, got %v", result.Error)
	}

	att, ok := result.Attachments["deck.pptx"]
	if !ok {
		t.Fatalf("expected deck.pptx attachment to exist")
	}
	if att.MediaType != pptxFromImagesMediaType {
		t.Fatalf("expected media type %q, got %q", pptxFromImagesMediaType, att.MediaType)
	}

	pptxBytes, err := base64.StdEncoding.DecodeString(att.Data)
	if err != nil {
		t.Fatalf("decode attachment base64: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(pptxBytes), int64(len(pptxBytes)))
	if err != nil {
		t.Fatalf("open pptx zip: %v", err)
	}

	assertZipContains(t, zr, "ppt/slides/slide1.xml")
	assertZipContains(t, zr, "ppt/slides/slide2.xml")
	assertZipContains(t, zr, "ppt/slides/_rels/slide1.xml.rels")
	assertZipContains(t, zr, "ppt/slides/_rels/slide2.xml.rels")
	assertZipContains(t, zr, "ppt/media/image1.png")
	assertZipContains(t, zr, "ppt/media/image2.png")

	presentation := readZipFile(t, zr, "ppt/presentation.xml")
	if !strings.Contains(presentation, `r:id="rId7"`) || !strings.Contains(presentation, `r:id="rId8"`) {
		t.Fatalf("expected presentation.xml to reference rId7 and rId8 slides, got %q", presentation)
	}

	presentationRels := readZipFile(t, zr, "ppt/_rels/presentation.xml.rels")
	if !strings.Contains(presentationRels, `Target="slides/slide1.xml"`) || !strings.Contains(presentationRels, `Target="slides/slide2.xml"`) {
		t.Fatalf("expected presentation rels to include slide targets, got %q", presentationRels)
	}

	slide1Rels := readZipFile(t, zr, "ppt/slides/_rels/slide1.xml.rels")
	if !strings.Contains(slide1Rels, pptxSlideLayoutTarget) {
		t.Fatalf("expected slide1 rels to include layout target %q, got %q", pptxSlideLayoutTarget, slide1Rels)
	}
	if !strings.Contains(slide1Rels, `Target="../media/image1.png"`) {
		t.Fatalf("expected slide1 rels to include image target, got %q", slide1Rels)
	}

	contentTypes := readZipFile(t, zr, "[Content_Types].xml")
	if !strings.Contains(contentTypes, `/ppt/slides/slide2.xml`) {
		t.Fatalf("expected content types to include slide2 override, got %q", contentTypes)
	}
}

func TestPPTXFromImagesRejectsEmptyInput(t *testing.T) {
	tool := NewPPTXFromImages()
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "call-1",
		Name:      "pptx_from_images",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("expected no execution error, got %v", err)
	}
	if result.Error == nil {
		t.Fatalf("expected tool result error")
	}
}

func mustPNG(t *testing.T, fill color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.SetRGBA(x, y, fill)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func assertZipContains(t *testing.T, zr *zip.Reader, name string) {
	t.Helper()
	for _, file := range zr.File {
		if file.Name == name {
			return
		}
	}
	t.Fatalf("expected zip to contain %q", name)
}

func readZipFile(t *testing.T, zr *zip.Reader, name string) string {
	t.Helper()
	for _, file := range zr.File {
		if file.Name != name {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open %s: %v", name, err)
		}
		defer rc.Close()
		data, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		return string(data)
	}
	t.Fatalf("zip entry %q not found", name)
	return ""
}
