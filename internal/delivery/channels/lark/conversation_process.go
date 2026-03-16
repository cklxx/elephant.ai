package lark

import (
	"context"
	"strings"
	"time"

	"alex/internal/app/agent/llmclient"
	ports "alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	runtimeconfig "alex/internal/shared/config"
)

// conversationVerdict represents the classification outcome for an incoming
// user message when the conversation process is enabled.
type conversationVerdict int

const (
	verdictAnswer   conversationVerdict = iota // direct reply via lightweight LLM
	verdictDelegate                            // spawn a Worker task
	verdictRelay                               // forward to running Worker's inputCh
	verdictFork                                // independent fork session
)

const (
	defaultConversationClassifyTimeout = 3 * time.Second
	defaultConversationAnswerTimeout   = 6 * time.Second
	conversationClassifyMaxTok         = 8
	conversationClassifyTemp           = 0.0
)

var conversationClassifySystemPrompt = strings.TrimSpace(`
你是消息分类器。根据用户消息和当前状态，输出一个词：ANSWER / DELEGATE / RELAY / FORK。

规则：
- 闲聊、问好、简单问答、问进度 → ANSWER
- 需要读写文件、执行命令、搜索等操作 → DELEGATE（无任务运行时）或 FORK（有任务运行时且不相关）
- 有任务运行中 + 消息是对当前任务的补充/修正 → RELAY
- 有任务运行中 + 消息是完全不相关的新问题 → FORK

只输出一个词。
`)

// conversationClassify calls a lightweight LLM to determine the verdict for
// the incoming message. Falls back to verdictDelegate on error or timeout so
// the message follows the normal task execution path.
func (g *Gateway) conversationClassify(ctx context.Context, msg string, snap workerSnapshot) conversationVerdict {
	if g.llmFactory == nil {
		return verdictDelegate
	}

	profile := g.resolveConversationProfile()
	client, _, err := llmclient.GetClientFromProfile(g.llmFactory, profile, nil, false)
	if err != nil {
		g.logger.Warn("conversation classify: failed to get LLM client: %v; fallback DELEGATE", err)
		return verdictDelegate
	}

	timeout := g.cfg.ConversationClassifyTimeout
	if timeout <= 0 {
		timeout = defaultConversationClassifyTimeout
	}

	userPrompt := "当前状态：" + snap.StatusSummary() + "\n\n用户消息：" + strings.TrimSpace(msg)

	llmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()

	resp, err := client.Complete(llmCtx, ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "system", Content: conversationClassifySystemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: conversationClassifyTemp,
		MaxTokens:   conversationClassifyMaxTok,
	})
	if err != nil {
		g.logger.Warn("conversation classify: LLM call failed: %v; fallback DELEGATE", err)
		return verdictDelegate
	}

	return parseClassifyVerdict(resp.Content, snap)
}

// parseClassifyVerdict maps the raw LLM response to a conversationVerdict.
// It validates state consistency: RELAY/FORK require a running worker.
func parseClassifyVerdict(raw string, snap workerSnapshot) conversationVerdict {
	token := strings.ToUpper(strings.TrimSpace(raw))

	switch {
	case strings.HasPrefix(token, "ANSWER"):
		return verdictAnswer
	case strings.HasPrefix(token, "DELEGATE"):
		return verdictDelegate
	case strings.HasPrefix(token, "RELAY"):
		if snap.IsRunning() {
			return verdictRelay
		}
		// No running worker — downgrade to DELEGATE.
		return verdictDelegate
	case strings.HasPrefix(token, "FORK"):
		if snap.IsRunning() {
			return verdictFork
		}
		return verdictDelegate
	default:
		return verdictDelegate
	}
}

// handleViaConversationProcess is the entry point when the conversation
// process feature flag is enabled. It classifies the message, then dispatches
// to the appropriate handler.
//
// Phase 1: skeleton — all verdicts fall through to the existing code path.
// The caller (handleMessageWithOptions) should continue its normal flow
// after this returns false. Returns true only when the message has been
// fully handled and the caller should return immediately.
func (g *Gateway) handleViaConversationProcess(ctx context.Context, msg *incomingMessage, slot *sessionSlot) bool {
	snap := g.snapshotWorker(msg.chatID)
	verdict := g.conversationClassify(ctx, msg.content, snap)

	g.logger.Info(
		"conversation process: chat=%s verdict=%s worker_phase=%d",
		msg.chatID, verdictString(verdict), snap.Phase,
	)

	// Phase 1: all verdicts fall through to existing code path.
	// Phase 2 will wire DELEGATE/RELAY/FORK to existing handlers.
	// Phase 3 will implement ANSWER via conversationAnswer().
	_ = verdict
	return false
}

// conversationAnswer generates a direct reply using a lightweight LLM call.
// Phase 3 implementation — currently a stub that returns empty string.
func (g *Gateway) conversationAnswer(ctx context.Context, msg *incomingMessage, snap workerSnapshot) string {
	// TODO(phase3): implement direct answer via narrateWithLLM pattern
	return ""
}

// resolveConversationProfile returns the LLM profile for conversation
// classification and answer generation.
func (g *Gateway) resolveConversationProfile() runtimeconfig.LLMProfile {
	if g.cfg.ConversationModel != "" {
		p := g.llmProfile
		p.Model = g.cfg.ConversationModel
		return p
	}
	return g.llmProfile
}

// conversationProcessEnabled reports whether the conversation process
// feature flag is on.
func (g *Gateway) conversationProcessEnabled() bool {
	if g.cfg.ConversationProcessEnabled == nil {
		return false
	}
	return *g.cfg.ConversationProcessEnabled
}

// verdictString returns a human-readable label for logging.
func verdictString(v conversationVerdict) string {
	switch v {
	case verdictAnswer:
		return "ANSWER"
	case verdictDelegate:
		return "DELEGATE"
	case verdictRelay:
		return "RELAY"
	case verdictFork:
		return "FORK"
	default:
		return "UNKNOWN"
	}
}

// injectUserInputForRelay is a convenience wrapper that injects a message into
// the running worker's input channel and sends an acknowledgement to the user.
func (g *Gateway) injectUserInputForRelay(ctx context.Context, slot *sessionSlot, msg *incomingMessage) {
	slot.mu.Lock()
	ch := slot.inputCh
	sessionID := slot.sessionID
	slot.mu.Unlock()

	if ch == nil {
		return
	}
	select {
	case ch <- agent.UserInput{Content: msg.content, SenderID: msg.senderID, MessageID: msg.messageID}:
		g.logger.Info("conversation relay: injected into session %s", sessionID)
	default:
		g.logger.Warn("conversation relay: inputCh full for session %s", sessionID)
	}
}
