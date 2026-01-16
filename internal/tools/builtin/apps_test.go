package builtin

import (
	"context"
	"strings"
	"testing"

	"alex/internal/agent/ports"
	"alex/internal/config"
)

func TestAppsToolListAndShow(t *testing.T) {
	tool := newAppsWithLoader(func(...config.Option) (config.FileConfig, string, error) {
		return config.FileConfig{}, "", nil
	})

	listResult, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{},
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
			"app": "xiaohongshu",
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
	tool := newAppsWithLoader(func(...config.Option) (config.FileConfig, string, error) {
		return config.FileConfig{}, "", nil
	})

	searchResult, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-3",
		Arguments: map[string]any{
			"query": "news",
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
			"app":    "weibo",
			"intent": "track trending topics for consumer electronics",
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

func TestAppsToolCustomPlugins(t *testing.T) {
	tool := newAppsWithLoader(func(...config.Option) (config.FileConfig, string, error) {
		return config.FileConfig{
			Apps: &config.AppsConfig{
				Plugins: []config.AppPluginConfig{
					{
						ID:              "internal-chat",
						Name:            "Internal Chat",
						Description:     "Internal chat connector.",
						Capabilities:    []string{"send", "receive"},
						IntegrationNote: "Requires internal auth.",
						Sources:         []string{"https://github.com/example/internal-chat"},
					},
				},
			},
		}, "", nil
	})

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-5",
		Arguments: map[string]any{
			"app": "internal-chat",
		},
	})
	if err != nil {
		t.Fatalf("execute show custom: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("show custom returned error: %v", result.Error)
	}
	if !strings.Contains(result.Content, "Internal Chat") {
		t.Fatalf("expected custom plugin to be rendered, got %q", result.Content)
	}
}
