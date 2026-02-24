package lark

import "testing"

func TestNotificationComposerDisabledKeepsTextUnchanged(t *testing.T) {
	c := newNotificationComposer(false)
	input := "原始消息"
	if got := c.Progress(input); got != input {
		t.Fatalf("expected unchanged progress text, got %q", got)
	}
	if got := c.Background("running", input); got != input {
		t.Fatalf("expected unchanged background text, got %q", got)
	}
	if got := c.PlanClarify(input, true); got != input {
		t.Fatalf("expected unchanged plan clarify text, got %q", got)
	}
	if got := c.InputRequest(input); got != input {
		t.Fatalf("expected unchanged input request text, got %q", got)
	}
	if got := c.FinalReply(input, notificationModeBlocking); got != input {
		t.Fatalf("expected unchanged final reply text, got %q", got)
	}
}

func TestNotificationComposerEnabledAddsPrefixes(t *testing.T) {
	c := newNotificationComposer(true)

	if got := c.Progress("搜索中"); got != "进度更新｜搜索中" {
		t.Fatalf("unexpected progress text: %q", got)
	}
	if got := c.Background("waiting_input", "任务等待中"); got != "需要你的确认\n任务等待中" {
		t.Fatalf("unexpected background text: %q", got)
	}
	if got := c.PlanClarify("请选择方案", true); got != "需要你确认｜请选择方案" {
		t.Fatalf("unexpected plan clarify text: %q", got)
	}
	if got := c.InputRequest("请审批"); got != "需要你决策\n请审批" {
		t.Fatalf("unexpected input request text: %q", got)
	}
	if got := c.FinalReply("补充参数", notificationModeBlocking); got != "请先确认｜补充参数" {
		t.Fatalf("unexpected final reply text: %q", got)
	}
}
