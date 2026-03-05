# File-Based Orchestration

## Overview

Orchestration now uses two CLI commands:
- `alex team run`: dispatch YAML tasks or team templates
- `alex team reply`: respond to external-agent input requests or inject free-form text

In agent runs, call these via `shell_exec`. Planning is still done via `write_file` (YAML task files), status monitoring via `read_file` (`.status` sidecar files).

## Task File Format

```yaml
version: "1"
plan_id: "my-plan"
defaults:
  agent_type: codex
  execution_mode: execute
tasks:
  - id: impl
    description: "Implement feature X"
    prompt: "Write code for feature X in internal/foo/"
    file_scope: ["internal/foo/"]
  - id: test
    description: "Test feature X"
    prompt: "Write tests for feature X"
    depends_on: [impl]
    inherit_context: true
```

### Task Spec Fields

| Field | Required | Description |
|-------|----------|-------------|
| `id` | yes | Unique task identifier |
| `prompt` | yes | Instructions for the agent |
| `description` | no | Human-readable summary |
| `agent_type` | no | `codex`, `claude_code`, `kimi`, `internal` |
| `execution_mode` | no | `execute` or `plan` |
| `autonomy_level` | no | `full` or `controlled` |
| `depends_on` | no | List of task IDs that must complete first |
| `workspace_mode` | no | `shared`, `branch`, `worktree` |
| `file_scope` | no | Advisory file/dir restrictions |
| `inherit_context` | no | Inherit dependency results as context |
| `verify` | no | Enable build/test/lint verification |
| `merge_on_success` | no | Auto-merge workspace on success |
| `retry_max` | no | Max retry attempts |

### Coding Defaults

For external coding agents (`codex`, `claude_code`, `kimi`), these defaults are applied automatically:

- **execute mode**: `verify=true`, `merge_on_success=true`, `retry_max=3`, `workspace_mode=worktree`
- **plan mode**: `verify=false`, `merge_on_success=false`, `retry_max=1`, `workspace_mode=shared`
- `autonomy_level=controlled` is promoted to `full`

## `alex team run`

```
alex team run --file path/to/tasks.yaml
alex team run --file path/to/tasks.yaml --wait
alex team run --template execute_and_report --goal "..."
alex team run --template list
alex team run --file path/to/tasks.yaml --task-id impl
```

Status is written to a `.status.yaml` sidecar file. Monitor via `read_file`.

## `alex team reply`

```
alex team reply --task-id "..." --request-id "..." --approved=true
alex team reply --task-id "..." --request-id "..." --message "Use approach B"
alex team reply --task-id "..." --message "continue"
```

## Team Templates

Team templates define reusable multi-agent workflows with roles and stages. Templates are loaded from config and can be invoked via `alex team run --template <name> --goal "..."`.

## Architecture

```
Agent writes YAML (write_file)
  → shell_exec runs `alex team run ...` → validates → resolves defaults → topo sort
    → BackgroundTaskManager.Dispatch() (unchanged)
      → coordinator.ExecuteTask / bridge.Execute (unchanged)
    ← status written to .status sidecar (2s polling)
  ← Agent reads .status (read_file)
```

The `taskfile` package (`internal/domain/agent/taskfile/`) handles parsing, validation, dependency ordering, defaults resolution, and status writing. `alex team run` delegates to this execution path.
