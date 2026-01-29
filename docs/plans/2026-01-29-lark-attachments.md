# Plan: Lark attachment uploads (2026-01-29)

## Goal
Enable Lark gateway to upload image/file attachments in replies (no new config).

## Plan
1. Inspect Lark gateway + attachment data flow to decide source of attachments and upload strategy.
2. Implement attachment collection + upload/send for image/file replies; keep event listener wiring; remove reaction behavior if present.
3. Add unit coverage for attachment helpers (name/type resolution and attachment collection).
4. Run `./dev.sh lint` and `./dev.sh test`; log any failures; commit changes.

## Progress
- 2026-01-29: Plan created. Inspecting Lark gateway + attachment flow.
- 2026-01-29: Implemented Lark attachment collection + image/file upload/send; wired broadcaster into gateway listener; removed reaction reply behavior.
- 2026-01-29: Added unit coverage for attachment helpers in Lark gateway tests.
