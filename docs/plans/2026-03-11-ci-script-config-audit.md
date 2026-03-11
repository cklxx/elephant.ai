# CI / Scripts / Config Audit

## Scope

- Review `Makefile` and `scripts/` for duplicated CI check logic.
- Run `shellcheck` over shell scripts in `scripts/` and fix actionable issues.
- Inspect `configs/` for stale or unused configuration.
- Fix security issues found during the audit and commit the changes.

## Plan

1. Map CI-related targets and script entrypoints.
2. Run static checks on shell scripts and inspect configs usage.
3. Implement minimal fixes for real issues.
4. Validate with targeted tests/lint/review and commit.

## Findings

- CI logic is duplicated across `.github/workflows/ci.yml`, `scripts/pre-push.sh`, `scripts/test.sh`, and `Makefile` targets. The local gate is only a curated subset plus local-only guards, not a strict single source of truth.
- `scripts/install.sh` previously allowed binary installation to continue when the release checksum manifest was missing or lacked the requested artifact entry. That was a fail-open supply-chain verification path.
- `configs/config-schema.json` was stale duplicate data. Runtime schema validation only consumes `internal/shared/config/schema/config-schema.json`.
- Repository-wide `shellcheck` still reports 76 findings, concentrated in older E2E/helper scripts. The touched audit files are now clean under `shellcheck -x`.

## Validation

- `shellcheck -x scripts/install.sh scripts/pre-push.sh scripts/lib/common/build.sh scripts/setup_local_runtime.sh scripts/publish-npm.sh scripts/lib/common/cgo.sh`
- `go test ./...`
- `python3 skills/code-review/run.py review`
