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
	GetRecentEvents(sessionID string, n int) []string
}

// ToolCallReader provides optional tool-call context for stall decisions
// and handoff diagnostics.
type ToolCallReader interface {
	GetRecentToolCall(sessionID string) (name, args, errStr string, ok bool)
	GetIterationCount(sessionID string) int
}

// ExecuteFunc sends a prompt to the LLM and returns the answer.
// sessionID identifies the conversation context to use — the leader reuses a
// stable session ID per runtime session so stall decisions accumulate context
// without creating unbounded new sessions.
type ExecuteFunc func(ctx context.Context, prompt, sessionID string) (string, error)

type afterFunc func(time.Duration, func()) *time.Timer

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

	// decisions stores per-session decision history so the LLM can see
	// what was tried before and avoid repeating ineffective strategies.
	decisions *decisionHistoryStore

	recoveryEvaluationDelay time.Duration
	afterFunc               afterFunc
}

// maxStallAttempts is the maximum number of stall handling attempts per runtime
// session before the leader gives up and escalates to a human operator.
const maxStallAttempts = 3

// markFailedRetries is the number of retry attempts for MarkFailed calls.
const markFailedRetries = 3

const (
	recoveryEvaluationDelayDefault = 30 * time.Second
	injectSuccessRateThreshold     = 0.3
	injectSuccessRateMinRecords    = 3
)

