# 2026-02-11 — Onboarding State Parse Recovery for `alex model use`

## Goal
Fix `alex model use` failure when onboarding state file contains trailing malformed JSON (error like `invalid character '}' after top-level value`) so model selection can proceed and state auto-heals.

## References
- Local: `docs/guides/engineering-practices.md`
- Global best practices:
  - Go error handling and data parsing resilience (`go.dev/wiki/CodeReviewComments`, Effective Go)
  - Robust file-format evolution and forward compatibility patterns (Kubernetes API machinery conventions and OSS config reader patterns)

## Plan
1. Add regression tests for malformed onboarding state parse behavior. (completed)
2. Implement tolerant onboarding-state decode path that accepts first valid JSON document and ignores trailing garbage. (completed)
3. Ensure `Set` path rewrites file into canonical JSON, effectively self-healing malformed files. (completed)
4. Run targeted + package tests, then full lint/test checks as required. (completed)
5. Perform mandatory code review workflow and summarize P0–P3 findings before commit. (completed)

## Progress Log
- 2026-02-11 22:39 +0800: Isolated fix in worktree branch `fix/onboarding-state-parse-20260211`; loaded engineering/memory context; identified strict `jsonx.Unmarshal` in `internal/app/subscription/onboarding_state_store.go` as direct failure point.
- 2026-02-11 22:40 +0800: Added regression tests for malformed onboarding files with trailing `}` and verified they fail on current strict parser (`parse onboarding state: invalid character '}' after top-level value`).
- 2026-02-11 22:41 +0800: Replaced strict unmarshal with first-document decode (`jsonx.NewDecoder`) to tolerate trailing garbage while preserving version validation and normalization.
- 2026-02-11 22:41 +0800: Verified targeted tests pass:
  - `go test ./internal/app/subscription -run "TestOnboardingStateStore(GetIgnoresTrailingGarbage|SetRepairsTrailingGarbage|SetGetClear|RejectsUnknownVersion)" -count=1`
  - `go test ./cmd/alex -run "Test(UseModelPersistsSelectionWithoutYAML|ExecuteSetupCommandWithYAML|ExecuteSetupCommandWithExplicitModel)" -count=1`
- 2026-02-11 22:43 +0800: Ran required checks:
  - `make fmt` ✅
  - `make vet` ✅
  - `make check-arch` ✅
  - `make test` ⚠️ failed due unrelated pre-existing environment-sensitive test `TestUseModelPickerWithSingleProviderSingleModel` in `cmd/alex` (multiple providers detected).
- 2026-02-11 22:44 +0800: Completed mandatory code-review workflow (scope: 2 files, 94 insertions, 1 deletion) with references loaded from `skills/code-review/references/*`; findings: no P0/P1/P2/P3 issues for this diff.
