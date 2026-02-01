# Plan: Refactor roadmap docs to be OKR-first

## Goal
Rewrite all roadmap documents in `docs/roadmap/` so OKR structure is the primary organizing principle, with milestones/initiatives derived from OKRs and aligned to the Calendar+Tasks north-star slice.

## Scope
- In scope: `docs/roadmap/*.md` (main roadmap + track roadmaps + audit note).
- Out of scope: implementation changes; only documentation updates.

## Plan
- [completed] Define OKR backbone (global + per-track) aligned to Calendar+Tasks and north-star metrics.
- [completed] Refactor main roadmap to OKR-first structure and update dependencies/acceptance to reflect OKRs.
- [completed] Refactor track roadmaps to OKR-first structure; preserve existing content as initiatives/milestones.
- [completed] Update implementation audit with a brief OKR mapping note.
- [completed] Run lint/tests and restart dev services.
- [in_progress] Commit changes in incremental steps.
