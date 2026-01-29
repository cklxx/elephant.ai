package lark

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"alex/internal/agent/domain"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/logging"
)

const (
	// progressFlushInterval is the minimum time between Lark API calls
	// to avoid rate-limiting (Lark imposes 5 QPS on message updates).
	progressFlushInterval = 2 * time.Second
)

// progressSender abstracts send/update for testability.
type progressSender interface {
	SendProgress(ctx context.Context, text string) (messageID string, err error)
	UpdateProgress(ctx context.Context, messageID, text string) error
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
// events and display progress in Lark via a single updating text message.
type progressListener struct {
	inner  agent.EventListener
	sender progressSender
	logger logging.Logger
	ctx    context.Context
	now    func() time.Time

	mu        sync.Mutex
	tools     []*toolStatus       // ordered by arrival
	toolIndex map[string]*toolStatus // callID → status
	messageID string              // set after first send
	dirty     bool                // pending changes since last flush
	timer     *time.Timer         // rate-limit timer
	lastFlush time.Time
	closed    bool
}

// newProgressListener creates a progress listener that delegates all events
// to inner while intercepting tool lifecycle events to display progress.
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

// OnEvent forwards the event to the inner listener and tracks tool lifecycle.
func (p *progressListener) OnEvent(event agent.AgentEvent) {
	// Always forward to inner listener first.
	if p.inner != nil {
		p.inner.OnEvent(event)
	}

	switch e := event.(type) {
	case *domain.WorkflowToolStartedEvent:
		p.onToolStarted(e)
	case *domain.WorkflowToolCompletedEvent:
		p.onToolCompleted(e)
	}
}

func (p *progressListener) onToolStarted(e *domain.WorkflowToolStartedEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	if _, exists := p.toolIndex[e.CallID]; exists {
		return // duplicate
	}

	ts := &toolStatus{
		callID:   e.CallID,
		toolName: e.ToolName,
		started:  p.clock(),
	}
	p.tools = append(p.tools, ts)
	p.toolIndex[e.CallID] = ts
	p.dirty = true
	p.scheduleFlush()
}

func (p *progressListener) onToolCompleted(e *domain.WorkflowToolCompletedEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	ts, ok := p.toolIndex[e.CallID]
	if !ok {
		return // unknown call
	}

	ts.done = true
	ts.errored = e.Error != nil
	ts.duration = e.Duration
	p.dirty = true
	p.scheduleFlush()
}

// scheduleFlush arranges a flush after the rate-limit interval.
// Must be called with p.mu held.
func (p *progressListener) scheduleFlush() {
	if p.timer != nil {
		return // already scheduled
	}

	elapsed := p.clock().Sub(p.lastFlush)
	if elapsed >= progressFlushInterval {
		// Enough time has passed; flush immediately in a goroutine.
		p.timer = time.AfterFunc(0, p.flush)
	} else {
		remaining := progressFlushInterval - elapsed
		p.timer = time.AfterFunc(remaining, p.flush)
	}
}

// flush sends or updates the progress message.
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

	if messageID == "" {
		// First send.
		newID, err := p.sender.SendProgress(p.ctx, text)
		if err != nil {
			p.logger.Warn("Lark progress send failed: %v", err)
			return
		}
		p.mu.Lock()
		p.messageID = newID
		p.mu.Unlock()
	} else {
		// Update existing message.
		if err := p.sender.UpdateProgress(p.ctx, messageID, text); err != nil {
			p.logger.Warn("Lark progress update failed: %v", err)
		}
	}
}

// Close performs a final synchronous flush and cleans up timers.
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

	// Final synchronous flush.
	if messageID == "" {
		newID, err := p.sender.SendProgress(p.ctx, text)
		if err != nil {
			p.logger.Warn("Lark progress final send failed: %v", err)
			return
		}
		p.mu.Lock()
		p.messageID = newID
		p.mu.Unlock()
	} else {
		if err := p.sender.UpdateProgress(p.ctx, messageID, text); err != nil {
			p.logger.Warn("Lark progress final update failed: %v", err)
		}
	}
}

// buildText constructs the progress display string.
// Must be called with p.mu held.
func (p *progressListener) buildText() string {
	if len(p.tools) == 0 {
		return "[处理中...]"
	}

	var b strings.Builder
	b.WriteString("[处理中...]\n")
	for _, ts := range p.tools {
		b.WriteString("> ")
		b.WriteString(ts.toolName)
		if ts.done {
			if ts.errored {
				b.WriteString(fmt.Sprintf(" [error %.1fs]", ts.duration.Seconds()))
			} else {
				b.WriteString(fmt.Sprintf(" [done %.1fs]", ts.duration.Seconds()))
			}
		} else {
			elapsed := p.clock().Sub(ts.started)
			b.WriteString(fmt.Sprintf(" [running %.0fs]", elapsed.Seconds()))
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (p *progressListener) clock() time.Time {
	if p.now != nil {
		return p.now()
	}
	return time.Now()
}

// larkProgressSender is the concrete progressSender backed by the Gateway.
type larkProgressSender struct {
	gateway   *Gateway
	chatID    string
	messageID string // parent message ID (for group reply threading)
	isGroup   bool
}

func (s *larkProgressSender) SendProgress(ctx context.Context, text string) (string, error) {
	if s.isGroup && s.messageID != "" {
		return s.gateway.replyMessageTypedWithID(ctx, s.messageID, "text", textContent(text))
	}
	return s.gateway.sendMessageTypedWithID(ctx, s.chatID, "text", textContent(text))
}

func (s *larkProgressSender) UpdateProgress(ctx context.Context, messageID, text string) error {
	return s.gateway.updateMessage(ctx, messageID, text)
}
