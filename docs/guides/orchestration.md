# File-Based Orchestration

## Overview

Orchestration uses two tools: `run_tasks` dispatches tasks, `reply_agent` responds to agent input requests. Planning is done via `write_file` (YAML task files), status monitoring via `read_file` (`.status` sidecar files).

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

## run_tasks Tool

```
run_tasks(file="path/to/tasks.yaml")              # async dispatch
run_tasks(file="path/to/tasks.yaml", wait=true)    # sync wait
run_tasks(template="execute_and_report", goal="…") # team template
run_tasks(template="list")                          # list templates
run_tasks(file="…", task_ids=["impl"])              # filter tasks
```

Status is written to a `.status.yaml` sidecar file. Monitor via `read_file`.

## reply_agent Tool

```
reply_agent(task_id="…", request_id="…", approved=true)
reply_agent(task_id="…", request_id="…", message="Use approach B")
```

## Team Templates

Team templates define reusable multi-agent workflows with roles and stages. Templates are loaded from config and can be invoked via `run_tasks(template="name", goal="…")`.

## Architecture

```
Agent writes YAML (write_file)
  → run_tasks reads YAML → validates → resolves defaults → topo sort
    → BackgroundTaskManager.Dispatch() (unchanged)
      → coordinator.ExecuteTask / bridge.Execute (unchanged)
    ← status written to .status sidecar (2s polling)
  ← Agent reads .status (read_file)
```

The `taskfile` package (`internal/domain/agent/taskfile/`) handles parsing, validation, dependency ordering, defaults resolution, and status writing. `run_tasks` is a thin tool that reads the file and delegates to the taskfile executor.
