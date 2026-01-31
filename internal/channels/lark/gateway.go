package lark

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"mime"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	appcontext "alex/internal/agent/app/context"
	ports "alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	storage "alex/internal/agent/ports/storage"
	toolports "alex/internal/agent/ports/tools"
	"alex/internal/channels"
	"alex/internal/jsonx"
	"alex/internal/logging"
	artifacts "alex/internal/tools/builtin/artifacts"
	"alex/internal/tools/builtin/shared"
	id "alex/internal/utils/id"

	lru "github.com/hashicorp/golang-lru/v2"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

const (
	messageDedupCacheSize = 2048
	messageDedupTTL       = 10 * time.Minute
)

// AgentExecutor is an alias for the shared channel executor interface.
type AgentExecutor = channels.AgentExecutor

// Gateway bridges Lark bot messages into the agent runtime.
type Gateway struct {
	channels.BaseGateway
	cfg             Config
	agent           AgentExecutor
	logger          logging.Logger
	client          *lark.Client
	wsClient        *larkws.Client
	eventListener   agent.EventListener
	emojiPicker     *emojiPicker
	dedupMu         sync.Mutex
	dedupCache      *lru.Cache[string, time.Time]
	now             func() time.Time
	planReviewStore PlanReviewStore
}

// NewGateway constructs a Lark gateway instance.
func NewGateway(cfg Config, agent AgentExecutor, logger logging.Logger) (*Gateway, error) {
	if agent == nil {
		return nil, fmt.Errorf("lark gateway requires agent executor")
	}
	if strings.TrimSpace(cfg.AppID) == "" || strings.TrimSpace(cfg.AppSecret) == "" {
		return nil, fmt.Errorf("lark gateway requires app_id and app_secret")
	}
	if strings.TrimSpace(cfg.SessionPrefix) == "" {
		cfg.SessionPrefix = "lark"
	}
	if strings.TrimSpace(cfg.SessionMode) == "" {
		cfg.SessionMode = "fresh"
	}
	if strings.TrimSpace(cfg.ToolPreset) == "" {
		cfg.ToolPreset = "full"
	}
	dedupCache, err := lru.New[string, time.Time](messageDedupCacheSize)
	if err != nil {
		return nil, fmt.Errorf("lark message deduper init: %w", err)
	}
	logger = logging.OrNop(logger)
	return &Gateway{
		cfg:         cfg,
		agent:       agent,
		logger:      logger,
		emojiPicker: newEmojiPicker(time.Now().UnixNano(), resolveEmojiPool(cfg.ReactEmoji)),
		dedupCache:  dedupCache,
		now:         time.Now,
	}, nil
}

// SetEventListener configures an optional listener to receive workflow events.
func (g *Gateway) SetEventListener(listener agent.EventListener) {
	if g == nil {
		return
	}
	g.eventListener = listener
}

// SetPlanReviewStore configures the pending plan review store.
func (g *Gateway) SetPlanReviewStore(store PlanReviewStore) {
	if g == nil {
		return
	}
	g.planReviewStore = store
}

