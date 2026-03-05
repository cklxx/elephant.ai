# Built-in Tool Catalog

Updated: 2026-03-05

Source of truth: `internal/app/toolregistry/registry.go` and `internal/app/toolregistry/registry_builtins.go`.

## 1) Core registered tools

The registry currently keeps a **small core surface**. Deprecated standalone tools were consolidated into unified tools.

| Group | Tools | Notes |
| --- | --- | --- |
| UI | `ask_user`, `context_checkpoint` | clarification/request gates + context pruning |
| Skill discovery | `skills` | skill catalog lookup (`list/show/search`) |
| Web retrieval | `web_search` | may be disabled in quickstart when key is unavailable |
| Platform execution | `read_file`, `write_file`, `replace_in_file`, `shell_exec` | implementation depends on `toolset` |
| Lark channel | `channel` | unified Lark messaging/calendar/task/doc/wiki/drive/... actions |

## 2) CLI orchestration (replaces tool-level orchestration)

`run_tasks` and `reply_agent` are no longer registered as built-in tools.
Use CLI orchestration commands (typically via `shell_exec`):
- `alex team run` — dispatch YAML tasks / team templates (supports wait/timeout/mode/task filter/prompt overrides)
- `alex team reply` — respond to external input requests or inject free-form input into a running task

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

Notes:
- `upload_file` sends audio files (`m4a/mp3/opus/wav/aac`) as `msg_type="audio"`; other files use `msg_type="file"`.

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
