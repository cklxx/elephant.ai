package builtin

import (
	"context"
	"strings"
	"testing"

	"alex/internal/agent/ports"
)

func TestAppsToolListAndShow(t *testing.T) {
	tool := NewApps()

	listResult, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"action": "list",
		},
	})
	if err != nil {
		t.Fatalf("execute list: %v", err)
	}
	if listResult.Error != nil {
		t.Fatalf("list returned error: %v", listResult.Error)
	}
	if !strings.Contains(listResult.Content, "xiaohongshu") {
		t.Fatalf("expected list to include xiaohongshu, got %q", listResult.Content)
	}

	showResult, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-2",
		Arguments: map[string]any{
			"action": "show",
			"app":    "xiaohongshu",
		},
	})
	if err != nil {
		t.Fatalf("execute show: %v", err)
	}
	if showResult.Error != nil {
		t.Fatalf("show returned error: %v", showResult.Error)
	}
	if !strings.Contains(showResult.Content, "Capabilities") {
		t.Fatalf("expected show to include capabilities, got %q", showResult.Content)
	}
}

func TestAppsToolSearchAndUse(t *testing.T) {
	tool := NewApps()

	searchResult, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-3",
		Arguments: map[string]any{
			"action": "search",
			"query":  "news",
		},
	})
	if err != nil {
		t.Fatalf("execute search: %v", err)
	}
	if searchResult.Error != nil {
		t.Fatalf("search returned error: %v", searchResult.Error)
	}
	if !strings.Contains(searchResult.Content, "news") {
		t.Fatalf("expected search to include news plugin, got %q", searchResult.Content)
	}

	useResult, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-4",
		Arguments: map[string]any{
			"action": "use",
			"app":    "weibo",
			"task":   "track trending topics for consumer electronics",
		},
	})
	if err != nil {
		t.Fatalf("execute use: %v", err)
	}
	if useResult.Error != nil {
		t.Fatalf("use returned error: %v", useResult.Error)
	}
	if !strings.Contains(useResult.Content, "Usage plan") {
		t.Fatalf("expected use to return usage plan, got %q", useResult.Content)
	}
}
