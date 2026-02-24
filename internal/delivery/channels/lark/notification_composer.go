package lark

import "strings"

type notificationComposer struct {
	enabled bool
}

func newNotificationComposer(enabled bool) *notificationComposer {
	return &notificationComposer{enabled: enabled}
}

func (c *notificationComposer) Progress(text string) string {
	return c.prefixedLine("进度更新｜", text)
}

func (c *notificationComposer) Background(status, text string) string {
	header := "后台任务更新"
	switch strings.TrimSpace(status) {
	case "waiting_input":
		header = "需要你的确认"
	case "completed":
		header = "后台任务已完成"
	case "failed":
		header = "后台任务失败"
	case "cancelled":
		header = "后台任务已取消"
	case "running":
		header = "后台任务进行中"
	}
	return c.prefixedBlock(header, text)
}

func (c *notificationComposer) PlanClarify(text string, needsInput bool) string {
	if needsInput {
		return c.prefixedLine("需要你确认｜", text)
	}
	return c.prefixedLine("计划更新｜", text)
}

func (c *notificationComposer) InputRequest(text string) string {
	return c.prefixedBlock("需要你决策", text)
}

func (c *notificationComposer) FinalReply(text string, mode notificationMode) string {
	if c == nil || !c.enabled {
		return text
	}
	if mode == notificationModeBlocking {
		return c.prefixedLine("请先确认｜", text)
	}
	return text
}

func (c *notificationComposer) prefixedLine(prefix, text string) string {
	if c == nil || !c.enabled {
		return text
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if strings.HasPrefix(text, prefix) {
		return text
	}
	return prefix + text
}

func (c *notificationComposer) prefixedBlock(title, text string) string {
	if c == nil || !c.enabled {
		return text
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if strings.HasPrefix(text, title) {
		return text
	}
	return title + "\n" + text
}
