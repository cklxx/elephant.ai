# Plan: Remove MCP-related code

Date: 2026-03-03
Owner: Codex

## Goal
Remove MCP-related implementation from runtime code paths while keeping non-MCP features (ACP/session/task execution/server health, etc.) compiling and passing tests.

## Scope
- Remove MCP command surfaces and runtime registry integration.
- Decouple ACP JSON-RPC primitives from MCP package.
- Remove MCP-specific config flags and health probe.
- Remove MCP infrastructure package and update tests.

## Steps
- [x] Inventory all MCP dependencies and entry points across cmd/internal.
- [x] Extract shared JSON-RPC primitives to non-MCP package and switch ACP imports.
- [x] Remove CLI MCP command and MCP permission server command.
- [x] Remove DI/container MCP registry lifecycle and tool registry MCP lane.
- [x] Remove server bootstrap MCP config/probe wiring.
- [x] Remove MCP package directory and update affected tests.
- [x] Update prompts/config/tool-display strings that reference MCP browser tools.
- [x] Run focused tests for touched packages.
- [x] Run full lint/tests as required gate.
- [x] Commit incremental changes.

## Risks
- ACP currently reuses MCP JSON-RPC structs; deleting MCP first will break ACP.
- External bridge currently references `mcp-permission-server` for interactive approval relay.

## Validation
- `go test ./cmd/alex ./internal/infra/acp ./internal/app/di ./internal/app/toolregistry ./internal/delivery/server/... ./internal/shared/config ./internal/infra/external/bridge`
- `go test ./...`
