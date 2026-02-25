package llm

import (
	"testing"

	"alex/internal/shared/json"
)

// --- parseToolArguments ---

func TestParseToolArguments_ValidJSON(t *testing.T) {
	raw := jsonx.RawMessage(`{"name":"test","count":42}`)
	got := parseToolArguments(raw)
	if got == nil {
		t.Fatal("expected non-nil result")
	}
	if got["name"] != "test" {
		t.Fatalf("expected name=test, got %v", got["name"])
	}
	if got["count"] != float64(42) {
		t.Fatalf("expected count=42, got %v", got["count"])
	}
}

func TestParseToolArguments_DoubleEncodedJSON(t *testing.T) {
	// JSON string wrapping a JSON object
	raw := jsonx.RawMessage(`"{\"key\":\"value\"}"`)
	got := parseToolArguments(raw)
	if got == nil {
		t.Fatal("expected non-nil result for double-encoded JSON")
	}
	if got["key"] != "value" {
		t.Fatalf("expected key=value, got %v", got["key"])
	}
}

func TestParseToolArguments_Empty(t *testing.T) {
	if got := parseToolArguments(nil); got != nil {
		t.Fatalf("expected nil for nil input, got %v", got)
	}
	if got := parseToolArguments(jsonx.RawMessage{}); got != nil {
		t.Fatalf("expected nil for empty input, got %v", got)
	}
}

func TestParseToolArguments_InvalidJSON(t *testing.T) {
	raw := jsonx.RawMessage(`not valid json`)
	if got := parseToolArguments(raw); got != nil {
		t.Fatalf("expected nil for invalid JSON, got %v", got)
	}
}

func TestParseToolArguments_StringNotJSON(t *testing.T) {
	// A valid JSON string, but not containing a JSON object
	raw := jsonx.RawMessage(`"just a plain string"`)
	if got := parseToolArguments(raw); got != nil {
		t.Fatalf("expected nil for non-object string, got %v", got)
	}
}

func TestParseToolArguments_EmptyObject(t *testing.T) {
	raw := jsonx.RawMessage(`{}`)
	got := parseToolArguments(raw)
	if got == nil {
		t.Fatal("expected non-nil for empty object")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty map, got %v", got)
	}
}

// --- flattenOutputText ---

func TestFlattenOutputText_String(t *testing.T) {
	got := flattenOutputText("hello world")
	if got != "hello world" {
		t.Fatalf("expected passthrough, got %q", got)
	}
}

func TestFlattenOutputText_StringSlice(t *testing.T) {
	got := flattenOutputText([]any{"part1", "part2", "part3"})
	if got != "part1part2part3" {
		t.Fatalf("expected concatenated, got %q", got)
	}
}

func TestFlattenOutputText_MixedSlice(t *testing.T) {
	got := flattenOutputText([]any{"text", 42, "more"})
	if got != "textmore" {
		t.Fatalf("expected only strings concatenated, got %q", got)
	}
}

