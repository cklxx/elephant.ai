# Cmd CLI Audit

Date: 2026-03-11
Branch: `audit/cmd-cli`

## Scope

Audit `cmd/` Go CLI code for:
- CLI entrypoint error handling
- flag parsing behavior
- unused or unregistered subcommands
- redundant code in the command surface

## Plan

1. Inspect `cmd/*/main.go` and `cmd/alex` command registration paths.
2. Identify dead or unreachable command code, inconsistent flag parsing, and weak error exits.
3. Implement the minimal cleanup, add focused tests, and run relevant validation.

## Progress

- [x] Command surface mapped
- [x] Redundant CLI code removed
- [x] Validation completed
- [ ] Merged back to `main`

## Findings

- `main.go` and `CLI.Run` had diverged command tables, so several real commands (`leader`, `runtime`, `dev`, `lark`) were only reachable through the standalone pre-container path.
- `health` and `leader` parsed flags manually, which silently accepted unknown flags and unexpected positional arguments.
- Top-level help text omitted valid CLI commands that were still supported by the binary, making part of the command surface effectively undiscoverable.

## Changes

- Centralized registered command dispatch in `CLI.runRegisteredCommand`, and reused it from both `CLI.Run` and the standalone pre-container path in `main.go`.
- Added the missing containerless commands to the registered command table and updated top-level usage text.
- Replaced manual flag loops in `health` and `leader` with `flag.FlagSet` parsing so unknown flags, missing values, and extra positional args fail with CLI-style exit code `2`.
- Added focused tests for containerless dispatch and stricter health/leader flag parsing.

## Validation

- `go test ./cmd/alex`
- `go test ./cmd/alex-server ./cmd/alex-web ./cmd/eval-server`
- `go test ./cmd/alex -run 'Test(RunRegisteredCommand|RunHealthCommand|RunLeader|CLIExitBehaviorFromError|IsTopLevelHelp)'`
- `CC=/usr/bin/clang ./scripts/run-golangci-lint.sh run ./cmd/alex/... ./cmd/alex-server ./cmd/alex-web ./cmd/eval-server`
