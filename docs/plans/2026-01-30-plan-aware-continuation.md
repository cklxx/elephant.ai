# Plan-Aware Continuation Mechanism

**Created**: 2026-01-30
**Updated**: 2026-01-31
**Status**: Revised
**Goal**: Prevent agent from terminating after completing only the first step of a multi-step plan.

---

## Problem Statement

Agent creates a plan with N steps, executes step 1, then terminates — forgetting steps 2..N. This makes deep, multi-step workflows impossible.

### Root Cause Chain

1. **Termination trigger**: `handleNoTools()` at `runtime.go:620` treats any non-empty LLM text response without tool calls as a final answer.
2. **No plan awareness at termination**: The termination decision doesn't check whether the plan has remaining uncompleted steps.
3. **Broken reminder heuristic**: `promptDistance()` (`prompts.go:108`) compares rune _lengths_ of goal vs plan, not semantic similarity or plan progress. When goal and plan are roughly the same length (common for prose), `abs(len(goal) - len(plan)) <= 800` evaluates true and the reminder is **silently skipped**. The metric is meaningless.
4. **Reminder only fires on tool iterations**: `appendGoalPlanReminder` is called from `observeTools()` (`runtime.go:585`), which only runs when the LLM calls tools. On the critical text-only iteration (when the LLM decides to stop), `observeTools()` is skipped entirely — there are no tool results to inject a reminder into. The continuation guard (Component 1) must handle this gap.
5. **Reminder duplication**: `appendGoalPlanReminder` appends the reminder text to **every** tool result message in the batch. With N parallel tool calls, the reminder is duplicated N times — wasting tokens.
6. **No structured step tracking**: `internal_plan` is an opaque blob — no completion state per step.

### Key Constraint

The first task's completion time is unpredictable (variable number of tool calls/iterations), so we can't pre-schedule a fixed-time reminder.

---

## Solution Design

Three components. Components 1 and 2 are tightly coupled and must ship together (see rationale below).

### Component 1: Plan-Aware Termination Guard

**Location**: `handleNoTools()` in `runtime.go`

Instead of immediately finalizing when the LLM produces text without tools, check if there's an active multi-step plan that hasn't been completed.

```go
func (it *reactIteration) handleNoTools() (*TaskResult, bool, error) {
    trimmed := strings.TrimSpace(it.thought.Content)
    if trimmed == "" {
        return nil, false, nil
    }

    // Plan-aware continuation: don't finalize if plan has remaining steps.
    if it.runtime.shouldContinuePlan() {
        it.runtime.injectPlanContinuation()
        return nil, false, nil  // continue loop, don't finalize
    }

    finalResult := it.runtime.finalizeResult("final_answer", nil, true, nil)
    return finalResult, true, nil
}
```

**Flow after continuation**: `handleNoTools()` returns `(nil, false, nil)` → `planTools()` returns the same → `runIteration()` hits the early-return at line 189 (`len(plan.calls)==0 && result==nil`) → returns `(nil, false, nil)` → main loop continues. `executeTools()`, `observeTools()`, and `finish()` are all skipped for this iteration. The continuation prompt injected by `injectPlanContinuation()` is seen by the LLM on the next `think()`.

**`shouldContinuePlan()`** returns true when ALL of:
- `planEmitted == true`
- `planComplexity == "complex"`
- `planCompleted == false` (new field on reactRuntime)
- `continuationAttempts < maxPlanContinuations` (safety valve, default 5)

**`injectPlanContinuation()`**:
1. The LLM's text-only response is already recorded in message history by `recordThought()` at `runtime.go:494` (called from `think()` before `planTools()`). No need to re-record it.
2. Appends a system message (same pattern as `injectOrchestratorCorrection` at `runtime.go:354`):
   ```
   <plan-continuation>
   你刚完成了计划中的一个步骤。上一条消息是你的进度报告。

   你的完整计划:
   {state.LatestPlanPrompt}

   你的目标:
   {state.LatestGoalPrompt}

   请继续执行计划的下一步。如果所有步骤已完成，调用 plan_progress(plan_status="completed") 标记计划完成。
   </plan-continuation>
   ```
