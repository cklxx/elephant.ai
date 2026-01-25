package builtin

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/config"
)

type appPlugin struct {
	ID              string
	Name            string
	Description     string
	Capabilities    []string
	IntegrationNote string
	Sources         []string
}

type appsTool struct {
	loadFileConfig func(...config.Option) (config.FileConfig, string, error)
}

func NewApps() tools.ToolExecutor {
	return newAppsWithLoader(config.LoadFileConfig)
}

func newAppsWithLoader(loader func(...config.Option) (config.FileConfig, string, error)) tools.ToolExecutor {
	return &appsTool{loadFileConfig: loader}
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

Call with no parameters to list app plugins. Use app to inspect a connector, query to search by
keyword, or app + intent to generate a usage plan. Custom plugins can be added via apps.plugins
in ~/.alex/config.yaml.`,
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"app": {
					Type:        "string",
					Description: "App plugin id for details or usage planning.",
				},
				"query": {
					Type:        "string",
					Description: "Search query for matching app plugins.",
				},
				"intent": {
					Type:        "string",
					Description: "Usage intent for a target app plugin.",
				},
			},
		},
	}
}

func (t *appsTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	_ = ctx
	plugins, err := t.loadPlugins()
	if err != nil {
		wrapped := fmt.Errorf("load app plugins: %w", err)
		return &ports.ToolResult{CallID: call.ID, Content: wrapped.Error(), Error: wrapped}, nil
	}

	app, _ := call.Arguments["app"].(string)
	app = strings.ToLower(strings.TrimSpace(app))
	query, _ := call.Arguments["query"].(string)
	query = strings.ToLower(strings.TrimSpace(query))
	intent, _ := call.Arguments["intent"].(string)
	intent = strings.TrimSpace(intent)

	if query != "" {
		if app != "" || intent != "" {
			err := errors.New("query cannot be combined with app or intent")
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		matches := searchPlugins(plugins, query)
		content := renderSearchResults(query, matches)
		return &ports.ToolResult{CallID: call.ID, Content: content, Metadata: map[string]any{"query": query, "count": len(matches)}}, nil
	}

	if app == "" {
		if intent != "" {
			err := errors.New("intent requires app")
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		content := renderAppList(plugins)
		return &ports.ToolResult{CallID: call.ID, Content: content, Metadata: map[string]any{"count": len(plugins)}}, nil
	}

	plugin, ok := findPlugin(plugins, app)
	if !ok {
		content := renderAppNotFound(app, plugins)
		err := fmt.Errorf("app plugin not found: %s", app)
		return &ports.ToolResult{CallID: call.ID, Content: content, Error: err}, nil
	}

	if intent == "" {
		return &ports.ToolResult{CallID: call.ID, Content: renderAppDetail(plugin), Metadata: map[string]any{"app": plugin.ID}}, nil
	}
	content := renderUsagePlan(plugin, intent)
	return &ports.ToolResult{CallID: call.ID, Content: content, Metadata: map[string]any{"app": plugin.ID, "intent": intent}}, nil
}

func (t *appsTool) loadPlugins() ([]appPlugin, error) {
	plugins := appPluginCatalog()
	if t.loadFileConfig == nil {
		return plugins, nil
	}
	cfg, _, err := t.loadFileConfig()
	if err != nil {
		return nil, err
	}
	if cfg.Apps == nil || len(cfg.Apps.Plugins) == 0 {
		return plugins, nil
	}
	custom := make([]appPlugin, 0, len(cfg.Apps.Plugins))
	for _, plugin := range cfg.Apps.Plugins {
		custom = append(custom, appPluginFromConfig(plugin))
	}
	return mergePlugins(plugins, custom), nil
}

func appPluginFromConfig(cfg config.AppPluginConfig) appPlugin {
	return appPlugin{
		ID:              strings.ToLower(strings.TrimSpace(cfg.ID)),
		Name:            strings.TrimSpace(cfg.Name),
		Description:     strings.TrimSpace(cfg.Description),
		Capabilities:    trimStringList(cfg.Capabilities),
		IntegrationNote: strings.TrimSpace(cfg.IntegrationNote),
		Sources:         trimStringList(cfg.Sources),
	}
}

func mergePlugins(base []appPlugin, extras []appPlugin) []appPlugin {
	if len(extras) == 0 {
		return base
	}
	merged := make(map[string]appPlugin, len(base)+len(extras))
	for _, plugin := range base {
		merged[plugin.ID] = plugin
	}
	for _, plugin := range extras {
		id := strings.ToLower(strings.TrimSpace(plugin.ID))
		if id == "" {
			continue
		}
		plugin.ID = id
		if strings.TrimSpace(plugin.Name) == "" {
			plugin.Name = id
		}
		merged[id] = plugin
	}
	out := make([]appPlugin, 0, len(merged))
	for _, plugin := range merged {
		out = append(out, plugin)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func trimStringList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	trimmed := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		trimmed = append(trimmed, value)
	}
	return trimmed
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
				"笔记详情抽取",
			},
			IntegrationNote: "内置开源插件连接器，适合做内容洞察与素材整理。",
			Sources: []string{
				"https://github.com/NanmiCoder/MediaCrawler",
				"https://github.com/xisuo67/XHS-Spider",
			},
		},
		{
			ID:          "wechat",
			Name:        "微信 (WeChat)",
			Description: "聊天会话收发与消息列表管理。",
			Capabilities: []string{
				"接收消息",
				"回复消息",
				"消息列表/会话拉取",
			},
			IntegrationNote: "内置开源插件连接器，面向聊天场景，需登录授权。",
			Sources: []string{
				"https://github.com/wechaty/wechaty",
				"https://github.com/eatmoreapple/openwechat",
			},
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
			Sources: []string{
				"https://github.com/Evil0ctal/Douyin_TikTok_Download_API",
				"https://github.com/loadchange/amemv-crawler",
			},
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
			Sources: []string{
				"https://github.com/SpiderClub/weibospider",
				"https://github.com/dataabc/weibo-crawler",
			},
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
			Sources: []string{
				"https://github.com/7sDream/zhihu-oauth",
			},
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
			Sources: []string{
				"https://github.com/fhamborg/news-please",
				"https://github.com/codelucas/newspaper",
			},
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
	builder.WriteString("\nUse app to view details, query to search, or app + intent to generate a usage plan.")
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
	builder.WriteString("\nUse app to inspect a plugin.")
	return strings.TrimSpace(builder.String())
}

func renderUsagePlan(plugin appPlugin, intent string) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Usage plan for %s\n\n", plugin.Name))
	builder.WriteString("Goal:\n")
	builder.WriteString(fmt.Sprintf("- %s\n\n", intent))
	builder.WriteString("Recommended plugin flow:\n")
	builder.WriteString("1) Validate scope and required keywords.\n")
	builder.WriteString("2) Use the app connector to fetch primary content streams.\n")
	builder.WriteString("3) Normalize fields (title, author, publish time, URL).\n")
	builder.WriteString("4) Summarize or classify based on the usage intent.\n\n")
	builder.WriteString("Plugin capabilities:\n")
	for _, cap := range plugin.Capabilities {
		builder.WriteString(fmt.Sprintf("- %s\n", cap))
	}
	builder.WriteString("\nIntegration note:\n")
	builder.WriteString(plugin.IntegrationNote)
	return strings.TrimSpace(builder.String())
}
