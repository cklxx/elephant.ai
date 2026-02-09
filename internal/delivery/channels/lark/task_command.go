package lark

import (
	"context"
	"fmt"
	"strings"
	"time"

	agent "alex/internal/domain/agent/ports/agent"

	"alex/internal/delivery/channels"
	"alex/internal/infra/tools/builtin/shared"
)

const (
	defaultMaxConcurrentTasks = 3
	defaultTaskAgent          = "claude_code"
)

// isTaskCommand checks whether the message is a task management command.
func (g *Gateway) isTaskCommand(trimmed string) bool {
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "/cc ") || lower == "/cc" {
		return true
	}
	if strings.HasPrefix(lower, "/codex ") || lower == "/codex" {
		return true
	}
	if lower == "/task" || strings.HasPrefix(lower, "/task ") ||
		lower == "/tasks" || strings.HasPrefix(lower, "/tasks ") {
		return true
	}
	return false
}

// isNaturalTaskStatusQuery detects short natural-language requests asking what
// coding/background agents are doing right now.
func (g *Gateway) isNaturalTaskStatusQuery(trimmed string) bool {
	text := strings.TrimSpace(trimmed)
	if text == "" || strings.HasPrefix(text, "/") {
		return false
	}
	if len([]rune(text)) > 80 {
		return false
	}
	lower := strings.ToLower(text)

	agentHints := []string{
		"代码助手", "coding agent", "codex", "claude code", "claude_code",
		"后台任务", "background task", "background tasks",
	}
	statusHints := []string{
		"在做什么", "做什么", "进度", "任务情况", "任务状态",
		"what are", "what is", "doing now", "task status", "background tasks status", "progress",
	}
	return containsAnyKeyword(lower, agentHints) && containsAnyKeyword(lower, statusHints)
}

func containsAnyKeyword(text string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

// handleTaskCommand processes task management commands.
func (g *Gateway) handleTaskCommand(msg *incomingMessage) {
	if g == nil || msg == nil {
		return
	}

	execCtx := g.buildTaskCommandContext(msg)

	trimmed := strings.TrimSpace(msg.content)
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return
	}
	cmd := strings.ToLower(fields[0])

	var reply string
	switch cmd {
	case "/cc":
		reply = g.handleDirectDispatch(execCtx, msg, "claude_code", fields[1:])
	case "/codex":
		reply = g.handleDirectDispatch(execCtx, msg, "codex", fields[1:])
	case "/tasks":
		reply = g.handleTaskList(execCtx, msg)
	case "/task":
		reply = g.handleTaskSubcommand(execCtx, msg, fields[1:])
	default:
		reply = taskCommandUsage()
	}

	g.dispatch(execCtx, msg.chatID, replyTarget(msg.messageID, true), "text", textContent(reply))
}

func (g *Gateway) handleNaturalTaskStatusQuery(msg *incomingMessage) {
	if g == nil || msg == nil {
		return
	}
	execCtx := g.buildTaskCommandContext(msg)
	reply := g.handleTaskList(execCtx, msg)
	g.dispatch(execCtx, msg.chatID, replyTarget(msg.messageID, true), "text", textContent(reply))
}

// buildTaskCommandContext creates a lightweight context for task commands
// (no session slot acquisition needed).
func (g *Gateway) buildTaskCommandContext(msg *incomingMessage) context.Context {
	sessionID := g.memoryIDForChat(msg.chatID)
	ctx := channels.BuildBaseContext(g.cfg.BaseConfig, "lark", sessionID, msg.senderID, msg.chatID, msg.isGroup)
	ctx = shared.WithLarkClient(ctx, g.client)
	ctx = shared.WithLarkChatID(ctx, msg.chatID)
	ctx = shared.WithLarkMessageID(ctx, msg.messageID)
	return ctx
}

