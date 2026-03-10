package lark

import (
	"context"
	"strings"
	"sync"
	"time"

	domain "alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
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
	var eventType string
	var signal slowProgressSignal
	var hasSignal bool

	switch e := event.(type) {
	case *domain.WorkflowEventEnvelope:
		if e == nil {
			return
		}
		eventType = strings.TrimSpace(e.Event)
		signal, hasSignal = signalFromEnvelope(e)
	case *domain.Event:
		if e == nil {
			return
		}
		eventType = strings.TrimSpace(e.Kind)
		signal, hasSignal = signalFromUnified(e)
	default:
		return
	}

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
	if hasSignal {
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
