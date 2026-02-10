# Plan: Image-Creation HTTP 400 Fix (2026-02-10)

## Goal
Make `skills/image-creation/run.py` robust enough to avoid current HTTP 400 failures and provide actionable diagnostics when backend rejects requests.

## Scope
- Reproduce current failure and capture backend error payload.
- Fix request construction and endpoint/model resolution behavior for ARK images API.
- Add tests for error payload extraction and endpoint/model fallback behavior.
- Validate via lint/tests and one real invocation.
- Remove duplicated `lark-messaging` skill now that unified Lark `channel` tool already covers message/file actions.

## Out of Scope
- Replacing ARK backend.
- Changing unrelated skills.

## Plan
1. [x] Reproduce 400 and inspect HTTP error response body.
2. [x] Update image-creation request logic and error handling.
3. [x] Add/adjust unit tests.
4. [x] Run lint + tests + real command verification.
5. [x] Remove duplicated `lark-messaging` skill and refresh generated skill assets/cases.

## Progress Log
- 2026-02-10 17:07: Created worktree and started root-cause analysis for `image-creation` HTTP 400.
- 2026-02-10 17:10: Reproduced backend rejection: requested image size must be at least 3,686,400 pixels.
- 2026-02-10 17:11: Implemented size normalization and HTTP error body propagation in `skills/image-creation/run.py`.
- 2026-02-10 17:12: Ran `pytest -q skills/image-creation/tests/test_image_creation.py` (10 passed).
- 2026-02-10 17:12: Real invocation succeeded with requested `1024x1024` auto-adjusted to `1920x1920`.
- 2026-02-10 17:16: Removed duplicated `skills/lark-messaging` entrypoint and tests; kept canonical Lark delivery through `channel` / `lark_*` built-in tools.
- 2026-02-10 17:16: Regenerated `evaluation/skills_e2e/cases.yaml` and `web/lib/generated/skillsCatalog.json` from current `skills/` tree (29 skills).
- 2026-02-10 17:17: Ran `pytest -q skills` (270 passed). Attempted web test command but local `next` binary was missing in this environment.