func TestFlattenOutputText_EmptySlice(t *testing.T) {
	got := flattenOutputText([]any{})
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestFlattenOutputText_Nil(t *testing.T) {
	if got := flattenOutputText(nil); got != "" {
		t.Fatalf("expected empty for nil, got %q", got)
	}
}

func TestFlattenOutputText_OtherType(t *testing.T) {
	if got := flattenOutputText(42); got != "" {
		t.Fatalf("expected empty for int, got %q", got)
	}
}

// --- parseResponsesOutput ---

func TestParseResponsesOutput_MessageWithText(t *testing.T) {
	resp := responsesResponse{
		Output: []responseOutputItem{
			{
				Type: "message",
				Content: []responseContent{
					{Type: "output_text", Text: "Hello "},
					{Type: "text", Text: "World"},
				},
			},
		},
	}
	content, toolCalls, thinking := parseResponsesOutput(resp)
	if content != "Hello World" {
		t.Fatalf("expected 'Hello World', got %q", content)
	}
	if len(toolCalls) != 0 {
		t.Fatalf("expected no tool calls, got %d", len(toolCalls))
	}
	if len(thinking.Parts) != 0 {
		t.Fatalf("expected no thinking, got %d parts", len(thinking.Parts))
	}
}

func TestParseResponsesOutput_FunctionCallItem(t *testing.T) {
	resp := responsesResponse{
		Output: []responseOutputItem{
			{
				Type:      "function_call",
				ID:        "call-1",
				Name:      "web_search",
				Arguments: jsonx.RawMessage(`{"query":"test"}`),
			},
		},
	}
	content, toolCalls, _ := parseResponsesOutput(resp)
	if content != "" {
		t.Fatalf("expected empty content, got %q", content)
	}
	if len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(toolCalls))
	}
	if toolCalls[0].ID != "call-1" {
		t.Fatalf("expected ID call-1, got %q", toolCalls[0].ID)
	}
	if toolCalls[0].Name != "web_search" {
		t.Fatalf("expected name web_search, got %q", toolCalls[0].Name)
	}
	if toolCalls[0].Arguments["query"] != "test" {
		t.Fatalf("expected query=test, got %v", toolCalls[0].Arguments)
	}
}

func TestParseResponsesOutput_ToolCallItem(t *testing.T) {
	resp := responsesResponse{
		Output: []responseOutputItem{
			{
				Type:      "tool_call",
				ID:        "tc-1",
				Name:      "shell_exec",
				Arguments: jsonx.RawMessage(`{"cmd":"ls"}`),
			},
		},
	}
	_, toolCalls, _ := parseResponsesOutput(resp)
	if len(toolCalls) != 1 || toolCalls[0].Name != "shell_exec" {
		t.Fatalf("expected tool_call parsed, got %v", toolCalls)
	}
}

func TestParseResponsesOutput_MessageWithToolCalls(t *testing.T) {
	resp := responsesResponse{
		Output: []responseOutputItem{
			{
				Type: "message",
				Content: []responseContent{
					{Type: "output_text", Text: "Let me search"},
				},
				ToolCalls: []responseToolCall{
					{
						ID:   "tc-msg-1",
						Type: "function",
						Function: struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						}{
							Name:      "web_search",
							Arguments: `{"q":"hello"}`,
						},
					},
				},
			},
		},
	}
	content, toolCalls, _ := parseResponsesOutput(resp)
	if content != "Let me search" {
		t.Fatalf("expected content, got %q", content)
	}
	if len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(toolCalls))
	}
	if toolCalls[0].ID != "tc-msg-1" || toolCalls[0].Name != "web_search" {
		t.Fatalf("unexpected tool call: %+v", toolCalls[0])
	}
}

func TestParseResponsesOutput_ReasoningItem(t *testing.T) {
	resp := responsesResponse{
		Output: []responseOutputItem{
			{
				Type: "reasoning",
				Content: []responseContent{
					{Type: "text", Text: "I need to think about this"},
				},
			},
			{
				Type: "message",
				Content: []responseContent{
					{Type: "output_text", Text: "Here is my answer"},
				},
			},
		},
	}
	content, _, thinking := parseResponsesOutput(resp)
	if content != "Here is my answer" {
		t.Fatalf("expected content, got %q", content)
	}
	if len(thinking.Parts) != 1 {
		t.Fatalf("expected 1 thinking part, got %d", len(thinking.Parts))
	}
	if thinking.Parts[0].Text != "I need to think about this" {
		t.Fatalf("expected thinking text, got %q", thinking.Parts[0].Text)
	}
}

func TestParseResponsesOutput_ThinkingItem(t *testing.T) {
	resp := responsesResponse{
		Output: []responseOutputItem{
			{
				Type: "thinking",
				Content: []responseContent{
					{Type: "text", Text: "Deep thoughts"},
				},
			},
		},
	}
	_, _, thinking := parseResponsesOutput(resp)
	if len(thinking.Parts) != 1 || thinking.Parts[0].Text != "Deep thoughts" {
		t.Fatalf("expected thinking parsed, got %+v", thinking)
	}
}

