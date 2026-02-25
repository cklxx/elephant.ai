# Plan: Lark background task completion notifications include task_id/status/merge outcome

## Goal
Ensure Lark background task completion notifications explicitly include `task_id`, `status`, and merge outcome (merged/success, merge failed, or not merged), and that a completion message is always sent when tasks finish, even if merge is skipped or fails.

## Steps
1. Inspect Lark background task completion notification path and related merge handling to identify current message structure and missing cases.
2. Update completion notification payload to include `task_id`, `status`, and merge outcome, and guarantee delivery on all finish paths (success, merge skipped, merge failed).
3. Add/adjust tests for completion notifications and merge outcomes.
4. Run full lint + tests.

## Status
- [x] Step 1: Inspect current notification path
- [x] Step 2: Implement notification updates
- [x] Step 3: Update tests
- [x] Step 4: Run lint + tests (lint ok; go test ./... failed: config key mismatch + path default)
