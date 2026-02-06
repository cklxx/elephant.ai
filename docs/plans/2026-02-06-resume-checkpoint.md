# Plan: Cross-process orchestration resume on server restart

Owner: cklxx
Date: 2026-02-06
Status: in_progress

## Goal
Inspect existing persistence and checkpointing for workflow orchestration across server restarts, identify gaps, and propose minimal patch plan with file-level targets and tests.

## Scope
- internal/infra/session/state_store
- internal/app/agent/coordinator (server coordinator)
- internal/domain/agent/react runtime + checkpoints
- delivery/server integration

## Steps
1. Inventory current state store + checkpoint implementations and where they are wired.
2. Trace coordinator/runtime restore behavior and server restart flow.
3. Identify missing persistence/resume gap(s).
4. Draft minimal patch plan with concrete files and tests.

## Updates
- 2026-02-06: Plan created.
- 2026-02-06: Completed inventory of state store, coordinator/task execution, and react checkpoint runtime.
- 2026-02-06: Identified restart resume gaps and drafted patch plan outline.
