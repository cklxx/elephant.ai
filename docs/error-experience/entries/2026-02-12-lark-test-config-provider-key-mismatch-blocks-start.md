# Lark test agent startup blocked by provider/key mismatch in test runtime config

Date: 2026-02-12

## Symptom

`scripts/lark/test.sh start` repeatedly exited early, so dual-agent validation could not proceed.

Observed log errors in `.worktrees/test/logs/lark-test.log`:

- `invalid llm profile for provider "codex"... key looks moonshot/kimi`
- `invalid llm profile for provider "anthropic"... key prefix=sk-kimi-... incompatible`

## Root Cause

`~/.alex/test.yaml` had provider/profile/credential combinations that violate runtime validation invariants:

- `overrides.llm_provider=codex` with a moonshot/kimi key and codex base URL combination mismatch.
- runtime provider switched to `anthropic` while key remained `sk-kimi-*` (moonshot/kimi-style).

## Remediation

1. Keep provider/base_url/api_key aligned by vendor family in test runtime config.
2. Before startup, validate with a dry-run config load command path (or add a startup precheck to `scripts/lark/test.sh` that fails fast with actionable hints).
3. For dual-agent kernel verification, treat test-agent-up as a prerequisite gate; if test agent cannot start, kernel-count conclusion is bounded to running agents only.
