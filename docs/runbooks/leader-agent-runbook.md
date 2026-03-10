# Leader Agent Runbook

Production operations guide for the elephant.ai leader agent subsystem.

Covers: feature toggles, rollback procedures, monitoring alerts, and common failure modes.

---

## 1. Disabling Individual Leader Features

Each leader feature has an independent `enabled` flag in the scheduler config. Set `enabled: false` to disable without affecting other features. Changes take effect on next restart.

Config file: `~/.alex/config.yaml`

### Full feature toggle reference

```yaml
proactive:
  scheduler:
    enabled: true  # master switch — disables ALL proactive jobs

    blocker_radar:
      enabled: false       # disable stuck-task scanning
      schedule: "*/10 * * * *"
      stale_threshold_seconds: 1800
      input_wait_seconds: 900
      channel: lark
      chat_id: oc_YOUR_CHAT_ID

    weekly_pulse:
      enabled: false       # disable Monday digest
      schedule: "0 9 * * 1"
      channel: lark
      chat_id: oc_YOUR_CHAT_ID

    milestone_checkin:
      enabled: false       # disable hourly progress snapshots
      schedule: "0 */1 * * *"
      lookback_seconds: 3600
      channel: lark
      chat_id: oc_YOUR_CHAT_ID

    prep_brief:
      enabled: false       # disable 1:1 meeting briefs
      schedule: "30 8 * * 1-5"
      lookback_seconds: 604800
      member_id: ou_TARGET_MEMBER
      channel: lark
      chat_id: oc_YOUR_CHAT_ID
```

### Quick disable commands

Disable a single feature — edit `~/.alex/config.yaml`, set the feature's `enabled: false`, then restart:

```bash
# Restart the server to pick up config changes
alex dev restart backend
```

Kill switch — disable the entire scheduler:

```yaml
proactive:
  scheduler:
    enabled: false
```

### Attention gate and rate limiter

These are separate from the scheduler and controlled under `channels`:

```yaml
# Attention gate (urgency classification + quiet hours)
channels:
  lark:
    attention_gate:
      enabled: false       # disable urgency gating
      budget_max: 0        # 0 = unlimited messages per window
      budget_window_seconds: 600
      quiet_hours_start: 22  # hour 0-23
      quiet_hours_end: 8

    # Rate limiter (notification throughput cap)
    rate_limiter:
      enabled: false       # disable rate limiting
      chat_hourly_limit: 10
      user_daily_limit: 50
```

---

## 2. Rolling Back Leader Agent Changes

### Config-only rollback (no deploy)

If a config change caused problems:

```bash
# 1. Revert config
cp ~/.alex/config.yaml.bak ~/.alex/config.yaml
# or manually set the offending feature to enabled: false

# 2. Restart
alex dev restart backend
```

Always keep a backup before config changes:

```bash
cp ~/.alex/config.yaml ~/.alex/config.yaml.bak
```

### Code rollback

```bash
# 1. Find the last known-good commit
git log --oneline --all | head -20

# 2. Revert to it
git revert <bad-commit-sha>
# or for multiple commits:
git revert <oldest-bad>^..<newest-bad>

# 3. Rebuild and restart
make build && alex dev restart backend
```

### Emergency stop

If the leader agent is sending unwanted notifications and you need it stopped immediately:

```bash
# Option 1: Disable scheduler via config + restart
# Edit ~/.alex/config.yaml, set proactive.scheduler.enabled: false
alex dev restart backend

# Option 2: If Lark credentials are the problem, unset them
unset LARK_APP_ID LARK_APP_SECRET
alex dev restart backend
```

### Rollback checklist

1. Identify the symptom (wrong alerts, too many alerts, no alerts, crashes).
2. Check `GET /health` for component status.
3. If a single feature: disable that feature via config, restart.
4. If the scheduler: set `proactive.scheduler.enabled: false`, restart.
5. If code change: `git revert`, rebuild, restart.
6. Verify fix: check `/health` endpoint, confirm the problem feature no longer fires.

---

## 3. Monitoring Alerts

### Health endpoint

`GET /health` returns JSON with per-component health:

```json
{
  "status": "healthy",
  "components": [
    {
      "name": "scheduler",
      "status": "ready",
      "details": {
        "__blocker_radar__": { "registered": true, "healthy": true, "last_run": "...", "next_run": "..." },
        "__weekly_pulse__":  { "registered": true, "healthy": true },
        "__milestone_checkin__": { "registered": true, "healthy": true },
        "__prep_brief__":    { "registered": true, "healthy": true }
      }
    }
  ]
}
```

Status values: `ready`, `not_ready`, `disabled`, `error`.

