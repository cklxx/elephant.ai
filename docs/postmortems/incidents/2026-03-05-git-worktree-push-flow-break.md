# Incident Postmortem — 2026-03-05 Git Push/Worktree Flow Break

## Incident
- Date: 2026-03-05
- Incident ID: inc-2026-03-05-git-worktree-push-flow-break
- Severity: S2 (internal workflow disruption)
- Component(s): Git workflow, branch synchronization, local pre-push gate
- Reporter: ckl

## What Happened
- Symptom: Branch state became confusing (`main`/`origin/main` divergence, temporary rebase helper branches), and push attempts from an ad-hoc temporary clone triggered incorrect behavior and confusion.
- Trigger condition: Rebase/merge work was executed in a temporary clone with `origin` pointing to a local repository path, then mixed with operations in the primary workspace.
- Detection channel: User correction during live operation (`pre-push/worktree` and identity confusion).

## Impact
- User-facing impact: Delayed completion of requested merge and CI stabilization.
- Internal impact: Repeated branch-state reconciliation, extra manual cleanup of worktrees/temporary clones, loss of execution confidence.
- Blast radius: Local repository state and delivery cadence for the current task.

## Timeline (absolute dates)
1. 2026-03-05 16:06 +08:00 — Started rebase/merge operations using an ad-hoc temporary clone.
2. 2026-03-05 16:27 +08:00 — `main` was updated, but branch lineage and execution context became mixed between temporary clone and primary workspace.
3. 2026-03-05 16:30 +08:00 — CI showed architecture policy failure; failure was not addressed immediately.
4. 2026-03-05 16:34 +08:00 — User flagged incorrect sequencing and repository handling.
5. 2026-03-05 16:38 +08:00 — Uncommitted changes were stashed; `main` sync was partially restored.
6. 2026-03-05 16:45 +08:00 — Postmortem + guardrails drafted and integrated.

## Root Cause
- Technical root cause:
  - Push/rebase context was not constrained to the primary repository/worktree lineage.
  - Missing hard guard for local-path `origin` in pre-push workflow allowed risky context.
  - Architecture policy exceptions lagged behind actual import graph, causing deterministic CI failure.
- Process root cause:
  - Failure-first execution discipline was not followed when CI produced first red signal.
  - Branch health checks were not enforced as a fixed gate before merge/push.
- Why existing checks did not catch it:
  - Existing pre-push checks validate code quality but did not validate repository context (local clone vs primary repo/worktree).

## Fix
- Code/config changes:
  - Added missing architecture-policy exceptions for known imports.
  - Added `origin` remote guard in `scripts/pre-push.sh` to reject local filesystem remotes by default.
  - Added explicit push-scope rule to `AGENTS.md` and `docs/guides/engineering-workflow.md`.
- Scope control / rollout strategy:
  - Restrict pushes to primary repo workspace and managed worktrees sharing the same `.git`.
  - Keep override (`ALLOW_LOCAL_ORIGIN_PUSH=1`) explicit for exceptional cases.
- Verification evidence:
  - `make check-arch-policy` passes after exception updates.
  - `scripts/pre-push.sh` now blocks local-path origin by default.
  - Branch sync checks show `main...origin/main` convergence after merge/push.

## Prevention Actions
1. Action: Enforce pre-push remote-context guard (reject local-path origin by default).
   Owner: cklxx
   Due date: 2026-03-06
   Validation: `scripts/pre-push.sh` contains `guard_origin_remote`; manual check with a local-path `origin` fails as expected.
2. Action: Keep push-scope rule in workflow docs and agent rules.
   Owner: cklxx
   Due date: 2026-03-05
   Validation: Rule present in `AGENTS.md` and `docs/guides/engineering-workflow.md`.
3. Action: Add mandatory branch-health gate before merge/push (`main...origin/main`, `git status --branch`).
   Owner: cklxx
   Due date: 2026-03-06
   Validation: Included in operator checklist and verified in delivery logs.

## Follow-ups
- Open risks:
  - Multiple long-lived local branches/worktrees can still create cognitive load.
- Deferred items:
  - Optional automation to prune stale temp branches/worktrees with safety confirmation.

## Metadata
- id: inc-2026-03-05-git-worktree-push-flow-break
- tags: [git, worktree, push, ci, incident]
- links:
  - docs/error-experience/entries/2026-03-05-git-worktree-push-context-mixup.md
  - docs/error-experience/summary/entries/2026-03-05-git-worktree-push-context-mixup.md
  - docs/postmortems/checklists/incident-prevention-checklist.md
