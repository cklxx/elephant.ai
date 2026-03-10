# Incident Response

Updated: 2026-03-10

Process for handling high-impact regressions, cross-channel leakage, and prompt/context safety incidents.

---

## Trigger Conditions

Create an incident file when any of these occur:
- **High-impact regression**: user-facing functionality broken by a code change.
- **Cross-channel leakage**: data or context from one channel (Lark, WeChat, CLI, web) appearing in another.
- **Prompt/context safety incident**: unintended context injection, manipulative framing, or approval gate bypass.
- **Leader agent failure**: missed pulses, stuck blocker radar, noisy/duplicate alerts, or attention gate malfunction.

---

## Workflow

### 1. Open Incident (within 1 working day)

Create: `docs/postmortems/incidents/YYYY-MM-DD-short-incident-slug.md`

Use the template at `docs/postmortems/templates/incident-postmortem-template.md`.

### 2. Fill All Mandatory Sections

| Section | Requirements |
|---------|-------------|
| **What happened** | Symptom, trigger condition, detection channel |
| **Impact** | User-facing impact, internal impact, blast radius |
| **Timeline** | Exact dates, not relative timestamps |
| **Root cause** | Technical root cause + process root cause + why existing checks missed it |
| **Fix** | Code/config changes, scope/rollout strategy, verification evidence |
| **Prevention actions** | Each with owner, due date, and validation method |
| **Follow-ups** | Open risks, deferred items |
| **Metadata** | id, tags, links to error-experience entries |

### 3. Create Experience Records

- Add an **error-experience entry** in `docs/error-experience/entries/`.
- Add an **error-experience summary** in `docs/error-experience/summary/entries/`.
- Include at least one prevention action that is **testable** (test/lint/check/process gate).
- Link the implementation PR/commit and validation evidence.

### 4. Complete Prevention Checklist

Before closing the incident, complete every item in `docs/postmortems/checklists/incident-prevention-checklist.md`:

**Boundary & Scope**
- The fix has an explicit scope boundary (channel/mode/tenant/user/session).
- A negative test exists for "must not apply here".
- A positive test exists for "must apply here".

**Regression Guards**
- At least one automated test reproduces the original failure path.
- Test names encode the boundary condition.
- Existing nearby tests reviewed for missing assertions on the same boundary.

**Prompt/Context Safety**
- Any injected context has a size budget or gating condition.
- Cross-channel leakage risk explicitly assessed.
- Runtime-only context not in user-facing default flows.

**Documentation & Memory**
- Full postmortem created.
- Error-experience entry + summary added.
- Prevention pattern documented in guides/checklist/template.

**Validation**
- Package tests pass.
- Lint/test gates executed.
- Code review completed with P0/P1 resolved.

---

## Leader Agent Failure Scenarios

These are the most common leader-specific incidents. For each: detect, contain, then fix.

### Missed weekly pulse

**Detect:** No Monday digest in the Lark group. Metric `alex_leader_pulse_generations_total` stops increasing. Prometheus alert `WeeklyPulseMissed` fires if no generation in 8 days.

**Contain:** Verify the scheduler is running:
```bash
curl -s http://localhost:8080/health | jq '.components[] | select(.name == "scheduler")'
```

**Fix — common causes:**
1. `weekly_pulse.enabled: false` in config. Set to `true`, restart.
2. Task store is empty or misconfigured. Check that tasks are written to the store (`alex_leader_pulse_task_count` metric at 0 confirms this).
3. Lark credentials expired or bot removed from group. Re-add bot, refresh `LARK_APP_ID` / `LARK_APP_SECRET`.
4. Cron expression wrong. Default: `"0 9 * * 1"` (Monday 9 AM). Check timezone.

### Stuck blocker radar

**Detect:** Blocker radar scans stop. Metric `alex_leader_blocker_scans_total` flat. Alert `BlockerRadarStalled` fires if no scan in 1 hour.

**Contain:** Check `/health` for the `blocker_radar` job status. Look for `registered: false` or `healthy: false`.

**Fix — common causes:**
1. Scheduler disabled. Check `proactive.scheduler.enabled`.
2. Task store nil. Bootstrap logs show `"TaskStore not available"`. Fix the task store path.
3. Cron parse error. Check server logs for `"failed to register"`.
4. Server crash loop. Check `alex-service.log` for panics.

### Noisy alerts (too many notifications)

**Detect:** Users complain about excessive Lark messages. Metric `alex_leader_blocker_notified_total` >> `alex_leader_blocker_detected_total`. Rate limiter suppression count `alex_leader_attention_decisions_total{suppressed="true"}` is low (meaning messages are getting through unchecked).

**Contain:** Immediately reduce noise:
```yaml
# Option 1: Disable the noisy feature
proactive:
  scheduler:
    blocker_radar:
      enabled: false

# Option 2: Tighten rate limiter
channels:
  lark:
    rate_limiter:
      enabled: true
      chat_hourly_limit: 5
```
Restart: `alex dev restart backend`.

**Fix — root causes:**
1. `stale_threshold_seconds` too low. Raise from 1800 to 3600+.
2. `schedule` too frequent. Change from `*/10 * * * *` to `0 */4 * * *`.
3. False positives — tasks appear stuck but are actually waiting for external input. Add `input_wait_seconds` buffer.
4. Rate limiter disabled. Enable it.

### Duplicate notifications (multi-instance)

**Detect:** Same alert arrives 2+ times in the same minute. Usually happens when multiple server instances run with `proactive.scheduler.enabled: true`.

**Contain:** Disable the scheduler on all but one instance.

**Fix:** Run exactly one instance with the scheduler enabled. The default has no distributed lock (`LeaderLock: nil`). For multi-instance setups, implement an external leader lock or use a single scheduler sidecar.

### Attention gate suppressing urgent alerts

**Detect:** Urgent blocker alerts not delivered during quiet hours (22:00–08:00 by default). Metric `alex_leader_attention_decisions_total{suppressed="true"}` shows suppressed high-urgency messages.

**Contain:** Temporarily disable quiet hours:
```yaml
channels:
  lark:
    attention_gate:
      quiet_hours_start: 0
      quiet_hours_end: 0
```

**Fix — root causes:**
1. Attention gate disabled entirely (`attention_gate.enabled: false`). When disabled, no urgency classification occurs — all messages use the standard notifier with no quiet-hours bypass. Enable it.
2. Message text missing urgency keywords (`blocked`, `urgent`, `error`). Check the urgency classifier logic.
3. Budget exhausted. `budget_max` reached within `budget_window_seconds`. Raise the budget or widen the window.

### Leader scheduler won't start

**Detect:** `/health` returns `scheduler` status as `not_ready` or missing entirely. No leader metrics exported.

**Fix:**
1. Check `proactive.scheduler.enabled: true` in config.
2. Check bootstrap logs for DI wiring errors (task store, notifier, Lark gateway).
3. Verify Lark credentials are set: `LARK_APP_ID`, `LARK_APP_SECRET`.
4. Rebuild: `make build && alex dev restart backend`.

---

## File Locations

| Artifact | Path |
|----------|------|
| Incident reports | `docs/postmortems/incidents/` |
| Postmortem template | `docs/postmortems/templates/incident-postmortem-template.md` |
| Prevention checklist | `docs/postmortems/checklists/incident-prevention-checklist.md` |
| Error experience entries | `docs/error-experience/entries/` |
| Error experience summaries | `docs/error-experience/summary/entries/` |
