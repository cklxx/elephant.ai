// Package leader provides a LeaderAgent that monitors the runtime event bus
// and makes LLM-assisted decisions when sessions stall, need input, or when
// child sessions complete (for orchestration continuity).
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
	"alex/internal/shared/notification"
)

// RuntimeReader is the minimal interface the Agent needs from Runtime.
type RuntimeReader interface {
	GetSession(id string) (session.SessionData, bool)
	InjectText(ctx context.Context, id, text string) error
	MarkFailed(id, errMsg string) error
}

// ExecuteFunc sends a prompt to the LLM and returns the answer.
// sessionID identifies the conversation context to use — the leader reuses a
// stable session ID per runtime session so stall decisions accumulate context
// without creating unbounded new sessions.
type ExecuteFunc func(ctx context.Context, prompt, sessionID string) (string, error)

// Agent subscribes to EventStalled, EventNeedsInput, and EventChildCompleted
// on the bus and calls the LLM to decide how to proceed:
//   - inject a nudge message (stall/needs-input)
//   - mark the session as failed
//   - continue orchestration after child completion
//   - log for human escalation (EventHandoffRequired published)
type Agent struct {
	rt       RuntimeReader
	bus      hooks.Bus
	execute  ExecuteFunc
	logger   logging.Logger
	notifier notification.Notifier // optional — used for escalation alerts

	// inflight tracks sessions currently being handled to prevent concurrent
	// stall handling for the same session (which causes duplicate LLM calls).
	inflight   map[string]struct{}
	inflightMu sync.Mutex

	// stallCounts tracks how many times each runtime session has been handled
	// for stall events, used to escalate after repeated failures.
	stallCounts   map[string]int
	stallCountsMu sync.Mutex
}

// maxStallAttempts is the maximum number of stall handling attempts per runtime
// session before the leader gives up and escalates to a human operator.
const maxStallAttempts = 3

// markFailedRetries is the number of retry attempts for MarkFailed calls.
const markFailedRetries = 3

// New creates a LeaderAgent.
func New(rt RuntimeReader, bus hooks.Bus, execute ExecuteFunc) *Agent {
	return &Agent{
		rt:          rt,
		bus:         bus,
		execute:     execute,
		logger:      logging.NewComponentLogger("LeaderAgent"),
		inflight:    make(map[string]struct{}),
		stallCounts: make(map[string]int),
	}
}

// SetNotifier sets an optional notifier for escalation alerts when MarkFailed
// retries are exhausted.
func (a *Agent) SetNotifier(n notification.Notifier) {
	a.notifier = n
}

// Run subscribes to the bus and processes stall/needs-input/child-completed events.
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
			case hooks.EventChildCompleted:
				go a.handleChildCompleted(ctx, ev)
			case hooks.EventCompleted, hooks.EventHeartbeat:
				// Session recovered — reset stall counter.
				a.resetStallCount(ev.SessionID)
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

// incrementStallCount increments and returns the stall count for a session.
func (a *Agent) incrementStallCount(sessionID string) int {
	a.stallCountsMu.Lock()
	defer a.stallCountsMu.Unlock()
	a.stallCounts[sessionID]++
	return a.stallCounts[sessionID]
}

// resetStallCount resets the stall counter when a session recovers.
func (a *Agent) resetStallCount(sessionID string) {
	a.stallCountsMu.Lock()
	defer a.stallCountsMu.Unlock()
	delete(a.stallCounts, sessionID)
}

// stallSessionID returns a stable, deterministic session ID for leader stall
// decisions about a given runtime session. This ensures repeated stall checks
// for the same runtime session reuse one conversation context instead of
// creating a new session each time.
func stallSessionID(runtimeSessionID string) string {
	return "leader-stall-" + runtimeSessionID
}

// handleStall makes an LLM decision for a stalled/needs-input session.
// The context disables session history loading since the stall prompt is
// self-contained and does not need conversation history.
func (a *Agent) handleStall(ctx context.Context, ev hooks.Event) {
	snap, ok := a.rt.GetSession(ev.SessionID)
	if !ok {
		return
	}

	// Check stall count — escalate after maxStallAttempts.
	count := a.incrementStallCount(ev.SessionID)
	if count > maxStallAttempts {
		a.logger.Info("Session %s stalled %d times (max %d) — escalating", ev.SessionID, count, maxStallAttempts)
		a.escalate(ev.SessionID, fmt.Sprintf("leader agent: session stalled %d times, escalating to human", count))
		return
	}

	var elapsed time.Duration
	if snap.StartedAt != nil {
		elapsed = time.Since(*snap.StartedAt).Round(time.Second)
	}

	prompt := buildStallPrompt(snap.ID, string(snap.Member), snap.Goal, elapsed, ev.Type, count)

	// Disable session history for stall decisions — the prompt is self-contained
	// and loading full history is the primary cause of memory explosion.
	stallCtx := appcontext.WithSessionHistory(ctx, false)

	// Reuse a stable session ID per runtime session to avoid session explosion.
	sid := stallSessionID(ev.SessionID)
	decision, err := a.execute(stallCtx, prompt, sid)
	if err != nil {
		// LLM unavailable — escalate to human.
		a.escalate(ev.SessionID, fmt.Sprintf("leader llm error: %v", err))
		return
	}

	a.applyDecision(ctx, ev.SessionID, strings.TrimSpace(decision))
}

