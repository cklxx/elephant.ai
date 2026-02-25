# Architecture Validation — OKR + Proactive + Memory + Session + External Agents

> Started: 2026-01-31
> Status: complete (tests failed in internal/memory)

## Scope
- Verify OKR + proactive architecture wiring (config → DI → hooks → scheduler/tooling).
- Verify memory capture/recall + proactive refresh behavior.
- Verify session history recall (storage + summarization + replay controls).
- Verify external code agents (Codex + Claude Code) integration and dispatch.
- Record validation results, gaps, and follow-ups.

## Plan
- [x] Review OKR + proactive architecture paths and existing tests.
- [x] Review memory capture/recall + proactive refresh paths and tests.
- [x] Review session history recall paths and tests.
- [x] Review external agent (Codex/Claude Code) config + executors + tests.
- [x] Run full lint + test suite (tests failed in `internal/memory`).
- [x] Document validation results and next actions.

## Validation Notes

### OKR + Proactive
- Config path: `configs/config.yaml` → `runtime.proactive.okr` with `enabled`, `goals_root`, `auto_inject`.
- Hook wiring: `internal/di/container_builder.go` registers `OKRContextHook` when proactive + OKR are enabled.
- Tools: `internal/toolregistry/registry.go` registers `okr_read`/`okr_write` with goals root override.
- Scheduler: `internal/scheduler/scheduler.go` syncs OKR goal triggers; `internal/server/bootstrap/server.go` gates scheduler start on `Runtime.Proactive.Scheduler.Enabled`; `internal/server/bootstrap/scheduler.go` resolves goals root.
- Tests: `internal/tools/builtin/okr/*_test.go`, `internal/scheduler/scheduler_test.go`.

### Memory + Proactive Refresh
- Hooks: `internal/agent/app/hooks/memory_recall.go`, `memory_capture.go`, `conversation_capture.go`; registered in `internal/di/container_builder.go`.
- Per-request policy: `internal/channels/base.go` sets `MemoryPolicy` on context.
- Service + stores: `internal/memory/service.go`, `file_store.go`, `hybrid_store.go`, `postgres_store.go`.
- Proactive mid-loop refresh: `internal/agent/domain/react/runtime.go` (`refreshContext`, `ProactiveContextRefreshEvent`).
- Tests: `internal/agent/app/hooks/memory_recall_test.go`, `memory_capture_test.go`, `internal/memory/*_test.go`, `internal/agent/domain/react/runtime_test.go`.

### Session History Recall
- History recall + stale handling: `internal/agent/app/preparation/history.go` (replay + summarization, stale reset).
- History injection: `internal/agent/app/preparation/service.go` (`Prepare` loads session history).
- SSE replay controls: `internal/server/http/sse_handler_stream.go` (`replay=full|session|none`).
- Tests: `internal/agent/app/preparation/service_history_test.go`, `service_stale_test.go`, `internal/context/manager_test.go`.

### External Agents (Codex + Claude Code)
- Config schema: `internal/config/types.go` (`ExternalAgentsConfig`); defaults in `configs/config.yaml` disabled.
- Registry: `internal/external/registry.go` wires executors when enabled.
- Dispatch: `internal/tools/builtin/orchestration/bg_dispatch.go` accepts `agent_type` (`claude_code`, `codex`).
- Tests: `internal/tools/builtin/orchestration/bg_tools_test.go`, `internal/agent/domain/react/background_test.go`, `internal/config/loader_test.go`, `internal/llm/openai_responses_client_test.go`.

### Config Snippets (YAML)
```yaml
runtime:
  proactive:
    enabled: true
    okr:
      enabled: true
      goals_root: "~/.alex/goals"
      auto_inject: true
    memory:
      enabled: true
      auto_recall: true
      auto_capture: true
```

```yaml
runtime:
  external_agents:
    claude_code:
      enabled: true
      binary: "claude"
    codex:
      enabled: true
      binary: "codex"
```

## Risks / Issues Observed
- `internal/agent/app/hooks/okr_context.go` risk icon selection uses `<25` before `<10`, so the `✗` branch is unreachable (should check `<10` first).
- `./dev.sh test` fails: `internal/memory/service_test.go` references missing `RetentionPolicy` + `NewServiceWithRetention` (build failure).

## Follow-ups
- [ ] Add integration/e2e coverage for scheduler → notifier delivery with real OKR goal files.
- [ ] Add integration coverage for proactive memory refresh injection (simulate tool results across iterations).
- [ ] Validate external agent CLIs and secrets in a real environment (manual).
- [ ] Decide whether to implement memory retention policy support or remove the failing tests.
