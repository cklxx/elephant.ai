# Session Evaluation Platform — Implementation Plan

**Status**: In Progress
**Branch**: `eval-platform`
**Updated**: 2026-02-07

## Batches

- [x] Batch 1: Scaffold eval-server + eval-web
- [ ] Batch 2: Eval-server core wiring
- [ ] Batch 3: RL pipeline backend (TDD)
- [ ] Batch 4: Task management backend (TDD)
- [ ] Batch 5: Frontend dashboard + evaluations
- [ ] Batch 6: Frontend sessions + RL data
- [ ] Batch 7: Migrate debug pages
- [ ] Batch 8: LLM Judge integration
- [ ] Batch 9: Polish + validation

## Architecture

Two independently deployable artifacts:
- `cmd/eval-server/` — Go binary, lightweight API server
- `eval-web/` — Next.js 16 admin SPA

## Key Decisions

- Eval-server reuses `EvaluationService` from `internal/delivery/server/app/`
- Middleware reused from `internal/delivery/server/http/`
- Frontend is a separate Next.js app with its own shadcn/ui components
- No auth needed (admin tool)
- YAML config only
