# Plan: Split ACP Executor Tool (2026-01-25)

## Goal
- Break `internal/tools/builtin/acp_executor.go` into smaller, focused files without behavior change.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Scope
1. Keep public tool surface (`ACPExecutorConfig`, `NewACPExecutor`, `Metadata`, `Definition`, `Execute`) in `acp_executor.go`.
2. Move RPC/session helpers into `acp_executor_client.go`.
3. Move handler/event processing into `acp_executor_handler.go`.
4. Move task package + prompt/attachment helpers into `acp_executor_prompt.go`.
5. Run `make fmt`, `make vet`, `make test`.

## Progress
- 2026-01-25: Plan created; engineering practices reviewed.
- 2026-01-25: Split ACP executor implementation into client/handler/prompt modules.
