# 2026-03-05 Cleanup Claire Facade and Unused Artifacts

## Goal
- Remove provably unused facade/worktree artifact files from tracked repository content.
- Keep cleanup conservative: only delete files/directories with zero runtime/build references and clear duplication evidence.

## Scope
- Target explicitly reported path: `.claire/worktrees/lark-md-to-docx/internal/infra/adapters`.
- Scan for additional tracked "worktree/facade leftovers" at project level.

## Plan
1. Confirm reference graph and usage for `.claire/worktrees/lark-md-to-docx/internal/infra/adapters/*`.
2. Identify other tracked files that are likely accidental worktree residues.
3. Remove confirmed-unused files and add ignore rules to prevent reintroduction.
4. Run validation (lint/test subset as applicable) and mandatory code review script.
5. Commit, merge back to `main` via `--ff-only`, and push.

## Progress
- [x] Created plan.
- [x] Usage/reference validation complete.
- [x] Cleanup changes applied.
- [x] Validation complete.
- [ ] Commit + merge + push complete.
