package app

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"alex/internal/agent/ports"
)

var (
	codeFenceRe = regexp.MustCompile("(?s)```.+?```")
)

func shouldUsePlanner(task string, analysis *ports.TaskAnalysis) bool {
	trimmed := strings.TrimSpace(task)
	if trimmed == "" {
		return false
	}

	if analysis != nil {
		if len(analysis.TaskBreakdown) >= 3 {
			return true
		}
		if len(analysis.SuccessCriteria) >= 3 && utf8.RuneCountInString(trimmed) > 80 {
			return true
		}
	}

	// Clear "simple question" patterns early to avoid planner overhead.
	if looksLikeSimpleQA(trimmed) {
		return false
	}

	// Inline code, multiple paragraphs, or pasted content usually implies multi-step work.
	if strings.Contains(trimmed, "\n") || codeFenceRe.MatchString(trimmed) {
		return true
	}

	lower := strings.ToLower(trimmed)
	if looksLikeSimpleSearch(lower) {
		return false
	}

	// Complex intent keywords.
	complexMarkers := []string{
		"设计", "架构", "方案", "落地",
		"实现", "开发", "重构", "修复", "排查", "优化",
		"集成", "迁移", "改造", "拆分", "合并",
		"数据库", "索引", "一致性", "事务", "并发",
		"react", "next.js", "tailwind", "ui",
		"planner", "react agent", "ReAct",
	}
	for _, marker := range complexMarkers {
		if strings.Contains(trimmed, marker) || strings.Contains(lower, strings.ToLower(marker)) {
			return true
		}
	}

	// Multi-request heuristic: chained conjunctions usually mean planning helps.
	if strings.Contains(trimmed, "然后") || strings.Contains(trimmed, "并且") || strings.Contains(trimmed, "同时") {
		return true
	}
	if strings.Count(trimmed, "；")+strings.Count(trimmed, ";") >= 2 {
		return true
	}

	// Default: keep it simple.
	return false
}

func looksLikeSimpleQA(task string) bool {
	// Short, single-sentence questions are typically better answered directly.
	runes := utf8.RuneCountInString(task)
	if runes > 64 {
		return false
	}
	if strings.Contains(task, "\n") {
		return false
	}
	if strings.Contains(task, "为什么") || strings.Contains(task, "怎么") || strings.Contains(task, "是什么") {
		return true
	}
	if strings.HasSuffix(task, "?") || strings.HasSuffix(task, "？") {
		return true
	}
	return false
}

func looksLikeSimpleSearch(lower string) bool {
	// User explicitly asks for web search / lookup; usually a single tool call.
	if strings.HasPrefix(lower, "search ") || strings.HasPrefix(lower, "websearch ") {
		return true
	}
	if strings.HasPrefix(lower, "搜索") || strings.HasPrefix(lower, "查一下") || strings.HasPrefix(lower, "查下") {
		return true
	}
	if strings.Contains(lower, "web search") || strings.Contains(lower, "websearch") {
		return true
	}
	return false
}

