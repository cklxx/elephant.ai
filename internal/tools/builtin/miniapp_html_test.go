package builtin

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"alex/internal/agent/ports"
)

type stubLLM struct {
	lastPrompt string
	messages   []ports.Message
}

func (s *stubLLM) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	s.lastPrompt = req.Messages[len(req.Messages)-1].Content
	s.messages = append([]ports.Message(nil), req.Messages...)
	return &ports.CompletionResponse{
		Content: "<html><body><div id=\"llm\">LLM HTML</div></body></html>",
	}, nil
}

func (*stubLLM) StreamComplete(ctx context.Context, req ports.CompletionRequest, callbacks ports.CompletionStreamCallbacks) (*ports.CompletionResponse, error) {
	return nil, nil
}

func (*stubLLM) Model() string { return "stub" }

func TestMiniAppHTMLBuildsAttachment(t *testing.T) {
	llm := &stubLLM{}
	tool := NewMiniAppHTMLWithLLM(llm)

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"prompt": "节奏敲击小游戏，击中闪烁的圆点得分",
			"title":  "Pulse Tap",
		},
	})
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if result == nil || len(result.Attachments) != 1 {
		t.Fatalf("expected one attachment, got %#v", result.Attachments)
	}
	var att ports.Attachment
	for _, a := range result.Attachments {
		att = a
		break
	}
	if att.MediaType != "text/html" || att.Format != "html" || att.PreviewProfile != "document.html" {
		t.Fatalf("unexpected attachment metadata: %#v", att)
	}
	if att.Data == "" {
		t.Fatalf("expected inline HTML payload")
	}
	decoded, err := base64.StdEncoding.DecodeString(att.Data)
	if err != nil {
		t.Fatalf("invalid base64 payload: %v", err)
	}
	if !strings.Contains(string(decoded), "LLM HTML") {
		t.Fatalf("expected LLM HTML content, got %s", decoded)
	}
	if !strings.Contains(att.Description, "节奏敲击") {
		t.Fatalf("expected description to include prompt, got %q", att.Description)
	}
	if result.Metadata["prefill_task"] == "" {
		t.Fatalf("expected prefill_task metadata")
	}
	if llm.lastPrompt == "" || len(llm.messages) == 0 {
		t.Fatalf("expected LLM to be invoked")
	}
	hasAssistant := false
	for _, msg := range llm.messages {
		if msg.Role == "assistant" {
			hasAssistant = true
			break
		}
	}
	if !hasAssistant {
		t.Fatalf("expected assistant prefill message in completion request")
	}
}