// handleChildCompleted processes a child session completion event.
// It uses the parent session ID for context continuity so the leader can
// maintain orchestration state across multiple child tasks.
func (a *Agent) handleChildCompleted(ctx context.Context, ev hooks.Event) {
	parentSessionID := ev.SessionID
	childID, _ := ev.Payload["child_id"].(string)
	childGoal, _ := ev.Payload["child_goal"].(string)
	childAnswer, _ := ev.Payload["child_answer"].(string)
	childError, _ := ev.Payload["child_error"].(string)

	var resultSummary string
	if childError != "" {
		resultSummary = fmt.Sprintf("FAILED with error: %s", childError)
	} else if childAnswer != "" {
		resultSummary = fmt.Sprintf("completed successfully. Result: %s", childAnswer)
	} else {
		resultSummary = "completed (no explicit result)"
	}

	prompt := fmt.Sprintf(`你是编程团队的 leader。
你之前派发的子任务已完成：

子任务 ID: %s
子任务目标: %s
子任务结果: %s

请决定下一步：
1. 如果还有后续任务 → 调用 POST /api/runtime/sessions 派发（记得设置 parent_session_id）
2. 如果所有任务完成 → 汇总结果并通知用户
3. 如果结果有问题 → 派发修复任务`, childID, childGoal, resultSummary)

	// Use the parent session ID for orchestration continuity.
	result, err := a.execute(ctx, prompt, parentSessionID)
	if err != nil {
		a.escalate(parentSessionID, fmt.Sprintf("leader llm error on child completion: %v", err))
		return
	}

	a.applyOrchestratorDecision(ctx, parentSessionID, strings.TrimSpace(result))
}

// applyOrchestratorDecision handles the leader's response after child completion.
// The LLM may respond with orchestration actions or a final summary.
func (a *Agent) applyOrchestratorDecision(ctx context.Context, parentSessionID, decision string) {
	upper := strings.ToUpper(decision)
	switch {
	case strings.HasPrefix(upper, "INJECT"):
		msg := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(decision, "INJECT"), "inject"))
		if msg == "" {
			msg = "Please continue with the task."
		}
		_ = a.rt.InjectText(ctx, parentSessionID, msg)
	case strings.HasPrefix(upper, "FAIL"):
		reason := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(decision, "FAIL"), "fail"))
		if reason == "" {
			reason = "leader agent: orchestration failed"
		}
		a.markFailedWithRetry(parentSessionID, reason)
	default:
		// The LLM produced a free-form response (orchestration action or summary).
		// Publish as handoff with structured context so downstream consumers
		// (e.g. Lark notifier) get actionable information via ParseHandoffContext.
		hctx := a.buildHandoffContext(parentSessionID, "orchestrator: free-form response")
		payload := hctx.ToPayload()
		payload["leader_response"] = decision
		a.bus.Publish(parentSessionID, hooks.Event{
			Type:      hooks.EventHandoffRequired,
			SessionID: parentSessionID,
			At:        time.Now(),
			Payload:   payload,
		})
	}
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
		a.markFailedWithRetry(sessionID, reason)
	default:
		// Unknown or ESCALATE.
		a.escalate(sessionID, "leader agent: escalating to human operator")
	}
}

// markFailedWithRetry calls MarkFailed with retries and exponential backoff.
// If all retries fail, logs at ERROR level and escalates via the notifier.
func (a *Agent) markFailedWithRetry(sessionID, reason string) {
	var lastErr error
	for attempt := 1; attempt <= markFailedRetries; attempt++ {
		if err := a.rt.MarkFailed(sessionID, reason); err != nil {
			lastErr = err
			a.logger.Error("MarkFailed attempt %d/%d for session %s failed: %v", attempt, markFailedRetries, sessionID, err)
			if attempt < markFailedRetries {
				time.Sleep(time.Duration(attempt*100) * time.Millisecond)
			}
			continue
		}
		return // success
	}
	// All retries exhausted — escalate.
	alertMsg := fmt.Sprintf("CRITICAL: MarkFailed exhausted %d retries for session %s (reason: %s): %v",
		markFailedRetries, sessionID, reason, lastErr)
	a.logger.Error("%s", alertMsg)
	a.escalate(sessionID, alertMsg)
	if a.notifier != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		target := notification.Target{Channel: notification.ChannelLark}
		if err := a.notifier.Send(ctx, target, alertMsg); err != nil {
			a.logger.Error("Failed to send MarkFailed escalation notification for session %s: %v", sessionID, err)
		}
	}
}

// escalate publishes an EventHandoffRequired with structured context
// so operators can be notified with actionable information.
func (a *Agent) escalate(sessionID, reason string) {
	ctx := a.buildHandoffContext(sessionID, reason)
	a.bus.Publish(sessionID, hooks.Event{
		Type:      hooks.EventHandoffRequired,
		SessionID: sessionID,
		At:        time.Now(),
		Payload:   ctx.ToPayload(),
	})
}

// buildStallPrompt constructs the short decision prompt for the LLM.
func buildStallPrompt(id, member, goal string, elapsed time.Duration, eventType hooks.EventType, attempt int) string {
	kind := "stalled"
	if eventType == hooks.EventNeedsInput {
		kind = "waiting for input"
	}
	return fmt.Sprintf(`You are a leader agent managing an AI coding session.

Session ID: %s
Member:     %s
Goal:       %s
Status:     %s for %s
Attempt:    %d of %d

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
		attempt,
		maxStallAttempts,
		kind,
	)
}
