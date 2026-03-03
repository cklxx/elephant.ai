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

// slowProgressSummaryListener emits periodic proactive progress summaries when
// a foreground task runs longer than the configured delay.
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
	summarySent int
	intervals   []time.Duration
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
	intervals := buildSlowSummaryIntervals(delay)
	firstDelay := delay
	if len(intervals) > 0 {
		firstDelay = intervals[0]
	}
	l := &slowProgressSummaryListener{
		inner:     inner,
		gateway:   gateway,
		ctx:       ctx,
		chatID:    strings.TrimSpace(chatID),
		replyToID: strings.TrimSpace(replyToID),
		delay:     delay,
		intervals: intervals,
		now:       time.Now,
		startedAt: time.Now(),
	}
	l.timer = time.AfterFunc(firstDelay, l.onDelayReached)
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
	if shouldSend {
		if l.gateway != nil {
			text := l.buildSummary(signals, elapsed)
			if text != "" {
				sendCtx, cancel := context.WithTimeout(l.dispatchContextBase(), 5*time.Second)
				defer cancel()
				if _, err := l.gateway.dispatchMessage(sendCtx, l.chatID, l.replyToID, "text", textContent(text)); err != nil {
					if l.gateway.logger != nil {
						l.gateway.logger.Warn("Lark slow progress summary send failed: %v", err)
					}
				}
			}
		}
	}
	l.scheduleNext()
}

func (l *slowProgressSummaryListener) prepareSummary() ([]slowProgressSignal, time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed || l.terminal {
		return nil, 0, false
	}
	if !l.isRunningLocked() {
		return nil, 0, false
	}

	l.summarySent++
	elapsed := l.clock().Sub(l.startedAt)
	signals := make([]slowProgressSignal, len(l.signals))
	copy(signals, l.signals)
	return signals, elapsed, true
}

func (l *slowProgressSummaryListener) scheduleNext() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed || l.terminal || l.timer == nil {
		return
	}
	l.timer.Reset(l.nextIntervalLocked())
}

func (l *slowProgressSummaryListener) nextIntervalLocked() time.Duration {
	if len(l.intervals) == 0 {
		if l.delay > 0 {
			return l.delay
		}
		return defaultSlowProgressSummaryDelay
	}
	idx := l.summarySent
	if idx >= len(l.intervals) {
		idx = len(l.intervals) - 1
	}
	next := l.intervals[idx]
	if next <= 0 {
		return defaultSlowProgressSummaryDelay
	}
	return next
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
	toolLines := buildHumanToolSignalLines(signals, 3)
	if l.gateway == nil || l.gateway.llmFactory == nil {
		return appendHumanToolSection(fallback, toolLines)
	}
	profile := l.resolveProfile()
	if utils.IsBlank(profile.Provider) || utils.IsBlank(profile.Model) {
		return appendHumanToolSection(fallback, toolLines)
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
		return appendHumanToolSection(fallback, toolLines)
	}
	summary = strings.TrimSpace(summary)
	if !isValidSlowProgressLLMSummary(summary) {
		return appendHumanToolSection(fallback, toolLines)
	}
	return truncateForLark(summary, slowProgressSummaryMaxReplyChars)
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
	systemPrompt := "你是运行进展播报助手。把事件整理成自然中文进展同步，输出 1 段 2-3 句，不使用列表。不要出现内部节点ID或键名（如 react:iter:...、call_xxx、step_description、payload），只描述已发生进展与当前状态，不给最终结论。"
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
	toolLines := buildHumanToolSignalLines(signals, 3)
	if len(toolLines) > 0 {
		b.WriteString("最近工具调用（人话）：\n")
		for i, line := range toolLines {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, line))
		}
	}
	b.WriteString("请输出一段对用户的中文同步，语气自然口语化。")
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
	return truncateForLark(appendHumanToolSection(b.String(), buildHumanToolSignalLines(signals, 3)), slowProgressSummaryMaxReplyChars)
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
		step := resolveSlowProgressStepLabel(asString(e.Payload["step_description"]), e.NodeID)
		if step == "" {
			return slowProgressSignal{}, false
		}
		return slowProgressSignal{at: e.Timestamp(), text: "开始步骤：" + step}, true
	case types.EventNodeCompleted:
		step := resolveSlowProgressStepLabel(asString(e.Payload["step_description"]), e.NodeID)
		if step == "" {
			return slowProgressSignal{}, false
		}
		return slowProgressSignal{at: e.Timestamp(), text: "完成步骤：" + step}, true
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
		content := sanitizeSlowProgressContent(asString(e.Payload["content"]))
		if content == "" {
			return slowProgressSignal{}, false
		}
		return slowProgressSignal{at: e.Timestamp(), text: "阶段输出：" + content}, true
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
		content := sanitizeSlowProgressContent(e.Data.Content)
		if content == "" {
			return slowProgressSignal{}, false
		}
		return slowProgressSignal{at: e.Timestamp(), text: "阶段输出：" + content}, true
	default:
		return slowProgressSignal{}, false
	}
}

func isValidSlowProgressLLMSummary(summary string) bool {
	trimmed := strings.TrimSpace(summary)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "empty response:") || strings.HasPrefix(lower, "empty completion:") {
		return false
	}
	return !containsInternalProgressIdentifier(trimmed)
}

