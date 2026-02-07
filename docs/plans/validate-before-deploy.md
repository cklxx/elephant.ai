# Plan: Restructure Lark Loop — Validate Before Deploy

**Status:** in-progress
**Branch:** feat/validate-before-deploy
**Created:** 2026-02-08

## Problem

`run_cycle()` in `loop.sh` deploys to the live test bot (line 291) before any gates run.
Users see unvalidated code. The supervisor can also auto-restart the test bot during validation.

## Solution

1. **loop.sh**: Stop test bot → reset → FAST GATE → SLOW GATE → merge → restart (validated code only)
2. **supervisor.sh**: Skip test bot restart during validation phases
3. On failure: restore test bot to last validated SHA

## Batches

### Batch 1: loop.sh
- [x] Add `LAST_VALIDATED_FILE` variable
- [x] Add `stop_test_agent()` helper
- [x] Add `restore_test_to_validated()` helper
- [x] Extend `write_loop_state()` with `last_validated_sha` and `validating_sha`
- [x] Restructure `run_cycle()`: stop→validate→deploy

### Batch 2: supervisor.sh
- [x] Add `VALIDATION_PHASES` and `is_validation_active()`
- [x] Guard `restart_with_backoff()` for test during validation
- [x] Guard `maybe_upgrade_for_sha_drift()` for test during validation
- [x] Extend observability with `last_validated_sha`

### Batch 3: Tests
- [x] Update smoke test for new status keys and validation-phase suppression
