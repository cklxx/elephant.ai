package lark

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	appcontext "alex/internal/agent/app/context"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/logging"
	id "alex/internal/utils/id"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

// AgentExecutor captures the agent execution surface needed by the gateway.
type AgentExecutor interface {
	ExecuteTask(ctx context.Context, task string, sessionID string, listener agent.EventListener) (*agent.TaskResult, error)
}

// Gateway bridges Lark bot messages into the agent runtime.
type Gateway struct {
	cfg          Config
	agent        AgentExecutor
	logger       logging.Logger
	client       *lark.Client
	wsClient     *larkws.Client
	sessionLocks sync.Map
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
	logger = logging.OrNop(logger)
	return &Gateway{
		cfg:    cfg,
		agent:  agent,
		logger: logger,
	}, nil
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

	sessionID := g.sessionIDForChat(chatID)
	lock := g.sessionLock(sessionID)
	lock.Lock()
	defer lock.Unlock()

	execCtx := context.Background()
	execCtx = id.WithSessionID(execCtx, sessionID)
	execCtx, _ = id.EnsureLogID(execCtx, id.NewLogID)
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

	result, execErr := g.agent.ExecuteTask(execCtx, content, sessionID, agent.NoopEventListener{})
	reply := g.buildReply(result, execErr)
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

// sendMessage sends a new message to the given chat (used for P2P).
func (g *Gateway) sendMessage(ctx context.Context, chatID, text string) {
	content := textContent(text)
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType("chat_id").
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(chatID).
			MsgType("text").
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

// replyMessage replies to a specific message (used for group chats).
func (g *Gateway) replyMessage(ctx context.Context, messageID, text string) {
	content := textContent(text)
	req := larkim.NewReplyMessageReqBuilder().
		MessageId(messageID).
		Body(larkim.NewReplyMessageReqBodyBuilder().
			MsgType("text").
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

// buildReply constructs the reply string from the agent result.
func (g *Gateway) buildReply(result *agent.TaskResult, execErr error) string {
	reply := ""
	if execErr != nil {
		reply = fmt.Sprintf("执行失败：%v", execErr)
	} else if result != nil {
		reply = strings.TrimSpace(result.Answer)
	}
	if reply == "" {
		return ""
	}
	if g.cfg.ReplyPrefix != "" {
		reply = g.cfg.ReplyPrefix + reply
	}
	return reply
}

// sessionIDForChat derives a deterministic session ID from a chat ID.
func (g *Gateway) sessionIDForChat(chatID string) string {
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

// deref safely dereferences a string pointer.
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
