# 2026-02-26 — Non-web core logic simplification with immediate reuse

Impact: Reduced cross-layer maintenance burden by removing duplicated core assembly/config branches and ensuring new abstractions were reused immediately in multiple call sites.

## What changed

- Extracted shared DI agent config assembly into `buildAgentAppConfig()` and reused it in both primary and alternate coordinator wiring paths.
- Reworked `applyLarkConfig` into composable helpers (typed assignment helpers + browser/persistence sub-application), preserving behavior while reducing branch-heavy logic.
- Replaced switch-heavy CLI override mutation with a handler registry + typed parsers, centralizing set/clear behavior per field.
- Consolidated HTTP limits application for file/override flows via one shared helper path.
- Introduced shared Lark pagination helpers and applied them across multiple services (`calendar`, `drive`, `task`, `vc`, `mail`, `okr`, `wiki`, `contact`, `bitable`, `calendar_multi`) so abstraction was directly consumed instead of left idle.

## Why this worked

- Refactors were behavior-preserving and boundary-local (DI/bootstrap/CLI/lark infra) with clear ownership.
- Shared helpers were introduced only where duplicated paths already existed and were adopted immediately at call sites.
- Validation gates were run end-to-end before merge, preventing readability-only edits from hiding regressions.

## Validation

- `./scripts/go-with-toolchain.sh test ./cmd/alex ./internal/delivery/server/bootstrap ./internal/app/di ./internal/shared/config ./internal/infra/lark`
- `./scripts/pre-push.sh`
- `python3 skills/code-review/run.py '{"action":"review"}'` + manual P0/P1 review

## Metadata
- id: good-2026-02-26-non-web-core-logic-simplification
- tags: [good, maintainability, readability, simplification, non-web, core-logic]
- links:
  - docs/plans/2026-02-26-non-web-systematic-maintainability-optimization.md
  - internal/delivery/server/bootstrap/config.go
  - cmd/alex/cli.go
  - internal/app/di/agent_app_config.go
  - internal/infra/lark/pagination.go
