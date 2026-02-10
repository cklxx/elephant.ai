package lark

import "strings"

// toolPhraseGroup maps a category of tools to a pool of friendly Chinese
// status phrases. The progress and background progress listeners use these
// to display warm, human-readable status lines instead of raw tool names.
type toolPhraseGroup struct {
	phrases []string
	// matchFn returns true if the given tool name belongs to this group.
	matchFn func(name string) bool
}

// thinkingPhrases are used when no tool is active (pure LLM thinking).
var thinkingPhrases = []string{"在思考…", "在酝酿…", "在构思…", "在琢磨…"}

// summarizingPhrases are used when all tools have completed (wrapping up).
var summarizingPhrases = []string{"在整理…", "在总结…", "在梳理…"}

// toolPhraseGroups is an ordered list of tool categories. The first match wins.
var toolPhraseGroups = []toolPhraseGroup{
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
				"Read", "Glob", "Grep", "find_file", "search_code", "list_directory",
				"view_file", "view_source")
		},
	},
	{
		phrases: []string{"在撰写…", "在书写…", "在落笔…"},
		matchFn: func(n string) bool {
			return hasAnyPrefix(n, "write_file", "replace_in_file", "create_file",
				"Write", "Edit", "insert_text", "apply_diff", "patch_file")
		},
	},
	{
		phrases: []string{"在运算…", "在执行…", "在实验…"},
		matchFn: func(n string) bool {
			return hasAnyPrefix(n, "shell_exec", "execute_code", "run_command",
				"Bash", "bash", "terminal", "exec")
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
			return hasAnyPrefix(n, "plan", "clarify", "Plan")
		},
	},
	{
		phrases: []string{"在深入…", "在调研…", "在拆解…"},
		matchFn: func(n string) bool {
			return hasAnyPrefix(n, "subagent", "explore", "sub_agent",
				"Task", "delegate")
		},
	},
}

// defaultPhrases are used when no tool group matches.
var defaultPhrases = []string{"在处理…", "在分析…", "在洞察…"}

// toolPhrase returns a friendly Chinese status phrase for the given tool name.
// The selector index provides deterministic phrase rotation (e.g. total tool count).
func toolPhrase(toolName string, selector int) string {
	lower := strings.ToLower(strings.TrimSpace(toolName))
	for _, g := range toolPhraseGroups {
		if g.matchFn(lower) {
			return pickPhrase(g.phrases, selector)
		}
	}
	return pickPhrase(defaultPhrases, selector)
}

// toolPhraseForBackground returns the same phrase mapping but accepts the
// raw tool name from bridge/background events (may be mixed-case).
func toolPhraseForBackground(toolName string, selector int) string {
	return toolPhrase(toolName, selector)
}

func pickPhrase(pool []string, selector int) string {
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
