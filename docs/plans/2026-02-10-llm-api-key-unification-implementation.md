# Plan: LLM_API_KEY Configuration Unification and Quickstart Gating

> Created: 2026-02-10
> Status: in-progress
> Trigger: Implement approved plan to unify single-key bootstrap with `LLM_API_KEY`, add profile-aware validation, and capability gating.

## Goal & Success Criteria
- **Goal**: Make local core flows runnable with a single `LLM_API_KEY` while preserving provider-specific precedence and production safety checks.
- **Done when**:
  - `LLM_API_KEY` is supported as unified fallback key.
  - Legacy unified key naming is no longer supported.
  - Runtime profile (`quickstart` / `standard` / `production`) is implemented in config load + server bootstrap + CLI validation.
  - Tool capability gating disables credential-dependent tools in quickstart mode with explicit diagnostics.
  - Docs/examples/tests are updated and full lint + tests pass.
- **Non-goals**:
  - Refactoring provider architecture to descriptor-registry pattern.
  - Relaxing production auth/session DB hard requirements.

## Current State
- Runtime provider/key resolution is split across `load.go` and `provider_resolver.go`.
- Server enforces missing-key hard failure for non-keyless providers; CLI can fallback to `mock`.
- No profile concept for runtime startup behavior.
- Tool registry always registers most builtins; missing optional credentials often fail at execution-time.
- Docs still use `OPENAI_API_KEY` as the primary bootstrap variable.

## Task Breakdown

| # | Task | Files | Size | Depends On |
|---|------|-------|------|------------|
| 1 | Add runtime profile + `LLM_API_KEY` env support and precedence rules | `internal/shared/config/{types.go,file_config.go,runtime_file_loader.go,runtime_env_loader.go,provider_resolver.go,load.go}`, tests | M | — |
| 2 | Add unified runtime validation API and CLI command `alex config validate` | `internal/shared/config/admin/readiness.go`, `cmd/alex/cli.go`, tests | M | T1 |
| 3 | Wire profile-aware server bootstrap checks | `internal/delivery/server/bootstrap/config.go`, tests | S | T1, T2 |
| 4 | Add tool capability gating for quickstart profile | `internal/app/toolregistry/{registry.go,registry_builtins.go}`, `internal/app/di/container_builder.go`, tests | M | T1, T2 |
| 5 | Update docs/config examples to `LLM_API_KEY` semantics | `.env.example`, `configs/config.yaml`, `docs/reference/CONFIG.md`, `docs/guides/quickstart.md`, `README.md`, `docs/operations/DEPLOYMENT.md` | M | T1-T4 |
| 6 | Run code review checklist, lint, full tests, then incremental commits and merge | code-review skill refs + repo checks | M | T1-T5 |

## Technical Design
- **Approach**:
  - Extend runtime config with `profile` field and normalize to one of `quickstart|standard|production`.
  - Keep key precedence deterministic: config/override `api_key` > provider-specific env > `LLM_API_KEY`.
  - Build profile-aware runtime validation helpers to generate blocking errors + warnings and disabled capabilities.
  - Use those capability decisions when constructing tool registries to skip unavailable tools in quickstart.
  - Surface the same validation output in `alex config validate` to keep operators and CLI behavior aligned.
- **Alternatives rejected**:
  - Immediate provider-descriptor refactor (too broad for first implementation).
  - Immediate removal of provider-specific keys (breaks compatibility for existing users).
- **Key decisions**:
  - `LLM_API_KEY` is the only unified key name.
  - Legacy unified key alias is removed immediately (no alias path).
  - Production remains strict for auth/session/database requirements.

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| CLI/server behavior drift after introducing profile logic | M | H | Single validation helper reused by both paths + integration tests |
| Tool gating accidentally removes essential tools | M | H | Gate only credential-dependent optional tools + regression tests on core tool presence |
| Docs and examples become inconsistent during migration | M | M | Single sweep replacing bootstrap guidance + grep verification before final checks |
| Hidden tests rely on old variable naming | M | M | Add compatibility tests and run full test suite |

## Verification
- Unit tests:
  - Provider key precedence with `LLM_API_KEY` fallback.
  - Profile normalization and validation behavior.
  - Tool gating behavior per profile/credential set.
- Integration checks:
  - `alex config validate` output in quickstart vs production.
  - Server config loading with each profile.
- Full repo quality gates:
  - `make fmt`
  - `make lint`
  - `make test`
- Rollback:
  - Revert feature commits in order if regressions found; keep commits scoped by subsystem for targeted rollback.
