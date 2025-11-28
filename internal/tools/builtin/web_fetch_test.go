package builtin

import (
	"strings"
	"testing"
)

func TestWebFetchBuildResultCreatesAttachment(t *testing.T) {
	tool := &webFetch{}
	content := "# Headline\n\nKey insight here."

	result, err := tool.buildResult("call-1", "https://news.example.com/update", content, false, nil)
	if err != nil {
		t.Fatalf("buildResult returned error: %v", err)
	}

	if len(result.Attachments) != 1 {
		t.Fatalf("expected single attachment, got %d", len(result.Attachments))
	}

	for name, att := range result.Attachments {
		if !strings.HasSuffix(name, ".md") {
			t.Fatalf("expected markdown attachment name, got %s", name)
		}
		if att.MediaType != "text/markdown" {
			t.Fatalf("expected markdown media type, got %s", att.MediaType)
		}
		if att.Data == "" || !strings.HasPrefix(att.URI, "data:text/markdown;base64,") {
			t.Fatalf("expected data URI payload, got %+v", att)
		}
		if att.Format != "markdown" {
			t.Fatalf("expected markdown format, got %s", att.Format)
		}
		if att.PreviewProfile != "document.markdown" {
			t.Fatalf("expected markdown preview profile, got %s", att.PreviewProfile)
		}
		if att.Source != "web_fetch" {
			t.Fatalf("expected web_fetch source, got %s", att.Source)
		}
	}
}