// Start creates the Lark SDK client, event dispatcher, and WebSocket client, then blocks.
func (g *Gateway) Start(ctx context.Context) error {
	if !g.cfg.Enabled {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// Build the REST client for sending replies.
	var clientOpts []lark.ClientOptionFunc
	if domain := strings.TrimSpace(g.cfg.BaseDomain); domain != "" {
		clientOpts = append(clientOpts, lark.WithOpenBaseUrl(domain))
	}
	g.client = lark.NewClient(g.cfg.AppID, g.cfg.AppSecret, clientOpts...)

	// Build the event dispatcher and register the message handler.
	eventDispatcher := dispatcher.NewEventDispatcher("", "")
	eventDispatcher.OnP2MessageReceiveV1(g.handleMessage)

	// Build and start the WebSocket client.
	var wsOpts []larkws.ClientOption
	wsOpts = append(wsOpts, larkws.WithEventHandler(eventDispatcher))
	wsOpts = append(wsOpts, larkws.WithLogLevel(larkcore.LogLevelInfo))
	if domain := strings.TrimSpace(g.cfg.BaseDomain); domain != "" {
		wsOpts = append(wsOpts, larkws.WithDomain(domain))
	}
	g.wsClient = larkws.NewClient(g.cfg.AppID, g.cfg.AppSecret, wsOpts...)

	g.logger.Info("Lark gateway connecting (app_id=%s)...", g.cfg.AppID)
	return g.wsClient.Start(ctx)
}

// Stop releases resources. The WebSocket client does not expose a Stop method;
// cancelling the context passed to Start is the primary shutdown mechanism.
func (g *Gateway) Stop() {
	// The Lark WS client is stopped by cancelling its context.
}

// handleMessage is the P2MessageReceiveV1 event handler.
func (g *Gateway) handleMessage(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	if event == nil || event.Event == nil || event.Event.Message == nil {
		return nil
	}
	msg := event.Event.Message

	// Only handle text messages.
	if deref(msg.MessageType) != "text" {
		return nil
	}

	chatType := deref(msg.ChatType)
	isGroup := chatType == "group"
	if isGroup && !g.cfg.AllowGroups {
		return nil
	}
	if !isGroup && !g.cfg.AllowDirect {
		return nil
	}

	// Extract text from JSON content.
	content := g.extractText(deref(msg.Content))
	if content == "" {
		return nil
	}

	chatID := deref(msg.ChatId)
	if chatID == "" {
		g.logger.Warn("Lark message has empty chat_id; skipping")
		return nil
	}

	messageID := deref(msg.MessageId)
	if messageID != "" && g.isDuplicateMessage(messageID) {
		g.logger.Debug("Lark duplicate message skipped: %s", messageID)
		return nil
	}

	senderID := extractSenderID(event)
	memoryID := g.memoryIDForChat(chatID)
	sessionID := memoryID
	if strings.EqualFold(strings.TrimSpace(g.cfg.SessionMode), "fresh") {
		sessionID = fmt.Sprintf("%s-%s", g.cfg.SessionPrefix, id.NewLogID())
	}

	lock := g.SessionLock(memoryID)
	lock.Lock()
	defer lock.Unlock()

	execCtx := channels.BuildBaseContext(g.cfg.BaseConfig, "lark", sessionID, senderID, chatID, isGroup)
	execCtx = appcontext.WithSessionHistory(execCtx, false)
	execCtx = shared.WithLarkClient(execCtx, g.client)
	execCtx = shared.WithLarkChatID(execCtx, chatID)
	execCtx = appcontext.WithPlanReviewEnabled(execCtx, g.cfg.PlanReviewEnabled)

	if strings.TrimSpace(content) == "/reset" {
		if resetter, ok := g.agent.(interface {
			ResetSession(ctx context.Context, sessionID string) error
		}); ok {
			if err := resetter.ResetSession(execCtx, memoryID); err != nil {
				g.logger.Warn("Lark session reset failed: %v", err)
			}
		}
		reply := "已清空对话历史，下次对话将从零开始。"
		if isGroup && messageID != "" {
			g.replyMessage(execCtx, messageID, reply)
		} else {
			g.sendMessage(execCtx, chatID, reply)
		}
		return nil
	}

	session, err := g.agent.EnsureSession(execCtx, sessionID)
	if err != nil {
		g.logger.Warn("Lark ensure session failed: %v", err)
		reply := g.buildReply(nil, fmt.Errorf("ensure session: %w", err))
		if reply == "" {
			reply = "（无可用回复）"
		}
		messageID := deref(msg.MessageId)
		if isGroup && messageID != "" {
			g.replyMessage(execCtx, messageID, reply)
		} else {
			g.sendMessage(execCtx, chatID, reply)
		}
		return nil
	}
	if session != nil && session.ID != "" && session.ID != sessionID {
		sessionID = session.ID
		execCtx = id.WithSessionID(execCtx, sessionID)
	}

	execCtx = channels.ApplyPresets(execCtx, g.cfg.BaseConfig)
	execCtx, cancelTimeout := channels.ApplyTimeout(execCtx, g.cfg.BaseConfig)
	defer cancelTimeout()

	listener := g.eventListener
	if listener == nil {
		listener = agent.NoopEventListener{}
	}
	startEmoji, endEmoji := g.pickReactionEmojis()
	if messageID != "" && startEmoji != "" {
		go g.addReaction(execCtx, messageID, startEmoji)
	}
	if g.cfg.ShowToolProgress {
		sender := &larkProgressSender{gateway: g, chatID: chatID, messageID: messageID, isGroup: isGroup}
		progressLn := newProgressListener(execCtx, listener, sender, g.logger)
		defer progressLn.Close()
		listener = progressLn
	}
	execCtx = shared.WithParentListener(execCtx, listener)

	// Auto chat context: fetch recent messages from the Lark chat.
	taskContent := content
	var pending PlanReviewPending
	hasPending := false
	if g.cfg.PlanReviewEnabled {
		pending, hasPending = g.loadPlanReviewPending(execCtx, session, senderID, chatID)
		if hasPending {
			taskContent = buildPlanFeedbackBlock(pending, content)
			if g.planReviewStore != nil {
				if err := g.planReviewStore.ClearPending(execCtx, senderID, chatID); err != nil {
					g.logger.Warn("Lark plan review pending clear failed: %v", err)
				}
			}
		}
	}
	if g.cfg.AutoChatContext && g.client != nil && isGroup {
		pageSize := g.cfg.AutoChatContextSize
		if pageSize <= 0 {
			pageSize = 20
		}
		if chatHistory, err := fetchRecentChatMessages(execCtx, g.client, chatID, pageSize); err != nil {
			g.logger.Warn("Lark auto chat context fetch failed: %v", err)
		} else if chatHistory != "" {
			if hasPending {
				taskContent = taskContent + "\n\n[近期对话]\n" + chatHistory
			} else {
				taskContent = "[近期对话]\n" + chatHistory + "\n\n" + taskContent
			}
		}
	}

	result, execErr := g.agent.ExecuteTask(execCtx, taskContent, sessionID, listener)
	if messageID != "" && endEmoji != "" {
		go g.addReaction(execCtx, messageID, endEmoji)
	}

	reply := ""
	if execErr == nil && result != nil && strings.EqualFold(strings.TrimSpace(result.StopReason), "await_user_input") && g.cfg.PlanReviewEnabled {
		if marker, ok := extractPlanReviewMarker(result.Messages); ok {
			reply = buildPlanReviewReply(marker, g.cfg.PlanReviewRequireConfirmation)
			if g.planReviewStore != nil {
				if err := g.planReviewStore.SavePending(execCtx, PlanReviewPending{
					UserID:        senderID,
					ChatID:        chatID,
					RunID:         marker.RunID,
					OverallGoalUI: marker.OverallGoalUI,
					InternalPlan:  marker.InternalPlan,
				}); err != nil {
					g.logger.Warn("Lark plan review pending save failed: %v", err)
				}
			}
		}
	}
	if reply == "" {
		reply = g.buildReply(result, execErr)
	}
	if reply == "" {
		reply = "（无可用回复）"
	}
	if summary := buildAttachmentSummary(result); summary != "" {
		reply += "\n\n" + summary
	}

	if isGroup && messageID != "" {
		g.replyMessage(execCtx, messageID, reply)
	} else {
		g.sendMessage(execCtx, chatID, reply)
	}
	g.sendAttachments(execCtx, chatID, messageID, isGroup, result)

	return nil
}

func (g *Gateway) isDuplicateMessage(messageID string) bool {
	if messageID == "" {
		return false
	}
	g.dedupMu.Lock()
	defer g.dedupMu.Unlock()

	if g.dedupCache == nil {
		cache, err := lru.New[string, time.Time](messageDedupCacheSize)
		if err != nil {
			return false
		}
		g.dedupCache = cache
	}

	nowFn := g.now
	if nowFn == nil {
		nowFn = time.Now
	}
	now := nowFn()

	if ts, ok := g.dedupCache.Get(messageID); ok {
		if now.Sub(ts) <= messageDedupTTL {
			return true
		}
		g.dedupCache.Remove(messageID)
	}
	g.dedupCache.Add(messageID, now)
	return false
}

// sendMessage sends a new message to the given chat (used for P2P).
func (g *Gateway) sendMessage(ctx context.Context, chatID, text string) {
	g.sendMessageTyped(ctx, chatID, "text", textContent(text))
}

// replyMessage replies to a specific message (used for group chats).
func (g *Gateway) replyMessage(ctx context.Context, messageID, text string) {
	g.replyMessageTyped(ctx, messageID, "text", textContent(text))
}

func (g *Gateway) sendMessageTyped(ctx context.Context, chatID, msgType, content string) {
	if g.client == nil {
		return
	}
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType("chat_id").
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(chatID).
			MsgType(msgType).
			Content(content).
			Build()).
		Build()

	resp, err := g.client.Im.Message.Create(ctx, req)
	if err != nil {
		g.logger.Warn("Lark send message failed: %v", err)
		return
	}
	if !resp.Success() {
		g.logger.Warn("Lark send message error: code=%d msg=%s", resp.Code, resp.Msg)
	}
}

