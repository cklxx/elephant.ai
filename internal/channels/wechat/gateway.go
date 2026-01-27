package wechat

import (
	"context"
	"crypto/sha1"
	"fmt"
	"io"
	"strings"
	"sync"

	appcontext "alex/internal/agent/app/context"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/logging"
	id "alex/internal/utils/id"

	"github.com/eatmoreapple/openwechat"
)

// AgentExecutor captures the agent execution surface needed by the gateway.
type AgentExecutor interface {
	ExecuteTask(ctx context.Context, task string, sessionID string, listener agent.EventListener) (*agent.TaskResult, error)
}

// Gateway bridges WeChat messages into the agent runtime.
type Gateway struct {
	cfg          Config
	agent        AgentExecutor
	logger       logging.Logger
	bot          *openwechat.Bot
	self         *openwechat.Self
	hotStorage   io.ReadWriteCloser
	sessionLocks sync.Map
}

// NewGateway constructs a WeChat gateway instance.
func NewGateway(cfg Config, agent AgentExecutor, logger logging.Logger) (*Gateway, error) {
	if agent == nil {
		return nil, fmt.Errorf("wechat gateway requires agent executor")
	}
	if strings.TrimSpace(cfg.SessionPrefix) == "" {
		cfg.SessionPrefix = "wechat"
	}
	if strings.TrimSpace(cfg.LoginMode) == "" {
		cfg.LoginMode = "desktop"
	}
	logger = logging.OrNop(logger)
	return &Gateway{
		cfg:    cfg,
		agent:  agent,
		logger: logger,
	}, nil
}

