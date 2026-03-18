package lark

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/app/agent/llmclient"
	ports "alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/shared/logging"
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
	Description: "Launch a background agent to execute a task asynchronously. The agent will notify the user when done.",
	Parameters: ports.ParameterSchema{
		Type: "object",
		Properties: map[string]ports.Property{
			"task": {
				Type:        "string",
				Description: "Task description preserving the user's original intent",
			},
		},
		Required: []string{"task"},
	},
}

var stopWorkerTool = ports.ToolDefinition{
	Name:        stopWorkerToolName,
	Description: "Stop the currently running background agent task.",
	Parameters:  ports.ParameterSchema{Type: "object", Properties: map[string]ports.Property{}},
}

var conversationSystemPrompt = strings.TrimSpace(`
You are a conversation router. Your only job is to reply and dispatch.

Decision (pick one):
- Reply directly: greetings, chitchat, progress queries, pure Q&A — anything that requires no action.
- dispatch_worker: everything else — whenever the user wants something done, dispatch it.

IMPORTANT: When calling dispatch_worker, you MUST also include a short text reply acknowledging the request (e.g. "好，我来看一下", "收到，马上处理"). Never call dispatch_worker with an empty text response.
Keep all replies short and natural. Match the user's language.

Task control:
- User sends a follow-up or correction for a running task → dispatch_worker (it injects into the running task).
- User asks to stop ("stop", "cancel", "never mind") and a task is running → stop_worker.

Safety:
- Never fabricate information, tool outputs, or task status.
- Never include secrets, API keys, or credentials in replies.
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
	reply, toolCalls := g.conversationLLM(ctx, msg.senderID, msg.content, snap, chatHistory)
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

	logger := logging.FromContext(ctx, g.logger)
	logger.Info("conversation: decision msg=%s hasDispatchWorker=%t reply_len=%d tool_calls=%d",
		msg.messageID, hasDispatchWorker, len(reply), len(toolCalls))

	// When no worker is dispatched, send the reply directly.
	// When a worker IS dispatched, the reply is held and sent as a quick
	// acknowledgment after we know whether we spawned or injected.
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
			if injected {
				logger.Info("conversation: INJECTED msg=%s into running worker", msg.messageID)
				// Notify user that their message was merged into the running task.
				g.dispatchFormattedReply(ctx, msg.chatID, "", "你的消息已加入当前任务")
			} else {
				logger.Info("conversation: SPAWNED new worker msg=%s task=%s", msg.messageID, utils.Truncate(taskArg, 60, "..."))
				// Send the conversation LLM's quick acknowledgment (e.g. "好，我来看一下").
				// This is NOT a duplicate of the worker's final result — it's an
				// instant ack so the user knows the task was accepted.
				if reply != "" {
					g.dispatchFormattedReply(ctx, msg.chatID, replyTarget(msg.messageID, true), reply)
				}
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
func (g *Gateway) conversationLLM(ctx context.Context, senderID, userMsg string, snap workerSnapshot, chatHistory string) (string, []ports.ToolCall) {
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

	systemPrompt := g.buildConversationSystemPrompt(ctx, senderID)

	var sb strings.Builder
	sb.WriteString("Worker status: ")
	sb.WriteString(snap.StatusSummary())
	if chatHistory != "" {
		sb.WriteString("\n\nRecent chat:\n")
		sb.WriteString(chatHistory)
	}
	sb.WriteString("\n\nUser message: ")
	sb.WriteString(strings.TrimSpace(userMsg))

	resp, err := client.Complete(llmCtx, ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "system", Content: systemPrompt},
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

// buildConversationSystemPrompt composes the conversation router's system
// prompt by prepending memory context (SOUL.md, USER.md, long-term) and
// appending date/timezone when available.
func (g *Gateway) buildConversationSystemPrompt(ctx context.Context, senderID string) string {
	var sections []string

	if g.conversationPromptLoader != nil {
		if memoryCtx := g.conversationPromptLoader(ctx, senderID); memoryCtx != "" {
			sections = append(sections, memoryCtx)
		}
	}

	sections = append(sections, conversationSystemPrompt)

	now := g.currentTime()
	sections = append(sections, fmt.Sprintf("Current date: %s (%s)", now.Format("2006-01-02"), now.Location().String()))

	return strings.Join(sections, "\n\n")
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

