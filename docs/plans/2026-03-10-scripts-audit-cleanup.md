Status: completed

# Scripts Audit Cleanup

## Goal

Audit `scripts/` for stale or unused scripts, remove dead entries, and simplify remaining scripts where the current logic is unnecessarily complex.

## Plan

1. Inventory every script under `scripts/` and search for repo references to determine whether it is still used.
2. Delete scripts with no credible call sites and simplify live scripts where the cleanup is low-risk and local.
3. Run proportional validation and review, then commit and merge back to `main` without pushing.

## Outcome

- Deleted stale one-off or obsolete scripts:
  - `scripts/create-release.sh`
  - `scripts/debug-tool-calls.sh`
  - `scripts/diagnose-visualizer-hooks.sh`
  - `scripts/setup-github-pages.sh`
  - `scripts/test-visualizer.sh`
  - `scripts/validate-deployment.sh`
- Kept active scripts with real entry points such as `pre-push.sh`, `test.sh`, `go-with-toolchain.sh`, `run-golangci-lint.sh`, bridge launchers, Kaku helpers, and CLI wrappers.
- Simplified `scripts/test.sh` by routing Go invocations through the existing toolchain wrapper instead of repeating direct `go` calls.
- Simplified `scripts/eval-quick.sh` to use the same toolchain wrapper for consistent execution.
