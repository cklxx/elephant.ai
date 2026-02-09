# 2026-02-09 — Eval Hard R8 + Context Routing Correction

## Goal
- Move tool-routing guardrails into context system-prompt composition path.
- Remove non-LLM basic collections from foundation suite and replace them with harder industry benchmark transfer collections.
- Run full suite and provide updated scoring report with x/x metrics.

## Checklist
- [x] Locate true context system-prompt path and apply routing guardrail section.
- [x] Add/adjust tests for context prompt section and routing guardrails.
- [x] Remove basic non-LLM collections from suite (keep them covered by unit tests).
- [x] Add harder collections with concrete names and dimensions.
- [x] Run full evaluation suite and record pass@1/pass@5 deltas.
- [x] Run lint + full tests.
- [ ] Commit incremental changes and merge back to main.

## Progress
- 2026-02-09 20:06: Created R8 worktree/branch and copied `.env`.
- 2026-02-09 20:10: Confirmed context prompt root is `internal/app/context/manager_prompt.go` and inserted `# Tool Routing Guardrails` section into `composeSystemPrompt`.
- 2026-02-09 20:15: Removed basic non-LLM collections (`tool-coverage`, `prompt-effectiveness`, `task-completion-speed`) from foundation suite.
- 2026-02-09 20:21: Added industry benchmark transfer collections:
  - `industry-benchmark-coding-workflow`
  - `industry-benchmark-web-and-computer-use`
  - `industry-benchmark-long-context-reasoning`
- 2026-02-09 20:23: Full suite result: `pass@1=291/326`, `pass@5=325/326`, failed `1`; report in `tmp/foundation-suite-r8-industry-transfer/foundation_suite_report_foundation-suite-20260209-120754.md`.
- 2026-02-09 20:29: Completed `golangci-lint` and `go test ./...` full pass.
