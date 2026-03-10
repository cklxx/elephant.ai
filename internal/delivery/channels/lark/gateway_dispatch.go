package lark

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/shared/utils"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// NotifyCompletion implements agent.BackgroundCompletionNotifier. It writes
// the final task status directly to TaskStore, ensuring persistence even when
// the event listener chain is broken (e.g. SerializingEventListener idle timeout).
func (g *Gateway) NotifyCompletion(ctx context.Context, taskID, status, answer, errText, mergeStatus string, tokensUsed int) {
	if g == nil || g.taskStore == nil {
		return
	}
	// Use a detached context: the caller's task context may already be cancelled
	// (e.g. for cancelled tasks), but this persistence write must still succeed.
	storeCtx := context.WithoutCancel(ctx)
	var opts []TaskUpdateOption
	if answer != "" {
		opts = append(opts, WithAnswerPreview(truncateForLark(answer, 1500)))
	}
	if errText != "" {
		opts = append(opts, WithErrorText(truncateForLark(errText, 1500)))
	}
	if tokensUsed > 0 {
		opts = append(opts, WithTokensUsed(tokensUsed))
	}
	if mergeStatus != "" {
		opts = append(opts, WithMergeStatus(mergeStatus))
	}
	if err := g.taskStore.UpdateStatus(storeCtx, taskID, status, opts...); err != nil {
		g.logger.Warn("CompletionNotifier: TaskStore update failed for %s: %v", taskID, err)
	}
}

// PersistBridgeMeta implements agent.BridgeMetaPersister. It saves bridge
// subprocess metadata to the task store for resilience (orphan adoption on restart).
func (g *Gateway) PersistBridgeMeta(ctx context.Context, taskID string, info any) {
	if g == nil || g.taskStore == nil {
		return
	}
	type bridgeMetaSetter interface {
		SetBridgeMeta(ctx context.Context, taskID string, info any) error
	}
	if setter, ok := g.taskStore.(bridgeMetaSetter); ok {
		storeCtx := context.WithoutCancel(ctx)
		if err := setter.SetBridgeMeta(storeCtx, taskID, info); err != nil {
			g.logger.Warn("BridgeMetaPersister: SetBridgeMeta failed for %s: %v", taskID, err)
		}
	}
}

// InjectMessage constructs a synthetic P2MessageReceiveV1 event and feeds it
// through handleMessage. This is the primary entry point for scenario tests:
// it exercises the full pipeline (dedup, session, context, execution, reply)
// without requiring a WebSocket connection.
func (g *Gateway) InjectMessage(ctx context.Context, chatID, chatType, senderID, messageID, text string) error {
	msgType := "text"
	contentJSON := textContent(text)
	if chatType == "" {
		chatType = "p2p"
	}
	if hub, ok := g.messenger.(*injectCaptureHub); ok && isInjectSyntheticMessageID(messageID) {
		hub.recordInjectedIncoming(chatID, messageID, senderID, msgType, contentJSON, g.currentTime())
	}

	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				MessageId:   &messageID,
				ChatId:      &chatID,
				ChatType:    &chatType,
				MessageType: &msgType,
				Content:     &contentJSON,
			},
			Sender: &larkim.EventSender{
				SenderId: &larkim.UserId{
					OpenId: &senderID,
				},
			},
		},
	}
	return g.handleMessage(ctx, event)
}

func (g *Gateway) isDuplicateMessage(messageID, eventID string) bool {
	return g.dedup.isDuplicate(messageID, eventID)
}

// dispatchMessage sends a message to a Lark chat. When replyToID is non-empty
// the message is sent as a reply to that message; otherwise a new message is
// created in the chat identified by chatID. Returns the new message ID.
func (g *Gateway) dispatchMessage(ctx context.Context, chatID, replyToID, msgType, content string) (string, error) {
	if g.messenger == nil {
		return "", fmt.Errorf("lark messenger not initialized")
	}

	send := func(currentType, currentContent string) (string, error) {
		if replyToID != "" {
			return g.messenger.ReplyMessage(ctx, replyToID, currentType, currentContent)
		}
		return g.messenger.SendMessage(ctx, chatID, currentType, currentContent)
	}

	messageID, err := send(msgType, content)
	if err == nil || !strings.EqualFold(strings.TrimSpace(msgType), "post") || !isPostPayloadInvalidError(err) {
		return messageID, err
	}

	fallbackText := flattenPostContentToText(content)
	if strings.TrimSpace(fallbackText) == "" {
		fallbackText = "本次富文本结果渲染失败，已回退为纯文本发送。"
	}
	g.logger.Warn("Lark post dispatch fallback to text: %v", err)
	return send("text", textContent(fallbackText))
}

func isPostPayloadInvalidError(err error) bool {
	if err == nil {
		return false
	}
	lower := utils.TrimLower(err.Error())
	if lower == "" {
		return false
	}
	return strings.Contains(lower, "message_content_text_tag") ||
		strings.Contains(lower, "invalid message content") ||
		strings.Contains(lower, "text field can't be nil")
}

// dispatch is a fire-and-forget wrapper around dispatchMessage that logs errors.
func (g *Gateway) dispatch(ctx context.Context, chatID, replyToID, msgType, content string) {
	if _, err := g.dispatchMessage(ctx, chatID, replyToID, msgType, content); err != nil {
		g.logger.Warn("Lark dispatch message failed: %v", err)
	}
}

// replyTarget returns the message ID to reply to when allowed.
// An empty ID or disallowed replies indicates no reply target.
func replyTarget(messageID string, allowReply bool) string {
	if !allowReply || messageID == "" {
		return ""
	}
	if isInjectSyntheticMessageID(messageID) {
		// Synthetic inject message IDs are not valid Lark open_message_id values.
		// Falling back to SendMessage keeps inject runs observable in real chats.
		return ""
	}
	return messageID
}

// updateMessage updates an existing message in-place using the given format.
func (g *Gateway) updateMessage(ctx context.Context, messageID, msgType, content string) error {
	if g.messenger == nil {
		return fmt.Errorf("lark messenger not initialized")
	}
	return g.messenger.UpdateMessage(ctx, messageID, msgType, content)
}

// addReaction adds an emoji reaction to the specified message and returns the reaction ID.
// Synthetic (inject_*) message IDs are handled transparently by the injectCaptureHub
// messenger wrapper, which records the call without hitting the real Lark API.
func (g *Gateway) addReaction(ctx context.Context, messageID, emojiType string) string {
	if g.messenger == nil || messageID == "" || emojiType == "" {
		g.logger.Warn("Lark add reaction skipped: messenger=%v messageID=%q emojiType=%q", g.messenger != nil, messageID, emojiType)
		return ""
	}
	reactionID, err := g.messenger.AddReaction(ctx, messageID, emojiType)
	if err != nil {
		g.logger.Warn("Lark add reaction failed: %v", err)
		return ""
	}
	return reactionID
}

// deleteReaction removes an emoji reaction from the specified message.
func (g *Gateway) deleteReaction(ctx context.Context, messageID, reactionID string) {
	if g.messenger == nil || messageID == "" || reactionID == "" {
		return
	}
	if err := g.messenger.DeleteReaction(ctx, messageID, reactionID); err != nil {
		g.logger.Warn("Lark delete reaction failed: %v", err)
	}
}
