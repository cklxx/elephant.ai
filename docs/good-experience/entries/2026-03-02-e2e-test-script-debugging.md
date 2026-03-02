# Good Experience: E2E Test Script Bash Gotchas

**Date**: 2026-03-02
**Area**: Testing / Bash scripting

## Summary

Debugging `scripts/test_agents_teams_e2e.sh` against a live server uncovered several
bash gotchas that collectively prevented the summary table from ever printing. All fixed
in commits `75038013` and `483b1af6`.

## Root cause chain

### 1. `((VAR++))` returns exit code 1 when VAR=0 under `set -e`
Bash arithmetic `((expr))` returns 1 when the result is 0 (falsy). With `set -euo pipefail`,
`((PASS++))` when `PASS=0` immediately exits the script.

**Fix**: Replace all `((VAR++))` with `VAR=$((VAR + 1))`.

### 2. `log` calls inside `$(inject ...)` contaminate stdout capture
`result="$(inject ...)"` captures ALL stdout from the function — including log lines — so
`result` becomes a multi-line string starting with log output, not the `OK:n:ms` token.
Case patterns never matched → always FAIL.

**Fix**: Redirect all `log` calls inside `inject` to stderr: `log "..." >&2`.

### 3. `should_run X && { run_X; cooldown; }` triggers `set -e` on skip
When `should_run X` returns 1 (test filtered), the `&&` expression returns 1. Inside a
`then` block this still triggers `set -e` → script exits after first skipped test, no
summary table printed.

**Fix**: Change all `&&` guard patterns to `if should_run X; then run_X; cooldown; fi`.

### 4. Config file priority: ~/.alex/config.yaml > ./configs/config.yaml
The server loaded `~/.alex/config.yaml` (user config, priority 2) not the project's
`configs/config.yaml` (priority 3). New templates added only to the project file were
invisible to the server.

**Fix**: Add new templates directly to `~/.alex/config.yaml`.

### 5. Timeout sizing for multi-stage pipelines
- Single-stage `claude_research`: p50≈142s, p99 needs ~360s buffer
- Multi-stage `claude_analysis` (2 parallel + synthesizer): ~387s, needs 720s
- `claude_debate` (analyst → auto-challenger → reviewer): ~270-558s, needs 720s

Initial timeouts (FAST=90→240, SLOW=240→480) were too tight; bumped to FAST=360, SLOW=720.

## Patterns confirmed
- Always redirect diagnostic output to stderr inside functions used in `$(...)` captures
- Use `if cmd; then ...; fi` instead of `cmd && { ...; }` under `set -e` for non-fatal guards
- Verify which config file the server is actually loading before debugging feature behavior
- claude_code subprocesses take 90-190s each; multi-stage pipelines stack linearly

## Metadata
```yaml
tags: [bash, testing, e2e, set-e, stdout-capture, config-priority]
links: []
```
