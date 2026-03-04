# 2026-03-04 Lark Inject Agents Team Latest Full Coverage

## Goal
- Use `lark inject`-driven verification to cover all critical paths introduced by latest agents team runtime changes:
  - team bootstrap artifacts
  - dynamic coding CLI discovery/probe
  - role-to-cli binding and fallback
  - tmux pane binding and env injection
  - role I/O loop (event logs + input injection)
  - failure isolation and recovery

## Scope Boundary
- Only agents team runtime paths are in scope:
  - `internal/infra/teamruntime/*`
  - `internal/infra/tools/builtin/orchestration/{run_tasks,reply_agent}.go`
  - `internal/domain/agent/react/background.go` (tmux input injection branch)
  - `internal/infra/coding/discovery.go`
  - `internal/infra/external/bridge/executor.go` (role event sink)
- Do not include unrelated larktools/doc/task/docx cases.

## Preconditions
1. Lark runtime healthy:
   - `./lark.sh status`
   - `main: healthy`, `kernel: healthy`
2. Inject endpoint reachable:
   - `go run ./cmd/alex lark inject --timeout 30 "agents team smoke"`
3. Record start timestamp for artifact filtering:
   - `date -u +"%Y-%m-%dT%H:%M:%SZ"`

## Evidence Paths
- Team runtime root (latest):
  - `find . .worktrees/test -path '*_team_runtime/*/teams/*' -type d 2>/dev/null | sort`
- Bootstrap artifacts:
  - `bootstrap.yaml`
  - `capabilities.yaml`
  - `role_registry.yaml`
  - `runtime_state.yaml`
  - `events.jsonl`
  - `logs/*.log`

## Coverage Matrix

### A. Bootstrap + Recovery (6 cases)
1. `BOOT-01` First dispatch must bootstrap before role dispatch.
   - Inject: team orchestration prompt that triggers multi-role execution.
   - Assert: `bootstrap_completed` exists in `events.jsonl` and timestamps precede first `role_started`.
2. `BOOT-02` All bootstrap artifacts present.
   - Assert: all 6 files exist under latest team dir.
3. `BOOT-03` Runtime state persists role entries.
   - Assert: `runtime_state.yaml.roles` contains all role IDs from `role_registry.yaml`.
4. `BOOT-04` Remove `runtime_state.yaml`, rerun inject.
   - Assert: file recreated and status set to `initialized`/updated.
5. `BOOT-05` Remove `role_registry.yaml`, rerun inject.
   - Assert: file recreated and role list restored.
6. `BOOT-06` tmux unavailable branch.
   - Pre: run on env without tmux or mask tmux.
   - Assert: `tmux_unavailable` event exists; team flow continues (no global crash).

### B. Dynamic CLI Discovery + TTL (8 cases)
1. `DISC-01` First run discovers CLIs within 5 seconds.
   - Assert: `capabilities.yaml.generated_at` exists and `capabilities` non-empty.
2. `DISC-02` Capability fields persisted per CLI.
   - Assert: each entry includes executable/version/plan/execute/stream/filesystem/network.
3. `DISC-03` Auth failure classification (`not_logged_in`).
4. `DISC-04` Auth failure classification (`unauthorized`).
5. `DISC-05` Probe timeout classification (`probe_timeout`).
6. `DISC-06` Target CLI selected when available.
   - Assert in `role_registry.yaml`: `target_cli == selected_cli`.
7. `DISC-07` Target CLI fallback when unavailable.
   - Assert: `selected_cli != target_cli` and `fallback_clis` populated.
8. `DISC-08` TTL behavior.
   - Within TTL rerun: `generated_at` unchanged.
   - After TTL expiry rerun: `generated_at` updated.

### C. Role + tmux Binding (6 cases)
1. `TMUX-01` One role one pane.
   - Assert: each role has unique `tmux_pane`.
2. `TMUX-02` Env injection command branch.
   - Assert via pane env/log bootstrap events (`tmux_pane_ready` per role).
3. `TMUX-03` Role log path isolation.
   - Assert: each role writes only to its own `logs/<role>.log`.
4. `TMUX-04` Session reuse.
   - Rerun with same team/session.
   - Assert: same `tmux_session` reused.
5. `TMUX-05` Pane bootstrap failure event.
   - Assert: `tmux_pane_bootstrap_failed` captured with role/pane.
6. `TMUX-06` Multi-role parallel stability.
   - Assert: all role panes active, no shared pane assignment.

### D. I/O Closed Loop (8 cases)
1. `IO-01` Role start event.
   - Assert: `role_started` present in role log and team log.
2. `IO-02` Tool call/result event stream.
   - Assert: `tool_call` and `result` appear during run.
3. `IO-03` Role completion event.
   - Assert: `role_completed` appears for successful roles.
4. `IO-04` Direct input injection branch.
   - Use `reply_agent(task_id=..., message="continue")`.
   - Assert: no error; new events appended after injection.
5. `IO-05` External input response branch.
   - Use `reply_agent(task_id=..., request_id=..., approved=...)`.
   - Assert: request resolved and progress continues.
6. `IO-06` Injection validation.
   - Missing `task_id`/empty `message` should return readable error.
7. `IO-07` No pane binding path.
   - Assert error contains `not bound to a tmux pane`.
8. `IO-08` Event dual-write.
   - Assert same event types appear in both role log and team event log.

### E. Failure Isolation + Degrade (6 cases)
1. `FAIL-01` One role CLI unavailable.
   - Assert other roles still run concurrently.
2. `FAIL-02` Fallback success.
   - Assert failed target role continues with fallback CLI.
3. `FAIL-03` Fallback exhausted.
   - Assert clear failure reason with role-level isolation.
4. `FAIL-04` Runtime cancellation.
   - Cancel one task; assert no global team teardown.
5. `FAIL-05` Restart recovery.
   - Restart lark runtime, rerun inject.
   - Assert bootstrap artifacts reused/recovered.
6. `FAIL-06` Cross-chat isolation.
   - Run two chat IDs concurrently.
   - Assert separate runtime dirs/team IDs and independent logs.

## Inject Command Template
```bash
go run ./cmd/alex lark inject \
  --chat-id "<chat_id>" \
  --chat-type p2p \
  --timeout 180 \
  "<message>"
```

## Team-Only Execution Sequence
1. Baseline health + smoke
   - `./lark.sh status`
   - `go run ./cmd/alex lark inject --timeout 30 "agents team smoke"`
2. Run team-focused scenario layer (mock, deterministic)
   - `go run ./cmd/alex lark scenario run --mode mock --dir tests/scenarios/lark --tag teams --json-out tmp/agents-team-mock.json --md-out tmp/agents-team-mock.md`
3. Run live inject matrix A-E using unique `chat_id` per case.
4. Collect artifacts + report failures by case ID.

## Pass Criteria
1. `BOOT-*` all pass: no role dispatch before bootstrap completion.
2. `DISC-*` all pass: capability snapshot complete, TTL and fallback verified.
3. `TMUX-*` all pass (or explicit `tmux_unavailable` degrade path pass).
4. `IO-*` all pass: output and input loops both observable and actionable.
5. `FAIL-*` all pass: single-role failure never blocks whole team.

## Status
- [x] Coverage matrix defined (A-E, 34 cases)
- [x] Inject-driven execution protocol defined
- [ ] Full matrix execution recorded
