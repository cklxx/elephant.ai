# 2026-02-10 — Context Autonomy Rules and Safe Environment Injection

Impact: Upgraded default context with stronger autonomous-exploration rules and runtime environment injection, while enforcing secret-safe redaction so prompt context stays actionable without leaking sensitive env values.

## What changed
- Expanded tool-routing guardrails in `internal/app/context/manager_prompt.go` with explicit autonomy loop and host-CLI fallback rules.
- Added runtime environment hint injection in `internal/infra/environment/summary.go` and `internal/infra/environment/utils.go`:
  - include safe hints (`SHELL`, language/runtime envs, selected ALEX routing keys)
  - summarize `PATH` structurally
  - redact secret-like env keys by default
- Added/updated tests:
  - `internal/app/context/manager_prompt_routing_test.go`
  - `internal/app/context/manager_test.go`
  - `internal/infra/environment/summary_test.go`
- Added high-density SOP guidance section and wired it into default knowledge:
  - `docs/reference/TASK_EXECUTION_FRAMEWORK.md`
  - `configs/context/knowledge/default.yaml`
  - `configs/context/policies/default.yaml`
  - `configs/context/worlds/default.yaml`

## Result
- Agent prompt now carries materially stronger autonomous instructions plus real host-environment signals.
- Environment hints are injected with safety filtering and bounded size, reducing secret-exposure risk.
- Validation passed:
  - `make fmt`
  - `make test`
  - `make check-arch`

## Why this worked
- Combined prompt-level behavior constraints with runtime context enrichment instead of relying on one side only.
- Treated environment injection as a security-sensitive surface and codified redaction + allowlist behavior in code + tests.
