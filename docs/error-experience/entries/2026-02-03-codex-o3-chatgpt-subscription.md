# 2026-02-03 - Codex default model `o3` fails with ChatGPT subscription auth

## Error
- Background tasks dispatched to the Codex external agent failed immediately when using `model="o3"`.
- The upstream returned: `The 'o3' model is not supported when using Codex with a ChatGPT account.`

## Impact
- `bg_dispatch(agent_type=codex)` is broken out-of-the-box for users authenticating Codex CLI via ChatGPT subscription (OAuth).

## Root Cause
- Repo defaults and config/docs examples set `runtime.external_agents.codex.default_model: "o3"`, but Codex CLI subscription auth does not support `o3`.

## Remediation
- Switch the default model to `gpt-5-codex` in:
  - baseline defaults (`internal/config/types.go`)
  - example config (`configs/config.yaml`)
  - docs (`docs/reference/external-agents-codex-claude-code.md`)
- Add a regression test to prevent reintroducing an incompatible default.

## Status
- fixed

