package lark

import (
	"context"
	"strings"
	"time"

	"alex/internal/app/agent/llmclient"
	ports "alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/shared/utils"
)

const (
	defaultConversationTimeout = 8 * time.Second
	conversationMaxTok         = 300
	conversationTemp           = 0.3

	dispatchWorkerToolName = "dispatch_worker"
	stopWorkerToolName     = "stop_worker"

	conversationChatHistoryMaxRounds = 5
)

var dispatchWorkerTool = ports.ToolDefinition{
	Name:        dispatchWorkerToolName,
	Description: "启动一个后台 Agent 来执行需要工具操作的任务（读写文件、执行命令、搜索等）。调用后 Agent 会在后台异步工作，完成后自动通知用户。",
	Parameters: ports.ParameterSchema{
		Type: "object",
		Properties: map[string]ports.Property{
			"task": {
				Type:        "string",
				Description: "要交给 Agent 执行的任务描述，保留用户原文意图",
			},
		},
		Required: []string{"task"},
	},
}

var stopWorkerTool = ports.ToolDefinition{
	Name:        stopWorkerToolName,
	Description: "停止当前正在后台运行的 Agent 任务。",
	Parameters:  ports.ParameterSchema{Type: "object", Properties: map[string]ports.Property{}},
}

var conversationSystemPrompt = strings.TrimSpace(`
你是用户的 AI 助手。你可以直接回答用户的问题，也可以调用 dispatch_worker 工具把需要执行操作的任务交给后台 Agent。

规则：
1. 闲聊、问好、简单问答、问当前任务进度 → 直接回复，不调用工具
2. 需要读写文件、执行命令、搜索代码、重构等操作 → 调用 dispatch_worker，同时给用户一句简短的确认（如"好，我来看一下"）
3. 回复简洁自然，像同事聊天，不超过 100 字
4. 如果有任务正在执行中，用户问进度，根据提供的状态信息回答
5. 如果有任务正在执行中，用户发来与当前任务相关的补充/修正，调用 dispatch_worker 把补充信息传达（后台会注入到运行中的任务）
6. 如果用户要求停止/取消当前任务（如"停一下""算了""先别做了"），且有任务在执行中 → 调用 stop_worker
`)

// handleViaConversationProcess sends the user message to the gateway's LLM
// with a single tool (dispatch_worker). The text reply goes to the user
// immediately; a dispatch_worker call spawns a background worker.
// Always returns true — fully owns the message lifecycle when enabled.
func (g *Gateway) handleViaConversationProcess(ctx context.Context, msg *incomingMessage, slot *sessionSlot) bool {
	snap := g.snapshotWorker(msg.chatID)

	// Fetch recent chat history for conversational context.
	chatHistory := g.fetchConversationChatHistory(ctx, msg)

	processingReactionID := g.addProcessingReaction(ctx, msg.messageID)
	reply, toolCalls := g.conversationLLM(ctx, msg.content, snap, chatHistory)
	g.removeProcessingReaction(ctx, msg.messageID, processingReactionID)

	// Check if any tool call is dispatch_worker — if so, the worker will
	// produce the final reply, so we suppress the LLM's text reply to
	// avoid sending a duplicate response for the same message.
	hasDispatchWorker := false
	for _, tc := range toolCalls {
		if tc.Name == dispatchWorkerToolName {
			hasDispatchWorker = true
			break
		}
	}

	if reply != "" && !hasDispatchWorker {
		g.dispatchFormattedReply(ctx, msg.chatID, replyTarget(msg.messageID, true), reply)
	}

	for _, tc := range toolCalls {
		switch tc.Name {
		case dispatchWorkerToolName:
			taskArg, _ := tc.Arguments["task"].(string)
			if taskArg == "" {
				taskArg = msg.content
			}
			injected := g.spawnWorker(ctx, msg, slot, snap, taskArg)
			if injected && reply != "" {
				// Notify user that their message was merged into the running task
				// instead of starting a new one.
				g.dispatchFormattedReply(ctx, msg.chatID, "", "你的消息已加入当前任务")
			}
		case stopWorkerToolName:
			g.stopWorkerFromConversation(msg.chatID, slot)
		}
	}

	return true
}

