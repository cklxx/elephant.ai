# Plan: Lark Compression Chain + User Input Preservation

## Context
- Session: `lark-3A8wbzb54gXZEBw0wW9D1SLU1K5`
- Symptom A: context compression can repeatedly recurse on prior compression summaries.
- Symptom B: latest user input can be compressed away before model reasoning.

## Tasks
- [completed] Inspect `internal/app/context/manager_compress.go` compaction boundaries.
- [completed] Keep latest turn (latest user input) out of compression candidates.
- [completed] Prevent recursive summarization of existing compression summaries.
- [completed] Align flush-hook payload with actual compacted messages.
- [completed] Add/adjust unit tests for compression behavior.
- [completed] Run targeted tests for context/react packages.
- [completed] Restructure SP history injection to JSON lines with `idx/role/summary` and cap summaries at 50 chars.
- [completed] Run injection E2E-related test suites (`internal/delivery/channels/lark`, `cmd/alex` scenario HTTP/mock).
- [completed] Inspect runtime logs for inject + budget/compression signals.
- [completed] Run mandatory code review skill and resolve P0/P1 findings (no P0/P1 found in touched files).

## Verification Notes
- `go test ./internal/app/context/... ./internal/domain/agent/react/...` ✅
- `go test ./internal/delivery/channels/lark -run 'Inject|inject' -v` ✅
- `go test ./cmd/alex -run 'TestRunLarkScenarioRun_HTTPPassWritesReports|TestRunLarkScenarioRun_PassWritesReports|TestRunLarkInjectCommandFlagParseErrorUsesExitCode2' -v` ✅
- `./dev.sh lint` ❌ (unrelated pre-existing web lint failure in `web/components/debug/DebugSurface.tsx`: conditional `useMemo` hook)

## Notes
- Prioritize safety/correctness first: no data-loss of current user turn.