A job is `healthy` when it is registered AND its last execution had no error. A job is `overdue` when its `next_run` is more than 10 minutes in the past.

### Prometheus metrics to alert on

All metrics are exported at `localhost:<prometheus_port>/metrics` when observability is enabled.

#### Missed weekly pulse

The weekly pulse should fire every Monday. Alert if no generation in 8 days:

```
# Prometheus alert rule
- alert: WeeklyPulseMissed
  expr: time() - alex_leader_pulse_generations_total_created > 691200
  for: 1h
  labels:
    severity: warning
  annotations:
    summary: "Weekly pulse has not fired in over 8 days"
```

Or check the rate of generations:

```
# No pulse generation in 8 days
increase(alex_leader_pulse_generations_total[8d]) == 0
```

#### Stuck blocker radar scans

The blocker radar runs every 10 minutes (default). Alert if scans stop:

```
- alert: BlockerRadarStalled
  expr: increase(alex_leader_blocker_scans_total[1h]) == 0
  for: 30m
  labels:
    severity: warning
  annotations:
    summary: "Blocker radar has not scanned in over 1 hour"
```

#### Alert delivery failures

Track sent vs failed notifications:

```
- alert: LeaderAlertDeliveryFailures
  expr: rate(alex_leader_alert_outcomes_total{outcome="failed"}[15m]) > 0.1
  for: 10m
  labels:
    severity: critical
  annotations:
    summary: "Leader alerts are failing to deliver (feature={{ $labels.feature }})"
```

#### High send latency

```
- alert: LeaderAlertHighLatency
  expr: histogram_quantile(0.95, rate(alex_leader_alert_send_latency_bucket[15m])) > 5000
  for: 10m
  labels:
    severity: warning
  annotations:
    summary: "Leader alert send latency p95 exceeds 5 seconds"
```

#### Rate limiter saturation

If the rate limiter is dropping too many messages, it means the leader is trying to send more than the budget allows:

```
- alert: LeaderRateLimiterSaturation
  expr: increase(alex_leader_attention_decisions_total{suppressed="true"}[1h]) > 50
  for: 15m
  labels:
    severity: warning
  annotations:
    summary: "Attention gate suppressed >50 messages in the last hour"
```

#### Leader job health (via health probe)

Script-based check against the `/health` endpoint:

```bash
#!/bin/bash
# health-check.sh — run via cron or external monitoring
HEALTH=$(curl -s http://localhost:8080/health)
STATUS=$(echo "$HEALTH" | jq -r '.status')

if [ "$STATUS" != "healthy" ]; then
  echo "ALERT: Leader agent health is $STATUS"
  echo "$HEALTH" | jq '.components[] | select(.name == "scheduler")'
  exit 1
fi
```

### Metric summary table

| Metric | Type | Labels | What to watch |
|---|---|---|---|
| `alex.leader.blocker.scans.total` | counter | — | Stops increasing = radar stalled |
| `alex.leader.blocker.detected.total` | counter | — | Sudden spike = possible false positives |
| `alex.leader.blocker.notified.total` | counter | — | >> detected = over-notification |
| `alex.leader.pulse.generations.total` | counter | — | Stops increasing = missed digest |
| `alex.leader.pulse.duration` | histogram | — | p95 > 30s = slow generation |
| `alex.leader.pulse.task_count` | histogram | — | Drops to 0 = task store issue |
| `alex.leader.alert.outcomes.total` | counter | feature, channel, outcome | outcome=failed rising = delivery problem |
| `alex.leader.alert.send.latency` | histogram | feature, channel | p95 > 5s = Lark API slow |
| `alex.leader.attention.decisions.total` | counter | urgency, suppressed | suppressed=true rising = budget exhaustion |
| `alex.leader.focus.suppressions.total` | counter | user_id | Unexpected suppression |

---

## 4. Common Failure Modes and Fixes

### No notifications are sent

**Symptoms:** All leader features enabled but no Lark messages appear.

**Check:**
1. `GET /health` — is the scheduler component `ready`?
2. Are Lark credentials set? Check `LARK_APP_ID` and `LARK_APP_SECRET` env vars.
3. Is `chat_id` correct? Must start with `oc_` for group chats.
4. Is the bot added to the target Lark group?

**Fix:**
- Set credentials: `export LARK_APP_ID=xxx LARK_APP_SECRET=yyy`
- Verify chat ID in Lark group settings.
- Restart: `alex dev restart backend`

### Blocker radar fires too often

**Symptoms:** Frequent blocker alerts for tasks that are not actually stuck.

