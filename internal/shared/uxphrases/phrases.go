// Package uxphrases provides friendly Chinese status phrases for tool activity
// display. It is used by both the Lark progress listeners and the hooks bridge
// to present warm, human-readable status lines instead of raw tool names.
package uxphrases

import (
	"alex/internal/shared/utils"
	"strings"
)

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
			return matchAny(n,
				exactMatch("read", "glob", "grep"),
				prefixMatch("read_file", "read_", "list_dir", "list_files",
					"search_file", "find_file", "search_code", "list_directory",
					"view_file", "view_source"),
			)
		},
	},
	{
		phrases: []string{"在撰写…", "在书写…", "在落笔…"},
		matchFn: func(n string) bool {
			return matchAny(n,
				exactMatch("write", "edit"),
				prefixMatch("write_file", "write_", "replace_in_file", "create_file",
					"edit_", "insert_text", "apply_diff", "patch_file"),
			)
		},
	},
	{
		phrases: []string{"在运算…", "在执行…", "在实验…"},
		matchFn: func(n string) bool {
			return matchAny(n,
				exactMatch("bash"),
				prefixMatch("shell_exec", "run_command",
					"terminal", "exec_"),
			)
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
			return hasAnyPrefix(n, "memory_save", "recall", "remember")
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
			return matchAny(n,
				exactMatch("plan", "ask_user"),
				prefixMatch("plan_", "ask_user_"),
			)
		},
	},
	{
		phrases: []string{"在深入…", "在调研…", "在拆解…"},
		matchFn: func(n string) bool {
			return matchAny(n,
				exactMatch("task"),
				prefixMatch("task_", "delegate"),
			)
		},
	},
}

// ToolPhrase returns a friendly Chinese status phrase for the given tool name.
// The selector index provides deterministic phrase rotation (e.g. total tool count).
func ToolPhrase(toolName string, selector int) string {
	lower := utils.TrimLower(toolName)
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

// matchAny returns true if any of the matcher functions return true for name.
func matchAny(name string, matchers ...func(string) bool) bool {
	for _, m := range matchers {
		if m(name) {
			return true
		}
	}
	return false
}

// exactMatch returns a matcher that checks for exact string equality.
func exactMatch(names ...string) func(string) bool {
	return func(name string) bool {
		for _, n := range names {
			if name == strings.ToLower(n) {
				return true
			}
		}
		return false
	}
}

// prefixMatch returns a matcher that checks for prefix matching.
func prefixMatch(prefixes ...string) func(string) bool {
	return func(name string) bool {
		return hasAnyPrefix(name, prefixes...)
	}
}

func hasAnyPrefix(name string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(name, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

// keyToolGroup pairs a matcher with conversational phrases spoken as a
// first-person narrator — the way a helpful colleague would talk, never
// mentioning tool names or technical jargon.
type keyToolGroup struct {
	phrases []string
	matchFn func(name string) bool
}

var keyToolGroups = []keyToolGroup{
	{
		phrases: []string{"让我查查看…", "我去找找看…", "稍等，我翻翻资料…"},
		matchFn: func(n string) bool {
			return hasAnyPrefix(n, "web_search", "web_fetch", "tavily", "search_web")
		},
	},
	{
		phrases: []string{"我试一下…", "让我跑一下看看…", "稍等，我验证一下…"},
		matchFn: func(n string) bool {
			return matchAny(n,
				exactMatch("bash"),
				prefixMatch("shell_exec", "run_command", "execute_code", "terminal", "exec_"),
			)
		},
	},
	{
		phrases: []string{"我来改一下…", "让我调整一下…", "稍等，在写了…"},
		matchFn: func(n string) bool {
			return matchAny(n,
				exactMatch("write", "edit"),
				prefixMatch("write_file", "write_", "replace_in_file", "create_file",
					"edit_", "insert_text", "apply_diff", "patch_file"),
			)
		},
	},
	{
		phrases: []string{"我去网上看看…", "让我打开看一下…", "稍等，在浏览了…"},
		matchFn: func(n string) bool {
			return hasAnyPrefix(n, "browser_", "navigate", "screenshot",
				"click", "scroll", "type_text")
		},
	},
	{
		phrases: []string{"让我画一个…", "在创作中，稍等…", "在画了…"},
		matchFn: func(n string) bool {
			return hasAnyPrefix(n, "text_to_image", "seedream", "generate_image",
				"image_gen", "dall_e", "dalle")
		},
	},
	{
		phrases: []string{"我帮你发一下…", "让我联系一下…", "稍等，在沟通了…"},
		matchFn: func(n string) bool {
			return hasAnyPrefix(n, "lark_", "feishu_", "send_lark", "get_lark", "channel")
		},
	},
	{
		phrases: []string{"让我想想怎么做…", "我理一下思路…", "我先捋一捋…"},
		matchFn: func(n string) bool {
			return matchAny(n, exactMatch("plan"), prefixMatch("plan_"))
		},
	},
	{
		phrases: []string{"我安排一下…", "让我分头处理…", "先拆开来做…"},
		matchFn: func(n string) bool {
			return matchAny(n,
				exactMatch("task", "subagent"),
				prefixMatch("task_", "delegate", "subagent_", "run_tasks"),
			)
		},
	},
}

// IsKeyTool returns true if the tool represents a user-visible action that
// warrants a conversational progress update. Internal tools (read, memory,
// clarify) are excluded — they are invisible housekeeping.
func IsKeyTool(toolName string) bool {
	lower := utils.TrimLower(toolName)
	for _, g := range keyToolGroups {
		if g.matchFn(lower) {
			return true
		}
	}
	return false
}

// KeyToolPhrase returns a natural first-person Chinese phrase for the given
// key tool, rotating through a phrase pool. Returns empty for non-key tools.
func KeyToolPhrase(toolName string, selector int) string {
	lower := utils.TrimLower(toolName)
	for _, g := range keyToolGroups {
		if g.matchFn(lower) {
			return PickPhrase(g.phrases, selector)
		}
	}
	return ""
}
