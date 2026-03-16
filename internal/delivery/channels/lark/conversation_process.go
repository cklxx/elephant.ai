package lark

import (
	"context"
	"strings"
	"time"

	"alex/internal/app/agent/llmclient"
	ports "alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
)

const (
	defaultConversationTimeout = 8 * time.Second
	conversationMaxTok         = 300
	conversationTemp           = 0.3

	dispatchWorkerToolName = "dispatch_worker"
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

var conversationSystemPrompt = strings.TrimSpace(`
你是用户的 AI 助手。你可以直接回答用户的问题，也可以调用 dispatch_worker 工具把需要执行操作的任务交给后台 Agent。

规则：
1. 闲聊、问好、简单问答、问当前任务进度 → 直接回复，不调用工具
2. 需要读写文件、执行命令、搜索代码、重构等操作 → 调用 dispatch_worker，同时给用户一句简短的确认（如"好，我来看一下"）
3. 回复简洁自然，像同事聊天，不超过 100 字
4. 如果有任务正在执行中，用户问进度，根据提供的状态信息回答
5. 如果有任务正在执行中，用户发来与当前任务相关的补充/修正，调用 dispatch_worker 把补充信息传达（后台会注入到运行中的任务）
`)

// handleViaConversationProcess sends the user message to the gateway's LLM
// with a single tool (dispatch_worker). The text reply goes to the user
// immediately; a dispatch_worker call spawns a background worker.
// Always returns true — fully owns the message lifecycle when enabled.
func (g *Gateway) handleViaConversationProcess(ctx context.Context, msg *incomingMessage, slot *sessionSlot) bool {
	snap := g.snapshotWorker(msg.chatID)
	reply, toolCalls := g.conversationLLM(ctx, msg.content, snap)

	if reply != "" {
		g.dispatch(ctx, msg.chatID, replyTarget(msg.messageID, true), "text", textContent(reply))
	}

	for _, tc := range toolCalls {
		if tc.Name != dispatchWorkerToolName {
			continue
		}
		taskArg, _ := tc.Arguments["task"].(string)
		if taskArg == "" {
			taskArg = msg.content
		}
		g.spawnWorker(ctx, msg, slot, snap, taskArg)
	}

	return true
}

// conversationLLM calls the gateway's shared LLM with the conversation
// system prompt, worker status, and user message. Returns text reply
// and any tool calls.
func (g *Gateway) conversationLLM(ctx context.Context, userMsg string, snap workerSnapshot) (string, []ports.ToolCall) {
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
	sb.WriteString("\n\n用户消息：")
	sb.WriteString(strings.TrimSpace(userMsg))

	resp, err := client.Complete(llmCtx, ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "system", Content: conversationSystemPrompt},
			{Role: "user", Content: sb.String()},
		},
		Tools:       []ports.ToolDefinition{dispatchWorkerTool},
		Temperature: conversationTemp,
		MaxTokens:   conversationMaxTok,
	})
	if err != nil {
		g.logger.Warn("conversation LLM: call failed: %v", err)
		return "", nil
	}

	g.logger.Info("conversation LLM: reply=%q tool_calls=%d", truncateLog(resp.Content, 80), len(resp.ToolCalls))
	return strings.TrimSpace(resp.Content), resp.ToolCalls
}

// spawnWorker launches a background worker via the existing runTask path.
// If a worker is already running, injects into its inputCh instead.
func (g *Gateway) spawnWorker(ctx context.Context, msg *incomingMessage, slot *sessionSlot, snap workerSnapshot, taskContent string) {
	if snap.IsRunning() {
		slot.mu.Lock()
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
		return
	}

	slot.mu.Lock()
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
	slot.lastTouched = g.currentTime()
	slot.mu.Unlock()

	workerMsg := *msg
	workerMsg.content = taskContent

	g.taskWG.Add(1)
	go func(taskCtx context.Context, taskCancel context.CancelFunc, taskToken uint64) {
		defer g.taskWG.Done()
		defer taskCancel()

		awaitingInput := g.runTask(taskCtx, &workerMsg, sessionID, inputCh, isResume, taskToken)

		slot.mu.Lock()
		if slot.intentionalCancelToken == taskToken {
			slot.intentionalCancelToken = 0
		}
		if slot.taskToken == taskToken {
			slot.inputCh = nil
			slot.taskCancel = nil
			if awaitingInput {
				slot.phase = slotAwaitingInput
				slot.lastSessionID = slot.sessionID
			} else {
				slot.phase = slotIdle
				slot.sessionID = ""
			}
			slot.lastTouched = g.currentTime()
		}
		slot.mu.Unlock()
		if awaitingInput {
			g.drainAndReprocess(inputCh, msg.chatID, msg.chatType)
		} else {
			g.discardPendingInputs(inputCh, msg.chatID)
		}
	}(taskCtx, taskCancel, taskToken)
}

// conversationProcessEnabled reports whether the conversation process is on.
func (g *Gateway) conversationProcessEnabled() bool {
	if g.cfg.ConversationProcessEnabled == nil {
		return false
	}
	return *g.cfg.ConversationProcessEnabled
}

func truncateLog(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "…"
}
