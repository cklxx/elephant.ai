package uxphrases

import (
	"testing"
)

func TestToolPhrase_SearchTools(t *testing.T) {
	tests := []string{"web_search", "web_fetch", "tavily_search", "search_web_query"}
	expected := []string{"在搜索…", "在探索…", "在挖掘…"}
	for _, tool := range tests {
		phrase := ToolPhrase(tool, 0)
		if !contains(expected, phrase) {
			t.Errorf("ToolPhrase(%q, 0) = %q, not in search phrases", tool, phrase)
		}
	}
}

func TestToolPhrase_ReadTools(t *testing.T) {
	tests := []string{"read", "glob", "grep", "read_file", "list_dir", "search_code"}
	expected := []string{"在翻阅…", "在研读…", "在查阅…"}
	for _, tool := range tests {
		phrase := ToolPhrase(tool, 0)
		if !contains(expected, phrase) {
			t.Errorf("ToolPhrase(%q, 0) = %q, not in read phrases", tool, phrase)
		}
	}
}

func TestToolPhrase_WriteTools(t *testing.T) {
	tests := []string{"write", "edit", "write_file", "create_file", "apply_diff"}
	expected := []string{"在撰写…", "在书写…", "在落笔…"}
	for _, tool := range tests {
		phrase := ToolPhrase(tool, 0)
		if !contains(expected, phrase) {
			t.Errorf("ToolPhrase(%q, 0) = %q, not in write phrases", tool, phrase)
		}
	}
}

func TestToolPhrase_BashTools(t *testing.T) {
	tests := []string{"bash", "shell_exec_cmd", "run_command_x", "terminal_open"}
	expected := []string{"在运算…", "在执行…", "在实验…"}
	for _, tool := range tests {
		phrase := ToolPhrase(tool, 0)
		if !contains(expected, phrase) {
			t.Errorf("ToolPhrase(%q, 0) = %q, not in bash phrases", tool, phrase)
		}
	}
}

func TestToolPhrase_BrowserTools(t *testing.T) {
	tests := []string{"browser_navigate", "navigate_to", "screenshot_page", "click_element"}
	expected := []string{"在浏览…", "在查看…", "在观察…"}
	for _, tool := range tests {
		phrase := ToolPhrase(tool, 0)
		if !contains(expected, phrase) {
			t.Errorf("ToolPhrase(%q, 0) = %q, not in browser phrases", tool, phrase)
		}
	}
}

func TestToolPhrase_MemoryTools(t *testing.T) {
	tests := []string{"memory_save", "recall_context", "remember_this"}
	expected := []string{"在回忆…", "在追溯…", "在检索…"}
	for _, tool := range tests {
		phrase := ToolPhrase(tool, 0)
		if !contains(expected, phrase) {
			t.Errorf("ToolPhrase(%q, 0) = %q, not in memory phrases", tool, phrase)
		}
	}
}

func TestToolPhrase_ImageTools(t *testing.T) {
	tests := []string{"text_to_image", "seedream_gen", "generate_image_x", "dalle_3"}
	expected := []string{"在创作…", "在绘制…", "在构图…"}
	for _, tool := range tests {
		phrase := ToolPhrase(tool, 0)
		if !contains(expected, phrase) {
			t.Errorf("ToolPhrase(%q, 0) = %q, not in image phrases", tool, phrase)
		}
	}
}

func TestToolPhrase_LarkTools(t *testing.T) {
	tests := []string{"lark_send_msg", "feishu_get_user", "send_lark_card", "get_lark_doc"}
	expected := []string{"在联络…", "在查询…", "在协调…"}
	for _, tool := range tests {
		phrase := ToolPhrase(tool, 0)
		if !contains(expected, phrase) {
			t.Errorf("ToolPhrase(%q, 0) = %q, not in lark phrases", tool, phrase)
		}
	}
}

