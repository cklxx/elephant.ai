# elephant.ai Roadmap

Updated: 2026-02-11

This roadmap is a **guided navigation + execution priority** for the current codebase.

## 1) Recommended reading order

1. `README.md` / `README.zh.md`
2. `docs/README.md`
3. `docs/reference/ARCHITECTURE_AGENT_FLOW.md`
4. `docs/reference/CONFIG.md`
5. `docs/guides/quickstart.md`
6. `docs/operations/DEPLOYMENT.md`

## 2) Current runtime map

- Delivery: `internal/delivery/*`, `cmd/*`, `web/`
- Application: `internal/app/*`
- Domain: `internal/domain/*`
- Infrastructure: `internal/infra/*`
- Shared cross-cutting: `internal/shared/*`

Key entrypoints:
- CLI: `cmd/alex/main.go`
- Server: `cmd/alex-server/main.go`
- Eval server: `cmd/eval-server/main.go`

## 3) Active architecture priorities

### P0: Reliability and resume semantics
- Harden cross-process task continuity and checkpoint recovery.
- Scope: `internal/domain/agent/react`, `internal/app/agent/coordinator`, `internal/infra/session`.

### P0: Coding gateway foundation (reprioritized)
- Prioritize gateway contract and local coding-adapter bring-up for exploration speed.
- Scope: `internal/coding/gateway.go`, `internal/coding/adapters/`, `internal/coding/adapters/detect.go`.

### P1: Tooling surface stability
- Keep core tool inventory stable and explicitly versioned in docs/eval.
- Scope: `internal/app/toolregistry`, `internal/infra/tools`, `evaluation/`.

### P1: Event consistency across Lark/Web/CLI
- Maintain one workflow envelope contract across channels.
- Scope: `internal/app/agent/coordinator/workflow_event_translator.go`, `internal/delivery/server/http/`, `internal/delivery/channels/lark/`, `web/hooks/useSSE/`.

### P2: Memory quality and policy tuning
- Improve memory retrieval precision and policy gating for proactive usage.
- Scope: `internal/infra/memory`, `internal/app/context`, `internal/app/agent/hooks`.

### P2: External agent orchestration maturity
- Improve external bridge robustness, permission flow UX, and observability.
- Scope: `internal/infra/external`, `internal/infra/tools/builtin/orchestration`.

## 4) Delivery quality gates

Before merge:

```bash
alex dev lint
alex dev test
npm --prefix web run lint
npm --prefix web run test
```

For runtime/eval regressions, also run the target evaluation suite under `evaluation/`.

## 5) Documentation maintenance rule

When architecture/tool/config behavior changes:
- Update non-record docs first (`docs/reference`, `docs/guides`, `docs/operations`, indexes).
- Keep record docs (`docs/plans`, `docs/analysis`, `docs/research`, experience entries) as historical artifacts.