3. Increments `continuationAttempts` counter.
4. Emits a `PlanContinuationEvent` for observability (iteration index, continuationAttempts, text content length).

**Safety valve**: After `maxPlanContinuations` nudges (default 5), the next text-only response is treated as final answer. This prevents infinite loops.

**`continuationAttempts` reset**: When `planVersion` increments (LLM calls `plan()` again for re-planning), `continuationAttempts` resets to 0 and `planCompleted` resets to false. Implemented in `updateOrchestratorState()`.

**Clarify gate interaction**: After continuation, the next `think()` call goes through `enforceOrchestratorGates()`. For complex plans:
- If `currentTaskID` is still set from a previous `clarify()` → gate passes, LLM can call action tools. This is the expected case (continuing within the same task).
- If the LLM starts working on a new task → the clarify gate blocks it, forcing a `clarify()` call first. This is correct behavior — the gate system enforces task declaration for new work.

### Component 2: `plan_progress` Tool

**Why Batch 1 and 2 must ship together**: The termination guard's `shouldContinuePlan()` checks `planCompleted == false`. The only way to set `planCompleted = true` is via `plan_progress(plan_status="completed")`. Without the tool, the LLM has no way to signal completion — every complex plan would exhaust all 5 continuation attempts before terminating. This wastes iterations and confuses the LLM with unnecessary continuation nudges.

**Location**: New file `internal/tools/builtin/ui/plan_progress.go`

A lightweight state-update tool with no external side effects. Gives the LLM an explicit way to signal plan progress.

**Parameters**:
| Parameter | Type | Required | Description |
|---|---|---|---|
| `step_id` | string | no | ID/name of the step just completed |
| `step_summary` | string | no | Brief summary of what was accomplished |
| `next_step` | string | no | ID/name of the next step to execute |
| `plan_status` | string | no | `"completed"` to signal the entire plan is done |

**Behavior**:
- If `plan_status="completed"` → sets `planCompleted = true` on runtime via `updateOrchestratorState()` → next text-only response correctly finalizes.
- If `step_id` provided → records step completion in state (for observability and future structured tracking).
- Returns a formatted acknowledgment that helps the LLM stay oriented.
- Tool is a pure state update with ~0ms execution; no external side effects.

**Registration**: Added to `registry.go:398` alongside `plan`, `clarify`, `request_user` in the static tool map.

**Orchestrator gate handling**: `plan_progress` must be treated as a UI/orchestration tool (like `request_user`), not as an action tool. In `enforceOrchestratorGates()`:
- Must be allowed solo (like plan/clarify/request_user — `len(calls) > 1` check).
- Must be exempt from the clarify gate (complex plans require clarify before action tools, but `plan_progress` is not an action tool).
- Must require `planEmitted == true` (no sense tracking progress without a plan).

Add to gate logic:
```go
case "plan_progress":
    if len(calls) > 1 {
        return true, "plan_progress() 必须单独调用。"
    }
    if !r.planEmitted {
        return true, r.planGatePrompt()
    }
    return false, ""
```

**Prompt integration**: Update `plan()` tool description to instruct:
```
When complexity="complex", after completing each major step, call plan_progress() to record progress.
When all steps are done, call plan_progress(plan_status="completed") to finalize.
```

### Component 3: Fix `appendGoalPlanReminder` Heuristic

**Location**: `prompts.go`

Two changes:

**3a. Replace trigger heuristic**: Replace `promptDistance()` (broken rune-length comparison) with an iteration-based trigger. Inject the reminder every N iterations after plan was emitted.

