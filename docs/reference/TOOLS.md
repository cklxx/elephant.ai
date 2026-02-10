# Built-in Tool Catalog

Updated: 2026-02-10

Source of truth: `internal/app/toolregistry/registry.go` and `internal/app/toolregistry/registry_builtins.go`.

## 1) Core registered tools

The registry currently keeps a **small core surface**. Deprecated standalone tools were consolidated into unified tools.

| Group | Tools | Notes |
| --- | --- | --- |
| Orchestration/UI | `plan`, `clarify`, `request_user` | planning + clarification + explicit user input gates |
| Memory/knowledge | `memory_search`, `memory_get`, `skills` | markdown memory recall + skill catalog |
| Web retrieval | `web_search` | may be disabled in quickstart when key is unavailable |
| Platform execution | `browser_action`, `read_file`, `write_file`, `replace_in_file`, `shell_exec`, `execute_code` | implementation depends on `toolset` |
| Lark channel | `channel` | unified Lark messaging/calendar/task operations |

## 2) Delegation tools (registered after coordinator wiring)

Once coordinator is available, registry additionally exposes:
- `subagent`
- `explore`
- `bg_dispatch`
- `bg_status`
- `bg_collect`
- `ext_reply`
- `ext_merge`

## 3) Dynamic MCP tools

MCP tools are registered at runtime and namespaced with prefix:
- `mcp__*`

## 4) `channel` actions (consolidated Lark operations)

`channel` replaces older per-feature tool names (`lark_send_message`, `lark_calendar_*`, `lark_task_manage`, etc.) with a single `action` argument.

Supported actions:
- read-only: `history`, `query_events`, `list_tasks`
- write/reversible: `send_message`, `upload_file`
- high-impact: `create_event`, `update_event`, `create_task`, `update_task`
- irreversible: `delete_event`, `delete_task`

## 5) Toolset switching

- `toolset: default`: sandbox-backed implementations for platform execution tools.
- `toolset: local` (`lark-local` alias): local browser/file/shell implementations.

## 6) Deprecated names

The following classes are intentionally not part of the default registry anymore:
- legacy file/search split tools (`grep`, `ripgrep`, `find`, `list_dir`, `search_file`, ...)
- legacy Lark split tools (`lark_send_message`, `lark_calendar_*`, `lark_task_manage`, ...)
- legacy browser split tools (`browser_info`, `browser_screenshot`, `browser_dom`)
- legacy artifact/media singleton tools (`artifacts_*`, `a2ui_emit`, `pptx_from_images`, ...)

See `internal/app/toolregistry/registry_test.go` (`TestNewRegistryRegistersOnlyCoreTools`) for enforced expectations.
