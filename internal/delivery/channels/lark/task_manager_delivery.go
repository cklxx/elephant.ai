package lark

import (
	"context"
	"fmt"
	"strings"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/shared/utils"
	id "alex/internal/shared/utils/id"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func (g *Gateway) buildTerminalDeliveryIntent(execCtx context.Context, msg *incomingMessage, result *agent.TaskResult, execErr error, progressMsgID, msgType, content string) DeliveryIntent {
	runID := ""
	sessionID := ""
	if result != nil {
		runID = strings.TrimSpace(result.RunID)
		sessionID = strings.TrimSpace(result.SessionID)
	}
	if runID == "" {
		runID = strings.TrimSpace(id.RunIDFromContext(execCtx))
	}
	if sessionID == "" {
		sessionID = strings.TrimSpace(id.SessionIDFromContext(execCtx))
	}
	eventType := "result_final"
	switch {
	case execErr != nil:
		eventType = "result_failed"
	case result != nil && strings.EqualFold(strings.TrimSpace(result.StopReason), "await_user_input"):
		eventType = "result_await"
	}

	intent := DeliveryIntent{
		Channel:           chatSessionBindingChannel,
		ChatID:            strings.TrimSpace(msg.chatID),
		ReplyToMessageID:  strings.TrimSpace(msg.messageID),
		ProgressMessageID: strings.TrimSpace(progressMsgID),
		SessionID:         sessionID,
		RunID:             runID,
		EventType:         eventType,
		Sequence:          1,
		MsgType:           msgType,
		Content:           content,
		Status:            DeliveryIntentPending,
	}
	if result != nil && len(result.Attachments) > 0 {
		intent.Attachments = filterReferencedAttachments(result.Attachments, content)
	}
	intent.IdempotencyKey = buildTerminalDeliveryIdempotencyKey(intent)
	return intent
}

func buildTerminalDeliveryIdempotencyKey(intent DeliveryIntent) string {
	runKey := strings.TrimSpace(intent.RunID)
	if runKey == "" {
		runKey = strings.TrimSpace(intent.ReplyToMessageID)
	}
	if runKey == "" {
		runKey = strings.TrimSpace(intent.SessionID)
	}
	if runKey == "" {
		runKey = "unknown"
	}
	return fmt.Sprintf("lark:%s:%s:%s:%d", strings.TrimSpace(intent.ChatID), runKey, strings.TrimSpace(intent.EventType), intent.Sequence)
}

func (g *Gateway) dispatchTerminalIntent(execCtx context.Context, intent DeliveryIntent) {
	mode := normalizeDeliveryMode(g.cfg.DeliveryMode)
	store := g.deliveryOutboxStore

	switch mode {
	case DeliveryModeOutbox:
		if store != nil && g.cfg.DeliveryWorker.Enabled {
			storeCtx, cancel := detachedContext(execCtx, 5*time.Second)
			enqueued, err := store.Enqueue(storeCtx, []DeliveryIntent{intent})
			cancel()
			if err == nil && len(enqueued) > 0 {
				return
			}
			if err != nil {
				g.logger.Warn("Lark outbox enqueue failed in outbox mode, fallback to direct dispatch: %v", err)
			} else {
				g.logger.Warn("Lark outbox enqueue returned empty in outbox mode, fallback to direct dispatch")
			}
		} else {
			g.logger.Warn("Lark outbox mode enabled but outbox store/worker unavailable, fallback to direct dispatch")
		}
	case DeliveryModeShadow:
		var stored DeliveryIntent
		if store != nil {
			storeCtx, cancel := detachedContext(execCtx, 5*time.Second)
			enqueued, err := store.Enqueue(storeCtx, []DeliveryIntent{intent})
			cancel()
			if err != nil {
				g.logger.Warn("Lark outbox enqueue failed in shadow mode: %v", err)
			} else if len(enqueued) > 0 {
				stored = enqueued[0]
			}
		}
		if stored.IntentID != "" && stored.Status == DeliveryIntentSent {
			return
		}
		if err := g.deliverIntent(execCtx, intent); err != nil {
			g.logger.Warn("Lark direct terminal delivery failed in shadow mode: %v", err)
			if store != nil && stored.IntentID != "" {
				storeCtx, cancel := detachedContext(execCtx, 5*time.Second)
				_ = store.MarkDead(storeCtx, stored.IntentID, err.Error())
				cancel()
			}
			return
		}
		if store != nil && stored.IntentID != "" {
			storeCtx, cancel := detachedContext(execCtx, 5*time.Second)
			_ = store.MarkSent(storeCtx, stored.IntentID, g.currentTime())
			cancel()
		}
		return
	}

	if err := g.deliverIntent(execCtx, intent); err != nil {
		g.logger.Warn("Lark direct terminal delivery failed: %v", err)
	}
}

// dispatchMultiMessageReply sends multiple chunks as separate messages,
// editing the progress message in-place for the first chunk and sending
// new reply messages for subsequent chunks with a delay between them.
func (g *Gateway) dispatchMultiMessageReply(execCtx context.Context, msg *incomingMessage, result *agent.TaskResult, execErr error, progressMsgID string, chunks []string) {
	for i, chunk := range chunks {
		chunkMsgType, chunkContent := smartContent(chunk)
		intent := g.buildTerminalDeliveryIntent(execCtx, msg, result, execErr, "", chunkMsgType, chunkContent)
		intent.Sequence = int64(i + 1)
		intent.IdempotencyKey = buildTerminalDeliveryIdempotencyKey(intent)

		if i == 0 && progressMsgID != "" {
			// First chunk: edit progress message in-place.
			intent.ProgressMessageID = progressMsgID
		}

		// Last chunk: attach any attachments.
		if i == len(chunks)-1 && result != nil && len(result.Attachments) > 0 {
			intent.Attachments = filterReferencedAttachments(result.Attachments, chunkContent)
		} else {
			intent.Attachments = nil
		}

		g.dispatchTerminalIntent(execCtx, intent)

		// Delay between messages to simulate typing rhythm.
		if i < len(chunks)-1 {
			time.Sleep(messageSplitDelay)
		}
	}
}

// drainAndReprocess drains any remaining messages from the input channel after
// a task finishes and reprocesses each as a new task. This handles messages that
// arrived between the last ReAct iteration drain and the task completion.
// Messages are processed sequentially in a single goroutine to preserve ordering.
func (g *Gateway) drainAndReprocess(ch chan agent.UserInput, chatID, chatType string) {
	var remaining []agent.UserInput
	for {
		select {
		case msg := <-ch:
			remaining = append(remaining, msg)
		default:
			goto done
		}
	}
done:
	if len(remaining) == 0 {
		return
	}
	g.taskWG.Add(1)
	go func() {
		defer g.taskWG.Done()
		for _, msg := range remaining {
			g.reprocessMessage(chatID, chatType, msg)
		}
	}()
}

// discardPendingInputs drains and drops remaining in-flight messages that were
// not consumed by the running task. This avoids automatically starting a new
// task round when the previous run has already produced a terminal answer.
func (g *Gateway) discardPendingInputs(ch chan agent.UserInput, chatID string) {
	dropped := 0
	for {
		select {
		case <-ch:
			dropped++
		default:
			if dropped > 0 {
				g.logger.Info("Discarded %d pending in-flight message(s) for chat %s", dropped, chatID)
			}
			return
		}
	}
}

// reprocessMessage re-injects a drained user input as if it were a fresh Lark
// message. This creates a synthetic P2MessageReceiveV1 event and feeds it back
// through handleMessage so the full pipeline (dedup, session, execution) runs.
func (g *Gateway) reprocessMessage(chatID, chatType string, input agent.UserInput) {
	msgID := input.MessageID
	content := input.Content

	g.logger.Info("Reprocessing drained message for chat %s (msg_id=%s)", chatID, msgID)

	chatType = utils.TrimLower(chatType)
	if chatType == "" {
		chatType = "p2p"
	}
	msgType := "text"
	contentJSON := textContent(content)

	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				MessageId:   &msgID,
				ChatId:      &chatID,
				ChatType:    &chatType,
				MessageType: &msgType,
				Content:     &contentJSON,
			},
			Sender: &larkim.EventSender{
				SenderId: &larkim.UserId{
					OpenId: &input.SenderID,
				},
			},
		},
	}
	if err := g.handleMessageWithOptions(context.Background(), event, messageProcessingOptions{skipDedup: true}); err != nil {
		g.logger.Warn("Reprocess message failed for chat %s: %v", chatID, err)
	}
}
