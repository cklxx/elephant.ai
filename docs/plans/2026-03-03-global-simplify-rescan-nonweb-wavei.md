# 2026-03-03 Non-web Simplify Rescan — Wave I

## Scope
Continue non-`web/` simplify on branch `codex/nonweb-rescan-opt5`.

## Goals
1. Replace remaining low-risk `strings.ToLower(strings.TrimSpace(...))` with `utils.TrimLower(...)`.
2. Replace low-risk blank checks with `utils.IsBlank(...)` / `utils.HasContent(...)`.
3. Dedupe repetitive `ports.ToolResult` error construction in `internal/app/toolregistry`.

## Plan
1. Apply utility normalization to selected production files (no behavior change).
2. Add local helper(s) in `toolregistry` and replace duplicated error result constructors.
3. Run targeted tests and lint for touched packages.
4. Run code review and commit Wave I.

## Progress
- [x] Utility normalization applied.
- [x] Toolregistry error-result dedupe applied.
- [x] Tests/lint/code-review passed.
- [ ] Wave I commit created.