func (g *Gateway) replyMessageTyped(ctx context.Context, messageID, msgType, content string) {
	if g.client == nil {
		return
	}
	req := larkim.NewReplyMessageReqBuilder().
		MessageId(messageID).
		Body(larkim.NewReplyMessageReqBodyBuilder().
			MsgType(msgType).
			Content(content).
			Build()).
		Build()

	resp, err := g.client.Im.Message.Reply(ctx, req)
	if err != nil {
		g.logger.Warn("Lark reply message failed: %v", err)
		return
	}
	if !resp.Success() {
		g.logger.Warn("Lark reply message error: code=%d msg=%s", resp.Code, resp.Msg)
	}
}

// sendMessageTypedWithID sends a new message and returns the message ID.
func (g *Gateway) sendMessageTypedWithID(ctx context.Context, chatID, msgType, content string) (string, error) {
	if g.client == nil {
		return "", fmt.Errorf("lark client not initialized")
	}
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType("chat_id").
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(chatID).
			MsgType(msgType).
			Content(content).
			Build()).
		Build()

	resp, err := g.client.Im.Message.Create(ctx, req)
	if err != nil {
		return "", err
	}
	if !resp.Success() {
		return "", fmt.Errorf("lark send message error: code=%d msg=%s", resp.Code, resp.Msg)
	}
	if resp.Data == nil || resp.Data.MessageId == nil {
		return "", fmt.Errorf("lark send message missing message_id")
	}
	return *resp.Data.MessageId, nil
}