func TestToolPhrase_PlanTools(t *testing.T) {
	tests := []string{"plan", "ask_user", "plan_task"}
	expected := []string{"在规划…", "在梳理…", "在分析…"}
	for _, tool := range tests {
		phrase := ToolPhrase(tool, 0)
		if !contains(expected, phrase) {
			t.Errorf("ToolPhrase(%q, 0) = %q, not in plan phrases", tool, phrase)
		}
	}
}

func TestToolPhrase_TaskTools(t *testing.T) {
	tests := []string{"task", "task_create", "delegate_work"}
	expected := []string{"在深入…", "在调研…", "在拆解…"}
	for _, tool := range tests {
		phrase := ToolPhrase(tool, 0)
		if !contains(expected, phrase) {
			t.Errorf("ToolPhrase(%q, 0) = %q, not in task phrases", tool, phrase)
		}
	}
}

func TestToolPhrase_UnknownToolFallsToDefault(t *testing.T) {
	phrase := ToolPhrase("unknown_xyz_tool", 0)
	expected := DefaultPhrases
	if !contains(expected, phrase) {
		t.Errorf("ToolPhrase for unknown tool = %q, not in default phrases", phrase)
	}
}

func TestToolPhrase_CaseInsensitive(t *testing.T) {
	lower := ToolPhrase("Read", 0)
	expected := []string{"在翻阅…", "在研读…", "在查阅…"}
	if !contains(expected, lower) {
		t.Errorf("ToolPhrase(Read) = %q, not in read phrases (case-insensitive)", lower)
	}
}

func TestToolPhrase_SelectorRotation(t *testing.T) {
	phrases := make(map[string]bool)
	for i := 0; i < 3; i++ {
		phrases[ToolPhrase("read", i)] = true
	}
	if len(phrases) < 2 {
		t.Errorf("expected rotation across selectors, got %d distinct phrases", len(phrases))
	}
}

func TestPickPhrase_DeterministicRotation(t *testing.T) {
	pool := []string{"a", "b", "c"}
	if got := PickPhrase(pool, 0); got != "a" {
		t.Errorf("PickPhrase(pool, 0) = %q, want a", got)
	}
	if got := PickPhrase(pool, 1); got != "b" {
		t.Errorf("PickPhrase(pool, 1) = %q, want b", got)
	}
	if got := PickPhrase(pool, 2); got != "c" {
		t.Errorf("PickPhrase(pool, 2) = %q, want c", got)
	}
	if got := PickPhrase(pool, 3); got != "a" {
		t.Errorf("PickPhrase(pool, 3) = %q, want a (wrap-around)", got)
	}
}

func TestPickPhrase_EmptyPool(t *testing.T) {
	if got := PickPhrase(nil, 0); got != "在处理…" {
		t.Errorf("PickPhrase(nil, 0) = %q, want fallback", got)
	}
	if got := PickPhrase([]string{}, 5); got != "在处理…" {
		t.Errorf("PickPhrase(empty, 5) = %q, want fallback", got)
	}
}

func TestPickPhrase_NegativeSelector(t *testing.T) {
	pool := []string{"a", "b", "c"}
	got := PickPhrase(pool, -1)
	if !contains(pool, got) {
		t.Errorf("PickPhrase(pool, -1) = %q, not in pool", got)
	}
}

func TestPickPhrase_LargeSelector(t *testing.T) {
	pool := []string{"a", "b"}
	if got := PickPhrase(pool, 1000); got != "a" {
		t.Errorf("PickPhrase(pool, 1000) = %q, want a", got)
	}
}

func TestThinkingPhrases_NotEmpty(t *testing.T) {
	if len(ThinkingPhrases) == 0 {
		t.Error("ThinkingPhrases should not be empty")
	}
}

func TestSummarizingPhrases_NotEmpty(t *testing.T) {
	if len(SummarizingPhrases) == 0 {
		t.Error("SummarizingPhrases should not be empty")
	}
}

func TestDefaultPhrases_NotEmpty(t *testing.T) {
	if len(DefaultPhrases) == 0 {
		t.Error("DefaultPhrases should not be empty")
	}
}

func contains(pool []string, s string) bool {
	for _, p := range pool {
		if p == s {
			return true
		}
	}
	return false
}
