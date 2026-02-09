package applescript

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
)

// mockRunner captures the script sent to RunScript and returns preconfigured output.
type mockRunner struct {
	script string
	output string
	err    error
}

func (m *mockRunner) RunScript(_ context.Context, script string) (string, error) {
	m.script = script
	return m.output, m.err
}

func TestChrome_MissingAction(t *testing.T) {
	tool := newChrome(&mockRunner{output: "true"})
	res, err := tool.Execute(context.Background(), ports.ToolCall{ID: "1", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Error == nil {
		t.Fatal("expected error for missing action")
	}
}

func TestChrome_UnsupportedAction(t *testing.T) {
	tool := newChrome(&mockRunner{output: "true"})
	res, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "1",
		Arguments: map[string]any{"action": "fly"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Error == nil || !strings.Contains(res.Content, "unsupported action") {
		t.Fatalf("expected unsupported action error, got: %s", res.Content)
	}
}

func TestChrome_ListTabs(t *testing.T) {
	// mockRunner returns "true" for isAppRunning, then tab data for list_tabs.
	// We use a sequencing mock.
	r := &sequenceMock{
		outputs: []mockResult{
			{output: "true"},
			{output: "1\t1\thttps://example.com\tExample\n1\t2\thttps://go.dev\tGo"},
		},
	}
	tool := newChrome(r)
	res, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "1",
		Arguments: map[string]any{"action": "list_tabs"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Error != nil {
		t.Fatalf("unexpected tool error: %v", res.Error)
	}

	var tabs []map[string]any
	if err := json.Unmarshal([]byte(res.Content), &tabs); err != nil {
		t.Fatalf("failed to parse JSON: %v; content=%s", err, res.Content)
	}
	if len(tabs) != 2 {
		t.Fatalf("expected 2 tabs, got %d", len(tabs))
	}
	if tabs[0]["url"] != "https://example.com" {
		t.Fatalf("unexpected url: %v", tabs[0]["url"])
	}
	if tabs[1]["title"] != "Go" {
		t.Fatalf("unexpected title: %v", tabs[1]["title"])
	}
}

func TestChrome_ListTabs_Limit(t *testing.T) {
	lines := ""
	for i := 1; i <= 10; i++ {
		lines += fmt.Sprintf("1\t%d\thttps://example.com/%d\tPage %d\n", i, i, i)
	}
	r := &sequenceMock{
		outputs: []mockResult{
			{output: "true"},
			{output: strings.TrimSpace(lines)},
		},
	}
	tool := newChrome(r)
	res, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "1",
		Arguments: map[string]any{"action": "list_tabs", "limit": float64(3)},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var tabs []map[string]any
	if err := json.Unmarshal([]byte(res.Content), &tabs); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if len(tabs) != 3 {
		t.Fatalf("expected 3 tabs, got %d", len(tabs))
	}
}

func TestChrome_ActiveTab(t *testing.T) {
	r := &sequenceMock{
		outputs: []mockResult{
			{output: "true"},
			{output: "https://example.com\tExample Page"},
		},
	}
	tool := newChrome(r)
	res, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "1",
		Arguments: map[string]any{"action": "active_tab"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Error != nil {
		t.Fatalf("unexpected tool error: %v", res.Error)
	}

	var result map[string]string
	if err := json.Unmarshal([]byte(res.Content), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if result["url"] != "https://example.com" {
		t.Fatalf("unexpected url: %s", result["url"])
	}
	if result["title"] != "Example Page" {
		t.Fatalf("unexpected title: %s", result["title"])
	}
}

func TestChrome_OpenURL_MissingURL(t *testing.T) {
	r := &sequenceMock{outputs: []mockResult{{output: "true"}}}
	tool := newChrome(r)
	res, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "1",
		Arguments: map[string]any{"action": "open_url"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Error == nil {
		t.Fatal("expected error for missing url")
	}
}

func TestChrome_OpenURL_InvalidScheme(t *testing.T) {
	r := &sequenceMock{outputs: []mockResult{{output: "true"}}}
	tool := newChrome(r)
	res, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "1",
		Arguments: map[string]any{"action": "open_url", "url": "javascript:alert(1)"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Error == nil || !strings.Contains(res.Content, "only http/https") {
		t.Fatalf("expected scheme error, got: %s", res.Content)
	}
}

func TestChrome_OpenURL_Success(t *testing.T) {
	r := &sequenceMock{
		outputs: []mockResult{
			{output: "true"},
			{output: ""},
		},
	}
	tool := newChrome(r)
	res, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "1",
		Arguments: map[string]any{"action": "open_url", "url": "https://example.com"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Error != nil {
		t.Fatalf("unexpected tool error: %v", res.Error)
	}
	if !strings.Contains(res.Content, "Opened") {
		t.Fatalf("unexpected content: %s", res.Content)
	}
	// Verify the script contains the escaped URL
	if !strings.Contains(r.scripts[1], `"https://example.com"`) {
		t.Fatalf("script doesn't contain URL: %s", r.scripts[1])
	}
}

func TestChrome_SwitchTab_MissingIndices(t *testing.T) {
	r := &sequenceMock{outputs: []mockResult{{output: "true"}}}
	tool := newChrome(r)
	res, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "1",
		Arguments: map[string]any{"action": "switch_tab", "window_index": float64(1)},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Error == nil {
		t.Fatal("expected error for missing tab_index")
	}
}

func TestChrome_SwitchTab_Success(t *testing.T) {
	r := &sequenceMock{
		outputs: []mockResult{
			{output: "true"},
			{output: ""},
		},
	}
	tool := newChrome(r)
	res, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "1",
		Arguments: map[string]any{"action": "switch_tab", "window_index": float64(1), "tab_index": float64(3)},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Error != nil {
		t.Fatalf("unexpected tool error: %v", res.Error)
	}
	if !strings.Contains(res.Content, "window 1, tab 3") {
		t.Fatalf("unexpected content: %s", res.Content)
	}
}

func TestChrome_Navigate_Success(t *testing.T) {
	r := &sequenceMock{
		outputs: []mockResult{
			{output: "true"},
			{output: ""},
		},
	}
	tool := newChrome(r)
	res, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "1",
		Arguments: map[string]any{
			"action":       "navigate",
			"window_index": float64(2),
			"tab_index":    float64(1),
			"url":          "https://go.dev",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Error != nil {
		t.Fatalf("unexpected tool error: %v", res.Error)
	}
	if !strings.Contains(r.scripts[1], `"https://go.dev"`) {
		t.Fatalf("script doesn't contain URL: %s", r.scripts[1])
	}
}

func TestChrome_CloseTab_Success(t *testing.T) {
	r := &sequenceMock{
		outputs: []mockResult{
			{output: "true"},
			{output: ""},
		},
	}
	tool := newChrome(r)
	res, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "1",
		Arguments: map[string]any{"action": "close_tab", "window_index": float64(1), "tab_index": float64(2)},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Error != nil {
		t.Fatalf("unexpected tool error: %v", res.Error)
	}
	if !strings.Contains(r.scripts[1], "close tab 2 of window 1") {
		t.Fatalf("unexpected script: %s", r.scripts[1])
	}
}

func TestChrome_RunnerError(t *testing.T) {
	r := &sequenceMock{
		outputs: []mockResult{
			{output: "true"},
			{err: fmt.Errorf("osascript: Chrome got an error")},
		},
	}
	tool := newChrome(r)
	res, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "1",
		Arguments: map[string]any{"action": "active_tab"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Error == nil {
		t.Fatal("expected tool error from runner failure")
	}
}

func TestChrome_NotRunning(t *testing.T) {
	tool := newChrome(&mockRunner{output: "false"})
	res, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "1",
		Arguments: map[string]any{"action": "list_tabs"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Error == nil || !strings.Contains(res.Content, "not running") {
		t.Fatalf("expected not running error, got: %s", res.Content)
	}
}

func TestChrome_ListTabs_EmptyOutput(t *testing.T) {
	r := &sequenceMock{
		outputs: []mockResult{
			{output: "true"},
			{output: ""},
		},
	}
	tool := newChrome(r)
	res, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "1",
		Arguments: map[string]any{"action": "list_tabs"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Error != nil {
		t.Fatalf("unexpected tool error: %v", res.Error)
	}
	// Should return null JSON (empty slice marshals to null)
	if res.Content != "null" {
		t.Fatalf("expected null for empty output, got: %s", res.Content)
	}
}

func TestChrome_SwitchTab_ZeroIndex(t *testing.T) {
	r := &sequenceMock{outputs: []mockResult{{output: "true"}}}
	tool := newChrome(r)
	res, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "1",
		Arguments: map[string]any{"action": "switch_tab", "window_index": float64(0), "tab_index": float64(1)},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Error == nil {
		t.Fatal("expected error for zero window_index")
	}
}

func TestChrome_EscapeURLWithQuotes(t *testing.T) {
	r := &sequenceMock{
		outputs: []mockResult{
			{output: "true"},
			{output: ""},
		},
	}
	tool := newChrome(r)
	_, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "1",
		Arguments: map[string]any{"action": "open_url", "url": `https://example.com/search?q="hello"`},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify quotes are escaped in the AppleScript
	if !strings.Contains(r.scripts[1], `\"hello\"`) {
		t.Fatalf("quotes not escaped in script: %s", r.scripts[1])
	}
}

// sequenceMock returns different outputs for successive RunScript calls.
type sequenceMock struct {
	outputs []mockResult
	call    int
	scripts []string
}

type mockResult struct {
	output string
	err    error
}

func (m *sequenceMock) RunScript(_ context.Context, script string) (string, error) {
	m.scripts = append(m.scripts, script)
	idx := m.call
	m.call++
	if idx >= len(m.outputs) {
		return "", fmt.Errorf("unexpected call %d", idx)
	}
	return m.outputs[idx].output, m.outputs[idx].err
}
