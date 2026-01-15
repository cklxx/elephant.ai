package builtin

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"alex/internal/agent/ports"
)

type appPlugin struct {
	ID              string
	Name            string
	Description     string
	Capabilities    []string
	IntegrationNote string
	Sources         []string
}

type appsTool struct{}

func NewApps() ports.ToolExecutor {
	return &appsTool{}
}

func (t *appsTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "apps",
		Version:  "1.0.0",
		Category: "apps",
		Tags:     []string{"apps", "plugins", "open-source", "social", "news"},
	}
}

func (t *appsTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name: "apps",
		Description: `Manage built-in app plugins powered by open-source connectors.

Use this tool to list available app plugins, inspect a specific app connector, search by keyword,
or generate a usage plan for a target app.`,
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"action": {
					Type:        "string",
					Description: "list|show|search|use",
					Enum:        []any{"list", "show", "search", "use"},
				},
				"app": {
					Type:        "string",
					Description: "App plugin id for action=show or action=use.",
				},
				"query": {
					Type:        "string",
					Description: "Search query for action=search.",
				},
				"task": {
					Type:        "string",
					Description: "Usage intent for action=use.",
				},
			},
			Required: []string{"action"},
		},
	}
}

func (t *appsTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	_ = ctx
	action, _ := call.Arguments["action"].(string)
	action = strings.ToLower(strings.TrimSpace(action))
	plugins := appPluginCatalog()

	switch action {
	case "list":
		content := renderAppList(plugins)
		return &ports.ToolResult{CallID: call.ID, Content: content, Metadata: map[string]any{"count": len(plugins)}}, nil
	case "show":
		app, _ := call.Arguments["app"].(string)
		app = strings.ToLower(strings.TrimSpace(app))
		if app == "" {
			err := errors.New("app is required for action=show")
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		plugin, ok := findPlugin(plugins, app)
		if !ok {
			content := renderAppNotFound(app, plugins)
			err := fmt.Errorf("app plugin not found: %s", app)
			return &ports.ToolResult{CallID: call.ID, Content: content, Error: err}, nil
		}
		return &ports.ToolResult{CallID: call.ID, Content: renderAppDetail(plugin), Metadata: map[string]any{"app": plugin.ID}}, nil
	case "search":
		query, _ := call.Arguments["query"].(string)
		query = strings.ToLower(strings.TrimSpace(query))
		if query == "" {
			err := errors.New("query is required for action=search")
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		matches := searchPlugins(plugins, query)
		content := renderSearchResults(query, matches)
		return &ports.ToolResult{CallID: call.ID, Content: content, Metadata: map[string]any{"query": query, "count": len(matches)}}, nil
	case "use":
		app, _ := call.Arguments["app"].(string)
		app = strings.ToLower(strings.TrimSpace(app))
		task, _ := call.Arguments["task"].(string)
		task = strings.TrimSpace(task)
		if app == "" {
			err := errors.New("app is required for action=use")
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		if task == "" {
			err := errors.New("task is required for action=use")
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		plugin, ok := findPlugin(plugins, app)
		if !ok {
			content := renderAppNotFound(app, plugins)
			err := fmt.Errorf("app plugin not found: %s", app)
			return &ports.ToolResult{CallID: call.ID, Content: content, Error: err}, nil
		}
		content := renderUsagePlan(plugin, task)
		return &ports.ToolResult{CallID: call.ID, Content: content, Metadata: map[string]any{"app": plugin.ID, "task": task}}, nil
	default:
		err := fmt.Errorf("unsupported action %q (expected list|show|search|use)", action)
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
}

func appPluginCatalog() []appPlugin {
	plugins := []appPlugin{
		{
			ID:          "xiaohongshu",
			Name:        "小红书 (Xiaohongshu)",
			Description: "UGC 笔记/话题/榜单采集与内容分析。",
			Capabilities: []string{
				"话题/笔记搜索",
				"热榜与趋势观察",
				"笔记内容抽取",
			},
			IntegrationNote: "内置开源插件连接器，适合做内容洞察与素材整理。",
			Sources:         []string{"open-source plugin"},
		},
		{
			ID:          "wechat",
			Name:        "微信 (WeChat)",
			Description: "公众号内容抓取与文章结构化。",
			Capabilities: []string{
				"公众号搜索",
				"文章抓取",
				"内容摘要",
			},
			IntegrationNote: "内置开源插件连接器，聚焦公众号文章与运营素材。",
			Sources:         []string{"open-source plugin"},
		},
		{
			ID:          "douyin",
			Name:        "抖音 (Douyin)",
			Description: "短视频热榜与话题趋势。",
			Capabilities: []string{
				"热榜抓取",
				"话题趋势",
				"视频元数据解析",
			},
			IntegrationNote: "内置开源插件连接器，结合 hot 榜单与趋势观察。",
			Sources:         []string{"open-source plugin"},
		},
		{
			ID:          "weibo",
			Name:        "新浪微博 (Weibo)",
			Description: "热搜榜与话题监测。",
			Capabilities: []string{
				"热搜列表",
				"话题追踪",
				"舆情摘要",
			},
			IntegrationNote: "内置开源插件连接器，适合实时话题跟踪。",
			Sources:         []string{"open-source plugin"},
		},
		{
			ID:          "zhihu",
			Name:        "知乎 (Zhihu)",
			Description: "问答检索与高质量观点抽取。",
			Capabilities: []string{
				"问题搜索",
				"回答抓取",
				"观点聚合",
			},
			IntegrationNote: "内置开源插件连接器，面向知识型内容。",
			Sources:         []string{"open-source plugin"},
		},
		{
			ID:          "news",
			Name:        "新闻站点",
			Description: "新闻站点与媒体内容抓取。",
			Capabilities: []string{
				"新闻站点列表",
				"栏目抓取",
				"正文结构化",
			},
			IntegrationNote: "内置开源插件连接器，支持多站点统一采集。",
			Sources:         []string{"open-source plugin"},
		},
	}

	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].ID < plugins[j].ID
	})
	return plugins
}

func findPlugin(plugins []appPlugin, id string) (appPlugin, bool) {
	for _, plugin := range plugins {
		if plugin.ID == id {
			return plugin, true
		}
	}
	return appPlugin{}, false
}

func searchPlugins(plugins []appPlugin, query string) []appPlugin {
	matches := make([]appPlugin, 0, len(plugins))
	for _, plugin := range plugins {
		if strings.Contains(plugin.ID, query) || strings.Contains(strings.ToLower(plugin.Name), query) || strings.Contains(strings.ToLower(plugin.Description), query) {
			matches = append(matches, plugin)
		}
	}
	return matches
}

func renderAppList(plugins []appPlugin) string {
	var builder strings.Builder
	builder.WriteString("Available app plugins:\n\n")
	for _, plugin := range plugins {
		builder.WriteString(fmt.Sprintf("- `%s` — %s\n", plugin.ID, plugin.Description))
	}
	builder.WriteString("\nUse action=show to view plugin details or action=use to generate a usage plan.")
	return builder.String()
}

func renderAppDetail(plugin appPlugin) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("%s (%s)\n\n", plugin.Name, plugin.ID))
	builder.WriteString(plugin.Description)
	builder.WriteString("\n\nCapabilities:\n")
	for _, cap := range plugin.Capabilities {
		builder.WriteString(fmt.Sprintf("- %s\n", cap))
	}
	builder.WriteString("\nIntegration:\n")
	builder.WriteString(plugin.IntegrationNote)
	builder.WriteString("\n\nSource:\n")
	for _, src := range plugin.Sources {
		builder.WriteString(fmt.Sprintf("- %s\n", src))
	}
	return strings.TrimSpace(builder.String())
}