```go
func (e *ReactEngine) appendGoalPlanReminder(state *TaskState, messages []Message, iteration int) []Message {
    if state == nil || len(messages) == 0 {
        return messages
    }
    goal := strings.TrimSpace(state.LatestGoalPrompt)
    plan := strings.TrimSpace(state.LatestPlanPrompt)
    if goal == "" || plan == "" {
        return messages
    }
    // Inject reminder every N iterations (default 3) after plan was created.
    if iteration <= 0 || iteration%goalPlanReminderInterval != 0 {
        return messages
    }
    reminder := buildGoalPlanReminder(goal, plan)
    // Inject once into the LAST message only — not duplicated per tool result.
    last := len(messages) - 1
    if strings.TrimSpace(messages[last].Content) == "" {
        messages[last].Content = reminder
    } else {
        messages[last].Content = strings.TrimSpace(messages[last].Content) + "\n\n" + reminder
    }
    return messages
}
```

**3b. Fix reminder duplication**: The current code iterates over ALL tool messages and appends the reminder to each one. With parallel tool calls (N tools per iteration), this duplicates the reminder N times. Fix: inject only into the **last** tool message in the batch. The LLM sees all messages in order; the reminder at the end is sufficient.

**Caller change**: `observeTools()` at `runtime.go:585` must pass the iteration index:
```go
toolMessages = it.runtime.engine.appendGoalPlanReminder(state, toolMessages, it.index)
```

**Scope note**: This component only addresses reminders on iterations where tools are executed. On text-only iterations (where the LLM stops calling tools), the continuation mechanism from Component 1 handles the nudge. These two mechanisms are complementary:
- Component 3: "keep the LLM on track while it's actively calling tools"
- Component 1: "catch it when it tries to stop prematurely"

---

## Channel Integration: Lark Plan Review + Feedback Injection

**Goal**: In Lark chats, send the plan to the user **before** executing action tools, collect feedback, then inject that feedback into the next tool-calling cycle.

### Why Lark needs a channel-specific bridge
- Lark gateway disables session history injection by default, so the model will not automatically see the plan/tool history on the next user turn.
- The plan tool output is stored as **tool messages**, not as user-visible assistant replies.
- Lark may run with `session_mode=fresh` (new session per message); relying on session messages alone will lose pending state.
- We need a deterministic, channel-aware mechanism to: (1) surface the plan, (2) pause, (3) resume with the feedback injected.

### Proposed Flow (Lark only)

1. **Plan emitted** (tool result):
   - When `plan()` completes and **Lark plan review** is enabled, runtime marks the run as **plan-review pending** and sets `pauseRequested=true` so execution stops after the plan tool result.
   - The runtime injects a **system marker** message into `state.Messages` so the session can be recognized as awaiting plan feedback.
   - The Lark gateway persists a **pending plan record** keyed by `user_id + chat_id` (so it survives fresh sessions).

2. **Lark gateway sends plan to user**:
   - After `ExecuteTask` returns with `StopReason=await_user_input`, the gateway extracts the plan from tool results and replies with a plan message (plus a short prompt asking for approval/edits).

3. **User reply arrives**:
   - Gateway loads pending plan by `user_id + chat_id`; if not found, it falls back to the session marker.
   - It **prefixes the new task** with a structured block containing:
     - The previously sent plan
     - The user’s feedback (current message)
     - A directive: “If feedback changes the plan, call plan() again; otherwise continue with the next step.”
   - The pending store entry is cleared (and the marker is cleared/replaced) to avoid repeated injection.

4. **Next iteration**:
   - The LLM sees the injected block and can either re-plan or proceed to action tools.

### Minimal plumbing required (no domain → app import cycle)

- **Context flag**: introduce a simple `PlanReviewPolicy` in app layer (config → context), then pass a boolean into the React runtime **without importing appcontext in domain**.
  - Example approach: add a bool to `TaskState` (or `Services`) and copy it during `PrepareExecution`.
- **Pending store (user_id + chat_id)**: use a small Postgres table (`lark_plan_review_pending`) in the session DB to persist pending state across fresh sessions.
  - Required fields: `user_id`, `chat_id`, `run_id`, `overall_goal_ui`, `internal_plan`, `created_at`, `expires_at`.
  - TTL recommended (e.g., 30–60 minutes) to prevent stale injection. *Note: KV store could be faster later.*

