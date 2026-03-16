package lark

import (
	"strings"
	"testing"
)

func TestSplitConclusion_Simple(t *testing.T) {
	text := "任务完成了\n\n这是详细信息。"
	conclusion, body := splitConclusion(text)
	if conclusion != "任务完成了" {
		t.Errorf("conclusion = %q, want %q", conclusion, "任务完成了")
	}
	if body != "这是详细信息。" {
		t.Errorf("body = %q, want %q", body, "这是详细信息。")
	}
}

func TestSplitConclusion_HeadingStripped(t *testing.T) {
	text := "## 结果\n\n详细内容在这里。"
	conclusion, body := splitConclusion(text)
	if conclusion != "结果" {
		t.Errorf("conclusion = %q, want %q", conclusion, "结果")
	}
	if body != "详细内容在这里。" {
		t.Errorf("body = %q, want %q", body, "详细内容在这里。")
	}
}

func TestSplitConclusion_SingleLine(t *testing.T) {
	conclusion, body := splitConclusion("一句话回复")
	if conclusion != "一句话回复" {
		t.Errorf("conclusion = %q, want %q", conclusion, "一句话回复")
	}
	if body != "" {
		t.Errorf("body = %q, want empty", body)
	}
}

func TestSplitConclusion_Empty(t *testing.T) {
	conclusion, body := splitConclusion("")
	if conclusion != "" || body != "" {
		t.Errorf("splitConclusion(\"\") = (%q, %q), want empty", conclusion, body)
	}
}

func TestSplitConclusion_NoParagraphBreak(t *testing.T) {
	text := "第一行\n第二行\n第三行"
	conclusion, body := splitConclusion(text)
	if conclusion != "第一行" {
		t.Errorf("conclusion = %q, want %q", conclusion, "第一行")
	}
	if !strings.Contains(body, "第二行") {
		t.Errorf("body should contain '第二行', got %q", body)
	}
}

func TestBuildResultCard_ContainsConclusion(t *testing.T) {
	card := buildResultCard("任务完成了\n\n详细信息在这里。")
	if !strings.Contains(card, "**任务完成了**") {
		t.Errorf("result card should contain bold conclusion, got %q", card)
	}
	if !strings.Contains(card, "详细信息在这里") {
		t.Errorf("result card should contain body text, got %q", card)
	}
}

func TestBuildResultCard_Empty(t *testing.T) {
	card := buildResultCard("")
	if !strings.Contains(card, "完成") {
		t.Errorf("empty result card should have fallback text, got %q", card)
	}
}

func TestBuildErrorCard_ContainsGuidance(t *testing.T) {
	card := buildErrorCard("连接超时")
	if !strings.Contains(card, "连接超时") {
		t.Errorf("error card should contain error message, got %q", card)
	}
	if !strings.Contains(card, "重试") {
		t.Errorf("error card should contain retry guidance, got %q", card)
	}
	if !strings.Contains(card, "诊断") {
		t.Errorf("error card should contain diagnose guidance, got %q", card)
	}
	if !strings.Contains(card, "执行失败") {
		t.Errorf("error card should have failure header, got %q", card)
	}
}

func TestBuildErrorCard_Empty(t *testing.T) {
	card := buildErrorCard("")
	if !strings.Contains(card, "执行过程中遇到了问题") {
		t.Errorf("empty error card should have fallback message, got %q", card)
	}
}

func TestBuildPlanReviewCard_WithGoalAndPlan(t *testing.T) {
	card := buildPlanReviewCard("优化数据库", "1. 分析\n2. 优化", true)
	if !strings.Contains(card, "优化数据库") {
		t.Errorf("plan card should contain goal, got %q", card)
	}
	if !strings.Contains(card, "分析") {
		t.Errorf("plan card should contain plan text, got %q", card)
	}
	if !strings.Contains(card, "OK") {
		t.Errorf("plan card should contain confirmation prompt, got %q", card)
	}
	if !strings.Contains(card, "计划确认") {
		t.Errorf("plan card should have header title, got %q", card)
	}
}

func TestBuildPlanReviewCard_NoConfirmation(t *testing.T) {
	card := buildPlanReviewCard("目标", "计划", false)
	if strings.Contains(card, "OK") {
		t.Errorf("plan card without confirmation should not mention OK, got %q", card)
	}
}

func TestSmartResultContent_Error(t *testing.T) {
	msgType, content := smartResultContent("连接超时", errStub{}, false)
	if msgType != "interactive" {
		t.Errorf("error should use interactive card, got %q", msgType)
	}
	if !strings.Contains(content, "连接超时") {
		t.Errorf("error card should contain error text, got %q", content)
	}
}

func TestSmartResultContent_ShortReply(t *testing.T) {
	msgType, _ := smartResultContent("好的", nil, false)
	if msgType == "interactive" {
		t.Errorf("short reply should not use card, got msgType=%q", msgType)
	}
}

func TestSmartResultContent_LongReply(t *testing.T) {
	long := strings.Repeat("这是一段很长的回复内容。", 20)
	msgType, content := smartResultContent(long, nil, false)
	if msgType != "interactive" {
		t.Errorf("long reply should use interactive card, got %q", msgType)
	}
	if content == "" {
		t.Error("long reply card content should not be empty")
	}
}

func TestSmartResultContent_AwaitUsesSmartContent(t *testing.T) {
	msgType, _ := smartResultContent("请选择一个选项", nil, true)
	// Await prompts should not use cards — they need text for option parsing.
	if msgType == "interactive" {
		t.Errorf("await reply should not use card, got msgType=%q", msgType)
	}
}

type errStub struct{}

func (errStub) Error() string { return "stub error" }
