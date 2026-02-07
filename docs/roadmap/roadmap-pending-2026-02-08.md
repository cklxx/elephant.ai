# Roadmap: Pending Backlog (Reprioritized 2026-02-08)

## Scope
- Source of truth: `docs/roadmap/roadmap.md`.
- This file contains only unfinished work, re-ranked by contribution to the North Star:
  - Calendar + Tasks closed loop reliability
  - WTCR / TimeSaved / Accuracy
- Previous snapshot `docs/roadmap/roadmap-pending-2026-02-06.md` is retained for history only.

## Priority Tiers
- **Now (P1):** direct reliability/completion impact on the current North Star loop.
- **Next (P2):** enables higher execution leverage after P1 is closed.
- **Later (P3):** strategic expansion, not required for current loop closure.
- **Hold (P3+):** low immediate NSM impact; revisit after metric uplift.

---

## Now (P1)

### Batch A — Steward reliability closure

| Task | Current status | Core-goal value | Dependencies | Definition of Done |
|------|----------------|-----------------|--------------|--------------------|
| Steward mode activation enforcement | Not started | Ensures stateful behavior actually activates in production sessions | Existing steward persona/policy configs | Steward auto-activates for targeted channels/sessions with deterministic fallback |
| Evidence ref enforcement loop | Not started | Improves decision correctness and explainability (Accuracy) | Observe stage hook | Decisions missing references trigger corrective feedback loop and structured retry |
| State compression on overflow | Not started | Prevents state loss/degradation under long-running sessions | Steward state parser + context budget | Overflow keeps high-priority state and preserves required fields |
| Safety level approval UX | Not started | Protects write operations while preserving completion rate | Approval interface + L3/L4 policy | L3/L4 approval surface includes rollback plan and alternatives |

### Batch B — Planning + memory core closure

| Task | Current status | Core-goal value | Dependencies | Definition of Done |
|------|----------------|-----------------|--------------|--------------------|
| Replan + sub-goal decomposition | Not started | Raises task completion under failures/complexity (WTCR) | ReAct orchestration + planner boundary | Tool failure can branch into deterministic replan and continue execution |
| Memory restructuring (D5) | Not started | Preserves cross-turn quality and retrieval relevance (Accuracy/TimeSaved) | Existing memory read/write + compaction hooks | Layered FileStore migration completes without data loss and with stable retrieval |

### Batch C — Evaluation closure

| Task | Current status | Core-goal value | Dependencies | Definition of Done |
|------|----------------|-----------------|--------------|--------------------|
| Evaluation automation | In progress | Makes reliability regressions visible before release | Existing eval runner + CI gate | Quick eval run outputs normalized report artifact for comparisons |
| Evaluation set construction | In progress | Covers real North Star failure modes | Baseline/challenge task definitions | Baseline + challenge suites include steward/replan/approval scenarios with scoring rubric |

---

## Next (P2)

### Batch D — Coding gateway bootstrap

| Task | Current status | Why next (not now) | Dependencies | Definition of Done |
|------|----------------|--------------------|--------------|--------------------|
| Gateway abstraction | Not started | Execution leverage, but not a blocker for current NSM slice | None | Unified Submit/Stream/Cancel/Status contract lands with tests |
| Multi-adapter framework | Not started | Future extensibility after gateway contract stabilizes | Gateway abstraction | At least one adapter can register via common interface |
| Local CLI auto-detect | Not started | Improves operability, not core-loop blocker | Adapter layer | Runtime detects installed coding CLIs and exposes availability |
| Build/test/lint verification contract | Not started | Needed for safe automation in later milestones | Gateway + adapter execution | Verification API returns stable pass/fail + diagnostics payload |

---

## Later (P3)

### Batch E — Shadow and strategic expansion

| Theme | Current status | Note |
|-------|----------------|------|
| Shadow Agent framework + orchestration | Not started | Start only after Batch A-D is stable and measurable |
| Deep Lark ecosystem (Docs/Sheets/Wiki automation) | Not started / partial library ready | Keep queued behind core-loop metric targets |
| Platform & interaction expansion (macOS companion, replay, session sync) | Not started | Valuable but non-blocking for current North Star |
| Data processing & self-evolution bets | Not started | Hold until evaluation automation + gateway baseline are mature |

---

## Confirmed De-scoped / Not Pending

| Item | Decision |
|------|----------|
| Tool result caching | Removed (de-scoped) |
| Tool SLA profile + dynamic routing | Done |