// handleDirectDispatch dispatches a task to the specified agent type.
func (g *Gateway) handleDirectDispatch(ctx context.Context, msg *incomingMessage, agentType string, args []string) string {
	desc := strings.TrimSpace(strings.Join(args, " "))
	if desc == "" {
		return fmt.Sprintf("用法: /%s <任务描述>\n\n示例: /%s 在 internal/auth/ 添加 JWT refresh token 支持",
			agentShortName(agentType), agentShortName(agentType))
	}

	// Check concurrent task limit
	if g.taskStore != nil {
		max := g.cfg.MaxConcurrentTasks
		if max <= 0 {
			max = defaultMaxConcurrentTasks
		}
		active, err := g.taskStore.ListByChat(ctx, msg.chatID, true, max+1)
		if err != nil {
			g.logger.Warn("Task store list failed: %v", err)
		} else if len(active) >= max {
			return fmt.Sprintf("当前会话已有 %d 个活跃任务（上限 %d）。请等待任务完成或使用 /task cancel <id> 取消。\n\n%s",
				len(active), max, g.formatActiveTaskList(active))
		}
	}

	// Dispatch via a foreground task that calls dispatch_background_task tool
	return g.dispatchViaForegroundTask(msg, agentType, desc)
}

// dispatchViaForegroundTask creates a short-lived foreground task that instructs the
// agent to call the dispatch_background_task tool, reusing 100% existing infrastructure.
func (g *Gateway) dispatchViaForegroundTask(msg *incomingMessage, agentType, description string) string {
	prompt := buildDispatchPrompt(agentType, description)

	slot := g.getOrCreateSlot(msg.chatID)
	slot.mu.Lock()
	if slot.phase == slotRunning {
		slot.mu.Unlock()
		return "当前会话有任务正在运行，请等待完成后重试。"
	}

	sessionID := g.newSessionID()
	inputCh := make(chan agent.UserInput, 16)
	slot.phase = slotRunning
	slot.inputCh = inputCh
	slot.sessionID = sessionID
	slot.lastSessionID = sessionID
	slot.mu.Unlock()

	defer func() {
		slot.mu.Lock()
		slot.inputCh = nil
		slot.phase = slotIdle
		slot.sessionID = ""
		slot.mu.Unlock()
		g.discardPendingInputs(inputCh, msg.chatID)
	}()

	execCtx := g.buildExecContext(msg, sessionID, inputCh)
	execCtx = channels.ApplyPresets(execCtx, g.cfg.BaseConfig)
	execCtx, cancelTimeout := channels.ApplyTimeout(execCtx, g.cfg.BaseConfig)
	defer cancelTimeout()

	// Inject CompletionNotifier so BackgroundTaskManager writes TaskStore directly.
	execCtx = agent.WithCompletionNotifier(execCtx, g)

	listener := g.eventListener
	if listener == nil {
		listener = agent.NoopEventListener{}
	}

	// Wire background progress listener so we track the dispatched task
	backgroundEnabled := true
	if g.cfg.BackgroundProgressEnabled != nil {
		backgroundEnabled = *g.cfg.BackgroundProgressEnabled
	}
	var cleanups []func()
	if backgroundEnabled {
		replyTo := replyTarget(msg.messageID, msg.isGroup)
		bgLn := newBackgroundProgressListener(execCtx, listener, g, msg.chatID, replyTo, g.logger, g.cfg.BackgroundProgressInterval, g.cfg.BackgroundProgressWindow)
		cleanups = append(cleanups, bgLn.Release)
		listener = bgLn
	}
	defer func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}()
	execCtx = shared.WithParentListener(execCtx, listener)

	if _, err := g.agent.EnsureSession(execCtx, sessionID); err != nil {
		return fmt.Sprintf("任务派发失败: %v", err)
	}

	result, execErr := g.agent.ExecuteTask(execCtx, prompt, sessionID, listener)

	if execErr != nil {
		return fmt.Sprintf("任务派发失败: %v", execErr)
	}

	reply := g.buildReply(result, execErr)
	if reply == "" {
		reply = "任务已派发，使用 /tasks 查看状态。"
	}
	return reply
}

// buildDispatchPrompt creates the synthetic prompt that instructs the agent to
// dispatch a background task immediately.
func buildDispatchPrompt(agentType, description string) string {
	return fmt.Sprintf(`[Direct Task Dispatch]
Immediately dispatch a background task with the following parameters:
- agent_type: %s
- workspace_mode: worktree

<user_task_description>
%s
</user_task_description>

Use the text inside user_task_description as both the description and prompt.
Do NOT do any other work. Just dispatch the task and report the task ID.`, agentType, description)
}