**Tune:**
```yaml
blocker_radar:
  stale_threshold_seconds: 3600   # raise from 1800 to 1 hour
  input_wait_seconds: 1800        # raise from 900 to 30 min
  notify_cooldown_seconds: 86400  # keep at 24h per-task cooldown
  schedule: "0 */4 * * *"         # run every 4 hours instead of every 10 min
```

**Or temporarily disable:**
```yaml
blocker_radar:
  enabled: false
```

### Weekly pulse shows 0 tasks

**Symptoms:** Digest arrives but reports no tasks completed.

**Check:**
1. Is the task store populated? Tasks must exist in the shared task store.
2. Is the lookback window correct? Pulse looks back 7 days by default.
3. Check `alex.leader.pulse.task_count` metric — if consistently 0, tasks are not being written to the store.

**Fix:** Verify task store path is correct and writable. Check that task creation flows (from Lark conversations or CLI) are storing tasks.

### Scheduler starts but no jobs register

**Symptoms:** `GET /health` shows scheduler as `ready` but all jobs have `registered: false`.

**Check:**
1. Is `container.TaskStore` nil? Jobs require a task store. Check bootstrap logs for `"TaskStore not available"` warnings.
2. Are features enabled in config? `DefaultLeaderConfig()` has all features disabled.
3. Check for cron parse errors in logs: `"failed to register"` warnings.

**Fix:** Ensure `proactive.scheduler.<feature>.enabled: true` is set for each desired feature, and that the task store path is valid.

### Rate limiter dropping messages

**Symptoms:** Notifications stop arriving mid-day. Metric `alex.leader.attention.decisions.total{suppressed="true"}` spikes.

**Check:**
1. Current per-chat limit: default is 10 messages/hour.
2. Current per-user limit: default is 50 messages/day.
3. How many leader features are enabled and how often do they fire?

**Fix — raise limits:**
```yaml
channels:
  lark:
    rate_limiter:
      chat_hourly_limit: 30
      user_daily_limit: 200
```

**Fix — disable rate limiter:**
```yaml
channels:
  lark:
    rate_limiter:
      enabled: false
```

### Quiet hours blocking urgent alerts

**Symptoms:** Urgent blocker alerts not delivered during configured quiet hours.

**Note:** The attention gate allows high-urgency messages through even during quiet hours. If urgent alerts are not arriving:

1. Check that the message matches urgency keywords (e.g., "blocked", "urgent", "error").
2. The attention gate might not be wired — check `channels.lark.attention_gate.enabled`.
3. If the attention gate is disabled, all messages go through the standard notifier with no urgency bypass.

**Fix:** Enable the attention gate to get urgency classification:
```yaml
channels:
  lark:
    attention_gate:
      enabled: true
      quiet_hours_start: 22
      quiet_hours_end: 8
```

### Lark API errors

**Symptoms:** `alex.leader.alert.outcomes.total{outcome="failed"}` increasing. Logs show `"lark send error"`.

**Common causes:**
- **code=99991663**: Bot not in the chat group. Add the bot to the Lark group.
- **code=99991668**: Bot lacks `im:message:send_as_bot` permission. Update bot permissions in Lark admin console.
- **code=99991672**: Rate limited by Lark API. Reduce scheduler frequency or enable the rate limiter.
- **Network errors**: Check connectivity to `open.feishu.cn` / `open.larksuite.com`.

**Fix:** Check Lark bot permissions, verify bot membership in target groups, confirm network access.

### Prep brief fires for wrong meetings

**Symptoms:** Briefs generated for meetings that are not 1:1s.

**Tune:**
- Set a specific `member_id` to scope briefs to a single report:
  ```yaml
  prep_brief:
    member_id: ou_SPECIFIC_MEMBER
  ```
- Adjust schedule to only fire before known 1:1 times.

### Scheduler leader lock issues (multi-instance)

**Symptoms:** Duplicate notifications — the same alert sent twice.

**Cause:** Multiple server instances running without a leader lock. The current default is `LeaderLock: nil` (single-process mode).

**Fix:** Ensure only one server instance runs the scheduler, or configure an external leader lock when deploying multiple instances. For single-instance deployments, this is not a problem.

---

## Quick Reference

| Task | Command / Config |
|---|---|
| Disable one feature | Set `proactive.scheduler.<feature>.enabled: false`, restart |
| Disable all proactive | Set `proactive.scheduler.enabled: false`, restart |
| Check health | `curl http://localhost:8080/health \| jq` |
| View leader metrics | `curl http://localhost:9090/metrics \| grep alex_leader` |
| Emergency stop | Unset `LARK_APP_ID`, restart |
| Tune alert frequency | Change `schedule` cron expression in config |
| Raise rate limits | `channels.lark.rate_limiter.chat_hourly_limit: 30` |
