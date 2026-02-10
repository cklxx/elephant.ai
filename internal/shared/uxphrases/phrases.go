// Package uxphrases provides friendly Chinese status phrases for tool activity
// display. It is used by both the Lark progress listeners and the hooks bridge
// to present warm, human-readable status lines instead of raw tool names.
package uxphrases

import "strings"

// phraseGroup maps a category of tools to a pool of friendly phrases.
type phraseGroup struct {
	phrases []string
	matchFn func(name string) bool
}

// ThinkingPhrases are used when no tool is active (pure LLM thinking).
var ThinkingPhrases = []string{"在思考…", "在酝酿…", "在构思…", "在琢磨…"}

// SummarizingPhrases are used when all tools have completed (wrapping up).
var SummarizingPhrases = []string{"在整理…", "在总结…", "在梳理…"}

// DefaultPhrases are used when no tool group matches.
var DefaultPhrases = []string{"在处理…", "在分析…", "在洞察…"}

// phraseGroups is an ordered list of tool categories. The first match wins.
var phraseGroups = []phraseGroup{
	{
		phrases: []string{"在搜索…", "在探索…", "在挖掘…"},
		matchFn: func(n string) bool {
			return hasAnyPrefix(n, "web_search", "web_fetch", "tavily", "search_web")
		},
	},
	{
		phrases: []string{"在翻阅…", "在研读…", "在查阅…"},
		matchFn: func(n string) bool {
			return hasAnyPrefix(n, "read_file", "list_dir", "search_file", "list_files",
				"read", "glob", "grep", "find_file", "search_code", "list_directory",
				"view_file", "view_source")
		},
	},
	{
		phrases: []string{"在撰写…", "在书写…", "在落笔…"},
		matchFn: func(n string) bool {
			return hasAnyPrefix(n, "write_file", "replace_in_file", "create_file",
				"write", "edit", "insert_text", "apply_diff", "patch_file")
		},
	},
	{
		phrases: []string{"在运算…", "在执行…", "在实验…"},
		matchFn: func(n string) bool {
			return hasAnyPrefix(n, "shell_exec", "execute_code", "run_command",
				"bash", "terminal", "exec")
		},
	},
	{
		phrases: []string{"在浏览…", "在查看…", "在观察…"},
		matchFn: func(n string) bool {
			return hasAnyPrefix(n, "browser_", "navigate", "screenshot",
				"click", "scroll", "type_text")
		},
	},
	{
		phrases: []string{"在回忆…", "在追溯…", "在检索…"},
		matchFn: func(n string) bool {
			return hasAnyPrefix(n, "memory_search", "memory_get", "memory_save",
				"recall", "remember")
		},
	},
	{
		phrases: []string{"在创作…", "在绘制…", "在构图…"},
		matchFn: func(n string) bool {
			return hasAnyPrefix(n, "text_to_image", "seedream", "generate_image",
				"image_gen", "dall_e", "dalle")
		},
	},
	{
		phrases: []string{"在联络…", "在查询…", "在协调…"},
		matchFn: func(n string) bool {
			return hasAnyPrefix(n, "lark_", "feishu_", "send_lark", "get_lark")
		},
	},
	{
		phrases: []string{"在规划…", "在梳理…", "在分析…"},
		matchFn: func(n string) bool {
			return hasAnyPrefix(n, "plan", "clarify")
		},
	},
	{
		phrases: []string{"在深入…", "在调研…", "在拆解…"},
		matchFn: func(n string) bool {
			return hasAnyPrefix(n, "subagent", "explore", "sub_agent",
				"task", "delegate")
		},
	},
}

// ToolPhrase returns a friendly Chinese status phrase for the given tool name.
// The selector index provides deterministic phrase rotation (e.g. total tool count).
func ToolPhrase(toolName string, selector int) string {
	lower := strings.ToLower(strings.TrimSpace(toolName))
	for _, g := range phraseGroups {
		if g.matchFn(lower) {
			return PickPhrase(g.phrases, selector)
		}
	}
	return PickPhrase(DefaultPhrases, selector)
}

// PickPhrase selects a phrase from the pool using deterministic rotation.
func PickPhrase(pool []string, selector int) string {
	if len(pool) == 0 {
		return "在处理…"
	}
	idx := selector % len(pool)
	if idx < 0 {
		idx += len(pool)
	}
	return pool[idx]
}

func hasAnyPrefix(name string, prefixes ...string) bool {
	lower := strings.ToLower(name)
	for _, p := range prefixes {
		if strings.HasPrefix(lower, strings.ToLower(p)) {
			return true
		}
	}
	return false
}
