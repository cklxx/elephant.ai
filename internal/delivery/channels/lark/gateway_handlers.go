package lark

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/shared/logging"
	"alex/internal/shared/utils/id"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// incomingMessage holds the parsed fields from a Lark message event.
type incomingMessage struct {
	chatID              string
	chatType            string
	messageID           string
	senderID            string
	content             string
	isGroup             bool
	isFromBot           bool
	aiChatSessionActive bool // true if this message is part of an AI chat session
}

// isResultAwaitingInput reports whether the task result indicates an
// await_user_input stop reason.
func isResultAwaitingInput(result *agent.TaskResult) bool {
	return result != nil && strings.EqualFold(strings.TrimSpace(result.StopReason), "await_user_input")
}

type messageProcessingOptions struct {
	skipDedup bool
}

// handleMessage is the P2MessageReceiveV1 event handler.
func (g *Gateway) handleMessage(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	return g.handleMessageWithOptions(ctx, event, messageProcessingOptions{})
}

func (g *Gateway) handleMessageWithOptions(ctx context.Context, event *larkim.P2MessageReceiveV1, opts messageProcessingOptions) error {
	msg := g.parseIncomingMessage(event, opts)
	if msg == nil {
		return nil
	}

	logID := id.NewLogID()
	ctx = id.WithLogID(ctx, logID)
	msgLogger := logging.WithLogID(g.logger, logID)
	msgLogger.Info("Lark message received: chat_id=%s msg_id=%s sender=%s group=%t len=%d", msg.chatID, msg.messageID, msg.senderID, msg.isGroup, len(msg.content))

	// AI Chat Coordination: Check if this is a multi-bot chat scenario
	if g.aiCoordinator != nil && msg.isGroup {
		// Skip processing if this is a message from another bot in an active AI chat session
		// to prevent infinite loops
		if msg.isFromBot && g.aiCoordinator.IsMessageFromParticipantBot(msg.chatID, msg.senderID) {
			g.logger.Info("Skipping message from participant bot in AI chat session: chat=%s sender=%s", msg.chatID, msg.senderID)
			return nil
		}

		// Check if this should trigger or participate in an AI chat session
		mentions := extractMentions(event)
		shouldParticipate, waitForTurn := g.aiCoordinator.DetectAndStartSession(
			msg.chatID, msg.messageID, msg.senderID, mentions, g.cfg.AppID,
		)

		if shouldParticipate {
			if waitForTurn {
				g.logger.Info("AI chat: waiting for turn in chat=%s", msg.chatID)
				return nil
			}
			// It's our turn to respond
			g.logger.Info("AI chat: our turn to respond in chat=%s", msg.chatID)
			msg.aiChatSessionActive = true
		}
	}

	slot := g.getOrCreateSlot(msg.chatID)
	slot.mu.Lock()
	slot.lastTouched = g.currentTime()
	trimmedContent := strings.TrimSpace(msg.content)

	// Natural-language status query should be handled immediately, even when a
	// foreground task is currently running.
	if g.isNaturalTaskStatusQuery(trimmedContent) {
		slot.mu.Unlock()
		g.handleNaturalTaskStatusQuery(msg)
		return nil
	}
	if g.isNoticeCommand(trimmedContent) {
		slot.mu.Unlock()
		g.handleNoticeCommand(msg)
		return nil
	}
	if g.isUsageCommand(trimmedContent) {
		slot.mu.Unlock()
		g.handleUsageCommand(msg)
		return nil
	}
	if g.isStopCommand(trimmedContent) {
		g.handleStopCommand(slot, msg) // releases slot.mu
		return nil
	}

	// Handle /new and /reset before in-flight input injection so command
	// intent is not swallowed by the running task input channel.
	if trimmedContent == "/new" {
		g.handleNewSessionCommand(slot, msg) // releases slot.mu
		return nil
	}

	if trimmedContent == "/reset" {
		g.handleResetCommand(slot, msg) // releases slot.mu
		return nil
	}

	// /model command must be handled before conversation process so it works
	// in chat+worker mode without being routed through the chat LLM.
	if strings.HasPrefix(trimmedContent, "/model") {
		slot.mu.Unlock()
		g.handleModelCommand(msg)
		return nil
	}

	// Conversation process: when enabled, a lightweight LLM handles ALL
	// non-command messages. It replies instantly and optionally dispatches
	// a background worker via its dispatch_worker tool.
	if g.conversationProcessEnabled() {
		slot.mu.Unlock()
		msgLogger.Info("message routed: conversation_process=true msg=%s", msg.messageID)
		g.handleViaConversationProcess(ctx, msg)
		return nil
	}

	// If a task is already running for this chat, either inject the new message
	// into the running ReAct loop or fork a child session (btw mode).
	if slot.phase == slotRunning {
		ch := slot.inputCh
		activeSessionID := slot.sessionID
		taskDesc := slot.taskDesc
		slot.mu.Unlock()
		// When btw fork mode is disabled, fall back to the legacy behaviour:
		// inject the message directly into the running task's input channel.
		if !g.cfg.BtwEnabled {
			select {
			case ch <- agent.UserInput{Content: msg.content, SenderID: msg.senderID, MessageID: msg.messageID}:
				g.logger.Info("btw disabled: injecting message into running session %s", activeSessionID)
			default:
				g.logger.Warn("btw disabled: inputCh full, dropping message for session %s", activeSessionID)
			}
			return nil
		}
		// Use the LLM intent router when enabled to decide whether to inject
		// the message into the running task or fork it as a side-question.
		intent := intentBTW
		if g.btwIntentRouterEnabled() {
			intent = g.classifyBtwIntent(ctx, taskDesc, msg.content)
		}
		if intent == intentInject {
			select {
			case ch <- agent.UserInput{Content: msg.content, SenderID: msg.senderID, MessageID: msg.messageID}:
				g.logger.Info("intent router: INJECT message into running session %s", activeSessionID)
			default:
				g.logger.Warn("intent router: inputCh full, falling back to fork for session %s", activeSessionID)
				g.launchForkSession(activeSessionID, ch, msg)
			}
		} else {
			// Fork a child session to handle the side-question independently.
			// The parent session continues uninterrupted.
			g.launchForkSession(activeSessionID, ch, msg)
		}
		return nil
	}

	isPlan := g.isPlanCommand(trimmedContent)
	if g.isTaskCommand(trimmedContent) || isPlan {
		slot.mu.Unlock()
		if isPlan {
			g.handlePlanModeCommand(msg)
		} else {
			g.handleTaskCommand(msg)
		}
		return nil
	}

	// Attention gate: auto-ack non-urgent messages without running a task.
	if g.attentionGate != nil && g.attentionGate.IsEnabled() {
		assessment := g.attentionGate.Assess(trimmedContent)
		if assessment.Route != AttentionRouteNotifyNow && assessment.Route != AttentionRouteEscalate {
			slot.mu.Unlock()
			g.logger.Info(
				"Attention gate: auto-ack message chat=%s msg=%s score=%d route=%s",
				msg.chatID,
				msg.messageID,
				assessment.Score,
				assessment.Route,
			)
			if g.attentionGate.RecordDispatch(msg.chatID, g.currentTime()) {
				g.dispatch(ctx, msg.chatID, replyTarget(msg.messageID, true), "text", textContent(g.attentionGate.AutoAckMessage()))
			}
			return nil
		}
	}

	// Resolve session ID: reuse the awaiting session or create a new one.
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
	slot.taskDesc = strings.TrimSpace(msg.content)
	slot.recentProgress = slot.recentProgress[:0]
	slot.lastTouched = g.currentTime()
	slot.taskStartTime = g.currentTime()
	slot.mu.Unlock()

	g.launchWorkerGoroutine(msg, slot, sessionID, inputCh, taskCancel, taskCtx, taskToken, isResume)

	return nil
}