// replyMessageTypedWithID replies to a message and returns the new message ID.
func (g *Gateway) replyMessageTypedWithID(ctx context.Context, messageID, msgType, content string) (string, error) {
	if g.client == nil {
		return "", fmt.Errorf("lark client not initialized")
	}
	req := larkim.NewReplyMessageReqBuilder().
		MessageId(messageID).
		Body(larkim.NewReplyMessageReqBodyBuilder().
			MsgType(msgType).
			Content(content).
			Build()).
		Build()

	resp, err := g.client.Im.Message.Reply(ctx, req)
	if err != nil {
		return "", err
	}
	if !resp.Success() {
		return "", fmt.Errorf("lark reply message error: code=%d msg=%s", resp.Code, resp.Msg)
	}
	if resp.Data == nil || resp.Data.MessageId == nil {
		return "", fmt.Errorf("lark reply message missing message_id")
	}
	return *resp.Data.MessageId, nil
}

// updateMessage updates an existing text message in-place using the Lark
// im/v1/messages/:message_id (PUT) API.
func (g *Gateway) updateMessage(ctx context.Context, messageID, text string) error {
	if g.client == nil {
		return fmt.Errorf("lark client not initialized")
	}
	req := larkim.NewUpdateMessageReqBuilder().
		MessageId(messageID).
		Body(larkim.NewUpdateMessageReqBodyBuilder().
			MsgType("text").
			Content(textContent(text)).
			Build()).
		Build()

	resp, err := g.client.Im.Message.Update(ctx, req)
	if err != nil {
		return err
	}
	if !resp.Success() {
		return fmt.Errorf("lark update message error: code=%d msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

// addReaction adds an emoji reaction to the specified message.
func (g *Gateway) addReaction(ctx context.Context, messageID, emojiType string) {
	if g.client == nil || messageID == "" || emojiType == "" {
		g.logger.Warn("Lark add reaction failed: client=%v messageID=%q emojiType=%q", g.client, messageID, emojiType)
		return
	}
	req := larkim.NewCreateMessageReactionReqBuilder().
		MessageId(messageID).
		Body(larkim.NewCreateMessageReactionReqBodyBuilder().
			ReactionType(larkim.NewEmojiBuilder().
				EmojiType(emojiType).
				Build()).
			Build()).
		Build()

	resp, err := g.client.Im.V1.MessageReaction.Create(ctx, req)
	if err != nil {
		g.logger.Warn("Lark add reaction failed: %v", err)
		return
	}
	if !resp.Success() {
		g.logger.Warn("Lark add reaction error: code=%d msg=%s", resp.Code, resp.Msg)
	}
}

// buildReply constructs the reply string from the agent result.
func (g *Gateway) buildReply(result *agent.TaskResult, execErr error) string {
	reply := channels.BuildReplyCore(g.cfg.BaseConfig, result, execErr)
	if reply == "" && result != nil {
		// Lark-specific fallback: check thinking content for models that reason but produce no text.
		if fallback := extractThinkingFallback(result.Messages); fallback != "" {
			reply = fallback
			if g.cfg.ReplyPrefix != "" {
				reply = g.cfg.ReplyPrefix + reply
			}
		}
	}
	return reply
}

// extractThinkingFallback scans messages in reverse for the last assistant
// message with non-empty thinking content. This is a safety net for models
// that reason but produce no text output.
func extractThinkingFallback(msgs []ports.Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		msg := msgs[i]
		if msg.Role != "assistant" {
			continue
		}
		for _, part := range msg.Thinking.Parts {
			text := strings.TrimSpace(part.Text)
			if text != "" {
				return text
			}
		}
	}
	return ""
}

// memoryIDForChat derives a deterministic memory identity from a chat ID.
// This stable ID is used for memory save/recall across fresh sessions.
func (g *Gateway) memoryIDForChat(chatID string) string {
	hash := sha1.Sum([]byte(chatID))
	return fmt.Sprintf("%s-%x", g.cfg.SessionPrefix, hash[:12])
}

// extractText parses the JSON content from a Lark text message.
// The content field is a JSON string like: {"text":"hello"}
func (g *Gateway) extractText(raw string) string {
	if raw == "" {
		return ""
	}
	var parsed struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		g.logger.Warn("Lark message content parse failed: %v", err)
		return ""
	}
	return strings.TrimSpace(parsed.Text)
}

// textContent builds the JSON content payload for a Lark text message.
func textContent(text string) string {
	payload, _ := json.Marshal(map[string]string{"text": text})
	return string(payload)
}

func imageContent(imageKey string) string {
	payload, _ := json.Marshal(map[string]string{"image_key": imageKey})
	return string(payload)
}

func fileContent(fileKey string) string {
	payload, _ := json.Marshal(map[string]string{"file_key": fileKey})
	return string(payload)
}

const (
	planReviewMarkerStart = "<plan_review_pending>"
	planReviewMarkerEnd   = "</plan_review_pending>"
)

type planReviewMarker struct {
	RunID         string
	OverallGoalUI string
	InternalPlan  any
}

func (g *Gateway) loadPlanReviewPending(ctx context.Context, session *storage.Session, userID, chatID string) (PlanReviewPending, bool) {
	if g == nil || userID == "" || chatID == "" {
		return PlanReviewPending{}, false
	}
	if g.planReviewStore != nil {
		pending, ok, err := g.planReviewStore.GetPending(ctx, userID, chatID)
		if err != nil {
			g.logger.Warn("Lark plan review pending load failed: %v", err)
			return PlanReviewPending{}, false
		}
		if ok {
			return pending, true
		}
		return PlanReviewPending{}, false
	}
	if session == nil || len(session.Messages) == 0 {
		return PlanReviewPending{}, false
	}
	if marker, ok := extractPlanReviewMarker(session.Messages); ok {
		return PlanReviewPending{
			UserID:        userID,
			ChatID:        chatID,
			RunID:         marker.RunID,
			OverallGoalUI: marker.OverallGoalUI,
			InternalPlan:  marker.InternalPlan,
		}, true
	}
	return PlanReviewPending{}, false
}

func extractPlanReviewMarker(messages []ports.Message) (planReviewMarker, bool) {
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if strings.ToLower(strings.TrimSpace(msg.Role)) != "system" {
			continue
		}
		if marker, ok := parsePlanReviewMarker(msg.Content); ok {
			return marker, true
		}
	}
	return planReviewMarker{}, false
}