func resolveSlowProgressStepLabel(stepDescription string, nodeID string) string {
	step := strings.TrimSpace(stepDescription)
	if step != "" {
		return truncateForLark(step, 120)
	}
	humanized := humanizeSlowProgressNodeID(nodeID)
	if humanized == "" {
		return ""
	}
	return truncateForLark(humanized, 120)
}

func humanizeSlowProgressNodeID(nodeID string) string {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return ""
	}
	if strings.HasPrefix(nodeID, "react:iter:") {
		parts := strings.Split(nodeID, ":")
		if len(parts) >= 4 {
			iter := strings.TrimSpace(parts[2])
			if iter == "" {
				iter = "?"
			}
			switch strings.TrimSpace(parts[3]) {
			case "think":
				return fmt.Sprintf("第 %s 轮思考", iter)
			case "plan":
				return fmt.Sprintf("第 %s 轮规划", iter)
			case "tools":
				return fmt.Sprintf("第 %s 轮工具执行", iter)
			case "tool":
				if len(parts) >= 5 {
					toolNode := strings.TrimSpace(parts[4])
					if toolNode != "" && !isOpaqueToolCallID(toolNode) {
						return fmt.Sprintf("第 %s 轮工具调用（%s）", iter, toolNode)
					}
				}
				return fmt.Sprintf("第 %s 轮工具调用", iter)
			default:
				return fmt.Sprintf("第 %s 轮执行", iter)
			}
		}
	}
	if containsInternalProgressIdentifier(nodeID) {
		return ""
	}
	return nodeID
}

func sanitizeSlowProgressContent(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	if containsInternalProgressIdentifier(content) {
		return ""
	}
	return truncateForLark(content, 120)
}

func containsInternalProgressIdentifier(text string) bool {
	lower := utils.TrimLower(text)
	if lower == "" {
		return false
	}
	if strings.Contains(lower, "react:iter:") ||
		strings.Contains(lower, "step_description") ||
		strings.Contains(lower, "payload") ||
		strings.Contains(lower, "nodeid") {
		return true
	}
	for _, token := range strings.FieldsFunc(lower, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' || r == '-' || r == ':')
	}) {
		if isOpaqueToolCallID(token) {
			return true
		}
	}
	return false
}

func isOpaqueToolCallID(token string) bool {
	token = utils.TrimLower(token)
	if token == "" {
		return false
	}
	if strings.HasPrefix(token, "call_") && len(token) >= len("call_")+6 {
		return true
	}
	if strings.HasPrefix(token, "call-") && len(token) >= len("call-")+6 {
		return true
	}
	return false
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

func buildSlowSummaryIntervals(first time.Duration) []time.Duration {
	if first <= 0 {
		first = defaultSlowProgressSummaryDelay
	}
	second := first * 2
	third := first * 6
	if second <= 0 {
		second = first
	}
	if third < second {
		third = second
	}
	return []time.Duration{first, second, third}
}

func appendHumanToolSection(base string, lines []string) string {
	base = strings.TrimSpace(base)
	if len(lines) == 0 {
		return base
	}
	var b strings.Builder
	b.WriteString(base)
	b.WriteString("\n\n最近工具调用（人话）：\n")
	for _, line := range lines {
		b.WriteString("- ")
		b.WriteString(line)
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func buildHumanToolSignalLines(signals []slowProgressSignal, max int) []string {
	if max <= 0 || len(signals) == 0 {
		return nil
	}
	lines := make([]string, 0, max)
	tail := tailSignals(signals, 16)
	for i := len(tail) - 1; i >= 0 && len(lines) < max; i-- {
		name, state, errText, ok := parseToolSignalLine(tail[i].text)
		if !ok {
			continue
		}
		selector := len(lines) + int(tail[i].at.Unix()%7)
		phrase := toolPhraseForBackground(name, selector)
		name = strings.TrimSpace(name)
		switch state {
		case "started":
			lines = append(lines, fmt.Sprintf("%s（%s）", phrase, name))
		case "completed":
			lines = append(lines, fmt.Sprintf("已完成 %s（%s）", phrase, name))
		case "failed":
			if errText != "" {
				lines = append(lines, fmt.Sprintf("%s（%s）失败：%s", phrase, name, truncateForLark(errText, 80)))
			} else {
				lines = append(lines, fmt.Sprintf("%s（%s）执行失败", phrase, name))
			}
		}
	}
	return lines
}

func parseToolSignalLine(text string) (name string, state string, errText string, ok bool) {
	text = strings.TrimSpace(text)
	switch {
	case strings.HasPrefix(text, "开始工具："):
		return strings.TrimSpace(strings.TrimPrefix(text, "开始工具：")), "started", "", true
	case strings.HasPrefix(text, "完成工具："):
		return strings.TrimSpace(strings.TrimPrefix(text, "完成工具：")), "completed", "", true
	case strings.HasPrefix(text, "工具失败："):
		body := strings.TrimSpace(strings.TrimPrefix(text, "工具失败："))
		if body == "" {
			return "", "", "", false
		}
		name = body
		if idx := strings.Index(body, "（"); idx >= 0 {
			name = strings.TrimSpace(body[:idx])
			rest := strings.TrimSpace(body[idx+len("（"):])
			rest = strings.TrimSuffix(rest, ")")
			rest = strings.TrimSuffix(rest, "）")
			errText = strings.TrimSpace(rest)
		}
		return name, "failed", errText, true
	default:
		return "", "", "", false
	}
}
