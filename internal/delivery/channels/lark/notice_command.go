package lark

import (
	"fmt"
	"strings"
)

// isNoticeCommand checks whether the message is a /notice command.
func (g *Gateway) isNoticeCommand(trimmed string) bool {
	lower := strings.ToLower(strings.TrimSpace(trimmed))
	return lower == "/notice" || strings.HasPrefix(lower, "/notice ")
}

// handleNoticeCommand processes /notice binding commands.
func (g *Gateway) handleNoticeCommand(msg *incomingMessage) {
	if g == nil || msg == nil {
		return
	}

	execCtx := g.buildTaskCommandContext(msg)
	fields := strings.Fields(strings.TrimSpace(msg.content))
	sub := ""
	if len(fields) > 1 {
		sub = strings.ToLower(strings.TrimSpace(fields[1]))
	}

	var reply string
	switch sub {
	case "", "bind", "set", "on":
		reply = g.bindNoticeChat(msg)
	case "status", "show":
		reply = g.noticeStatus()
	case "off", "clear", "disable":
		reply = g.clearNoticeChat(msg)
	case "help", "-h", "--help":
		reply = noticeCommandUsage()
	default:
		reply = noticeCommandUsage()
	}

	g.dispatch(execCtx, msg.chatID, replyTarget(msg.messageID, true), "text", textContent(reply))
}

func (g *Gateway) bindNoticeChat(msg *incomingMessage) string {
	if g == nil || msg == nil {
		return "设置通知群失败：网关未初始化。"
	}
	if !msg.isGroup {
		return "请在目标群里发送 /notice 来绑定通知群。"
	}
	if g.noticeState == nil {
		return "设置通知群失败：通知状态存储不可用。"
	}

	binding, err := g.noticeState.Save(msg.chatID, msg.senderID, "")
	if err != nil {
		g.logger.Warn("Notice bind failed: %v", err)
		return fmt.Sprintf("设置通知群失败：%v", err)
	}

	if g.cfg.CCHooksAutoConfig != nil {
		g.taskWG.Add(1)
		go func() {
			defer g.taskWG.Done()
			g.runCCHooksSetup(msg)
		}()
	}

	reply := fmt.Sprintf("已将当前群设置为通知群。\nchat_id: %s\nset_at: %s", binding.ChatID, binding.UpdatedAt)
	if g.cfg.CCHooksAutoConfig != nil {
		reply += "\n正在配置 Claude Code hooks..."
	}
	return reply
}

func (g *Gateway) noticeStatus() string {
	if g == nil || g.noticeState == nil {
		return "通知状态存储不可用。"
	}
	binding, ok, err := g.noticeState.Load()
	if err != nil {
		g.logger.Warn("Notice status load failed: %v", err)
		return fmt.Sprintf("读取通知群状态失败：%v", err)
	}
	if !ok {
		return "当前未设置通知群。\n\n请在目标群里发送 /notice 进行绑定。"
	}

	setBy := binding.SetByUserID
	if setBy == "" {
		setBy = "unknown"
	}
	setAt := binding.SetAt
	if setAt == "" {
		setAt = binding.UpdatedAt
	}
	if setAt == "" {
		setAt = "unknown"
	}

	return fmt.Sprintf("当前通知群:\nchat_id: %s\nset_by: %s\nset_at: %s", binding.ChatID, setBy, setAt)
}

func (g *Gateway) clearNoticeChat(msg *incomingMessage) string {
	if g == nil || g.noticeState == nil {
		return "通知状态存储不可用。"
	}
	if err := g.noticeState.Clear(); err != nil {
		g.logger.Warn("Notice clear failed: %v", err)
		return fmt.Sprintf("清除通知群失败：%v", err)
	}

	if g.cfg.CCHooksAutoConfig != nil {
		g.taskWG.Add(1)
		go func() {
			defer g.taskWG.Done()
			g.runCCHooksRemove(msg)
		}()
	}

	reply := "已清除通知群绑定。"
	if g.cfg.CCHooksAutoConfig != nil {
		reply += "\n正在移除 Claude Code hooks..."
	}
	return reply
}

func noticeCommandUsage() string {
	return strings.TrimSpace(`
Notice command usage:
  /notice          Bind current group chat as notice target
  /notice status   Show current notice target
  /notice off      Clear current notice target
`)
}
