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
	"sync"
	"time"

	appcontext "alex/internal/app/agent/context"
	"alex/internal/runtime/hooks"
	"alex/internal/runtime/session"
	"alex/internal/shared/logging"
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
	logger  logging.Logger

	// inflight tracks sessions currently being handled to prevent concurrent
	// stall handling for the same session (which causes duplicate history loads).
	inflight   map[string]struct{}
	inflightMu sync.Mutex
}

// New creates a LeaderAgent.
func New(rt RuntimeReader, bus hooks.Bus, execute ExecuteFunc) *Agent {
	return &Agent{
		rt:       rt,
		bus:      bus,
		execute:  execute,
		logger:   logging.NewComponentLogger("LeaderAgent"),
		inflight: make(map[string]struct{}),
	}
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
				if a.tryAcquire(ev.SessionID) {
					go func(ev hooks.Event) {
						defer a.release(ev.SessionID)
						a.handleStall(ctx, ev)
					}(ev)
				} else {
					a.logger.Debug("Skipping stall event for session %s — already in-flight", ev.SessionID)
				}
			}
		}
	}
}

// tryAcquire attempts to mark a session as in-flight.
// Returns false if the session is already being handled.
func (a *Agent) tryAcquire(sessionID string) bool {
	a.inflightMu.Lock()
	defer a.inflightMu.Unlock()
	if _, ok := a.inflight[sessionID]; ok {
		return false
	}
	a.inflight[sessionID] = struct{}{}
	return true
}

// release marks a session as no longer in-flight.
func (a *Agent) release(sessionID string) {
	a.inflightMu.Lock()
	defer a.inflightMu.Unlock()
	delete(a.inflight, sessionID)
}

// handleStall makes an LLM decision for a stalled/needs-input session.
// The context disables session history loading since the stall prompt is
// self-contained and does not need conversation history.
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

	// Disable session history for stall decisions — the prompt is self-contained
	// and loading full history is the primary cause of memory explosion.
	stallCtx := appcontext.WithSessionHistory(ctx, false)

	decision, err := a.execute(stallCtx, prompt)
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