func parsePlanReviewMarker(content string) (planReviewMarker, bool) {
	start := strings.Index(content, planReviewMarkerStart)
	end := strings.Index(content, planReviewMarkerEnd)
	if start == -1 || end == -1 || end <= start {
		return planReviewMarker{}, false
	}
	body := strings.TrimSpace(content[start+len(planReviewMarkerStart) : end])
	if body == "" {
		return planReviewMarker{}, false
	}
	marker := planReviewMarker{}
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "run_id:"):
			marker.RunID = strings.TrimSpace(strings.TrimPrefix(line, "run_id:"))
		case strings.HasPrefix(line, "overall_goal_ui:"):
			marker.OverallGoalUI = strings.TrimSpace(strings.TrimPrefix(line, "overall_goal_ui:"))
		case strings.HasPrefix(line, "internal_plan:"):
			raw := strings.TrimSpace(strings.TrimPrefix(line, "internal_plan:"))
			if raw != "" {
				var plan any
				if err := jsonx.Unmarshal([]byte(raw), &plan); err == nil {
					marker.InternalPlan = plan
				} else {
					marker.InternalPlan = raw
				}
			}
		}
	}
	if strings.TrimSpace(marker.OverallGoalUI) == "" {
		return planReviewMarker{}, false
	}
	return marker, true
}

