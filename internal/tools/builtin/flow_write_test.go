package builtin

import (
	"context"
	"encoding/base64"
	"testing"

	"alex/internal/agent/ports"
)

type flowWriteStubLLM struct {
	lastReq ports.CompletionRequest
}

func (s *flowWriteStubLLM) Complete(_ context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	s.lastReq = req
	return &ports.CompletionResponse{
		Content: "rewritten draft content",
	}, nil
}

func (s *flowWriteStubLLM) Model() string { return "stub" }

func TestFlowWriteExecutesWithAction(t *testing.T) {
	llm := &flowWriteStubLLM{}
	tool := NewFlowWrite(llm)

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"action": "polish",
			"draft":  "原稿内容",
			"notes":  "保持语气",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(llm.lastReq.Messages) == 0 {
		t.Fatalf("expected LLM to be invoked")
	}
	if result.Attachments["flow_write.txt"].Data == "" {
		t.Fatalf("expected rewritten attachment to be populated")
	}
	data, _ := base64.StdEncoding.DecodeString(result.Attachments["flow_write.txt"].Data)
	if string(data) != "rewritten draft content" {
		t.Fatalf("unexpected attachment content: %s", data)
	}
	if result.Attachments["draft.txt"].Data == "" {
		t.Fatalf("expected original draft attachment")
	}
	if result.Metadata["action"] != "polish" {
		t.Fatalf("expected metadata action=polish, got %v", result.Metadata["action"])
	}
}

func TestFlowWriteRequiresInput(t *testing.T) {
	tool := NewFlowWrite(&flowWriteStubLLM{})

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "call-2",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Error == nil {
		t.Fatalf("expected validation error when inputs missing")
	}
}
