# Plan: OpenClaw-Inspired Personality + Response Prompt Optimization (2026-02-06)

## Goal
- Research OpenClaw's current/public system prompt setup.
- Optimize this project's system prompt guidance (personality + response behavior) for better practicality and consistency.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.
- Loaded recent error/good summaries and long-term memory notes relevant to prompt and delivery practices.

## Scope
1. Locate the active system prompt composition path and editable persona config.
2. Verify OpenClaw prompt traits from official docs/source.
3. Update prompt instructions focused on:
   - personality calibration (direct, pragmatic, low-fluff)
   - response behavior (action-first, evidence-backed, concise-by-default)
4. Add/adjust tests if prompt assertions are impacted.
5. Run full lint and test before delivery.

## Progress
- 2026-02-06 09:00: Created worktree branch `eli/openclaw-personality-response-opt` from `main` and copied `.env`.
- 2026-02-06 09:00: Reviewed engineering practices and memory summaries/entries.
- 2026-02-06 09:00: Located prompt composition and persona config entry points.
- 2026-02-06 09:00: Verified OpenClaw system prompt setup from official docs (`/concepts/system-prompt`, `/concepts/soul`, `/hooks/templates/agents`).
- 2026-02-06 09:00: Updated `configs/context/personas/default.yaml` with clearer Personality / Execution posture / Response contract guidance.
- 2026-02-06 09:00: Added explicit proactive response triggers to persona voice (progress updates, blocker handling, completion reporting, adjacent issue handling, implicit required checks).

## Validation
- `make fmt`
- `make vet`
- `make test`
