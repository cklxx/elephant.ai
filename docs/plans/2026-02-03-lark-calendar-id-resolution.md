# Plan: Lark calendar_id auto-resolution (primary → real calendar_id)

Date: 2026-02-03

## Context
- Calendar tools/wrapper sometimes use `"primary"` as `calendar_id`.
- Lark Calendar v4 APIs require a real `calendar_id` (e.g. `cal_xxx...`); passing `"primary"` can fail as “invalid calendar_id”, blocking schedule testing.

## Goal
- When callers pass `calendar_id="primary"` (or omit it in wrapper defaults), automatically resolve it to the current user’s actual primary calendar ID, then proceed with create/query/update/delete.

## Approach
1) Add `ResolveCalendarID(ctx, calendarID, ...)` to `internal/lark.CalendarService`:
   - If `calendarID` is not `"primary"`/empty: return as-is.
   - Otherwise: list calendars (`Calendar.List`) and pick the first `type=="primary"` calendar ID; fallback to an owned calendar, then the first available.
2) Wire resolver into:
   - `internal/lark` calendar event APIs (List/Create/Patch/Delete) and batch ops.
   - Lark builtin tools (`lark_calendar_create/query/update/delete`) so `"primary"` works end-to-end.
3) TDD:
   - Unit tests for resolver selection logic + error behavior.
   - Update existing tests that assumed `"primary"` was a valid calendar_id.

## Progress
- [x] Audit current `"primary"` usage and failure mode.
- [x] Implement `ResolveCalendarID` in `internal/lark`.
- [x] Integrate resolver into calendar tools + wrapper methods.
- [x] Add/adjust tests.
- [x] Run `make fmt && make test`; commit in small increments.

## Result
- `calendar_id="primary"` now resolves to the user’s real primary `calendar_id` via Calendar.List before calling event APIs.
- Covered by unit tests in `internal/lark` plus an integration-style tool test for `lark_calendar_create`.
