# 2026-03-03 · Leader Rubric CI Flake Stabilization

## Goal
Stabilize `TestLeaderAgent_ScoredRubric_E2E` so remote CI stops failing on transient external task failures while preserving rubric signal.

## Observed Failure
- CI run `22619412886` failed in `TestLeaderAgent_ScoredRubric_E2E`.
- `dependency_ordering` scored `0/2` with `team-researcher_codex` marked `failed`.
- `context_inheritance` scored `0/2` because no internal prompts were captured.
- Local stress runs (`-count=80`) did not reproduce, indicating transient/flaky behavior.

## Plan
1. Improve diagnostics for dependency-ordering failures to include failed task error payload.
2. Refactor scored rubric test into a reusable `runScenario` closure for deterministic reruns.
3. Add one bounded retry when stage-0 researcher tasks fail, then preserve original pass assertions.
4. Validate with targeted stress + integration package + full `go test ./...` + lint.

## Validation
- `go test ./internal/infra/integration -run TestLeaderAgent_ScoredRubric_E2E -count=20`
- `go test ./internal/infra/integration`
- `go test ./...`
- `./scripts/run-golangci-lint.sh run`

## Result
Plan completed. Test now retries once on transient stage-0 external failure and logs failed task errors for faster root-cause analysis.
