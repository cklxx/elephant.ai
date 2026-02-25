# Plan: Kernel/Lark Prompt & Notice Reliability (2026-02-25)

## Context
- User observed oversized first system prompt in Lark (`~83.5k` cumulative) and suspected kernel context leakage.
- Kernel appears not replying in subscription group.
- Need root-cause timeline, fix, postmortem records, and process hardening in `AGENTS.md` / `CLAUDE.md`.

## Objectives
1. Eliminate oversized first-turn prompt inflation from bootstrap injection path.
2. Restore deterministic `/notice` persistence path so kernel cycle notifications can be resolved after restart.
3. Clarify session reuse behavior (`lark-*` vs `kernel-*`) and prevent misdiagnosis.
4. Record incident + prevention actions in postmortem/error/good docs.
5. Restart and verify runtime behavior.

## Implementation Steps
1. Context prompt fix
   - Add bootstrap payload budgeting safeguards (avoid unbounded per-file accumulation).
   - Avoid redundant SOUL/USER raw bootstrap injection where memory snapshot already covers identity.
   - Add/adjust context tests.
2. Notice state reliability fix
   - Change notice state default path resolution to stable runtime-config sibling path.
   - Keep env override `LARK_NOTICE_STATE_FILE` as highest priority.
   - Update tests.
3. Session observability
   - Add clear comments/log hints for Lark session reuse semantics.
4. Incident records
   - New postmortem incident file with absolute-date timeline, root causes, prevention checklist linkage.
   - Add matching error/good entries and summary entries.
   - Update AGENTS/CLAUDE with concrete anti-regression bullets.
5. Validation
   - Run focused Go tests for modified packages.
   - Restart backend and check status/log evidence.

## Verification Commands
- `go test ./internal/app/context ./internal/delivery/channels/lark -count=1`
- `./alex dev restart backend`
- `./alex dev status`
- `tail -n 120 logs/lark-main.log logs/lark-kernel.log`

## Progress
- [x] Plan created
- [ ] Code fixes complete
- [ ] Tests passing
- [ ] Docs/postmortem updates complete
- [ ] Restart + runtime verification complete
