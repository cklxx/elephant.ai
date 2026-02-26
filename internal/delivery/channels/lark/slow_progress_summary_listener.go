package lark

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	appcontext "alex/internal/app/agent/context"
	"alex/internal/app/agent/llmclient"
	domain "alex/internal/domain/agent"
	ports "alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/utils"
)

const (
	defaultSlowProgressSummaryDelay      = 30 * time.Second
	defaultSlowProgressSummaryLLMTimeout = 8 * time.Second
	defaultSlowProgressSummaryMaxSignals = 24
	slowProgressSummaryMaxPromptChars    = 2400
	slowProgressSummaryMaxReplyChars     = 900
)

type slowProgressSignal struct {
	at   time.Time
	text string
}

// slowProgressSummaryListener emits one proactive progress summary when a
// foreground task runs longer than the configured delay.
type slowProgressSummaryListener struct {
	inner   agent.EventListener
	gateway *Gateway
	ctx     context.Context

	chatID    string
	replyToID string
	delay     time.Duration
	now       func() time.Time

	mu          sync.Mutex
	timer       *time.Timer
	closed      bool
	terminal    bool
	summarySent bool
	startedAt   time.Time
	signals     []slowProgressSignal
}

func newSlowProgressSummaryListener(
	ctx context.Context,
	inner agent.EventListener,
	gateway *Gateway,
	chatID string,
	replyToID string,
	delay time.Duration,
) *slowProgressSummaryListener {
	if delay <= 0 {
		delay = defaultSlowProgressSummaryDelay
	}
	l := &slowProgressSummaryListener{
		inner:     inner,
		gateway:   gateway,
		ctx:       ctx,
		chatID:    strings.TrimSpace(chatID),
		replyToID: strings.TrimSpace(replyToID),
		delay:     delay,
		now:       time.Now,
		startedAt: time.Now(),
	}
	l.timer = time.AfterFunc(delay, l.onDelayReached)
	return l
}

func (l *slowProgressSummaryListener) OnEvent(event agent.AgentEvent) {
	l.capture(event)
	if l.inner != nil {
		l.inner.OnEvent(event)
	}
}

func (l *slowProgressSummaryListener) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.closed = true
	if l.timer != nil {
		l.timer.Stop()
		l.timer = nil
	}
}

func (l *slowProgressSummaryListener) capture(event agent.AgentEvent) {
	if event == nil {
		return
	}
	switch e := event.(type) {
	case *domain.WorkflowEventEnvelope:
		l.captureEnvelope(e)
	case *domain.Event:
		l.captureUnified(e)
	}
}

func (l *slowProgressSummaryListener) captureEnvelope(e *domain.WorkflowEventEnvelope) {
	if e == nil {
		return
	}
	eventType := strings.TrimSpace(e.Event)
	if eventType == "" {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return
	}
	if isSlowSummaryTerminalEvent(eventType) {
		l.terminal = true
	}
	if signal, ok := signalFromEnvelope(e); ok {
		l.appendSignal(signal)
	}
}

func (l *slowProgressSummaryListener) captureUnified(e *domain.Event) {
	if e == nil {
		return
	}
	kind := strings.TrimSpace(e.Kind)
	if kind == "" {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return
	}
	if isSlowSummaryTerminalEvent(kind) {
		l.terminal = true
	}
	if signal, ok := signalFromUnified(e); ok {
		l.appendSignal(signal)
	}
}

func (l *slowProgressSummaryListener) appendSignal(signal slowProgressSignal) {
	if signal.text == "" {
		return
	}
	if signal.at.IsZero() {
		signal.at = l.clock()
	}
	l.signals = append(l.signals, signal)
	if len(l.signals) > defaultSlowProgressSummaryMaxSignals {
		excess := len(l.signals) - defaultSlowProgressSummaryMaxSignals
		l.signals = append([]slowProgressSignal(nil), l.signals[excess:]...)
	}
}

func (l *slowProgressSummaryListener) onDelayReached() {
	signals, elapsed, shouldSend := l.prepareSummary()
	if !shouldSend {
		return
	}
	if l.gateway == nil {
		return
	}
	text := l.buildSummary(signals, elapsed)
	if text == "" {
		return
	}

	sendCtx, cancel := context.WithTimeout(l.dispatchContextBase(), 5*time.Second)
	defer cancel()
	if _, err := l.gateway.dispatchMessage(sendCtx, l.chatID, l.replyToID, "text", textContent(text)); err != nil {
		if l.gateway.logger != nil {
			l.gateway.logger.Warn("Lark slow progress summary send failed: %v", err)
		}
	}
}

func (l *slowProgressSummaryListener) prepareSummary() ([]slowProgressSignal, time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed || l.summarySent || l.terminal {
		return nil, 0, false
	}
	if !l.isRunningLocked() {
		return nil, 0, false
	}

	l.summarySent = true
	elapsed := l.clock().Sub(l.startedAt)
	signals := make([]slowProgressSignal, len(l.signals))
	copy(signals, l.signals)
	return signals, elapsed, true
}

