# 2026-02-12 — Lark test early exit due invalid LLM profile

## Background
- Symptom: `scripts/lark/supervisor.sh` repeatedly reports `test restart failed` and keeps `test=down`.
- Immediate failure in `lark-test.log`: config load aborts with `invalid llm profile for provider "codex"` due key/base URL mismatch.
- Root mismatch pattern observed:
  - managed override sets `llm_provider=codex` and `base_url=https://chatgpt.com/backend-api/codex`
  - runtime key is vendor-specific `sk-kimi-*`
  - pre-override runtime `base_url` is Kimi-compatible, but override replaces it.

## Goals
1. Prevent override-induced startup abort loop in local Lark test/supervisor flow.
2. Keep strict mismatch validation as default (no broad relaxation).
3. Cover the new behavior with focused tests.

## Plan
- [x] Reproduce from logs and isolate exact mismatch path.
- [x] Identify where override precedence causes invalid atomic profile.
- [x] Implement targeted repair in runtime config loading:
  - only for non-production profile
  - only when mismatch is caused by managed `base_url` override
  - fallback to pre-override `base_url` and re-validate.
- [x] Add/adjust loader tests for repair success and non-repair failure cases.
- [x] Run focused tests, then full lint + full tests.
- [x] Run mandatory code review workflow and address findings.
- [ ] Commit incrementally, merge back to `main`, remove temp worktree.

## Notes
- This fix is constrained to local config loading path and avoids changing supervisor restart policy.
- Maintain architecture guardrail: no memory/RAG deps in `agent/ports`.