func TestParseResponsesOutput_MessageWithReasoningContent(t *testing.T) {
	resp := responsesResponse{
		Output: []responseOutputItem{
			{
				Type: "message",
				Content: []responseContent{
					{Type: "reasoning", Text: "Let me reason"},
					{Type: "output_text", Text: "The answer is 42"},
				},
			},
		},
	}
	content, _, thinking := parseResponsesOutput(resp)
	if content != "The answer is 42" {
		t.Fatalf("expected content, got %q", content)
	}
	if len(thinking.Parts) != 1 || thinking.Parts[0].Text != "Let me reason" {
		t.Fatalf("expected thinking from message content, got %+v", thinking)
	}
}

func TestParseResponsesOutput_FallbackToOutputText(t *testing.T) {
	resp := responsesResponse{
		Output:     nil,
		OutputText: "fallback text",
	}
	content, _, _ := parseResponsesOutput(resp)
	if content != "fallback text" {
		t.Fatalf("expected fallback to OutputText, got %q", content)
	}
}

func TestParseResponsesOutput_FallbackToOutputTextSlice(t *testing.T) {
	resp := responsesResponse{
		Output:     nil,
		OutputText: []any{"part1", "part2"},
	}
	content, _, _ := parseResponsesOutput(resp)
	if content != "part1part2" {
		t.Fatalf("expected fallback to flattened OutputText, got %q", content)
	}
}

func TestParseResponsesOutput_EmptyOutput(t *testing.T) {
	resp := responsesResponse{}
	content, toolCalls, thinking := parseResponsesOutput(resp)
	if content != "" {
		t.Fatalf("expected empty content, got %q", content)
	}
	if len(toolCalls) != 0 {
		t.Fatalf("expected no tool calls, got %d", len(toolCalls))
	}
	if len(thinking.Parts) != 0 {
		t.Fatalf("expected no thinking, got %d parts", len(thinking.Parts))
	}
}

func TestParseResponsesOutput_MixedOutputTypes(t *testing.T) {
	resp := responsesResponse{
		Output: []responseOutputItem{
			{
				Type: "reasoning",
				Content: []responseContent{
					{Type: "text", Text: "thinking step"},
				},
			},
			{
				Type: "message",
				Content: []responseContent{
					{Type: "output_text", Text: "answer"},
				},
			},
			{
				Type:      "function_call",
				ID:        "fc-1",
				Name:      "tool",
				Arguments: jsonx.RawMessage(`{"x":1}`),
			},
		},
	}
	content, toolCalls, thinking := parseResponsesOutput(resp)
	if content != "answer" {
		t.Fatalf("expected 'answer', got %q", content)
	}
	if len(toolCalls) != 1 || toolCalls[0].Name != "tool" {
		t.Fatalf("expected 1 tool call, got %v", toolCalls)
	}
	if len(thinking.Parts) != 1 {
		t.Fatalf("expected 1 thinking part, got %d", len(thinking.Parts))
	}
}

func TestParseResponsesOutput_CaseInsensitiveTypes(t *testing.T) {
	resp := responsesResponse{
		Output: []responseOutputItem{
			{
				Type: "  MESSAGE  ",
				Content: []responseContent{
					{Type: "  OUTPUT_TEXT  ", Text: "case test"},
				},
			},
		},
	}
	content, _, _ := parseResponsesOutput(resp)
	if content != "case test" {
		t.Fatalf("expected case-insensitive match, got %q", content)
	}
}

func TestParseResponsesOutput_ReasoningEmptyTextSkipped(t *testing.T) {
	resp := responsesResponse{
		Output: []responseOutputItem{
			{
				Type: "reasoning",
				Content: []responseContent{
					{Type: "text", Text: "   "},
					{Type: "text", Text: "actual thought"},
				},
			},
		},
	}
	_, _, thinking := parseResponsesOutput(resp)
	if len(thinking.Parts) != 1 {
		t.Fatalf("expected 1 thinking part (whitespace skipped), got %d", len(thinking.Parts))
	}
}
