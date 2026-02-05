# Plan: Peekaboo (non-MCP) integration for Lark-local toolset

Date: 2026-02-05
Owner: cklxx
Branch: `eli/peekaboo-exec`

## Goal
- Add a builtin tool `peekaboo_exec` to run the macOS Peekaboo CLI directly (no MCP).
- Register it only for `ToolsetLarkLocal` on `darwin` so it is usable by Lark bot running on a GUI Mac, and never exposed on Linux servers.
- Default to a temporary working directory and return any images generated under that directory as embedded tool attachments.

## Scope
### In
- New tool package: `internal/tools/builtin/peekaboo`
- Tool registry wiring for `ToolsetLarkLocal` + `darwin`
- Docs update for Lark deployment requirements on macOS
- Add `peekaboo --version` to environment capability probe (helps tool selection)

### Out
- MCP server integration
- New Lark config fields
- Non-macOS support

## Interface
Tool: `peekaboo_exec`

Parameters:
- `args` (required) `[]string`: args passed to `peekaboo` (binary not included)
- `cwd` (optional) `string`: working directory; defaults to a temp dir
- `timeout_seconds` (optional) `number`: process timeout; default 120
- `max_attachments` (optional) `number`: cap returned images; default 8

Behavior:
- Execute with `exec.CommandContext` (no shell).
- Collect `stdout`, `stderr`, `exit_code` into `ToolResult.Metadata`.
- Best-effort JSON parse of `stdout` into `metadata["json"]` when `stdout` looks like JSON.
- Scan `cwd` for images (`.png/.jpg/.jpeg/.gif/.webp`), attach up to `max_attachments`, list as `[name.ext]` in content.

## Testing plan
- Unit tests for argument validation, temp dir usage, JSON parsing, and attachment scanning/limits.
- Registry test: `peekaboo_exec` is present on `darwin` and absent otherwise.

## Rollout / acceptance
1. Install Peekaboo on the Mac host running the Lark gateway:
   - `brew install steipete/tap/peekaboo`
2. Grant permissions for the host process (Terminal/launchd service):
   - Screen Recording + Accessibility
   - `peekaboo permissions check`
3. In Lark: run a task that uses `peekaboo_exec` to produce an image under the tool's temp cwd and confirm it arrives as an attachment.

## Progress log
- [ ] Docs + plan file landed
- [ ] Tool implemented + unit tests
- [ ] Registry wiring + capability probe
- [ ] `make fmt && make vet && make test` green

