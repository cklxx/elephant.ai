package lark

import (
	"context"
	"errors"
	"strings"
	"time"

	"alex/internal/delivery/channels"
	ports "alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	builtinshared "alex/internal/infra/tools/builtin/shared"
)

// dispatchResult builds the reply from the execution result and sends it to
// the Lark chat, including any attachments. When progressMsgID is non-empty,
// the progress message is edited in-place to become the final reply, avoiding
// message fragmentation.
func (g *Gateway) dispatchResult(execCtx context.Context, msg *incomingMessage, result *agent.TaskResult, execErr error, awaitTracker *awaitQuestionTracker, progressMsgID string, taskToken uint64, guardState *toolFailureGuardState) {
	if errors.Is(execErr, context.Canceled) && g.isIntentionalTaskCancellation(msg.chatID, taskToken) {
		g.logger.Info("Lark task cancelled intentionally: chat=%s msg=%s token=%d", msg.chatID, msg.messageID, taskToken)
		return
	}
	if guardState != nil && guardState.Tripped() {
		dispatchCtx, cancel := detachedContext(execCtx, 15*time.Second)
		defer cancel()
		g.dispatch(dispatchCtx, msg.chatID, replyTarget(msg.messageID, true), "text", textContent(guardState.UserNotice()))
		return
	}

	isAwait := execErr == nil && isResultAwaitingInput(result)
	awaitPrompt, hasAwaitPrompt := agent.AwaitUserInputPrompt{}, false
	if isAwait && result != nil {
		awaitPrompt, hasAwaitPrompt = agent.ExtractAwaitUserInputPrompt(result.Messages)
	}
	reply := ""
	replyContent := ""
	replyMsgType := "text"
	attachmentSummary := ""

	if isAwait && g.cfg.PlanReviewEnabled {
		reply, _, replyContent = g.buildPlanReviewReplyContent(execCtx, msg, result)
	}

	skipReply := isAwait && awaitTracker.Sent()

	if replyContent == "" && !skipReply {
		var textOnlyAttachments map[string]ports.Attachment
		if result != nil && len(result.Attachments) > 0 {
			cfg := builtinshared.GetAutoUploadConfig(execCtx)
			_, textOnlyAttachments = partitionUploadableAttachments(
				result.Attachments, normalizeExtensions(cfg.AllowExts),
			)
		}
		attachmentSummary = buildAttachmentSummary(textOnlyAttachments)
		if reply == "" && isAwait {
			switch {
			case hasAwaitPrompt && len(awaitPrompt.Options) > 0:
				reply = formatNumberedOptions(awaitPrompt.Question, awaitPrompt.Options)
				// Store pending options so numeric replies can be resolved.
				slot := g.getOrCreateSlot(msg.chatID)
				slot.mu.Lock()
				slot.pendingOptions = awaitPrompt.Options
				slot.mu.Unlock()
			case hasAwaitPrompt:
				reply = awaitPrompt.Question
			default:
				reply = "需要你补充信息后继续。"
			}
		}
		if reply == "" {
			reply = g.buildReply(execCtx, result, execErr)
		}
		if reply == "" {
			switch {
			case attachmentSummary != "":
				reply = attachmentSummary
				attachmentSummary = ""
			case execErr != nil:
				sanitized := channels.SanitizeErrorForUser(execErr.Error())
				reply = "不好意思，这次没弄好：" + sanitized + "\n你可以再跟我说一次，或者换个方式描述一下？"
			case isAwait:
				reply = "还需要你补充点信息我才能继续，直接回复就好。"
			default:
				reply = "这次没有生成文本结果。你可以告诉我希望看到什么：总结、下一步计划，或者让我重试？"
			}
		}
		if attachmentSummary != "" {
			reply += "\n\n" + attachmentSummary
		}

		// Enforce 200-rune cap: if reply is still long after rephrase,
		// upload the full text as a document and keep the chat reply short.
		if !isAwait && len([]rune(reply)) > 200 {
			reply = g.truncateWithDoc(execCtx, msg.chatID, msg.messageID, reply)
		}

		replyMsgType, replyContent = smartContent(reply)
	}

	if !skipReply {
		intent := g.buildTerminalDeliveryIntent(execCtx, msg, result, execErr, progressMsgID, replyMsgType, replyContent)
		g.dispatchTerminalIntent(execCtx, intent)
	}
}