func buildPlanReviewReply(marker planReviewMarker, requireConfirmation bool) string {
	var sb strings.Builder
	sb.WriteString("计划确认\n")
	if marker.OverallGoalUI != "" {
		sb.WriteString("目标: ")
		sb.WriteString(marker.OverallGoalUI)
		sb.WriteString("\n")
	}
	if marker.InternalPlan != nil {
		sb.WriteString("\n计划:\n")
		if data, err := jsonx.MarshalIndent(marker.InternalPlan, "", "  "); err == nil {
			sb.WriteString(string(data))
		} else {
			sb.WriteString(fmt.Sprintf("%v", marker.InternalPlan))
		}
		sb.WriteString("\n")
	}
	if requireConfirmation {
		sb.WriteString("\n请回复 OK 继续，或直接回复修改意见。")
	}
	return strings.TrimSpace(sb.String())
}

func buildPlanFeedbackBlock(pending PlanReviewPending, userFeedback string) string {
	var sb strings.Builder
	sb.WriteString("<plan_feedback>\n")
	sb.WriteString("plan:\n")
	if pending.OverallGoalUI != "" {
		sb.WriteString("goal: ")
		sb.WriteString(strings.TrimSpace(pending.OverallGoalUI))
		sb.WriteString("\n")
	}
	if pending.InternalPlan != nil {
		sb.WriteString("internal_plan: ")
		if data, err := jsonx.MarshalIndent(pending.InternalPlan, "", "  "); err == nil {
			sb.WriteString(string(data))
		} else {
			sb.WriteString(fmt.Sprintf("%v", pending.InternalPlan))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\nuser_feedback:\n")
	sb.WriteString(strings.TrimSpace(userFeedback))
	sb.WriteString("\n\ninstruction: If the feedback changes the plan, call plan() again; otherwise continue with the next step.\n")
	sb.WriteString("</plan_feedback>")
	return strings.TrimSpace(sb.String())
}

func (g *Gateway) sendAttachments(ctx context.Context, chatID, messageID string, isGroup bool, result *agent.TaskResult) {
	if result == nil || g.client == nil {
		return
	}

	attachments := filterNonA2UIAttachments(result.Attachments)
	if len(attachments) == 0 {
		return
	}

	ctx = shared.WithAllowLocalFetch(ctx)
	ctx = toolports.WithAttachmentContext(ctx, attachments, nil)
	client := artifacts.NewAttachmentHTTPClient(artifacts.AttachmentFetchTimeout, "LarkAttachment")

	names := sortedAttachmentNames(attachments)
	for _, name := range names {
		att := attachments[name]
		payload, mediaType, err := artifacts.ResolveAttachmentBytes(ctx, "["+name+"]", client)
		if err != nil {
			g.logger.Warn("Lark attachment %s resolve failed: %v", name, err)
			continue
		}

		if isImageAttachment(att, mediaType, name) {
			imageKey, err := g.uploadImage(ctx, payload)
			if err != nil {
				g.logger.Warn("Lark image upload failed (%s): %v", name, err)
				continue
			}
			if isGroup && messageID != "" {
				g.replyMessageTyped(ctx, messageID, "image", imageContent(imageKey))
			} else {
				g.sendMessageTyped(ctx, chatID, "image", imageContent(imageKey))
			}
			continue
		}

		fileName := fileNameForAttachment(att, name)
		fileType := larkFileType(fileTypeForAttachment(fileName, mediaType))
		fileKey, err := g.uploadFile(ctx, payload, fileName, fileType)
		if err != nil {
			g.logger.Warn("Lark file upload failed (%s): %v", name, err)
			continue
		}
		if isGroup && messageID != "" {
			g.replyMessageTyped(ctx, messageID, "file", fileContent(fileKey))
		} else {
			g.sendMessageTyped(ctx, chatID, "file", fileContent(fileKey))
		}
	}
}

func filterNonA2UIAttachments(attachments map[string]ports.Attachment) map[string]ports.Attachment {
	if len(attachments) == 0 {
		return nil
	}
	filtered := make(map[string]ports.Attachment, len(attachments))
	for name, att := range attachments {
		if isA2UIAttachment(att) {
			continue
		}
		filtered[name] = att
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func collectAttachmentsFromResult(result *agent.TaskResult) map[string]ports.Attachment {
	if result == nil || len(result.Messages) == 0 {
		return nil
	}

	attachments := make(map[string]ports.Attachment)
	for _, msg := range result.Messages {
		mergeAttachments(attachments, msg.Attachments)
		if len(msg.ToolResults) > 0 {
			for _, res := range msg.ToolResults {
				mergeAttachments(attachments, res.Attachments)
			}
		}
	}
	if len(attachments) == 0 {
		return nil
	}
	return attachments
}

func mergeAttachments(out map[string]ports.Attachment, incoming map[string]ports.Attachment) {
	if len(incoming) == 0 {
		return
	}
	for key, att := range incoming {
		name := strings.TrimSpace(key)
		if name == "" {
			name = strings.TrimSpace(att.Name)
		}
		if name == "" {
			continue
		}
		if _, exists := out[name]; exists {
			continue
		}
		if att.Name == "" {
			att.Name = name
		}
		out[name] = att
	}
}

// buildAttachmentSummary creates a text summary of non-A2UI attachments
// with CDN URLs appended to the reply. This consolidates attachment
// references into the summary message so users see everything in one place.
func buildAttachmentSummary(result *agent.TaskResult) string {
	if result == nil {
		return ""
	}
	attachments := result.Attachments
	if len(attachments) == 0 {
		return ""
	}
	names := sortedAttachmentNames(attachments)
	var lines []string
	for _, name := range names {
		att := attachments[name]
		if isA2UIAttachment(att) {
			continue
		}
		uri := strings.TrimSpace(att.URI)
		if uri == "" || strings.HasPrefix(strings.ToLower(uri), "data:") {
			lines = append(lines, fmt.Sprintf("- %s", name))
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", name, uri))
	}
	if len(lines) == 0 {
		return ""
	}
	return "---\n[Attachments]\n" + strings.Join(lines, "\n")
}

func sortedAttachmentNames(attachments map[string]ports.Attachment) []string {
	names := make([]string, 0, len(attachments))
	for name := range attachments {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func isA2UIAttachment(att ports.Attachment) bool {
	media := strings.ToLower(strings.TrimSpace(att.MediaType))
	format := strings.ToLower(strings.TrimSpace(att.Format))
	profile := strings.ToLower(strings.TrimSpace(att.PreviewProfile))
	return strings.Contains(media, "a2ui") || format == "a2ui" || strings.Contains(profile, "a2ui")
}

func isImageAttachment(att ports.Attachment, mediaType, name string) bool {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(mediaType)), "image/") {
		return true
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(att.MediaType)), "image/") {
		return true
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".bmp", ".webp":
		return true
	default:
		return false
	}
}

func fileNameForAttachment(att ports.Attachment, fallback string) string {
	name := strings.TrimSpace(att.Name)
	if name == "" {
		name = strings.TrimSpace(fallback)
	}
	if name == "" {
		name = "attachment"
	}
	if filepath.Ext(name) == "" {
		if ext := extensionForMediaType(att.MediaType); ext != "" {
			name += ext
		}
	}
	return name
}

// larkSupportedFileTypes lists the file_type values accepted by the Lark
// im/v1/files upload API. Any extension not in this set must be sent as "stream".
var larkSupportedFileTypes = map[string]bool{
	"opus": true, "mp4": true, "pdf": true,
	"doc": true, "xls": true, "ppt": true,
	"stream": true,
}

// larkFileType maps a raw file extension to a Lark-compatible file_type value.
func larkFileType(ext string) string {
	lower := strings.ToLower(strings.TrimSpace(ext))
	if larkSupportedFileTypes[lower] {
		return lower
	}
	return "stream"
}

func fileTypeForAttachment(name, mediaType string) string {
	if ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), "."); ext != "" {
		return ext
	}
	if ext := strings.TrimPrefix(extensionForMediaType(mediaType), "."); ext != "" {
		return ext
	}
	return "bin"
}

func extensionForMediaType(mediaType string) string {
	trimmed := strings.TrimSpace(mediaType)
	if trimmed == "" {
		return ""
	}
	exts, err := mime.ExtensionsByType(trimmed)
	if err != nil || len(exts) == 0 {
		return ""
	}
	return exts[0]
}

func (g *Gateway) uploadImage(ctx context.Context, payload []byte) (string, error) {
	if g.client == nil {
		return "", fmt.Errorf("lark client not initialized")
	}
	req := larkim.NewCreateImageReqBuilder().
		Body(larkim.NewCreateImageReqBodyBuilder().
			ImageType("message").
			Image(bytes.NewReader(payload)).
			Build()).
		Build()

	resp, err := g.client.Im.V1.Image.Create(ctx, req)
	if err != nil {
		return "", err
	}
	if !resp.Success() {
		return "", fmt.Errorf("lark image upload failed: code=%d msg=%s", resp.Code, resp.Msg)
	}
	if resp.Data == nil || resp.Data.ImageKey == nil || strings.TrimSpace(*resp.Data.ImageKey) == "" {
		return "", fmt.Errorf("lark image upload missing image_key")
	}
	return *resp.Data.ImageKey, nil
}

func (g *Gateway) uploadFile(ctx context.Context, payload []byte, fileName, fileType string) (string, error) {
	if g.client == nil {
		return "", fmt.Errorf("lark client not initialized")
	}
	req := larkim.NewCreateFileReqBuilder().
		Body(larkim.NewCreateFileReqBodyBuilder().
			FileType(fileType).
			FileName(fileName).
			File(bytes.NewReader(payload)).
			Build()).
		Build()

	resp, err := g.client.Im.V1.File.Create(ctx, req)
	if err != nil {
		return "", err
	}
	if !resp.Success() {
		return "", fmt.Errorf("lark file upload failed: code=%d msg=%s", resp.Code, resp.Msg)
	}
	if resp.Data == nil || resp.Data.FileKey == nil || strings.TrimSpace(*resp.Data.FileKey) == "" {
		return "", fmt.Errorf("lark file upload missing file_key")
	}
	return *resp.Data.FileKey, nil
}

// extractSenderID extracts the sender open_id from a Lark message event.
func extractSenderID(event *larkim.P2MessageReceiveV1) string {
	if event == nil || event.Event == nil || event.Event.Sender == nil || event.Event.Sender.SenderId == nil {
		return ""
	}
	return deref(event.Event.Sender.SenderId.OpenId)
}

// deref safely dereferences a string pointer.
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