// conversationLLM calls the gateway's shared LLM with the conversation
// system prompt, worker status, and user message. Returns text reply
// and any tool calls.
func (g *Gateway) conversationLLM(ctx context.Context, userMsg string, snap workerSnapshot, chatHistory string) (string, []ports.ToolCall) {
	if g.llmFactory == nil {
		return "", nil
	}

	client, _, err := llmclient.GetClientFromProfile(g.llmFactory, g.llmProfile, nil, false)
	if err != nil {
		g.logger.Warn("conversation LLM: failed to get client: %v", err)
		return "", nil
	}

	llmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultConversationTimeout)
	defer cancel()

	var sb strings.Builder
	sb.WriteString("当前 Worker 状态：")
	sb.WriteString(snap.StatusSummary())
	if chatHistory != "" {
		sb.WriteString("\n\n最近聊天记录：\n")
		sb.WriteString(chatHistory)
	}
	sb.WriteString("\n\n用户消息：")
	sb.WriteString(strings.TrimSpace(userMsg))

	resp, err := client.Complete(llmCtx, ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "system", Content: conversationSystemPrompt},
			{Role: "user", Content: sb.String()},
		},
		Tools:       []ports.ToolDefinition{dispatchWorkerTool, stopWorkerTool},
		Temperature: conversationTemp,
		MaxTokens:   conversationMaxTok,
	})
	if err != nil {
		g.logger.Warn("conversation LLM: call failed: %v", err)
		return "", nil
	}

	g.logger.Info("conversation LLM: reply=%q tool_calls=%d", utils.Truncate(resp.Content, 80, "…"), len(resp.ToolCalls))
	return strings.TrimSpace(resp.Content), resp.ToolCalls
}

// spawnWorker launches a background worker via the existing runTask path.
// If a worker is already running, injects into its inputCh instead.
//
// The running check is done under slot.mu using the live slot.phase — NOT the
// stale snapshot — so that concurrent calls for different messages cannot both
// bypass the guard and spawn a second worker.
// spawnWorker launches a background worker or injects into a running one.
// Returns true if the message was injected into an existing worker, false if
// a new worker was spawned.
func (g *Gateway) spawnWorker(ctx context.Context, msg *incomingMessage, slot *sessionSlot, _ workerSnapshot, taskContent string) bool {
	// Single lock: check current state and either inject or claim the slot.
	slot.mu.Lock()
	if slot.phase == slotRunning {
		ch := slot.inputCh
		sessionID := slot.sessionID
		slot.mu.Unlock()
		if ch != nil {
			select {
			case ch <- agent.UserInput{Content: taskContent, SenderID: msg.senderID, MessageID: msg.messageID}:
				g.logger.Info("conversation: injected into running worker session %s", sessionID)
			default:
				g.logger.Warn("conversation: worker inputCh full for session %s", sessionID)
			}
		}
		return true
	}

	sessionID, isResume := g.resolveSessionForNewTask(ctx, msg.chatID, slot)
	inputCh := make(chan agent.UserInput, 16)
	taskCtx, taskCancel := context.WithCancel(context.Background())
	slot.phase = slotRunning
	slot.inputCh = inputCh
	slot.taskCancel = taskCancel
	slot.taskToken++
	taskToken := slot.taskToken
	slot.sessionID = sessionID
	slot.lastSessionID = sessionID
	slot.taskDesc = strings.TrimSpace(taskContent)
	slot.recentProgress = slot.recentProgress[:0]
	slot.lastTouched = g.currentTime()
	slot.taskStartTime = g.currentTime()
	slot.mu.Unlock()

	workerMsg := *msg
	workerMsg.content = taskContent

	g.launchWorkerGoroutine(&workerMsg, slot, sessionID, inputCh, taskCancel, taskCtx, taskToken, isResume)
	return false
}

// stopWorkerFromConversation cancels the currently running worker for the
// given chat, replicating the /stop semantics used by handleStopCommand.
func (g *Gateway) stopWorkerFromConversation(chatID string, slot *sessionSlot) {
	slot.mu.Lock()
	cancel := slot.taskCancel
	running := slot.phase == slotRunning && cancel != nil
	if running {
		slot.intentionalCancelToken = slot.taskToken
	}
	slot.mu.Unlock()

	if running {
		cancel()
		g.logger.Info("conversation: stopped worker for chat %s", chatID)
	}
}

// fetchConversationChatHistory retrieves recent chat rounds for the
// conversation process LLM context. Returns empty string on failure.
func (g *Gateway) fetchConversationChatHistory(ctx context.Context, msg *incomingMessage) string {
	if g.messenger == nil {
		return ""
	}
	history, err := g.fetchRecentChatRounds(ctx, msg.chatID, msg.messageID, 50, conversationChatHistoryMaxRounds)
	if err != nil {
		g.logger.Warn("conversation: chat history fetch failed: %v", err)
		return ""
	}
	return history
}

// conversationProcessEnabled reports whether the conversation process is on.
func (g *Gateway) conversationProcessEnabled() bool {
	if g.cfg.ConversationProcessEnabled == nil {
		return false
	}
	return *g.cfg.ConversationProcessEnabled
}

