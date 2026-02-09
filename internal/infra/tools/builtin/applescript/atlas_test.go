package applescript

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
)

func TestAtlas_MissingAction(t *testing.T) {
	tool := newAtlas(&mockRunner{output: "true"})
	res, err := tool.Execute(context.Background(), ports.ToolCall{ID: "1", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Error == nil {
		t.Fatal("expected error for missing action")
	}
}

func TestAtlas_UnsupportedAction(t *testing.T) {
	tool := newAtlas(&mockRunner{output: "true"})
	res, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "1",
		Arguments: map[string]any{"action": "delete_all"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Error == nil || !strings.Contains(res.Content, "unsupported action") {
		t.Fatalf("expected unsupported action error, got: %s", res.Content)
	}
}

func TestAtlas_ListConversations(t *testing.T) {
	r := &sequenceMock{
		outputs: []mockResult{
			{output: "true"},
			{output: "Chat about Go\nDesign review\nWeekly standup"},
		},
	}
	tool := newAtlas(r)
	res, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "1",
		Arguments: map[string]any{"action": "list_conversations"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Error != nil {
		t.Fatalf("unexpected tool error: %v", res.Error)
	}

	var items []string
	if err := json.Unmarshal([]byte(res.Content), &items); err != nil {
		t.Fatalf("failed to parse JSON: %v; content=%s", err, res.Content)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[0] != "Chat about Go" {
		t.Fatalf("unexpected first item: %s", items[0])
	}
}

func TestAtlas_ListConversations_Limit(t *testing.T) {
	r := &sequenceMock{
		outputs: []mockResult{
			{output: "true"},
			{output: "A\nB\nC\nD\nE"},
		},
	}
	tool := newAtlas(r)
	res, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "1",
		Arguments: map[string]any{"action": "list_conversations", "limit": float64(2)},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var items []string
	if err := json.Unmarshal([]byte(res.Content), &items); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
}

func TestAtlas_SwitchConversation_MissingName(t *testing.T) {
	r := &sequenceMock{outputs: []mockResult{{output: "true"}}}
	tool := newAtlas(r)
	res, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "1",
		Arguments: map[string]any{"action": "switch_conversation"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Error == nil {
		t.Fatal("expected error for missing conversation name")
	}
}

func TestAtlas_SwitchConversation_Success(t *testing.T) {
	r := &sequenceMock{
		outputs: []mockResult{
			{output: "true"},
			{output: "true"},
		},
	}
	tool := newAtlas(r)
	res, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "1",
		Arguments: map[string]any{"action": "switch_conversation", "conversation": "Design review"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Error != nil {
		t.Fatalf("unexpected tool error: %v", res.Error)
	}
	if !strings.Contains(res.Content, "Design review") {
		t.Fatalf("unexpected content: %s", res.Content)
	}
	// Verify the script contains the escaped conversation name
	if !strings.Contains(r.scripts[1], `"Design review"`) {
		t.Fatalf("script doesn't contain conversation name: %s", r.scripts[1])
	}
}

func TestAtlas_SwitchConversation_NotFound(t *testing.T) {
	r := &sequenceMock{
		outputs: []mockResult{
			{output: "true"},
			{output: "false"},
		},
	}
	tool := newAtlas(r)
	res, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "1",
		Arguments: map[string]any{"action": "switch_conversation", "conversation": "Nonexistent"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Error == nil || !strings.Contains(res.Content, "not found") {
		t.Fatalf("expected not found error, got: %s", res.Content)
	}
}

func TestAtlas_SwitchConversation_EscapedName(t *testing.T) {
	r := &sequenceMock{
		outputs: []mockResult{
			{output: "true"},
			{output: "true"},
		},
	}
	tool := newAtlas(r)
	_, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "1",
		Arguments: map[string]any{"action": "switch_conversation", "conversation": `Chat about "Go" and \stuff`},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify escaping: " → \" and \ → \\
	script := r.scripts[1]
	if !strings.Contains(script, `Chat about \"Go\" and \\stuff`) {
		t.Fatalf("name not properly escaped in script: %s", script)
	}
}

func TestAtlas_SwitchConversation_NewlineEscaped(t *testing.T) {
	r := &sequenceMock{
		outputs: []mockResult{
			{output: "true"},
			{output: "true"},
		},
	}
	tool := newAtlas(r)
	_, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "1",
		Arguments: map[string]any{"action": "switch_conversation", "conversation": "Line1\nLine2\rLine3"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	script := r.scripts[1]
	// Newlines and carriage returns must be escaped
	if strings.Contains(script, "\n") && !strings.Contains(script, `\n`) {
		t.Fatalf("newline not escaped in script: %s", script)
	}
	if strings.Contains(script, "\r") && !strings.Contains(script, `\r`) {
		t.Fatalf("carriage return not escaped in script: %s", script)
	}
}

func TestAtlas_ListConversations_EmptyOutput(t *testing.T) {
	r := &sequenceMock{
		outputs: []mockResult{
			{output: "true"},
			{output: ""},
		},
	}
	tool := newAtlas(r)
	res, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "1",
		Arguments: map[string]any{"action": "list_conversations"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Error != nil {
		t.Fatalf("unexpected tool error: %v", res.Error)
	}
	// Empty list should marshal to null
	if res.Content != "null" {
		t.Fatalf("expected null for empty output, got: %s", res.Content)
	}
}

func TestAtlas_ReadBookmarks(t *testing.T) {
	r := &sequenceMock{
		outputs: []mockResult{
			{output: "true"},
			{output: "Saved link 1\nSaved link 2"},
		},
	}
	tool := newAtlas(r)
	res, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "1",
		Arguments: map[string]any{"action": "read_bookmarks"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Error != nil {
		t.Fatalf("unexpected tool error: %v", res.Error)
	}

	var items []string
	if err := json.Unmarshal([]byte(res.Content), &items); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
}

func TestAtlas_ViewHistory(t *testing.T) {
	r := &sequenceMock{
		outputs: []mockResult{
			{output: "true"},
			{output: "Today\nYesterday\nLast week"},
		},
	}
	tool := newAtlas(r)
	res, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "1",
		Arguments: map[string]any{"action": "view_history"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Error != nil {
		t.Fatalf("unexpected tool error: %v", res.Error)
	}

	var items []string
	if err := json.Unmarshal([]byte(res.Content), &items); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
}

func TestAtlas_RunnerError(t *testing.T) {
	r := &sequenceMock{
		outputs: []mockResult{
			{output: "true"},
			{err: fmt.Errorf("osascript: access denied")},
		},
	}
	tool := newAtlas(r)
	res, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "1",
		Arguments: map[string]any{"action": "list_conversations"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Error == nil {
		t.Fatal("expected tool error from runner failure")
	}
}

func TestAtlas_NotRunning(t *testing.T) {
	tool := newAtlas(&mockRunner{output: "false"})
	res, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "1",
		Arguments: map[string]any{"action": "list_conversations"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Error == nil || !strings.Contains(res.Content, "not running") {
		t.Fatalf("expected not running error, got: %s", res.Content)
	}
}