// Start runs the WeChat gateway and blocks until the bot exits.
func (g *Gateway) Start(ctx context.Context) error {
	if !g.cfg.Enabled {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	bot := openwechat.DefaultBot(g.loginMode(), openwechat.WithContextOption(ctx))
	bot.MessageHandler = g.handleMessage
	g.bot = bot

	if err := g.login(bot); err != nil {
		return err
	}
	self, err := bot.GetCurrentUser()
	if err != nil {
		return fmt.Errorf("wechat current user: %w", err)
	}
	g.self = self
	g.logger.Info("WeChat gateway ready: user=%s", strings.TrimSpace(self.NickName))

	return bot.Block()
}

// Stop attempts to log out and release resources.
func (g *Gateway) Stop() {
	if g.bot != nil {
		if err := g.bot.Logout(); err != nil {
			g.logger.Warn("WeChat logout failed: %v", err)
		}
	}
	if g.hotStorage != nil {
		_ = g.hotStorage.Close()
		g.hotStorage = nil
	}
}

func (g *Gateway) loginMode() openwechat.BotPreparer {
	mode := strings.ToLower(strings.TrimSpace(g.cfg.LoginMode))
	switch mode {
	case "", "desktop":
		return openwechat.Desktop
	case "normal", "web":
		return openwechat.Normal
	default:
		g.logger.Warn("Unknown WeChat login_mode %q; defaulting to desktop", g.cfg.LoginMode)
		return openwechat.Desktop
	}
}

func (g *Gateway) login(bot *openwechat.Bot) error {
	if g.cfg.HotLogin && strings.TrimSpace(g.cfg.HotLoginStoragePath) != "" {
		storage := openwechat.NewFileHotReloadStorage(g.cfg.HotLoginStoragePath)
		g.hotStorage = storage
		return bot.HotLogin(storage, openwechat.NewRetryLoginOption())
	}
	return bot.Login()
}

func (g *Gateway) handleMessage(msg *openwechat.Message) {
	if msg == nil {
		return
	}
	if msg.IsSendBySelf() || !msg.IsText() {
		return
	}
	isGroup := msg.IsSendByGroup()
	if isGroup && !g.cfg.AllowGroups {
		return
	}
	if !isGroup && !g.cfg.AllowDirect {
		return
	}
	sender, err := msg.Sender()
	if err != nil || sender == nil {
		g.logger.Warn("WeChat sender lookup failed: %v", err)
		return
	}
	conversationKey := g.conversationKey(sender)
	if conversationKey == "" {
		g.logger.Warn("WeChat conversation key empty; skipping")
		return
	}
	if !g.isConversationAllowed(conversationKey) {
		g.logger.Debug("WeChat conversation blocked: %s", conversationKey)
		return
	}

	content := strings.TrimSpace(msg.Content)
	if isGroup && g.cfg.MentionOnly {
		if !g.isMentioned(content) {
			return
		}
		content = g.stripMention(content)
	}
	if content == "" {
		return
	}

	sessionID := g.sessionIDForConversation(conversationKey)
	lock := g.sessionLock(sessionID)
	lock.Lock()
	defer lock.Unlock()

	ctx := context.Background()
	ctx = id.WithSessionID(ctx, sessionID)
	ctx, _ = id.EnsureLogID(ctx, id.NewLogID)
	if g.cfg.AgentPreset != "" || g.cfg.ToolPreset != "" {
		ctx = context.WithValue(ctx, appcontext.PresetContextKey{}, appcontext.PresetConfig{
			AgentPreset: g.cfg.AgentPreset,
			ToolPreset:  g.cfg.ToolPreset,
		})
	}
	if g.cfg.ReplyTimeout > 0 {
		timeoutCtx, cancel := context.WithTimeout(ctx, g.cfg.ReplyTimeout)
		ctx = timeoutCtx
		defer cancel()
	}

	result, execErr := g.agent.ExecuteTask(ctx, content, sessionID, agent.NoopEventListener{})
	reply := g.buildReply(msg, result, execErr)
	if reply == "" {
		reply = "（无可用回复）"
	}
	if _, err := msg.ReplyText(reply); err != nil {
		g.logger.Warn("WeChat reply failed: %v", err)
	}
}

func (g *Gateway) buildReply(msg *openwechat.Message, result *agent.TaskResult, execErr error) string {
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
	if msg != nil && msg.IsSendByGroup() && g.cfg.ReplyWithMention {
		sender, err := msg.SenderInGroup()
		if err == nil && sender != nil {
			if name := displayName(sender); name != "" {
				reply = fmt.Sprintf("@%s %s", name, reply)
			}
		}
	}
	return reply
}

func (g *Gateway) conversationKey(user *openwechat.User) string {
	if user == nil {
		return ""
	}
	if value := strings.TrimSpace(user.ID()); value != "" {
		return value
	}
	if value := strings.TrimSpace(user.UserName); value != "" {
		return value
	}
	if value := strings.TrimSpace(user.NickName); value != "" {
		return value
	}
	return ""
}

func (g *Gateway) sessionIDForConversation(key string) string {
	hash := sha1.Sum([]byte(key))
	return fmt.Sprintf("%s-%x", g.cfg.SessionPrefix, hash[:8])
}

func (g *Gateway) isConversationAllowed(key string) bool {
	if len(g.cfg.AllowedConversationIDs) == 0 {
		return true
	}
	for _, value := range g.cfg.AllowedConversationIDs {
		if strings.TrimSpace(value) == key {
			return true
		}
	}
	return false
}

func (g *Gateway) isMentioned(content string) bool {
	if g.self == nil {
		return true
	}
	for _, token := range g.selfMentionTokens() {
		if strings.Contains(content, token) {
			return true
		}
	}
	return false
}

func (g *Gateway) stripMention(content string) string {
	if g.self == nil {
		return strings.TrimSpace(content)
	}
	trimmed := content
	for _, token := range g.selfMentionTokens() {
		trimmed = strings.ReplaceAll(trimmed, token, "")
	}
	return strings.TrimSpace(trimmed)
}

func (g *Gateway) selfMentionTokens() []string {
	if g.self == nil {
		return nil
	}
	names := []string{
		strings.TrimSpace(g.self.NickName),
		strings.TrimSpace(g.self.DisplayName),
		strings.TrimSpace(g.self.RemarkName),
	}
	tokens := make([]string, 0, len(names))
	for _, name := range names {
		if name == "" {
			continue
		}
		tokens = append(tokens, "@"+name)
	}
	return tokens
}

func (g *Gateway) sessionLock(sessionID string) *sync.Mutex {
	if sessionID == "" {
		return &sync.Mutex{}
	}
	value, _ := g.sessionLocks.LoadOrStore(sessionID, &sync.Mutex{})
	return value.(*sync.Mutex)
}

func displayName(user *openwechat.User) string {
	if user == nil {
		return ""
	}
	if value := strings.TrimSpace(user.DisplayName); value != "" {
		return value
	}
	if value := strings.TrimSpace(user.RemarkName); value != "" {
		return value
	}
	if value := strings.TrimSpace(user.NickName); value != "" {
		return value
	}
	if value := strings.TrimSpace(user.UserName); value != "" {
		return value
	}
	if value := strings.TrimSpace(user.ID()); value != "" {
		return value
	}
	return ""
}
