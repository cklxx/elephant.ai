# Plan: internal/agent duplication refactor

Goal: Remove repeated logic in internal/agent by introducing shared text utilities and consolidating deep-clone helpers while preserving behavior.

Status legend: [ ] pending, [x] done, [~] in progress

## Steps
- [x] 1) Baseline + setup: load repo practices, run claude -p, and capture duplication inventory.
- [x] 2) Add shared text utilities (keyword extraction, similarity, truncate, normalize) with tests (TDD).
- [x] 3) Refactor hooks/coordinator to use shared utilities; remove duplicate helpers.
- [x] 4) Refactor react context/runtime to reuse shared utilities and exported clone helpers.
- [x] 5) Export CloneMapAny in ports/agent and reuse from react context.
- [x] 6) Update tests if needed; run full lint + test.
- [x] 7) Record changes and finalize plan updates.

## Notes
- Keep agent/ports free of memory/RAG deps.
- Preserve existing behavior; new utilities must be drop-in replacements.
