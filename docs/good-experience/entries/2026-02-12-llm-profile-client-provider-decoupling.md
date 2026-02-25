# 2026-02-12 — LLM Profile + Client Provider Decoupling

Impact: Removed provider/key/base_url assembly from execution components and centralized it into an atomic runtime profile plus client-provider helper, reducing cross-module coupling and mismatch failures.

## What changed
- Added shared atomic profile resolution:
  - `internal/shared/config/llm_profile.go`
  - `ResolveLLMProfile` + fail-fast mismatch checks for provider/key/base_url consistency.
- Enforced profile validation at load/validate boundaries:
  - `internal/shared/config/load.go`
  - `internal/shared/config/validate.go`
- Added app-layer client provider helper:
  - `internal/app/agent/llmclient/provider.go`
  - Components now call `GetIsolatedClientFromProfile` / `GetClientFromProfile`.
- Refactored execution path to consume profile-first config:
  - `internal/app/agent/config/config.go`
  - `internal/app/agent/coordinator/coordinator.go`
  - `internal/app/agent/preparation/service.go`
  - `internal/app/agent/preparation/analysis.go`
  - `internal/app/agent/hooks/memory_capture.go`
- Reduced tool-registry config coupling:
  - Removed LLM fields from `toolregistry.Config`; kept only tool-relevant dependencies.

## Why this worked
- Treating LLM runtime data as one atomic profile follows common platform design practice: compose once at boundary, consume everywhere else.
- A small app-layer client provider keeps low-level config translation in one place and keeps components focused on business flow.
- Fail-fast mismatch validation prevents silent runtime misroutes.

## Validation
- Lint: `./scripts/run-golangci-lint.sh run ./...` ✅
- Focused packages:
  - `go test ./internal/shared/config ./internal/app/agent/llmclient ./internal/app/agent/preparation ./internal/app/agent/coordinator ./internal/app/agent/hooks ./internal/app/toolregistry ./internal/app/di ./internal/app/agent/kernel ./internal/app/scheduler` ✅
- Full suite:
  - `go test ./...` ✅
