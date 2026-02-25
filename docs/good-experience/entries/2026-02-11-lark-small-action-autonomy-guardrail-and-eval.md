# 2026-02-11 — Lark 小事自治修复：Delegated Low-Risk No-Reconfirm

Impact: Closed a real autonomy regression where delegated low-risk Lark actions still triggered repeated confirmation loops ("我的理解是...对吗？"), while keeping high-risk confirmation boundaries intact.

## What changed
- Added explicit delegated-autonomy guardrails in prompt surfaces:
  - `configs/context/personas/default.yaml`
  - `internal/app/context/manager_prompt.go`
  - `internal/app/agent/preparation/service.go`
  - `internal/shared/agent/presets/prompts.go`
- Added regression coverage:
  - Prompt/routing assertions in related tests (`manager`, `presets`, `default_prompt`).
  - Foundation heuristic regression ensuring delegated low-risk intents rank `channel` above `request_user/clarify`.
  - Motivation-aware dataset case:
    - `motivation-delegated-low-risk-action-no-reconfirm`

## Why this worked
- Root cause was prompt-policy mismatch: persona had a strong universal confirmation style that overrode small-action autonomy.
- Fix made confirmation conditional by risk class instead of unconditional style.
- Added explicit evaluation case so this behavior is continuously measurable.

## Validation
- Targeted tests passed:
  - `go test ./internal/app/context ./internal/shared/agent/presets ./internal/app/agent/preparation ./evaluation/agent_eval`
- Motivation-aware suite before/after:
  - baseline `pass@1`: `8/9` (88.9%)
  - post-fix `pass@1`: `9/10` (90.0%)
  - new delegated case top1: `channel(111.10)`
- Full checks:
  - `make fmt` ✅
  - `make vet` ✅
  - `make check-arch` ✅
  - `make test` ⚠️ existing unrelated failure in `internal/infra/analytics` due missing `docs/analytics/tracking-plan.yaml`
