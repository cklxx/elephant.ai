package telegram

import (
	"context"
	"strings"
	"sync"
	"time"

	domain "alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
	"alex/internal/shared/logging"
	"alex/internal/shared/uxphrases"
)

const progressFlushInterval = 2 * time.Second

// progressSender abstracts send/update for testability.
type progressSender interface {
	SendProgress(ctx context.Context, text string) (messageID int, err error)
	UpdateProgress(ctx context.Context, messageID int, text string) error
}

// toolStatus tracks the lifecycle of a single tool call.
type toolStatus struct {
	callID   string
	toolName string
	started  time.Time
	done     bool
	errored  bool
	duration time.Duration
}

// progressListener wraps an EventListener to intercept tool start/complete
// events and display progress via a single updating Telegram message.
type progressListener struct {
	inner  agent.EventListener
	sender progressSender
	logger logging.Logger
	ctx    context.Context
	now    func() time.Time

	mu        sync.Mutex
	tools     []*toolStatus
	toolIndex map[string]*toolStatus
	messageID int
	dirty     bool
	timer     *time.Timer
	lastFlush time.Time
	closed    bool
	iteration int
}

func newProgressListener(ctx context.Context, inner agent.EventListener, sender progressSender, logger logging.Logger) *progressListener {
	return &progressListener{
		inner:     inner,
		sender:    sender,
		logger:    logging.OrNop(logger),
		ctx:       ctx,
		now:       time.Now,
		toolIndex: make(map[string]*toolStatus),
	}
}

// MessageID returns the Telegram message ID of the progress message.
func (p *progressListener) MessageID() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.messageID
}

func (p *progressListener) OnEvent(event agent.AgentEvent) {
	if p.inner != nil {
		p.inner.OnEvent(event)
	}

	switch e := event.(type) {
	case *domain.Event:
		p.onUnifiedEvent(e)
	case *domain.WorkflowEventEnvelope:
		p.onEnvelope(e)
	}
}

func (p *progressListener) onUnifiedEvent(e *domain.Event) {
	if e == nil {
		return
	}
	switch e.Kind {
	case types.EventNodeStarted:
		p.mu.Lock()
		if !p.closed {
			p.iteration = e.Data.Iteration
			p.dirty = true
			p.scheduleFlush()
		}
		p.mu.Unlock()
	case types.EventToolStarted:
		p.mu.Lock()
		if !p.closed {
			if _, exists := p.toolIndex[e.Data.CallID]; !exists {
				ts := &toolStatus{callID: e.Data.CallID, toolName: e.Data.ToolName, started: p.clock()}
				p.tools = append(p.tools, ts)
				p.toolIndex[e.Data.CallID] = ts
				p.dirty = true
				p.scheduleFlush()
			}
		}
		p.mu.Unlock()
	case types.EventToolCompleted:
		p.mu.Lock()
		if !p.closed {
			if ts, ok := p.toolIndex[e.Data.CallID]; ok {
				ts.done = true
				ts.errored = e.Data.Error != nil
				ts.duration = e.Data.Duration
				p.dirty = true
				p.scheduleFlush()
			}
		}
		p.mu.Unlock()
	}
}

func (p *progressListener) onEnvelope(e *domain.WorkflowEventEnvelope) {
	if e == nil {
		return
	}
	event := strings.TrimSpace(e.Event)
	switch event {
	case types.EventNodeStarted:
		p.mu.Lock()
		if !p.closed {
			p.iteration = envelopeInt(e, "iteration")
			p.dirty = true
			p.scheduleFlush()
		}
		p.mu.Unlock()
	case types.EventToolStarted:
		callID := envelopeStr(e, "call_id")
		if callID == "" {
			callID = strings.TrimSpace(e.NodeID)
		}
		if callID == "" {
			return
		}
		toolName := envelopeStr(e, "tool_name")
		if toolName == "" {
			toolName = "tool"
		}
		p.mu.Lock()
		if !p.closed {
			if _, exists := p.toolIndex[callID]; !exists {
				ts := &toolStatus{callID: callID, toolName: toolName, started: p.clock()}
				p.tools = append(p.tools, ts)
				p.toolIndex[callID] = ts
				p.dirty = true
				p.scheduleFlush()
			}
		}
		p.mu.Unlock()
	case types.EventToolCompleted:
		callID := envelopeStr(e, "call_id")
		if callID == "" {
			callID = strings.TrimSpace(e.NodeID)
		}
		if callID == "" {
			return
		}
		p.mu.Lock()
		if !p.closed {
			if ts, ok := p.toolIndex[callID]; ok {
				ts.done = true
				ts.errored = envelopeStr(e, "error") != ""
				ts.duration = p.clock().Sub(ts.started)
				p.dirty = true
				p.scheduleFlush()
			}
		}
		p.mu.Unlock()
	}
}

