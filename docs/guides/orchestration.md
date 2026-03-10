# Team Orchestration (CLI-first)

CLI commands for multi-agent team workflows:

```bash
alex team run       # dispatch workflow
alex team status    # observe runtime
alex team inject    # send follow-up input
alex team terminal  # inspect terminal output
```

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

## CLI Examples

```bash
# Run from template, file, or prompt
alex team run --template claude_research --goal "Compare A vs B"
alex team run --template list
alex team run --file /tmp/team-task.yaml
alex team run --prompt "Audit current branch and list top 3 risks"

# Status
alex team status --json
alex team status --all --tail 50 --json

# Inject message to running task
alex team inject --task-id analyst_a-1 --message "continue with stricter evidence"

# Terminal view
alex team terminal --mode attach
alex team terminal --mode capture --lines 200
alex team terminal --task-id team-researcher --mode capture
```

## Notes

- Artifacts stored under `.elephant/tasks/_team_runtime`.
- Use `skills/team-cli` for LLM-facing prompts to stay aligned with CLI contract.
- Legacy `run_tasks/reply_agent` are internal implementation details.
