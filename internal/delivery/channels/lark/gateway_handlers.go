package lark

import (
	"context"
	"strings"

	agent "alex/internal/domain/agent/ports/agent"

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
	g.logger.Info("Lark message received: chat_id=%s msg_id=%s sender=%s group=%t len=%d", msg.chatID, msg.messageID, msg.senderID, msg.isGroup, len(msg.content))

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

	// If a task is already running for this chat, inject the new message
	// into the running ReAct loop instead of starting a new task.
	if slot.phase == slotRunning {
		ch := slot.inputCh
		activeSessionID := slot.sessionID
		slot.mu.Unlock()
		if g.tryResolveInputReply(ctx, msg.chatID, strings.TrimSpace(msg.content)) {
			return nil
		}
		g.injectUserInput(ch, activeSessionID, msg)
		return nil
	}

	if strings.HasPrefix(trimmedContent, "/model") || strings.HasPrefix(trimmedContent, "/models") {
		slot.mu.Unlock()
		g.handleModelCommand(msg)
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
	slot.lastTouched = g.currentTime()
	slot.mu.Unlock()

	// Run the task asynchronously so the Lark SDK event handler returns
	// immediately and can ACK the WS frame. Without this, long-running
	// tasks delay the ACK, causing the Lark server to re-deliver the
	// event and produce duplicate responses.
	g.taskWG.Add(1)
	go func(taskCtx context.Context, taskCancel context.CancelFunc, taskToken uint64) {
		defer g.taskWG.Done()
		defer taskCancel()

		awaitingInput := g.runTask(taskCtx, msg, sessionID, inputCh, isResume, taskToken)

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

	return nil
}
