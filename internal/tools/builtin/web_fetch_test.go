package builtin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"alex/internal/agent/ports"
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

func TestWebFetchAnalyzeLLMLogsRequestAndResponse(t *testing.T) {
	logDir := t.TempDir()
	t.Setenv("ALEX_REQUEST_LOG_DIR", logDir)

	tool := &webFetch{
		llmClient: &stubLLMClient{response: &ports.CompletionResponse{
			Content:  "analysis",
			Metadata: map[string]any{"request_id": "tool-call-123"},
		}},
	}

	_, err := tool.buildResult("tool-call-123", "https://example.com", "page content", false, "What matters?")
	if err != nil {
		t.Fatalf("buildResult returned error: %v", err)
	}

	logPath := filepath.Join(logDir, "streaming.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read request log: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "tool-call-123] [request]") {
		t.Fatalf("request payload not logged, content: %s", content)
	}
	if !strings.Contains(content, "tool-call-123] [response]") {
		t.Fatalf("response payload not logged, content: %s", content)
	}
}

type stubLLMClient struct {
	response *ports.CompletionResponse
}

func (s *stubLLMClient) Complete(_ context.Context, _ ports.CompletionRequest) (*ports.CompletionResponse, error) {
	return s.response, nil
}

func (*stubLLMClient) Model() string { return "stub-model" }