func (l *slowProgressSummaryListener) isRunningLocked() bool {
	if l.gateway == nil {
		return false
	}
	raw, ok := l.gateway.activeSlots.Load(l.chatID)
	if !ok {
		return false
	}
	slot, ok := raw.(*sessionSlot)
	if !ok || slot == nil {
		return false
	}
	slot.mu.Lock()
	phase := slot.phase
	slot.mu.Unlock()
	return phase == slotRunning
}

func (l *slowProgressSummaryListener) buildSummary(signals []slowProgressSignal, elapsed time.Duration) string {
	fallback := l.buildFallbackSummary(signals, elapsed)
	if l.gateway == nil || l.gateway.llmFactory == nil {
		return fallback
	}
	profile := l.resolveProfile()
	if utils.IsBlank(profile.Provider) || utils.IsBlank(profile.Model) {
		return fallback
	}

	baseCtx := context.Background()
	if l.ctx != nil {
		baseCtx = context.WithoutCancel(l.ctx)
	}
	llmCtx, cancel := context.WithTimeout(baseCtx, defaultSlowProgressSummaryLLMTimeout)
	defer cancel()

	summary, err := l.generateLLMSummary(llmCtx, signals, elapsed)
	if err != nil {
		if l.gateway.logger != nil {
			l.gateway.logger.Warn("Lark slow progress summary LLM fallback: %v", err)
		}
		return fallback
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return fallback
	}
	text := fmt.Sprintf("任务已运行 %s，最近进展：\n%s\n\n我会在完成后继续给你最终结果。", formatDuration(elapsed), summary)
	return truncateForLark(text, slowProgressSummaryMaxReplyChars)
}

// resolveProfile returns the pinned subscription profile from the task context
// if available, otherwise falls back to the gateway's shared runtime profile.
func (l *slowProgressSummaryListener) resolveProfile() runtimeconfig.LLMProfile {
	if l.ctx != nil {
		if selection, ok := appcontext.GetLLMSelection(l.ctx); ok {
			if utils.HasContent(selection.Provider) && utils.HasContent(selection.Model) {
				return runtimeconfig.LLMProfile{
					Provider: selection.Provider,
					Model:    selection.Model,
					APIKey:   selection.APIKey,
					BaseURL:  selection.BaseURL,
					Headers:  selection.Headers,
				}
			}
		}
	}
	return l.gateway.llmProfile
}

func (l *slowProgressSummaryListener) generateLLMSummary(
	ctx context.Context,
	signals []slowProgressSignal,
	elapsed time.Duration,
) (string, error) {
	client, _, err := llmclient.GetClientFromProfile(l.gateway.llmFactory, l.resolveProfile(), nil, false)
	if err != nil {
		return "", err
	}
	systemPrompt := "你是运行进展播报助手。根据已发生事件写中文进展播报，不猜测未发生事项，不给最终结论。输出 2-4 行，每行以“- ”开头。"
	userPrompt := l.buildLLMPrompt(signals, elapsed)

	resp, err := client.Complete(ctx, ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.2,
		MaxTokens:   220,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Content), nil
}

func (l *slowProgressSummaryListener) buildLLMPrompt(signals []slowProgressSignal, elapsed time.Duration) string {
	var b strings.Builder
	b.WriteString("任务还在执行中。\n")
	b.WriteString("已运行时长：")
	b.WriteString(formatDuration(elapsed))
	b.WriteString("\n最近事件：\n")
	if len(signals) == 0 {
		b.WriteString("- 暂无可用事件细节（可能仍在准备上下文或等待工具返回）。\n")
	} else {
		for i, signal := range tailSignals(signals, 8) {
			offset := signal.at.Sub(l.startedAt)
			if offset < 0 {
				offset = 0
			}
			b.WriteString(fmt.Sprintf("%d. [t+%s] %s\n", i+1, formatDuration(offset), signal.text))
		}
	}
	prompt := b.String()
	if len(prompt) > slowProgressSummaryMaxPromptChars {
		return prompt[:slowProgressSummaryMaxPromptChars]
	}
	return prompt
}

func (l *slowProgressSummaryListener) buildFallbackSummary(signals []slowProgressSignal, elapsed time.Duration) string {
	var b strings.Builder
	b.WriteString("任务已运行 ")
	b.WriteString(formatDuration(elapsed))
	b.WriteString("，仍在执行中。\n")
	if len(signals) == 0 {
		b.WriteString("最近进展：正在准备上下文或等待工具返回。\n")
	} else {
		b.WriteString("最近进展：")
		parts := make([]string, 0, 3)
		for _, signal := range tailSignals(signals, 3) {
			parts = append(parts, signal.text)
		}
		b.WriteString(strings.Join(parts, "；"))
		b.WriteString("\n")
	}
	b.WriteString("我会在完成后继续给你最终结果。")
	return truncateForLark(b.String(), slowProgressSummaryMaxReplyChars)
}