// New creates a LeaderAgent.
func New(rt RuntimeReader, bus hooks.Bus, execute ExecuteFunc) *Agent {
	return &Agent{
		rt:                      rt,
		bus:                     bus,
		execute:                 execute,
		logger:                  logging.NewComponentLogger("LeaderAgent"),
		inflight:                make(map[string]struct{}),
		stallCounts:             make(map[string]int),
		decisions:               newDecisionHistoryStore(),
		recoveryEvaluationDelay: recoveryEvaluationDelayDefault,
		afterFunc:               time.AfterFunc,
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

	history := a.decisions.Get(ev.SessionID)

	// Check stall count — escalate after maxStallAttempts.
	count := a.incrementStallCount(ev.SessionID)
	injectSuccessRate := history.InjectSuccessRate()
	if history.InjectEvaluatedCount() >= injectSuccessRateMinRecords && injectSuccessRate < injectSuccessRateThreshold {
		reason := fmt.Sprintf("leader agent: inject success rate %.0f%% below %.0f%%, escalating to human",
			injectSuccessRate*100, injectSuccessRateThreshold*100)
		a.logger.Info("Session %s inject recovery success rate %.2f below threshold %.2f — escalating",
			ev.SessionID, injectSuccessRate, injectSuccessRateThreshold)
		history.Add(DecisionRecord{
			Attempt:   count,
			Action:    "ESCALATE",
			Argument:  reason,
			Timestamp: time.Now(),
			Outcome:   "still_stalled",
			OutcomeAt: time.Now(),
		})
		a.escalate(ev.SessionID, reason)
		return
	}
	if count > maxStallAttempts {
		a.logger.Info("Session %s stalled %d times (max %d) — escalating", ev.SessionID, count, maxStallAttempts)
		a.escalate(ev.SessionID, fmt.Sprintf("leader agent: session stalled %d times, escalating to human", count))
		return
	}

	var elapsed time.Duration
	if snap.StartedAt != nil {
		elapsed = time.Since(*snap.StartedAt).Round(time.Second)
	}

	// Gather optional tool-call context.
	var toolName, toolArgs, toolErr string
	var iterationCount int
	if tcr, ok := a.rt.(ToolCallReader); ok {
		toolName, toolArgs, toolErr, _ = tcr.GetRecentToolCall(ev.SessionID)
		iterationCount = tcr.GetIterationCount(ev.SessionID)
	}

	prompt := buildStallPrompt(snap.ID, string(snap.Member), snap.Goal, elapsed, ev.Type, count, history, toolName, toolArgs, toolErr, iterationCount)

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

	trimmed := strings.TrimSpace(decision)
	action, arg := parseDecision(trimmed)

	// Record the decision for future prompts.
	actionName := "ESCALATE"
	switch action {
	case actionInject:
		actionName = "INJECT"
	case actionFail:
		actionName = "FAIL"
	case actionRetryTool:
		actionName = "RETRY_TOOL"
	case actionSwitchStrategy:
		actionName = "SWITCH_STRATEGY"
	}
	history.Add(DecisionRecord{
		Attempt:   count,
		Action:    actionName,
		Argument:  arg,
		Timestamp: time.Now(),
	})

	a.applyDecision(ctx, ev.SessionID, trimmed)
	a.scheduleRecoveryEvaluation(ev.SessionID, count, history)
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
	siblingTotal, hasSiblingTotal := payloadInt(ev.Payload, "sibling_total")
	siblingCompleted, hasSiblingCompleted := payloadInt(ev.Payload, "sibling_completed")

	var resultSummary string
	switch {
	case childError != "":
		resultSummary = fmt.Sprintf("FAILED with error: %s", childError)
	case childAnswer != "":
		resultSummary = fmt.Sprintf("completed successfully. Result: %s", childAnswer)
	default:
		resultSummary = "completed (no explicit result)"
	}

	progressLine := ""
	allCompletedHint := ""
	if hasSiblingTotal && hasSiblingCompleted && siblingTotal > 0 {
		progressLine = fmt.Sprintf("\n子任务进度: %d/%d 已完成\n", siblingCompleted, siblingTotal)
		if siblingCompleted == siblingTotal {
			allCompletedHint = "\n所有子任务已完成，请汇总结果。\n"
		}
	}

	prompt := fmt.Sprintf(`你是编程团队的 leader。
你之前派发的子任务已完成：

子任务 ID: %s
子任务目标: %s
子任务结果: %s
%s%s

请决定下一步：
1. 如果还有后续任务 → 调用 POST /api/runtime/sessions 派发（记得设置 parent_session_id）
2. 如果所有任务完成 → 汇总结果并通知用户
3. 如果结果有问题 → 派发修复任务`, childID, childGoal, resultSummary, progressLine, allCompletedHint)

	// Use the parent session ID for orchestration continuity.
	result, err := a.execute(ctx, prompt, parentSessionID)
	if err != nil {
		a.escalate(parentSessionID, fmt.Sprintf("leader llm error on child completion: %v", err))
		return
	}

	a.applyOrchestratorDecision(ctx, parentSessionID, strings.TrimSpace(result))
}

func payloadInt(payload map[string]any, key string) (int, bool) {
	switch v := payload[key].(type) {
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

func (a *Agent) currentStallCount(sessionID string) (int, bool) {
	a.stallCountsMu.Lock()
	defer a.stallCountsMu.Unlock()
	count, ok := a.stallCounts[sessionID]
	return count, ok
}

func (a *Agent) scheduleRecoveryEvaluation(sessionID string, attempt int, history *DecisionHistory) {
	if history == nil || a.afterFunc == nil {
		return
	}
	a.afterFunc(a.recoveryEvaluationDelay, func() {
		outcome := "still_stalled"
		count, ok := a.currentStallCount(sessionID)
		if !ok || count < attempt {
			outcome = "recovered"
		}
		history.RecordOutcomeForAttempt(attempt, outcome)
		switch outcome {
		case "recovered":
			a.logger.Info("alex.leader.stall.recovery.success session=%s attempt=%d", sessionID, attempt)
		default:
			a.logger.Info("alex.leader.stall.recovery.failure session=%s attempt=%d", sessionID, attempt)
		}
	})
}

// decisionAction classifies the LLM response into inject/fail/unknown.
type decisionAction int

const (
	actionUnknown decisionAction = iota
	actionInject
	actionFail
	actionRetryTool
	actionSwitchStrategy
)

// parseDecision extracts the action keyword and its argument from an LLM response.
func parseDecision(raw string) (decisionAction, string) {
	upper := strings.ToUpper(raw)
	switch {
	case strings.HasPrefix(upper, "RETRY_TOOL"):
		toolName := strings.TrimSpace(raw[len("RETRY_TOOL"):])
		return actionRetryTool, toolName
	case strings.HasPrefix(upper, "SWITCH_STRATEGY"):
		hint := strings.TrimSpace(raw[len("SWITCH_STRATEGY"):])
		return actionSwitchStrategy, hint
	case strings.HasPrefix(upper, "INJECT"):
		msg := strings.TrimSpace(raw[len("INJECT"):])
		if msg == "" {
			msg = "Please continue with the task."
		}
		return actionInject, msg
	case strings.HasPrefix(upper, "FAIL"):
		reason := strings.TrimSpace(raw[len("FAIL"):])
		return actionFail, reason
	default:
		return actionUnknown, raw
	}
}

// applyOrchestratorDecision handles the leader's response after child completion.
func (a *Agent) applyOrchestratorDecision(ctx context.Context, parentSessionID, decision string) {
	action, arg := parseDecision(decision)
	switch action {
	case actionInject:
		_ = a.rt.InjectText(ctx, parentSessionID, arg)
	case actionFail:
		if arg == "" {
			arg = "leader agent: orchestration failed"
		}
		a.markFailedWithRetry(parentSessionID, arg)
	default:
		// Free-form response — publish as handoff for downstream consumers.
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

// applyDecision executes the LLM's recommendation for stall handling.
func (a *Agent) applyDecision(ctx context.Context, sessionID, decision string) {
	action, arg := parseDecision(decision)
	switch action {
	case actionInject:
		_ = a.rt.InjectText(ctx, sessionID, arg)
	case actionFail:
		if arg == "" {
			arg = "leader agent: session abandoned after stall"
		}
		a.markFailedWithRetry(sessionID, arg)
	case actionRetryTool:
		var errStr string
		if tcr, ok := a.rt.(ToolCallReader); ok {
			_, _, errStr, _ = tcr.GetRecentToolCall(sessionID)
		}
		msg := fmt.Sprintf("请重试工具 %s，上次错误: %s", arg, errStr)
		_ = a.rt.InjectText(ctx, sessionID, msg)
	case actionSwitchStrategy:
		msg := fmt.Sprintf("请换一种方式完成任务: %s", arg)
		_ = a.rt.InjectText(ctx, sessionID, msg)
	default:
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

// buildStallPrompt constructs the decision prompt for the LLM, including
// any previous decision history so the LLM can avoid repeating failed strategies.
func buildStallPrompt(id, member, goal string, elapsed time.Duration, eventType hooks.EventType, attempt int, history *DecisionHistory, toolName, toolArgs, toolErr string, iterationCount int) string {
	kind := "stalled"
	if eventType == hooks.EventNeedsInput {
		kind = "waiting for input"
	}

	historySummary := ""
	if history != nil {
		historySummary = history.SummaryForPrompt(maxStallAttempts)
	}

	var b strings.Builder
	fmt.Fprintf(&b, `You are a leader agent managing an AI coding session.

Session ID: %s
Member:     %s
Goal:       %s
Status:     %s for %s
Attempt:    %d of %d
`, id, member, goal, kind, elapsed, attempt, maxStallAttempts)

	if toolName != "" {
		fmt.Fprintf(&b, "Last tool call: %s(%s)\n", toolName, toolArgs)
	}
	if toolErr != "" {
		fmt.Fprintf(&b, "Last error:     %s\n", toolErr)
	}
	if iterationCount > 0 {
		fmt.Fprintf(&b, "Iteration:      %d tool calls so far\n", iterationCount)
	}

	if historySummary != "" {
		fmt.Fprintf(&b, "\n%s", historySummary)
	}

	fmt.Fprintf(&b, `
The session has been %s. Decide what to do next. Reply with EXACTLY one of:

INJECT <a short message to send to the session to unblock it>
FAIL <reason — give up on this session>
ESCALATE
RETRY_TOOL <tool_name> — retry the last failed tool
SWITCH_STRATEGY <hint> — try a different approach

If previous INJECT attempts failed, try a different message or consider FAIL/ESCALATE.
Reply only with one of the above. No explanation.`, kind)

	return b.String()
}