// scheduleFlush arranges a flush after the rate-limit interval. Must hold p.mu.
func (p *progressListener) scheduleFlush() {
	if p.timer != nil {
		return
	}
	elapsed := p.clock().Sub(p.lastFlush)
	if elapsed >= progressFlushInterval {
		p.timer = time.AfterFunc(0, p.flush)
	} else {
		p.timer = time.AfterFunc(progressFlushInterval-elapsed, p.flush)
	}
}

func (p *progressListener) flush() {
	p.mu.Lock()
	if !p.dirty || p.closed {
		p.timer = nil
		p.mu.Unlock()
		return
	}
	text := p.buildText()
	messageID := p.messageID
	p.dirty = false
	p.lastFlush = p.clock()
	p.timer = nil
	p.mu.Unlock()

	if messageID == 0 {
		newID, err := p.sender.SendProgress(p.ctx, text)
		if err != nil {
			p.logger.Warn("Telegram progress send failed: %v", err)
			return
		}
		p.mu.Lock()
		p.messageID = newID
		p.mu.Unlock()
	} else {
		if err := p.sender.UpdateProgress(p.ctx, messageID, text); err != nil {
			p.logger.Warn("Telegram progress update failed: %v", err)
		}
	}
}

// Close performs a final synchronous flush and stops timers.
func (p *progressListener) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	if p.timer != nil {
		p.timer.Stop()
		p.timer = nil
	}
	dirty := p.dirty
	text := ""
	if dirty {
		text = p.buildText()
	}
	messageID := p.messageID
	p.dirty = false
	p.mu.Unlock()

	if !dirty {
		return
	}
	if messageID == 0 {
		newID, err := p.sender.SendProgress(p.ctx, text)
		if err != nil {
			p.logger.Warn("Telegram progress final send failed: %v", err)
			return
		}
		p.mu.Lock()
		p.messageID = newID
		p.mu.Unlock()
	} else {
		if err := p.sender.UpdateProgress(p.ctx, messageID, text); err != nil {
			p.logger.Warn("Telegram progress final update failed: %v", err)
		}
	}
}

// buildText constructs the progress display string. Must hold p.mu.
func (p *progressListener) buildText() string {
	var activeTool *toolStatus
	for i := len(p.tools) - 1; i >= 0; i-- {
		if !p.tools[i].done {
			activeTool = p.tools[i]
			break
		}
	}
	if activeTool != nil {
		return uxphrases.ToolPhrase(activeTool.toolName, len(p.tools))
	}
	if len(p.tools) > 0 {
		return uxphrases.PickPhrase(uxphrases.SummarizingPhrases, len(p.tools))
	}
	return uxphrases.PickPhrase(uxphrases.ThinkingPhrases, p.iteration)
}

func (p *progressListener) clock() time.Time {
	if p.now != nil {
		return p.now()
	}
	return time.Now()
}

// telegramProgressSender is the concrete progressSender backed by the Gateway.
type telegramProgressSender struct {
	gateway *Gateway
	chatID  int64
}

func (s *telegramProgressSender) SendProgress(ctx context.Context, text string) (int, error) {
	if s.gateway.messenger == nil {
		return 0, nil
	}
	return s.gateway.messenger.SendText(ctx, s.chatID, text, 0)
}

func (s *telegramProgressSender) UpdateProgress(ctx context.Context, messageID int, text string) error {
	if s.gateway.messenger == nil {
		return nil
	}
	return s.gateway.messenger.EditText(ctx, s.chatID, messageID, text)
}

// envelope payload helpers

func envelopeStr(e *domain.WorkflowEventEnvelope, key string) string {
	if e == nil || e.Payload == nil {
		return ""
	}
	raw, ok := e.Payload[key]
	if !ok {
		return ""
	}
	s, _ := raw.(string)
	return strings.TrimSpace(s)
}

func envelopeInt(e *domain.WorkflowEventEnvelope, key string) int {
	if e == nil || e.Payload == nil {
		return 0
	}
	raw, ok := e.Payload[key]
	if !ok {
		return 0
	}
	switch v := raw.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}