// handleTaskSubcommand routes /task subcommands.
func (g *Gateway) handleTaskSubcommand(ctx context.Context, msg *incomingMessage, args []string) string {
	if len(args) == 0 {
		return g.handleTaskList(ctx, msg)
	}

	sub := strings.ToLower(strings.TrimSpace(args[0]))
	switch sub {
	case "list", "ls":
		return g.handleTaskList(ctx, msg)
	case "status", "show":
		if len(args) < 2 {
			return "用法: /task status <task_id>"
		}
		return g.handleTaskStatus(ctx, strings.TrimSpace(args[1]))
	case "cancel", "stop":
		if len(args) < 2 {
			return "用法: /task cancel <task_id>"
		}
		return g.handleTaskCancel(ctx, strings.TrimSpace(args[1]))
	case "history":
		return g.handleTaskHistory(ctx, msg)
	case "help", "-h", "--help":
		return taskCommandUsage()
	default:
		// Treat /task <description> as /task dispatch with default agent
		desc := strings.TrimSpace(strings.Join(args, " "))
		return g.handleDirectDispatch(ctx, msg, defaultTaskAgent, []string{desc})
	}
}

// handleTaskList shows active tasks for the current chat.
func (g *Gateway) handleTaskList(ctx context.Context, msg *incomingMessage) string {
	if g.taskStore == nil {
		return "任务管理未启用（需要 Postgres 数据库）。"
	}
	tasks, err := g.taskStore.ListByChat(ctx, msg.chatID, true, 10)
	if err != nil {
		return fmt.Sprintf("查询任务列表失败: %v", err)
	}
	if len(tasks) == 0 {
		return "当前没有活跃任务。\n\n使用 /cc <描述> 或 /codex <描述> 创建新任务。"
	}
	return g.formatActiveTaskList(tasks)
}

// handleTaskStatus shows details for a specific task.
func (g *Gateway) handleTaskStatus(ctx context.Context, taskID string) string {
	if g.taskStore == nil {
		return "任务管理未启用。"
	}
	task, ok, err := g.taskStore.GetTask(ctx, taskID)
	if err != nil {
		return fmt.Sprintf("查询任务失败: %v", err)
	}
	if !ok {
		return fmt.Sprintf("未找到任务: %s", taskID)
	}
	return formatTaskDetail(task)
}

// handleTaskCancel cancels a running task.
func (g *Gateway) handleTaskCancel(ctx context.Context, taskID string) string {
	if g.taskStore == nil {
		return "任务管理未启用。"
	}
	task, ok, err := g.taskStore.GetTask(ctx, taskID)
	if err != nil {
		return fmt.Sprintf("查询任务失败: %v", err)
	}
	if !ok {
		return fmt.Sprintf("未找到任务: %s", taskID)
	}
	if task.Status == "completed" || task.Status == "failed" || task.Status == "cancelled" {
		return fmt.Sprintf("任务 %s 已经是 %s 状态，无需取消。", taskID, task.Status)
	}

	if err := g.taskStore.UpdateStatus(ctx, taskID, "cancelled", WithErrorText("user cancelled")); err != nil {
		return fmt.Sprintf("取消任务失败: %v", err)
	}

	// Best-effort: cancel the running process via BackgroundTaskCanceller.
	if canceller, ok := g.agent.(agent.BackgroundTaskCanceller); ok {
		if err := canceller.CancelBackgroundTask(ctx, taskID); err != nil {
			g.logger.Warn("Background cancel %s: %v", taskID, err)
		}
	}

	return fmt.Sprintf("已取消任务: %s (%s)", taskID, truncateForLark(task.Description, 60))
}

// handleTaskHistory shows completed tasks.
func (g *Gateway) handleTaskHistory(ctx context.Context, msg *incomingMessage) string {
	if g.taskStore == nil {
		return "任务管理未启用。"
	}
	tasks, err := g.taskStore.ListByChat(ctx, msg.chatID, false, 10)
	if err != nil {
		return fmt.Sprintf("查询任务历史失败: %v", err)
	}
	if len(tasks) == 0 {
		return "没有任务记录。"
	}
	return formatTaskHistory(tasks)
}

