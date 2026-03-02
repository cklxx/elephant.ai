package lark

import (
	"context"
	"fmt"
	"strings"

	appcontext "alex/internal/app/agent/context"
	"alex/internal/app/subscription"
	"alex/internal/delivery/channels"
	"alex/internal/shared/utils"
)

// PlanMode controls whether tasks require plan review before execution.
type PlanMode string

const (
	PlanModeOn   PlanMode = "on"   // All tasks require plan review
	PlanModeOff  PlanMode = "off"  // Direct execution, no plan review
	PlanModeAuto PlanMode = "auto" // Agent decides (use config default)
)

const planModeSelectionKey = "lark_plan_mode"

// isPlanCommand checks whether the message is a /plan command.
func (g *Gateway) isPlanCommand(trimmed string) bool {
	lower := strings.ToLower(trimmed)
	return lower == "/plan" || strings.HasPrefix(lower, "/plan ")
}

// handlePlanModeCommand processes /plan commands.
func (g *Gateway) handlePlanModeCommand(msg *incomingMessage) {
	if g == nil || msg == nil {
		return
	}

	sessionID := g.memoryIDForChat(msg.chatID)
	execCtx := channels.BuildBaseContext(g.cfg.BaseConfig, "lark", sessionID, msg.senderID, msg.chatID, msg.isGroup)
	execCtx = g.withLarkContext(execCtx, msg.chatID, msg.messageID)

	trimmed := strings.TrimSpace(msg.content)
	fields := strings.Fields(trimmed)
	sub := ""
	if len(fields) > 1 {
		sub = utils.TrimLower(fields[1])
	}
	isGlobal := hasFlag(fields, "--global")

	var reply string
	switch sub {
	case "on":
		reply = g.setPlanMode(execCtx, msg, PlanModeOn, isGlobal)
	case "off":
		reply = g.setPlanMode(execCtx, msg, PlanModeOff, isGlobal)
	case "auto":
		reply = g.setPlanMode(execCtx, msg, PlanModeAuto, isGlobal)
	case "status", "show", "":
		reply = g.buildPlanModeStatus(execCtx, msg)
	case "help", "-h", "--help":
		reply = planModeUsage()
	default:
		reply = planModeUsage()
	}

	g.dispatch(execCtx, msg.chatID, replyTarget(msg.messageID, true), "text", textContent(reply))
}

// setPlanMode stores the plan mode at the appropriate scope.
func (g *Gateway) setPlanMode(ctx context.Context, msg *incomingMessage, mode PlanMode, isGlobal bool) string {
	if g.llmSelections == nil {
		return "Plan mode 存储不可用。"
	}

	selection := subscription.Selection{
		Mode:     string(mode),
		Provider: planModeSelectionKey,
		Model:    string(mode),
	}
	scope := planModeChannelScope()
	if !isGlobal {
		scope = planModeChatScope(msg)
	}
	if err := g.llmSelections.Set(ctx, scope, selection); err != nil {
		return fmt.Sprintf("设置 plan mode 失败: %v", err)
	}

	scopeLabel := "[全局]"
	if !isGlobal {
		scopeLabel = "[当前会话]"
	}
	return fmt.Sprintf("Plan mode 已设置为 %s %s", mode, scopeLabel)
}

// resolvePlanMode resolves the effective plan mode for a message context.
func (g *Gateway) resolvePlanMode(ctx context.Context, msg *incomingMessage) PlanMode {
	if g.llmSelections == nil {
		return g.defaultPlanMode()
	}

	scopes := planModeScopes(msg)
	selection, _, ok, err := g.llmSelections.GetWithFallback(ctx, scopes...)
	if err != nil || !ok {
		return g.defaultPlanMode()
	}
	if selection.Provider != planModeSelectionKey {
		return g.defaultPlanMode()
	}
	switch PlanMode(selection.Model) {
	case PlanModeOn, PlanModeOff, PlanModeAuto:
		return PlanMode(selection.Model)
	default:
		return g.defaultPlanMode()
	}
}

// defaultPlanMode returns the configured default plan mode.
func (g *Gateway) defaultPlanMode() PlanMode {
	if g.cfg.DefaultPlanMode != "" {
		return g.cfg.DefaultPlanMode
	}
	return PlanModeAuto
}

// applyPlanModeToContext injects plan review settings based on the resolved plan mode.
func (g *Gateway) applyPlanModeToContext(ctx context.Context, msg *incomingMessage) context.Context {
	mode := g.resolvePlanMode(ctx, msg)
	if mode == PlanModeOn {
		return appcontext.WithPlanReviewEnabled(ctx, true)
	}
	if mode == PlanModeOff {
		return appcontext.WithPlanReviewEnabled(ctx, false)
	}
	// PlanModeAuto and any invalid value fall back to the config default already in ctx.
	return ctx
}

// buildPlanModeStatus returns the current plan mode status.
func (g *Gateway) buildPlanModeStatus(ctx context.Context, msg *incomingMessage) string {
	if g.llmSelections == nil {
		return fmt.Sprintf("当前 plan mode: %s (配置默认值)", g.defaultPlanMode())
	}

	scopes := planModeScopes(msg)
	selection, matchedScope, ok, err := g.llmSelections.GetWithFallback(ctx, scopes...)
	if err != nil {
		return fmt.Sprintf("查询失败: %v", err)
	}
	if !ok || selection.Provider != planModeSelectionKey {
		return fmt.Sprintf("当前 plan mode: %s (配置默认值)\n\n%s", g.defaultPlanMode(), planModeUsage())
	}

	scopeLabel := planModeScopeLabel(matchedScope)
	return fmt.Sprintf("当前 plan mode: %s %s\n\n%s", selection.Model, scopeLabel, planModeUsage())
}

func planModeUsage() string {
	return strings.TrimSpace(`
Plan mode usage:
  /plan on              Enable plan review for this chat
  /plan off             Disable plan review for this chat
  /plan auto            Use auto strategy for this chat
  /plan on --global     Set global default to plan review
  /plan status          Show current plan mode

Plan modes:
  on   - All tasks require plan confirmation before execution
  off  - Tasks execute directly without plan review
  auto - Agent decides whether to plan (config default)
`)
}

func planModeChannelScope() subscription.SelectionScope {
	return subscription.SelectionScope{Channel: planModeSelectionKey}
}

func planModeChatID(msg *incomingMessage) string {
	if msg == nil {
		return ""
	}
	return strings.TrimSpace(msg.chatID)
}

func planModeChatScope(msg *incomingMessage) subscription.SelectionScope {
	chatID := planModeChatID(msg)
	if chatID == "" {
		return planModeChannelScope()
	}
	return subscription.SelectionScope{Channel: planModeSelectionKey, ChatID: chatID}
}

func planModeScopes(msg *incomingMessage) []subscription.SelectionScope {
	scopes := make([]subscription.SelectionScope, 0, 2)
	if chatID := planModeChatID(msg); chatID != "" {
		scopes = append(scopes, subscription.SelectionScope{Channel: planModeSelectionKey, ChatID: chatID})
	}
	scopes = append(scopes, planModeChannelScope())
	return scopes
}

func planModeScopeLabel(scope subscription.SelectionScope) string {
	if utils.HasContent(scope.ChatID) {
		return "[当前会话]"
	}
	return "[全局]"
}
