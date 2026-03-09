# Roadmap: Pending Backlog (Reprioritized 2026-03-09)

## Scope
- Source of truth: `docs/roadmap/roadmap.md`.
- This file contains only unfinished work, re-ranked by contribution to the North Star:
  - Calendar + Tasks closed loop reliability
  - WTCR / TimeSaved / Accuracy
- Previous snapshot `docs/roadmap/roadmap-pending-2026-02-06.md` is retained for history only.

## Priority Tiers
- **Immediate (P0):** highest-priority execution leverage required for current exploration speed.
- **Now (P1):** direct reliability/completion impact on the current North Star loop.
- **Next (P2):** enables higher execution leverage after P1 is closed.
- **Later (P3):** strategic expansion, not required for current loop closure.
- **Hold (P3+):** low immediate NSM impact; revisit after metric uplift.

---

## Immediate (P0)

### Batch 0 — CLI Runtime + Kaku foundation

| Task | Current status | Core-goal value | Dependencies | Definition of Done |
|------|----------------|-----------------|--------------|--------------------|
| Runtime skeleton | Not started | Establishes one stable multi-session runtime/panel | None | Runtime supports create/start/stop/resume/cancel/list/status with persisted tape/state |
| Member adapter contract | Not started | Lets Codex / Claude Code / Kimi / AnyGen / Colab share one runtime model | Runtime skeleton | Common `MemberAdapter` contract lands without upper-layer branching |
| First CLI member adapters | Not started | Proves the shared runtime on real members | Runtime skeleton + adapter contract | At least one coding member runs end to end through the shared runtime |
| Local member detection | Not started | Reduces manual wiring and exposes availability for scheduling | Adapter layer | Runtime detects installed local members and surfaces availability |

---

## Now (P1)

### Batch A — Runtime operability + memory/eval closure

| Task | Current status | Core-goal value | Dependencies | Definition of Done |
|------|----------------|-----------------|--------------|--------------------|
| Session snapshot / context introspection | Not started | Makes runtime/session state legible instead of TODO-gap placeholders | Existing snapshot surface | diff/world/feedback become visible in the snapshot surface |
| Memory restructuring (D5) | Not started | Preserves cross-turn quality and retrieval relevance (Accuracy/TimeSaved) | Existing memory read/write + compaction hooks | Layered FileStore migration completes without data loss and with stable retrieval |
| Evaluation automation | In progress | Makes reliability regressions visible before release | Existing eval runner + CI gate | Quick eval run outputs normalized report artifact for comparisons |
| Evaluation set construction | In progress | Covers real North Star failure modes | Baseline/challenge task definitions | Baseline + challenge suites cover runtime/session/memory/scheduler failure modes with scoring rubric |

---

## Next (P2)

### Batch B — Hooks + scheduler closure

| Task | Current status | Why next (not now) | Dependencies | Definition of Done |
|------|----------------|--------------------|--------------|--------------------|
| Hooks + runtime scheduler | Not started | Needed once the runtime/member base is stable | Runtime skeleton + member adapters | started/heartbeat/needs_input/completed/failed/stalled events drive continuation |
| Proactive scheduler / reminder defaults | Not started | Turns dark proactive subsystems into usable product behavior | Scheduler + config | Scheduler/reminders/heartbeat ship with clear operational defaults |
| Kernel outreach executor enablement | Not started | Removes planner/runtime mismatch in kernel dispatch buckets | Kernel config + planner | Outreach bucket is either enabled or removed from the planning prompt |

---

## Later (P3)

### Batch C — Panel / UX / Team recipe

| Theme | Current status | Note |
|-------|----------------|------|
| Kaku panel / multi-session UX | Not started | Start after Batch 0-B is stable and measurable |
| Team recipe simplification | Not started | Keep runtime as substrate and team as templates, not a competing runtime |
| Shadow Agent framework + orchestration | Not started | Start only after Batch 0-C is stable and measurable |
| Deep Lark ecosystem (Docs/Sheets/Wiki automation) | Not started | Keep queued behind core-loop metric targets |
| Platform & interaction expansion (macOS companion, replay, session sync) | Not started | Valuable but non-blocking for current North Star |
| Data processing & self-evolution bets | Not started | Hold until evaluation automation + runtime baseline are mature |

---

## Confirmed De-scoped / Deleted From Active Roadmap

| Item | Decision |
|------|----------|
| Steward reliability closure batch | Deleted from active roadmap (2026-03-09) |
| Replan + sub-goal decomposition | Deleted from active roadmap (2026-03-09) |
| A2UI preview/render candidate | Explicitly not adopted into roadmap (2026-03-09) |
| Meeting preparation assistant wiring | Deleted from active roadmap (2026-03-09) |
| Calendar suggestions wiring | Deleted from active roadmap (2026-03-09) |
| Tool result caching | Removed (de-scoped) |
| Tool SLA profile + dynamic routing | Done |
