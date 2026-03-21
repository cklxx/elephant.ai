package lark

import (
	"context"
	"errors"
	"strings"
	"time"

	"alex/internal/delivery/channels"
	ports "alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/shared/errsanitize"
	larkclient "alex/internal/infra/lark"
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
				reply = g.rephraseForUser(execCtx, "状态：等待输入\n需要用户补充信息后继续。", rephraseForeground)
			}
		}
		if reply == "" {
			reply = g.tieredDelivery(execCtx, msg.chatID, msg.messageID, result, execErr)
		}
		if reply == "" {
			switch {
			case attachmentSummary != "":
				reply = attachmentSummary
				attachmentSummary = ""
			case execErr != nil:
				sanitized := errsanitize.ForUser(execErr.Error())
				reply = g.rephraseForUser(execCtx, "状态：失败\n原因："+sanitized, rephraseForeground)
			case isAwait:
				reply = g.rephraseForUser(execCtx, "状态：等待输入\n需要用户补充信息后继续。", rephraseForeground)
			default:
				reply = g.rephraseForUser(execCtx, "状态：完成\n未生成文本结果。用户可以选择：总结、下一步计划，或重试。", rephraseForeground)
			}
		}
		if attachmentSummary != "" {
			reply += "\n\n" + attachmentSummary
		}

		replyMsgType, replyContent = smartContent(reply)
	}

	if !skipReply {
		// For await prompts, keep the reply as a single message so numbered
		// options are never split across separate messages.
		chunks := splitMessage(reply)
		if !isAwait && len(chunks) > 1 {
			g.dispatchMultiMessageReply(execCtx, msg, result, execErr, progressMsgID, chunks)
		} else {
			intent := g.buildTerminalDeliveryIntent(execCtx, msg, result, execErr, progressMsgID, replyMsgType, replyContent)
			g.dispatchTerminalIntent(execCtx, intent)
		}
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

// docOverflowThreshold is the rune count above which delivery-layer output is
// converted to a Feishu doc (or uploaded as a file) instead of sent inline.
const docOverflowThreshold = 800

// overflowToDoc creates a Feishu document when fullText exceeds the overflow
// threshold, returning a short summary with a doc link. Falls back to file
// upload, then hard truncation.
func (g *Gateway) overflowToDoc(ctx context.Context, chatID, replyToID, fullText, title string) string {
	if len([]rune(fullText)) <= docOverflowThreshold {
		return fullText
	}

	// Try creating a Feishu doc with the full content.
	if g.client != nil {
		lc := larkclient.Wrap(g.client)
		doc, err := lc.Docx().CreateDocument(ctx, larkclient.CreateDocumentRequest{
			Title: title,
		})
		if err == nil {
			// Best-effort: make the doc editable via link for org members.
			if permErr := lc.Drive().SetLinkShareEdit(ctx, doc.DocumentID, "docx"); permErr != nil {
				g.logger.Warn("Lark overflowToDoc set edit permission failed: %v", permErr)
			}
			// Get the page block to write content into.
			blocks, _, _, listErr := lc.Docx().ListDocumentBlocks(ctx, doc.DocumentID, 1, "")
			if listErr == nil && len(blocks) > 0 {
				writeErr := lc.Docx().WriteMarkdown(ctx, doc.DocumentID, blocks[0].BlockID, fullText)
				if writeErr == nil {
					docURL := larkclient.BuildDocumentURL(g.cfg.BaseDomain, doc.DocumentID)
					summary := []rune(fullText)
					if len(summary) > 150 {
						summary = summary[:150]
					}
					return string(summary) + "…\n\n详细内容见文档: " + docURL
				}
			}
		}
		g.logger.Warn("Lark overflowToDoc doc creation failed, falling back to file upload: %v", err)
	}

	// Fallback: upload as .txt file.
	return g.truncateWithDoc(ctx, chatID, replyToID, fullText)
}

// tieredDelivery builds the reply using a three-tier strategy based on rune length:
//   - short (≤ DeliveryShortThreshold): send directly, no LLM rephrase
//   - medium (≤ DeliveryDocThreshold): rephrase with maxTok=400
//   - long (> DeliveryDocThreshold): create doc with full content, rephrase summary + doc link
//
// When doc creation fails (both doc and file upload), falls back to the full
// shaped reply so splitMessage can chunk it in the caller.
func (g *Gateway) tieredDelivery(ctx context.Context, chatID, replyToID string, result *agent.TaskResult, execErr error) string {
	raw := channels.BuildReplyCore(g.cfg.BaseConfig, result, execErr)
	if result == nil {
		if execErr != nil {
			sanitized := errsanitize.ForUser(execErr.Error())
			raw = "不好意思，这次没弄好：" + sanitized + "\n你可以再跟我说一次，或者换个方式描述一下？"
		}
		return channels.ShapeReply7C(raw)
	}
	shaped := channels.ShapeReply7C(raw)
	runeCount := len([]rune(shaped))

	shortThreshold := g.cfg.DeliveryShortThreshold
	if shortThreshold <= 0 {
		shortThreshold = defaultDeliveryShortThreshold
	}
	docThreshold := g.cfg.DeliveryDocThreshold
	if docThreshold <= 0 {
		docThreshold = defaultDeliveryDocThreshold
	}

	switch {
	case runeCount <= shortThreshold:
		g.logger.Info("delivery: tier=short runes=%d", runeCount)
		return shaped

	case runeCount <= docThreshold:
		g.logger.Info("delivery: tier=medium runes=%d", runeCount)
		return g.rephraseForUser(ctx, shaped, rephraseForeground)

	default:
		g.logger.Info("delivery: tier=long runes=%d", runeCount)
		docResult := g.overflowToDoc(ctx, chatID, replyToID, shaped, "ALEX 回复详情")
		// If overflowToDoc truncated without creating a doc or uploading a
		// file (i.e. the result is shorter than the input and contains no doc
		// link or file reference), return the full shaped text so the
		// caller's splitMessage path can chunk it instead of losing content.
		docRunes := len([]rune(docResult))
		if docRunes < runeCount && !strings.Contains(docResult, "详细内容见") {
			return shaped
		}
		return g.rephraseForUser(ctx, docResult, rephraseForeground)
	}
}
