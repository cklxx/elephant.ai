## Goal

Audit `evaluation/` Go code and land only safe cleanups in three buckets:

- unused exported functions that can be removed or unexported
- redundant error wrapping that does not add useful context
- `if` / `else` chains that can be simplified without changing behavior

## Scope

- Only touch `evaluation/**`
- Keep external behavior and file formats unchanged
- Prefer narrow refactors backed by existing or added tests

## Approach

1. Scan exported top-level functions and verify whether they have non-test callers.
2. Scan for `fmt.Errorf(...: %w, err)` wrappers that only restate the package/function boundary.
3. Scan for nested or repetitive `if` / `else` chains that can collapse into early returns or simpler switches.
4. Apply the smallest safe batch and validate targeted packages.

## Validation

- `go test ./evaluation/...`
- relevant lint on touched packages
- `python3 skills/code-review/run.py review`
