// Package leader provides a LeaderAgent that monitors the runtime event bus
// and makes LLM-assisted decisions when sessions stall or need input.
//
// Each stall decision is an independent short prompt — the leader does not
// maintain a conversation session. This keeps it stateless and avoids
// accumulating context over time.
package leader

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/runtime/hooks"
	"alex/internal/runtime/session"
)


// RuntimeReader is the minimal interface the Agent needs from Runtime.
type RuntimeReader interface {
	GetSession(id string) (session.SessionData, bool)
	InjectText(ctx context.Context, id, text string) error
	MarkFailed(id, errMsg string) error
}

// ExecuteFunc is a function that sends a prompt to the LLM and returns the answer.
// It is satisfied by AgentCoordinator.ExecuteTask when wired at bootstrap.
type ExecuteFunc func(ctx context.Context, prompt string) (string, error)

// Agent subscribes to EventStalled and EventNeedsInput on the bus and calls
// the LLM to decide how to proceed:
//   - inject a nudge message
//   - mark the session as failed
//   - log for human escalation (EventHandoffRequired published)
type Agent struct {
	rt      RuntimeReader
	bus     hooks.Bus
	execute ExecuteFunc
}

// New creates a LeaderAgent.
func New(rt RuntimeReader, bus hooks.Bus, execute ExecuteFunc) *Agent {
	return &Agent{rt: rt, bus: bus, execute: execute}
}

// Run subscribes to the bus and processes stall/needs-input events.
// Blocks until ctx is cancelled.
func (a *Agent) Run(ctx context.Context) {
	ch, cancel := a.bus.SubscribeAll()
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-ch:
			switch ev.Type {
			case hooks.EventStalled, hooks.EventNeedsInput:
				go a.handleStall(ctx, ev)
			}
		}
	}
}

// handleStall makes an LLM decision for a stalled/needs-input session.
func (a *Agent) handleStall(ctx context.Context, ev hooks.Event) {
	snap, ok := a.rt.GetSession(ev.SessionID)
	if !ok {
		return
	}

	var elapsed time.Duration
	if snap.StartedAt != nil {
		elapsed = time.Since(*snap.StartedAt).Round(time.Second)
	}

	prompt := buildStallPrompt(snap.ID, string(snap.Member), snap.Goal, elapsed, ev.Type)

	decision, err := a.execute(ctx, prompt)
	if err != nil {
		// LLM unavailable — escalate to human.
		a.escalate(ev.SessionID, fmt.Sprintf("leader llm error: %v", err))
		return
	}

	a.applyDecision(ctx, ev.SessionID, strings.TrimSpace(decision))
}

// applyDecision executes the LLM's recommendation.
// Expected first line keywords: INJECT <message>, FAIL <reason>, ESCALATE.
func (a *Agent) applyDecision(ctx context.Context, sessionID, decision string) {
	upper := strings.ToUpper(decision)
	switch {
	case strings.HasPrefix(upper, "INJECT"):
		msg := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(decision, "INJECT"), "inject"))
		if msg == "" {
			msg = "Please continue with the task."
		}
		_ = a.rt.InjectText(ctx, sessionID, msg)
	case strings.HasPrefix(upper, "FAIL"):
		reason := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(decision, "FAIL"), "fail"))
		if reason == "" {
			reason = "leader agent: session abandoned after stall"
		}
		_ = a.rt.MarkFailed(sessionID, reason)
	default:
		// Unknown or ESCALATE.
		a.escalate(sessionID, "leader agent: escalating to human operator")
	}
}

// escalate publishes an EventHandoffRequired so operators can be notified.
func (a *Agent) escalate(sessionID, reason string) {
	a.bus.Publish(sessionID, hooks.Event{
		Type:      hooks.EventHandoffRequired,
		SessionID: sessionID,
		At:        time.Now(),
		Payload:   map[string]any{"reason": reason},
	})
}

// buildStallPrompt constructs the short decision prompt for the LLM.
func buildStallPrompt(id, member, goal string, elapsed time.Duration, eventType hooks.EventType) string {
	kind := "stalled"
	if eventType == hooks.EventNeedsInput {
		kind = "waiting for input"
	}
	return fmt.Sprintf(`You are a leader agent managing an AI coding session.

Session ID: %s
Member:     %s
Goal:       %s
Status:     %s for %s

The session has been %s. Decide what to do next. Reply with EXACTLY one of:

INJECT <a short message to send to the session to unblock it>
FAIL <reason — give up on this session>
ESCALATE

Reply only with one of the above. No explanation.`,
		id,
		member,
		goal,
		kind,
		elapsed,
		kind,
	)
}