// launchWorkerGoroutine starts the background task goroutine that runs the
// agent, then cleans up the slot when done. Used by both handleMessage and
// spawnWorker to avoid duplicating the goroutine lifecycle logic.
func (g *Gateway) launchWorkerGoroutine(msg *incomingMessage, slot *sessionSlot, sessionID string, inputCh chan agent.UserInput, taskCancel context.CancelFunc, taskCtx context.Context, taskToken uint64, isResume bool) {
	g.taskWG.Add(1)
	go func(taskCtx context.Context, taskCancel context.CancelFunc, taskToken uint64) {
		defer g.taskWG.Done()
		defer taskCancel()

		// Panic recovery: reset slot to idle and apologize to user so the
		// slot is never stuck in slotRunning after an unexpected panic.
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				g.logger.Warn("CRITICAL: worker goroutine panicked: %v\n%s", r, stack)
				slot.mu.Lock()
				if slot.taskToken == taskToken {
					slot.phase = slotIdle
					slot.inputCh = nil
					slot.taskCancel = nil
					slot.taskStartTime = time.Time{}
					slot.lastTouched = g.currentTime()
				}
				slot.mu.Unlock()
				apologyCtx, apologyCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer apologyCancel()
				g.dispatch(apologyCtx, msg.chatID, replyTarget(msg.messageID, true), "text",
					textContent(fmt.Sprintf("抱歉，任务执行时遇到了意外错误。请重试，或联系管理员。(panic: %v)", r)))
			}
		}()

		awaitingInput, _ := g.runTask(taskCtx, msg, sessionID, inputCh, isResume, taskToken)

		slot.mu.Lock()
		if slot.taskToken == taskToken {
			if slot.intentionalCancelToken == taskToken {
				slot.intentionalCancelToken = 0
			}
			slot.inputCh = nil
			slot.taskCancel = nil
			slot.taskStartTime = time.Time{}
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
			g.drainAndReprocess(inputCh, msg.chatID, msg.chatType, taskToken)
		} else {
			g.discardPendingInputs(inputCh, msg.chatID)
		}
	}(taskCtx, taskCancel, taskToken)
}
