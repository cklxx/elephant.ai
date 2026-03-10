# Built-in Tool Catalog

Updated: 2026-03-11

Source of truth: `internal/app/toolregistry/registry.go` and `registry_builtins.go`.

## Core Tools

| Group | Tools | Notes |
|-------|-------|-------|
| Orchestration | `plan`, `clarify`, `request_user` | Planning + clarification + user input gates |
| Memory | `memory_search`, `memory_get`, `skills` | Markdown memory recall + skill catalog |
| Web | `web_search` | Disabled when key unavailable |
| Platform | `browser_action`, `read_file`, `write_file`, `replace_in_file`, `shell_exec`, `execute_code` | Depends on `toolset` |
| Lark | `channel` | Unified Lark messaging/calendar/task |

## Team Orchestration (CLI-first)

`alex team` is the **only user-facing entrypoint** for multi-agent team workflows:

- `alex team run` — dispatch workflow from template, file, or prompt
- `alex team status` — inspect runtime status, roles, events
- `alex team inject` — send follow-up input to a running role
- `alex team terminal` — inspect or attach to terminal output

LLMs discover this via the `team-cli` skill.

## Internal-only Orchestration Tools

The following tools are **internal implementation details** used by the agent
runtime. They must not be exposed in user documentation, skill prompts, or
product materials.

| Tool | Purpose | Internal location |
|------|---------|-------------------|
| `run_tasks` | Dispatch taskfile execution | `internal/infra/tools/builtin/orchestration/run_tasks.go` |
| `reply_agent` | Send input to a running background task | `internal/infra/tools/builtin/orchestration/reply_agent.go` |

Users interact with team workflows exclusively through `alex team ...` CLI commands.

## Channel Actions

`channel` consolidates all Lark operations via `action` argument:

- Read-only: `history`, `query_events`, `list_tasks`
- Write: `send_message`, `upload_file`
- High-impact: `create_event`, `update_event`, `create_task`, `update_task`
- Irreversible: `delete_event`, `delete_task`

Audio files (`m4a/mp3/opus/wav/aac`) sent as `msg_type="audio"`; others as `msg_type="file"`.

## Toolset Switching

- `toolset: default` — sandbox-backed platform tools.
- `toolset: local` (`lark-local` alias) — local implementations.

## Dynamic Tools

MCP tools registered at runtime with `mcp__*` prefix.