func (l *slowProgressSummaryListener) dispatchContextBase() context.Context {
	baseCtx := context.Background()
	if l.ctx != nil {
		baseCtx = context.WithoutCancel(l.ctx)
	}
	return baseCtx
}

func (l *slowProgressSummaryListener) clock() time.Time {
	if l.now != nil {
		return l.now()
	}
	return time.Now()
}

func isSlowSummaryTerminalEvent(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case types.EventResultFinal, types.EventResultCancelled:
		return true
	default:
		return false
	}
}

func signalFromEnvelope(e *domain.WorkflowEventEnvelope) (slowProgressSignal, bool) {
	if e == nil {
		return slowProgressSignal{}, false
	}
	toolName := strings.TrimSpace(envelopeToolName(e))
	switch strings.TrimSpace(e.Event) {
	case types.EventNodeStarted:
		step := strings.TrimSpace(asString(e.Payload["step_description"]))
		if step == "" {
			step = strings.TrimSpace(e.NodeID)
		}
		if step == "" {
			return slowProgressSignal{}, false
		}
		return slowProgressSignal{at: e.Timestamp(), text: "开始步骤：" + truncateForLark(step, 120)}, true
	case types.EventNodeCompleted:
		step := strings.TrimSpace(asString(e.Payload["step_description"]))
		if step == "" {
			step = strings.TrimSpace(e.NodeID)
		}
		if step == "" {
			return slowProgressSignal{}, false
		}
		return slowProgressSignal{at: e.Timestamp(), text: "完成步骤：" + truncateForLark(step, 120)}, true
	case types.EventToolStarted:
		if toolName == "" {
			return slowProgressSignal{}, false
		}
		return slowProgressSignal{at: e.Timestamp(), text: "开始工具：" + toolName}, true
	case types.EventToolCompleted:
		if toolName == "" {
			toolName = "tool"
		}
		errText := strings.TrimSpace(asString(e.Payload["error"]))
		if errText != "" {
			return slowProgressSignal{
				at:   e.Timestamp(),
				text: "工具失败：" + toolName + "（" + truncateForLark(errText, 80) + ")",
			}, true
		}
		return slowProgressSignal{at: e.Timestamp(), text: "完成工具：" + toolName}, true
	case types.EventNodeOutputSummary:
		content := strings.TrimSpace(asString(e.Payload["content"]))
		if content == "" {
			return slowProgressSignal{}, false
		}
		return slowProgressSignal{at: e.Timestamp(), text: "阶段输出：" + truncateForLark(content, 120)}, true
	default:
		return slowProgressSignal{}, false
	}
}

func signalFromUnified(e *domain.Event) (slowProgressSignal, bool) {
	if e == nil {
		return slowProgressSignal{}, false
	}
	switch strings.TrimSpace(e.Kind) {
	case types.EventNodeStarted:
		step := strings.TrimSpace(e.Data.StepDescription)
		if step == "" {
			return slowProgressSignal{}, false
		}
		return slowProgressSignal{at: e.Timestamp(), text: "开始步骤：" + truncateForLark(step, 120)}, true
	case types.EventNodeCompleted:
		step := strings.TrimSpace(e.Data.StepDescription)
		if step == "" {
			return slowProgressSignal{}, false
		}
		return slowProgressSignal{at: e.Timestamp(), text: "完成步骤：" + truncateForLark(step, 120)}, true
	case types.EventToolStarted:
		toolName := strings.TrimSpace(e.Data.ToolName)
		if toolName == "" {
			return slowProgressSignal{}, false
		}
		return slowProgressSignal{at: e.Timestamp(), text: "开始工具：" + toolName}, true
	case types.EventToolCompleted:
		toolName := strings.TrimSpace(e.Data.ToolName)
		if toolName == "" {
			toolName = "tool"
		}
		if e.Data.Error != nil {
			return slowProgressSignal{
				at:   e.Timestamp(),
				text: "工具失败：" + toolName + "（" + truncateForLark(e.Data.Error.Error(), 80) + ")",
			}, true
		}
		return slowProgressSignal{at: e.Timestamp(), text: "完成工具：" + toolName}, true
	case types.EventNodeOutputSummary:
		content := strings.TrimSpace(e.Data.Content)
		if content == "" {
			return slowProgressSignal{}, false
		}
		return slowProgressSignal{at: e.Timestamp(), text: "阶段输出：" + truncateForLark(content, 120)}, true
	default:
		return slowProgressSignal{}, false
	}
}

func tailSignals(signals []slowProgressSignal, n int) []slowProgressSignal {
	if len(signals) <= n {
		out := make([]slowProgressSignal, len(signals))
		copy(out, signals)
		return out
	}
	out := make([]slowProgressSignal, n)
	copy(out, signals[len(signals)-n:])
	return out
}