### Suggested marker format (system message)
```
<plan_review_pending>
overall_goal_ui: ...
internal_plan: ...
run_id: ...
</plan_review_pending>
```

### Lark reply template (user-visible)
- Title: “计划确认”
- Body: plan content
- Prompt: “请回复 **OK** 继续，或直接回复修改意见。”

### Injection block (prepended to next task)
```
<plan_feedback>
plan:
...

user_feedback:
...

instruction: If the feedback changes the plan, call plan() again; otherwise continue with the next step.
</plan_feedback>
```

### Config (YAML-only)
```yaml
# config.yaml
channels:
  lark:
    plan_review_enabled: true
    plan_review_require_confirmation: true
    plan_review_pending_ttl_minutes: 60
```

---

## Implementation Plan

### Batch 1: Plan-Aware Termination Guard + `plan_progress` Tool

These ship together because the guard depends on `plan_progress` for clean plan completion signaling.

1. Add `planCompleted bool` and `continuationAttempts int` fields to `reactRuntime`.
2. Add `maxPlanContinuations` to engine config (default 5).
3. Implement `shouldContinuePlan()` and `injectPlanContinuation()` on `reactRuntime`.
4. Modify `handleNoTools()` to call the guard before finalizing.
5. Add `continuationAttempts`/`planCompleted` reset logic in `updateOrchestratorState()` when `planVersion` increments.
6. Create `internal/tools/builtin/ui/plan_progress.go` with tool definition and execute logic.
7. Register `plan_progress` in `registry.go` static tool map.
8. Add `plan_progress` case to `enforceOrchestratorGates()` (solo call, plan-gate required, clarify-gate exempt).
9. Add `plan_progress` case to `updateOrchestratorState()` to set `planCompleted`.
10. Update `plan()` tool description to instruct LLM to use `plan_progress`.
11. Add `PlanContinuationEvent` to domain events.
12. Tests:
    - Complex plan with 3 steps executes to completion via `plan_progress(plan_status="completed")`.
    - Simple plan bypasses continuation guard entirely.
    - Safety valve: 5 continuations without `plan_progress` → forces finalization.
    - `continuationAttempts` resets on new `planVersion`.
    - `plan_progress` blocked when `planEmitted == false`.
    - `plan_progress` allowed without prior `clarify()`.
    - `plan_progress` must be solo call (no batching with other tools).
13. **Commit**: "feat(react): add plan-aware termination guard and plan_progress tool"

### Batch 2: Fix Reminder Heuristic

1. Replace `promptDistance()` with iteration-based interval check in `appendGoalPlanReminder()`.
2. Add `iteration int` parameter to `appendGoalPlanReminder()` signature.
3. Update caller in `observeTools()` (`runtime.go:585`) to pass `it.index`.
4. Fix reminder duplication: inject into last message only, not all messages.
5. Add `goalPlanReminderInterval` config (default 3).
6. Remove dead code: `promptDistance()`, `goalPlanPromptDistanceThreshold`.
7. Tests:
    - Reminder injected on iteration 3, 6, 9 (interval-based).
    - Reminder NOT injected on iteration 1, 2, 4, 5 (non-interval).
    - Reminder injected into last tool message only (no duplication).
    - Empty goal or plan → no injection.
8. **Commit**: "fix(react): replace broken string-length heuristic with interval-based plan reminder"

### Batch 3: Lark Plan Review + Feedback Injection (Channel-Scoped)

1. Add `PlanReviewPolicy` to app config + context; plumb a simple bool into execution prep (no domain → app import).
   Add a `PlanReviewStore` keyed by `user_id + chat_id` with TTL.
2. On plan tool result, if plan review enabled:
   - mark `planReviewPending` in runtime
   - inject `<plan_review_pending>` system marker into `state.Messages`
   - set `pauseRequested=true` to stop after plan tool.
