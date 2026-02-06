package web

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"alex/internal/agent/ports"
	"alex/internal/utils"
)

func TestWebFetchBuildResultCreatesAttachment(t *testing.T) {
	tool := &webFetch{}
	content := "# Headline\n\nKey insight here."

	result, err := tool.buildResult(context.Background(), "call-1", "https://news.example.com/update", content, false, nil)
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

	_, err := tool.buildResult(context.Background(), "tool-call-123", "https://example.com", "page content", false, "What matters?")
	if err != nil {
		t.Fatalf("buildResult returned error: %v", err)
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
	if !hasRequestLogEntry(entries, "tool-call-123", "request") {
		t.Fatalf("request payload not logged, entries: %#v", entries)
	}
	if !hasRequestLogEntry(entries, "tool-call-123", "response") {
		t.Fatalf("response payload not logged, entries: %#v", entries)
	}
}

type stubLLMClient struct {
	response *ports.CompletionResponse
}

func (s *stubLLMClient) Complete(_ context.Context, _ ports.CompletionRequest) (*ports.CompletionResponse, error) {
	return s.response, nil
}

func (*stubLLMClient) Model() string { return "stub-model" }

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
