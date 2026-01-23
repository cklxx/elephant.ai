# Plan: Merge main into feat/apps-connectors

## Goal
Bring `feat/apps-connectors` up to date with `main`, resolve conflicts cleanly, and push the updated branch.

## Steps
1. Fetch latest `main`, check out `feat/apps-connectors`, and merge `origin/main`.
2. Resolve merge conflicts in `web/lib/api.ts` and `web/lib/types.ts`, keeping apps config types and new runtime catalog APIs.
3. Run full lint and tests.
4. Commit the merge and push.

## Progress
- 2026-01-23: Fetched `origin/main`, checked out `feat/apps-connectors`, and started the merge; conflicts in `web/lib/api.ts` and `web/lib/types.ts`.
- 2026-01-23: Resolved conflicts by keeping apps config APIs/types and adopting the new types re-export structure.
- 2026-01-23: Ran full lint and test suite.