3. Lark gateway:
   - When result.StopReason == "await_user_input" and marker present, send plan summary + confirmation prompt.
   - Persist pending plan into `PlanReviewStore` (key: user_id + chat_id).
4. On next Lark message:
   - load pending plan from `PlanReviewStore` (fallback to marker if missing)
   - prefix task with `<plan_feedback>` block (plan + user feedback)
   - clear pending store entry and marker to avoid repeat injection.
5. Tests:
   - plan review pauses immediately after plan tool (no action tools run)
   - Lark reply contains plan + confirmation prompt
   - next user message is prepended with feedback block
   - marker cleared after injection
6. **Commit**: "feat(lark): plan review handshake and feedback injection"

---

## Architecture Diagram

```
                    handleNoTools() ─── text + no tools
                         │
                    shouldContinuePlan()?
                    ╱                ╲
                  yes                no
                  │                   │
    injectPlanContinuation()    finalizeResult()
    (thought already recorded     (terminate as today)
     by recordThought();
     inject system message;
     increment counter;
     emit PlanContinuationEvent)
                  │
                  ↓
            continue loop ──→ think() ──→ enforceOrchestratorGates()
                                          │
                              ┌───────────┴───────────┐
                              │                       │
                         currentTaskID set?     new task needed?
                         (continue same task)   (clarify gate → clarify())
                              │                       │
                              ↓                       ↓
                         LLM calls action tools  LLM calls clarify()
                         to execute next step    then action tools
                              │
                              ↓
                         plan_progress(plan_status="completed")
                         → planCompleted = true
                         → next text-only response finalizes normally
```

---

## Trade-offs & Risks

| Risk | Mitigation |
|---|---|
| Infinite continuation loops | `maxPlanContinuations` counter (default 5); resets on re-plan |
| LLM ignores `plan_progress`, never calls it | Safety valve still forces finalization after 5 nudges; `plan_progress` is recommended but not strictly required |
| Extra tool call overhead from `plan_progress` | Pure state update, ~0ms execution, no external I/O |
| Backward compatibility | Only activates for `complexity="complex"` plans; simple plans unchanged; `plan_progress` tool is no-op if not called |
| Token cost from continuation prompts | Compact prompt (~200 chars + plan/goal text); amortized over avoided re-runs that would repeat the entire context |
| Iteration budget consumption | Each continuation nudge consumes one iteration toward `maxIterations`; with default 5 nudges this is bounded and acceptable |
| `plan_progress` + clarify gate confusion | Gate explicitly exempts `plan_progress` (UI tool, not action tool); documented in gate logic |

---

## Success Criteria

1. Complex plans with 3+ steps execute to completion without manual intervention.
2. Simple plans behave identically to today (zero behavioral change).
3. Safety valve prevents runaway loops (verified by test: 5 continuations → forced finalization).
4. Agent explicitly marks plan as done via `plan_progress(plan_status="completed")` (observable via events/logs).
5. Goal/plan reminder fires reliably every N iterations (no longer dependent on string length heuristic).
6. Reminder is injected once per tool batch, not duplicated per tool message.

---

## Out of Scope (future work)

- **Structured step tracking**: Parsing `internal_plan` into discrete steps with individual completion states. The current design treats plan progress as opaque — `plan_progress` records step IDs for observability but doesn't validate against the plan structure.
- **Automatic plan re-planning**: Detecting when the LLM's actions diverge from the plan and triggering a re-plan. Currently the LLM can re-plan voluntarily by calling `plan()` again.
- **Per-step token budgets**: Allocating token budgets per plan step to prevent one step from consuming the entire budget.

---

## Progress Log

- 2026-01-30: Initial design draft.
- 2026-01-31: Revised after code review. Fixed root cause #4 description (timing vs. trigger heuristic). Merged Batch 1+2 (tightly coupled). Added gate handling for plan_progress. Added continuationAttempts reset on re-plan. Fixed reminder duplication bug. Added observability event. Added detailed test scenarios. Added out-of-scope section.
