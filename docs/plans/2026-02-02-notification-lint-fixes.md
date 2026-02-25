# Plan: Notification test + golangci-lint fixes (2026-02-02)

## Goals
- Fix the hanging notification test so `make test` completes.
- Resolve (or remove) the listed golangci-lint findings.

## Plan
1. Diagnose the notification test hang and patch the notification center logic.
2. Run package tests for notification; update/add tests if needed.
3. Re-run golangci-lint; fix remaining findings.
4. Run full test suite and restart dev stack if required.

## Progress
- [x] Diagnose and fix notification test hang.
- [x] Notification tests green.
- [x] golangci-lint green.
- [x] Full tests green.
