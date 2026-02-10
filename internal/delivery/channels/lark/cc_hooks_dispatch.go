package lark

import (
	"fmt"

	"alex/internal/delivery/channels"
	agent "alex/internal/domain/agent/ports/agent"
)

// runCCHooksSetup dispatches a lightweight agent task that activates the
// cc-hooks-setup skill to write Claude Code hooks into .claude/settings.local.json.
func (g *Gateway) runCCHooksSetup(msg *incomingMessage) {
	cfg := g.cfg.CCHooksAutoConfig
	if cfg == nil {
		return
	}

	prompt := fmt.Sprintf(
		"请使用 cc-hooks-setup skill 配置 Claude Code hooks。\n"+
			"执行以下命令：\n"+
			"```bash\npython3 skills/cc-hooks-setup/run.py '%s'\n```\n"+
			"配置完成后，简短回复确认信息（中文）。",
		ccHooksSetupArgs(cfg.ServerURL, cfg.Token),
	)

	g.dispatchCCHooksTask(msg, prompt)
}

// runCCHooksRemove dispatches a lightweight agent task that removes Claude Code
// hooks from .claude/settings.local.json via the cc-hooks-setup skill.
func (g *Gateway) runCCHooksRemove(msg *incomingMessage) {
	prompt := "请使用 cc-hooks-setup skill 移除 Claude Code hooks。\n" +
		"执行以下命令：\n" +
		"```bash\npython3 skills/cc-hooks-setup/run.py '{\"action\":\"remove\"}'\n```\n" +
		"完成后，简短回复确认信息（中文）。"

	g.dispatchCCHooksTask(msg, prompt)
}

// dispatchCCHooksTask runs the given prompt as a fire-and-forget agent task
// and sends the result (or error) back to the originating chat.
func (g *Gateway) dispatchCCHooksTask(msg *incomingMessage, prompt string) {
	sessionID := g.newSessionID()
	ctx := g.buildTaskCommandContext(msg)
	ctx = channels.ApplyPresets(ctx, g.cfg.BaseConfig)

	if _, err := g.agent.EnsureSession(ctx, sessionID); err != nil {
		g.logger.Warn("cc-hooks-setup: EnsureSession failed: %v", err)
	}

	result, err := g.agent.ExecuteTask(ctx, prompt, sessionID, agent.NoopEventListener{})

	reply := g.buildReply(result, err)
	if reply == "" && err != nil {
		reply = fmt.Sprintf("Claude Code hooks 自动配置失败：%v\n请参考 scripts/cc_hooks/settings.example.json 手动配置", err)
	} else if reply == "" {
		reply = "Claude Code hooks 配置完成。"
	}

	execCtx := g.buildTaskCommandContext(msg)
	g.dispatch(execCtx, msg.chatID, replyTarget("", msg.isGroup), "text", textContent(reply))
}

// ccHooksSetupArgs builds the JSON argument string for run.py setup action.
func ccHooksSetupArgs(serverURL, token string) string {
	if token != "" {
		return fmt.Sprintf(`{"action":"setup","server_url":"%s","token":"%s"}`, serverURL, token)
	}
	return fmt.Sprintf(`{"action":"setup","server_url":"%s"}`, serverURL)
}
