# 2026-03-03 Non-web Simplify Rescan — Wave H

## Scope
Non-`web/` global simplify pass on fresh worktree `codex/nonweb-rescan-opt5`.

## Goals
1. Remove repeated Lark tool error-result boilerplate via local helpers.
2. Normalize obvious `strings.ToLower(strings.TrimSpace(...))` and blank checks to shared string utilities.
3. Keep behavior unchanged and validate with targeted tests + lint.

## Plan
1. Add `larktools` error helper(s) and replace duplicate `chat_id` missing / tool-error return sites.
2. Apply low-risk utility replacements from rescan shortlist:
   - `utils.TrimLower(...)`
   - `utils.IsBlank(...)`
   - `utils.HasContent(...)`
3. Run targeted `go test` for touched packages.
4. Run targeted `golangci-lint` for touched packages.
5. Commit Wave H and continue next optimization batch.

## Progress
- [x] Parallel subagent rescans completed (non-web only).
- [x] Larktools duplicate error paths simplified.
- [x] String utility replacements applied.
- [x] Targeted tests and lint passed.
- [ ] Wave H commit created.
