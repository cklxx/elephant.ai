# Plan: Lark/CLI Await-Question Option Selection

Date: 2026-02-07
Branch: feat/lark-cli-choice-20260207

## Goal
When the agent asks for user input, support selectable options instead of only free-text:
- Lark: show interactive card option buttons.
- CLI (native line mode): show up/down selectable list and submit with Enter.

## Scope
1. Add structured await prompt extraction (`question` + `options`).
2. Extend UI tools (`clarify`, `request_user`) to accept optional `options` array.
3. Lark delivery: render option cards for await prompts and wire callback to injected input.
4. CLI line chat: detect await prompt options and present terminal selector.
5. Add tests for parser, tools, Lark listener/callback, and CLI loop behavior.

## Work Breakdown
- [x] A. Domain prompt extraction + tool option schema/metadata
- [x] B. Lark option card rendering for clarify/await flow
- [x] C. CLI option selector (up/down + Enter) and loop wiring
- [x] D. Lint/tests and final verification

## Risks
- Lark card callback should keep backward compatibility with existing action tags.
- Terminal raw mode selector must restore terminal state on all exits.
- Preserve old behavior when options are absent.

## Acceptance
- Await prompts with options produce selectable UI in Lark and CLI.
- Existing text-only await prompts still work.
- All updated tests pass; full lint/test executed before delivery.

## Validation Log
- `go test ./internal/domain/agent/ports/agent ./internal/infra/tools/builtin/ui` ✅
- `go test ./internal/infra/lark/cards ./internal/delivery/channels/lark` ✅
- `go test ./cmd/alex` ✅
- `./dev.sh test` ✅
- `./dev.sh lint` ⚠️ blocked by pre-existing unrelated lint issue:
  - `internal/domain/agent/react/steward_state_parser.go:16:2: const maxStewardStateBytes is unused`