func renderAppNotFound(app string, plugins []appPlugin) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("App plugin not found: %q\n\n", app))
	builder.WriteString("Available plugins:\n")
	for _, plugin := range plugins {
		builder.WriteString(fmt.Sprintf("- %s\n", plugin.ID))
	}
	return strings.TrimSpace(builder.String())
}

func renderSearchResults(query string, matches []appPlugin) string {
	if len(matches) == 0 {
		return fmt.Sprintf("No app plugins matched %q.", query)
	}
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Matches for %q:\n\n", query))
	for _, plugin := range matches {
		builder.WriteString(fmt.Sprintf("- `%s` — %s\n", plugin.ID, plugin.Description))
	}
	builder.WriteString("\nUse action=show to inspect a plugin.")
	return strings.TrimSpace(builder.String())
}

func renderUsagePlan(plugin appPlugin, task string) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Usage plan for %s\n\n", plugin.Name))
	builder.WriteString("Goal:\n")
	builder.WriteString(fmt.Sprintf("- %s\n\n", task))
	builder.WriteString("Recommended plugin flow:\n")
	builder.WriteString("1) Validate scope and required keywords.\n")
	builder.WriteString("2) Use the app connector to fetch primary content streams.\n")
	builder.WriteString("3) Normalize fields (title, author, publish time, URL).\n")
	builder.WriteString("4) Summarize or classify based on the task intent.\n\n")
	builder.WriteString("Plugin capabilities:\n")
	for _, cap := range plugin.Capabilities {
		builder.WriteString(fmt.Sprintf("- %s\n", cap))
	}
	builder.WriteString("\nIntegration note:\n")
	builder.WriteString(plugin.IntegrationNote)
	return strings.TrimSpace(builder.String())
}