// formatActiveTaskList formats a list of active tasks.
func (g *Gateway) formatActiveTaskList(tasks []TaskRecord) string {
	max := g.cfg.MaxConcurrentTasks
	if max <= 0 {
		max = defaultMaxConcurrentTasks
	}

	activeCount := 0
	for _, t := range tasks {
		if t.Status == "pending" || t.Status == "running" || t.Status == "waiting_input" {
			activeCount++
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("活跃任务 (%d/%d)\n", activeCount, max))

	for i, t := range tasks {
		elapsed := time.Since(t.CreatedAt)
		statusIcon := taskStatusLabel(t.Status)
		sb.WriteString(fmt.Sprintf("\n[%d] %s · %s · %s · %s",
			i+1, shortID(t.TaskID), t.AgentType, statusIcon, formatDuration(elapsed)))
		if t.Description != "" {
			sb.WriteString(fmt.Sprintf("\n    %s", truncateForLark(t.Description, 60)))
		}
	}

	sb.WriteString("\n\n回复 /task status <id> 查看详情，/task cancel <id> 取消任务。")
	return sb.String()
}

// formatTaskDetail formats a single task's details.
func formatTaskDetail(t TaskRecord) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("任务详情: %s\n", t.TaskID))
	sb.WriteString(fmt.Sprintf("类型: %s\n", t.AgentType))
	sb.WriteString(fmt.Sprintf("状态: %s %s\n", taskStatusLabel(t.Status), t.Status))
	if t.Description != "" {
		sb.WriteString(fmt.Sprintf("描述: %s\n", t.Description))
	}
	sb.WriteString(fmt.Sprintf("创建: %s\n", t.CreatedAt.Format("15:04:05")))
	if !t.CompletedAt.IsZero() {
		sb.WriteString(fmt.Sprintf("完成: %s\n", t.CompletedAt.Format("15:04:05")))
		sb.WriteString(fmt.Sprintf("耗时: %s\n", formatDuration(t.CompletedAt.Sub(t.CreatedAt))))
	} else {
		sb.WriteString(fmt.Sprintf("已运行: %s\n", formatDuration(time.Since(t.CreatedAt))))
	}
	if t.TokensUsed > 0 {
		sb.WriteString(fmt.Sprintf("Tokens: %s\n", formatTokens(t.TokensUsed)))
	}
	if t.Error != "" {
		sb.WriteString(fmt.Sprintf("\n错误:\n%s\n", truncateForLark(t.Error, 500)))
	}
	if t.AnswerPreview != "" {
		sb.WriteString(fmt.Sprintf("\n结果预览:\n%s\n", truncateForLark(t.AnswerPreview, 800)))
	}
	return strings.TrimRight(sb.String(), "\n")
}

// formatTaskHistory formats completed tasks for display.
func formatTaskHistory(tasks []TaskRecord) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("任务历史 (最近 %d 条)\n", len(tasks)))

	for i, t := range tasks {
		statusIcon := taskStatusLabel(t.Status)
		duration := ""
		if !t.CompletedAt.IsZero() {
			duration = formatDuration(t.CompletedAt.Sub(t.CreatedAt))
		} else {
			duration = formatDuration(time.Since(t.CreatedAt))
		}
		sb.WriteString(fmt.Sprintf("\n[%d] %s · %s · %s · %s",
			i+1, shortID(t.TaskID), t.AgentType, statusIcon, duration))
		if t.Description != "" {
			sb.WriteString(fmt.Sprintf("\n    %s", truncateForLark(t.Description, 60)))
		}
		if t.TokensUsed > 0 {
			sb.WriteString(fmt.Sprintf(" · %s tokens", formatTokens(t.TokensUsed)))
		}
	}
	return sb.String()
}

func taskCommandUsage() string {
	return strings.TrimSpace(`
Task command usage:
  /cc <desc>              Dispatch to Claude Code
  /codex <desc>           Dispatch to Codex
  /task <desc>            Dispatch to default agent
  /tasks                  List active tasks
  /task status <id>       Show task details
  /task cancel <id>       Cancel a running task
  /task history           Show completed tasks
  /task help              Show this help

Examples:
  /cc refactor auth module
  /codex optimize database queries
  /task status bg-abc123
`)
}

func taskStatusLabel(status string) string {
	switch status {
	case "pending":
		return "pending"
	case "running":
		return "running"
	case "waiting_input":
		return "waiting"
	case "completed":
		return "done"
	case "failed":
		return "failed"
	case "cancelled":
		return "cancelled"
	default:
		return status
	}
}

func shortID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:12]
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}

func formatTokens(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

func agentShortName(agentType string) string {
	switch agentType {
	case "claude_code":
		return "cc"
	case "codex":
		return "codex"
	default:
		return "task"
	}
}
