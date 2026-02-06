package llm

import (
	"context"
	"testing"

	"alex/internal/agent/ports"
	portsllm "alex/internal/agent/ports/llm"
	"alex/internal/parser"
)

type stubLLMClient struct {
	resp *ports.CompletionResponse
}

func (s stubLLMClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	return s.resp, nil
}

func (s stubLLMClient) Model() string { return "stub" }

func (s stubLLMClient) StreamComplete(
	ctx context.Context,
	req ports.CompletionRequest,
	callbacks ports.CompletionStreamCallbacks,
) (*ports.CompletionResponse, error) {
	if cb := callbacks.OnContentDelta; cb != nil {
		cb(ports.ContentDelta{Delta: s.resp.Content})
		cb(ports.ContentDelta{Final: true})
	}
	return s.resp, nil
}

func TestToolCallParsingClientAddsParsedToolCalls(t *testing.T) {
	t.Parallel()

	underlying := stubLLMClient{
		resp: &ports.CompletionResponse{
			Content: `<tool_call>{"name":"valid_tool","args":{"foo":"bar"}}</tool_call>`,
		},
	}

	client := WrapWithToolCallParsing(underlying, parser.New())
	resp, err := client.Complete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{{Role: "user", Content: "hi"}},
		Tools: []ports.ToolDefinition{{
			Name:        "valid_tool",
			Description: "ok",
			Parameters:  ports.ParameterSchema{Type: "object"},
		}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if resp == nil || len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %#v", resp)
	}
	if resp.ToolCalls[0].Name != "valid_tool" {
		t.Fatalf("unexpected tool call: %#v", resp.ToolCalls[0])
	}
	if resp.ToolCalls[0].Arguments["foo"] != "bar" {
		t.Fatalf("unexpected arguments: %#v", resp.ToolCalls[0].Arguments)
	}
}

func TestToolCallParsingClientSkipsUnknownToolCalls(t *testing.T) {
	t.Parallel()

	underlying := stubLLMClient{
		resp: &ports.CompletionResponse{
			Content: `<tool_call>{"name":"unknown_tool","args":{"foo":"bar"}}</tool_call>`,
		},
	}

	client := WrapWithToolCallParsing(underlying, parser.New())
	resp, err := client.Complete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{{Role: "user", Content: "hi"}},
		Tools: []ports.ToolDefinition{{
			Name:        "valid_tool",
			Description: "ok",
			Parameters:  ports.ParameterSchema{Type: "object"},
		}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp == nil {
		t.Fatalf("expected response")
	}
	if len(resp.ToolCalls) != 0 {
		t.Fatalf("expected tool calls to be filtered, got %#v", resp.ToolCalls)
	}
}

func TestToolCallParsingClientAppliesToStreamingResponses(t *testing.T) {
	t.Parallel()

	underlying := stubLLMClient{
		resp: &ports.CompletionResponse{
			Content: `<tool_call>{"name":"valid_tool","args":{"foo":"bar"}}</tool_call>`,
		},
	}

	client := WrapWithToolCallParsing(underlying, parser.New())
	streaming := portsllm.EnsureStreamingClient(client).(portsllm.StreamingLLMClient)

	resp, err := streaming.StreamComplete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{{Role: "user", Content: "hi"}},
		Tools: []ports.ToolDefinition{{
			Name:        "valid_tool",
			Description: "ok",
			Parameters:  ports.ParameterSchema{Type: "object"},
		}},
	}, ports.CompletionStreamCallbacks{})
	if err != nil {
		t.Fatalf("StreamComplete: %v", err)
	}

	if resp == nil || len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %#v", resp)
	}
}