func detachedContext(execCtx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	baseCtx := context.Background()
	if execCtx != nil {
		baseCtx = context.WithoutCancel(execCtx)
	}
	return context.WithTimeout(baseCtx, timeout)
}

func (g *Gateway) isIntentionalTaskCancellation(chatID string, taskToken uint64) bool {
	if taskToken == 0 {
		return false
	}
	raw, ok := g.activeSlots.Load(strings.TrimSpace(chatID))
	if !ok {
		return false
	}
	slot, ok := raw.(*sessionSlot)
	if !ok || slot == nil {
		return false
	}
	slot.mu.Lock()
	defer slot.mu.Unlock()
	return slot.intentionalCancelToken == taskToken
}

// buildPlanReviewReplyContent handles plan review marker extraction,
// pending store save, and returns the reply text, message type,
// and content payload.
func (g *Gateway) buildPlanReviewReplyContent(execCtx context.Context, msg *incomingMessage, result *agent.TaskResult) (reply, msgType, content string) {
	marker, ok := extractPlanReviewMarker(result.Messages)
	if !ok {
		return "", "", ""
	}

	reply = buildPlanReviewReply(marker, g.cfg.PlanReviewRequireConfirmation)

	if g.planReviewStore != nil {
		if err := g.planReviewStore.SavePending(execCtx, PlanReviewPending{
			UserID:        msg.senderID,
			ChatID:        msg.chatID,
			RunID:         marker.RunID,
			OverallGoalUI: marker.OverallGoalUI,
			InternalPlan:  marker.InternalPlan,
		}); err != nil {
			g.logger.Warn("Lark plan review pending save failed: %v", err)
		}
	}

	return reply, "text", ""
}

// truncateWithDoc uploads the full reply as a text file and returns a short
// summary. If the upload fails, it falls back to rune-level truncation.
func (g *Gateway) truncateWithDoc(ctx context.Context, chatID, replyToID, fullText string) string {
	// Try to upload the full content as a text file.
	if g.messenger != nil {
		fileKey, err := g.messenger.UploadFile(ctx, []byte(fullText), "详细内容.txt", "stream")
		if err == nil && fileKey != "" {
			// Send the file as a separate message.
			fileContent := buildFileContent(fileKey, "详细内容.txt")
			g.dispatch(ctx, chatID, replyTarget(replyToID, true), "file", fileContent)

			// Return a short summary for the chat reply.
			runes := []rune(fullText)
			if len(runes) > 150 {
				return string(runes[:150]) + "…\n\n详细内容见上方文档。"
			}
			return fullText
		}
	}
	// Fallback: hard truncate.
	return truncateForLark(fullText, 200)
}

func buildFileContent(fileKey, fileName string) string {
	return `{"file_key":"` + fileKey + `","file_name":"` + fileName + `"}`
}

// buildReply constructs the reply string from the agent result, then rephrases
// it into natural conversational Chinese via LLM when available.
func (g *Gateway) buildReply(ctx context.Context, result *agent.TaskResult, execErr error) string {
	reply := channels.BuildReplyCore(g.cfg.BaseConfig, result, execErr)
	if result == nil {
		if execErr != nil {
			sanitized := channels.SanitizeErrorForUser(execErr.Error())
			reply = "不好意思，这次没弄好：" + sanitized + "\n你可以再跟我说一次，或者换个方式描述一下？"
		}
		return channels.ShapeReply7C(reply)
	}
	reply = channels.ShapeReply7C(reply)
	return g.rephraseForUser(ctx, reply, rephraseForeground)
}
