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
	"alex/internal/agent/domain"
	ports "alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	storage "alex/internal/agent/ports/storage"
	toolports "alex/internal/agent/ports/tools"
	"alex/internal/logging"
	"alex/internal/memory"
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

// AgentExecutor captures the agent execution surface needed by the gateway.
type AgentExecutor interface {
	EnsureSession(ctx context.Context, sessionID string) (*storage.Session, error)
	ExecuteTask(ctx context.Context, task string, sessionID string, listener agent.EventListener) (*agent.TaskResult, error)
}

// Gateway bridges Lark bot messages into the agent runtime.
type Gateway struct {
	cfg           Config
	agent         AgentExecutor
	logger        logging.Logger
	client        *lark.Client
	wsClient      *larkws.Client
	sessionLocks  sync.Map
	eventListener agent.EventListener
	memoryMgr     *larkMemoryManager
	dedupMu       sync.Mutex
	dedupCache    *lru.Cache[string, time.Time]
	now           func() time.Time
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
	dedupCache, err := lru.New[string, time.Time](messageDedupCacheSize)
	if err != nil {
		return nil, fmt.Errorf("lark message deduper init: %w", err)
	}
	logger = logging.OrNop(logger)
	return &Gateway{
		cfg:        cfg,
		agent:      agent,
		logger:     logger,
		dedupCache: dedupCache,
		now:        time.Now,
	}, nil
}

// SetEventListener configures an optional listener to receive workflow events.
func (g *Gateway) SetEventListener(listener agent.EventListener) {
	if g == nil {
		return
	}
	g.eventListener = listener
}

// SetMemoryManager enables automatic memory save/recall for the gateway.
func (g *Gateway) SetMemoryManager(svc memory.Service) {
	if g == nil || svc == nil {
		return
	}
	g.memoryMgr = newLarkMemoryManager(svc, g.logger)
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

// emojiReactionInterceptor wraps an EventListener to intercept emoji events
// and send a Lark reaction before delegating to the original listener.
type emojiReactionInterceptor struct {
	delegate  agent.EventListener
	gateway   *Gateway
	messageID string
	ctx       context.Context
	once      sync.Once
	fired     bool
}

func (i *emojiReactionInterceptor) OnEvent(event agent.AgentEvent) {
	if emojiEvent, ok := event.(*domain.WorkflowPreAnalysisEmojiEvent); ok && emojiEvent.ReactEmoji != "" {
		i.once.Do(func() {
			i.fired = true
			i.gateway.addReaction(i.ctx, i.messageID, emojiEvent.ReactEmoji)
		})
	}
	i.delegate.OnEvent(event)
}

// sendFallback sends the config-level fallback emoji if no dynamic emoji was received.
func (i *emojiReactionInterceptor) sendFallback() {
	if i.fired {
		return
	}
	fallback := i.gateway.cfg.ReactEmoji
	if fallback == "" {
		return
	}
	i.once.Do(func() {
		i.fired = true
		i.gateway.addReaction(i.ctx, i.messageID, fallback)
	})
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

	// Each message gets a fresh session (zero history). Memory recall
	// injects relevant context instead of accumulating session messages.
	memoryID := g.memoryIDForChat(chatID)
	sessionID := fmt.Sprintf("%s-%s", g.cfg.SessionPrefix, id.NewLogID())

	lock := g.sessionLock(memoryID)
	lock.Lock()
	defer lock.Unlock()

	execCtx := context.Background()
	execCtx = id.WithSessionID(execCtx, sessionID)
	execCtx, _ = id.EnsureLogID(execCtx, id.NewLogID)
	execCtx = shared.WithLarkClient(execCtx, g.client)
	execCtx = shared.WithLarkChatID(execCtx, chatID)

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

	if g.cfg.AgentPreset != "" || g.cfg.ToolPreset != "" {
		execCtx = context.WithValue(execCtx, appcontext.PresetContextKey{}, appcontext.PresetConfig{
			AgentPreset: g.cfg.AgentPreset,
			ToolPreset:  g.cfg.ToolPreset,
		})
	}
	if g.cfg.ReplyTimeout > 0 {
		timeoutCtx, cancel := context.WithTimeout(execCtx, g.cfg.ReplyTimeout)
		execCtx = timeoutCtx
		defer cancel()
	}

	listener := g.eventListener
	if listener == nil {
		listener = agent.NoopEventListener{}
	}
	var emojiInterceptor *emojiReactionInterceptor
	if messageID != "" {
		emojiInterceptor = &emojiReactionInterceptor{
			delegate:  listener,
			gateway:   g,
			messageID: messageID,
			ctx:       execCtx,
		}
		listener = emojiInterceptor
	}
	if g.cfg.ShowToolProgress {
		sender := &larkProgressSender{gateway: g, chatID: chatID, messageID: messageID, isGroup: isGroup}
		progressLn := newProgressListener(execCtx, listener, sender, g.logger)
		defer progressLn.Close()
		listener = progressLn
	}
	execCtx = shared.WithParentListener(execCtx, listener)

	// Memory recall: inject relevant past learnings into the task context.
	// Uses memoryID (stable per chat) so memories persist across fresh sessions.
	taskContent := content
	if g.memoryMgr != nil {
		if recalled := g.memoryMgr.RecallForTask(execCtx, memoryID, content); recalled != "" {
			taskContent = recalled + "\n\n" + content
		}
	}

	result, execErr := g.agent.ExecuteTask(execCtx, taskContent, sessionID, listener)
	if emojiInterceptor != nil {
		emojiInterceptor.sendFallback()
	}

	// Memory save: persist important notes from the result.
	if g.memoryMgr != nil {
		g.memoryMgr.SaveFromResult(execCtx, memoryID, result)
	}

	reply := g.buildReply(result, execErr)
	if reply == "" {
		reply = "（无可用回复）"
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
	reply := ""
	if execErr != nil {
		reply = fmt.Sprintf("执行失败：%v", execErr)
	} else if result != nil {
		reply = strings.TrimSpace(result.Answer)
		if reply == "" {
			reply = extractThinkingFallback(result.Messages)
		}
	}
	if reply == "" {
		return ""
	}
	if g.cfg.ReplyPrefix != "" {
		reply = g.cfg.ReplyPrefix + reply
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
	return fmt.Sprintf("%s-%x", g.cfg.SessionPrefix, hash[:8])
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

func (g *Gateway) sessionLock(sessionID string) *sync.Mutex {
	if sessionID == "" {
		return &sync.Mutex{}
	}
	value, _ := g.sessionLocks.LoadOrStore(sessionID, &sync.Mutex{})
	return value.(*sync.Mutex)
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

func (g *Gateway) sendAttachments(ctx context.Context, chatID, messageID string, isGroup bool, result *agent.TaskResult) {
	if result == nil || g.client == nil {
		return
	}

	// Only send attachments explicitly referenced in the final answer.
	attachments := result.Attachments
	if len(attachments) == 0 {
		return
	}
	for name, att := range attachments {
		if isA2UIAttachment(att) {
			delete(attachments, name)
		}
	}
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

// deref safely dereferences a string pointer.
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
