# Phase 8: P3 North Star Aligned Tasks

Created: 2026-02-02
Purpose: Implement remaining P2 gap + high-impact P3 items supporting the Calendar+Tasks North Star.

---

## Status Summary

All P0, P1, and P2 items are done (C1-C44 committed, C45-C47 uncommitted by another agent).
Only remaining P2: User preference learning.
Phase 8 moves into P3, prioritizing items that directly support the North Star vertical slice.

---

## Batch 1 (parallel, no dependencies)

| # | Task | Priority | Description | Code Path |
|---|------|----------|-------------|-----------|
| C48 | User preference learning | P2 | Extract preferences (language, format, tools, style, timezone) from interaction patterns. FilePreferenceStore per user. | `internal/memory/preferences.go` |
| C49 | Meeting preparation assistant | P3 | Auto-gather calendar context, past notes, related decisions, and pending tasks before meetings. Produce structured prep doc. | `internal/lark/calendar/meeting_prep.go` |
| C50 | Calendar suggestions | P3 | Suggest meeting times based on free slots, conflict detection, and historical patterns (preferred times, durations). | `internal/lark/calendar/suggestions.go` |
| C51 | User-defined custom skills | P3 | Load user-authored skill files from configurable directory. Validate, register, and serve alongside built-in skills. | `internal/skills/custom.go` |
| C52 | Unified notification center | P3 | Route notifications to user's preferred channel (Lark, webhook, CLI). Channel registry, priority routing, delivery tracking. | `internal/notification/` |

---

## Execution Status

| Task | Status | Commit |
|------|--------|--------|
| C48 | | |
| C49 | | |
| C50 | | |
| C51 | | |
| C52 | | |

---
