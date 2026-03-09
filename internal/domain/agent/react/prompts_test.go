package react

import (
	"strings"
	"testing"
)

func TestAppendLongRunningToolReminder(t *testing.T) {
	calls := []ToolCall{
		{ID: "c1", Name: "web_search"},
		{ID: "c2", Name: "read_file"},
	}

	t.Run("no reminder under threshold", func(t *testing.T) {
		results := []ToolResult{
			{CallID: "c1", Content: "ok", Metadata: map[string]any{"_duration_ms": int64(30_000)}},
			{CallID: "c2", Content: "ok", Metadata: map[string]any{"_duration_ms": int64(100)}},
		}
		messages := []Message{
			{Content: "result1"},
			{Content: "result2"},
		}
		out := appendLongRunningToolReminder(calls, results, messages)
		for _, m := range out {
			if strings.Contains(m.Content, "<system-reminder>") {
				t.Fatalf("unexpected reminder in message: %s", m.Content)
			}
		}
	})

	t.Run("reminder injected above threshold", func(t *testing.T) {
		results := []ToolResult{
			{CallID: "c1", Content: "ok", Metadata: map[string]any{"_duration_ms": int64(150_000)}}, // 2.5 min
			{CallID: "c2", Content: "ok", Metadata: map[string]any{"_duration_ms": int64(50_000)}},
		}
		messages := []Message{
			{Content: "result1"},
			{Content: "result2"},
		}
		out := appendLongRunningToolReminder(calls, results, messages)
		if !strings.Contains(out[0].Content, "<system-reminder>") {
			t.Fatalf("expected reminder for slow tool, got: %s", out[0].Content)
		}
		if !strings.Contains(out[0].Content, `"web_search"`) {
			t.Fatalf("expected tool name in reminder, got: %s", out[0].Content)
		}
		if !strings.Contains(out[0].Content, "150s") {
			t.Fatalf("expected duration in reminder, got: %s", out[0].Content)
		}
		if strings.Contains(out[1].Content, "<system-reminder>") {
			t.Fatalf("unexpected reminder for fast tool: %s", out[1].Content)
		}
	})

	t.Run("empty messages unchanged", func(t *testing.T) {
		out := appendLongRunningToolReminder(nil, nil, nil)
		if out != nil {
			t.Fatalf("expected nil, got %v", out)
		}
	})

	t.Run("missing metadata no panic", func(t *testing.T) {
		results := []ToolResult{
			{CallID: "c1", Content: "ok"},
		}
		messages := []Message{
			{Content: "result1"},
		}
		out := appendLongRunningToolReminder(calls, results, messages)
		if strings.Contains(out[0].Content, "<system-reminder>") {
			t.Fatalf("unexpected reminder without metadata: %s", out[0].Content)
		}
	})
}
