# Merge Unmerged Worktrees

## Goal

Review current worktrees and local branches that are not merged into `main`, keep branches with clear user value and acceptable risk, and merge suitable changes into `main`.

## Selection Criteria

- Exclude branches already at `main` or with no unique patch versus `main`.
- Prefer fixes and targeted behavioral improvements over broad architectural refactors without active context.
- Reject stale branches that are heavily behind `main` unless the change is obviously isolated and still relevant.
- Validate each merge candidate with focused tests and a review pass before landing.

## Steps

- Inventory all current worktrees and unmerged local branches.
- Group duplicates / stale branches / active candidates by patch uniqueness and scope.
- Inspect candidate diffs, tests, and merge risk.
- Merge low-risk, high-value branches into the integration branch.
- Run targeted validation and code review.
- Fast-forward `main`, clean merged worktrees/branches, and report remaining follow-ups.

## Progress

- 2026-03-09: Created integration worktree and started inventory.
- 2026-03-09: Classified current local-only branches into duplicates/stale refactors/feature branches/active worktree deltas.
- 2026-03-09: Selected `fix/review-followup-round2` and `fix/team-cli-more-cases` for merge; both are focused team-cli correctness/usability improvements with targeted test coverage.
- 2026-03-09: Rejected for now:
  - Large stale refactor branches (`codex/nonweb-rescan-*`, `elephant/analyze_and_patch*`, `worktree-agent-*`, `wt-branch`) due broad blast radius and heavy drift from `main`.
  - Feature branches (`feat/anygen-skills-cli`, `ckl/notebooklm-cli-independent-20260305`) due larger product surface and no immediate need to land during cleanup.
  - `fix/ci-arch-policy-exceptions-20260304` because current architecture baseline already passes without the extra exceptions.
  - Active uncommitted `fix/bg-stale-slot-selfheal` because it lacks tests / a demonstrated repro and should be finished before merge.
- 2026-03-09: User explicitly requested merging `ckl/notebooklm-cli-independent-20260305`, `feat/anygen-skills-cli`, and `fix/bg-stale-slot-selfheal`; re-opened follow-up integration pass to land them with focused validation.
- 2026-03-09: Merged `fix/bg-stale-slot-selfheal` after adding a regression test for stale active-task counting.
- 2026-03-09: Confirmed `ckl/notebooklm-cli-independent-20260305` and `feat/anygen-skills-cli` are functionally superseded by the current CLI-contract implementations already on `main`; consumed both branches without regressing the newer contract.
