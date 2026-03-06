# Team Orchestration（CLI-first）

## Overview

Team orchestration now uses **CLI + skill** as the primary path:

- Dispatch: `alex team run`
- Observe runtime: `alex team status`
- Send follow-up input: `alex team inject`
- Inspect terminal output: `alex team terminal`

Legacy `run_tasks/reply_agent` remains internal implementation detail and should not be used as user-facing contract.

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

## CLI Commands

### 1) Run

```bash
alex team run --template claude_research --goal "Compare A vs B"
alex team run --template list
alex team run --file /tmp/team-task.yaml
alex team run --prompt "Audit current branch and list top 3 risks"
```

### 2) Status

```bash
alex team status --json
alex team status --all --tail 50 --json
```

### 3) Inject

```bash
alex team inject --task-id analyst_a-1 --message "continue with stricter evidence"
```

### 4) Terminal View

```bash
alex team terminal --mode attach
alex team terminal --mode capture --lines 200
alex team terminal --task-id team-researcher --mode capture
```

## Notes

- Runtime artifacts are under `.elephant/tasks/_team_runtime` (or worktree runtime roots).
- Prefer `skills/team-cli` for LLM-facing instructions so prompts stay aligned with CLI contract.

