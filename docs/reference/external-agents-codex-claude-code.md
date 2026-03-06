# External Agents: Codex, Claude Code, Kimi

Updated: 2026-02-24

## Scope
- Run Codex / Claude Code / Kimi as **external agents** (background delegated execution).
- Distinguish external-agent usage from core `llm_provider` selection.
- Support team orchestration with file-based run records.
- Prefer the CLI-first user-facing contract: `alex team run/status/inject/terminal`.

## 1) Runtime architecture

Current external-agent path is bridge-based:
- Registry: `internal/infra/external/registry.go`
- Executor: `internal/infra/external/bridge/executor.go`
- Permission relay: `internal/infra/external/bridge/permission.go`
- Python bridges:
  - Claude Code: `scripts/cc_bridge/cc_bridge.py`
  - Codex/Kimi: `scripts/codex_bridge/codex_bridge.py`

User-facing orchestration contract:
- `alex team run`
- `alex team status`
- `alex team inject`
- `alex team terminal`

Legacy `run_tasks` / `reply_agent` remain internal implementation details and should not be presented as the primary product contract.

Team run file-based audit:
- team execution writes status/runtime artifacts and team-run records.
- Default runtime location: `.elephant/tasks/_team_runtime` (or worktree runtime roots).

## 2) Config (YAML)

```yaml
runtime:
  external_agents:
    claude_code:
      enabled: true
      default_mode: "autonomous"
      default_model: "claude-3-5-sonnet"
      autonomous_allowed_tools: ["Read", "Glob", "Grep", "WebSearch", "Write", "Edit", "Bash"]
      timeout: "30m"
      env:
        ANTHROPIC_API_KEY: "${ANTHROPIC_API_KEY}"

    codex:
      enabled: true
      default_model: "gpt-5.2-codex"
      approval_policy: "never"
      sandbox: "danger-full-access"
      timeout: "30m"
      env:
        OPENAI_API_KEY: "${OPENAI_API_KEY}"

    kimi:
      enabled: true
      binary: "kimi"
      default_model: "kimi-k2-0905-preview"
      approval_policy: "never"
      sandbox: "danger-full-access"
      timeout: "30m"
      env:
        KIMI_API_KEY: "${KIMI_API_KEY}"

    teams:
      - name: "execute_review_report"
        description: "Codex executes, Kimi reviews, Claude reports"
        roles:
          - name: "executor"
            agent_type: "codex"
            prompt_template: "Implement goal directly: {GOAL}"
            execution_mode: "execute"
            autonomy_level: "full"
            workspace_mode: "worktree"
            config:
              task_kind: "coding"
              approval_policy: "never"
              sandbox: "danger-full-access"
          - name: "reviewer"
            agent_type: "kimi"
            prompt_template: "Review implementation risk and correctness: {GOAL}"
            execution_mode: "execute"
            autonomy_level: "full"
            workspace_mode: "shared"
            inherit_context: true
          - name: "reporter"
            agent_type: "claude_code"
            prompt_template: "Summarize completion and residual risks: {GOAL}"
            execution_mode: "execute"
            autonomy_level: "full"
            workspace_mode: "shared"
            inherit_context: true
        stages:
          - name: "execute"
            roles: ["executor"]
          - name: "review"
            roles: ["reviewer"]
          - name: "report"
            roles: ["reporter"]
```

## 3) Dispatch + collect flow

1. Write a YAML task file via `write_file`, then call `run_tasks(file=...)`.
2. Monitor progress by reading the `.status.yaml` sidecar file via `read_file`.
3. For interactive input requests, respond via `reply_agent`.
4. For team workflows, call `run_tasks(template=..., goal=...)`.

## 4) Per-task override keys

- `claude_code`: `mode`, `model`, `max_turns`, `max_budget_usd`, `allowed_tools`
- `codex`: `model`, `approval_policy`, `sandbox`, `plan_approval_policy`, `plan_sandbox`
- `kimi`: `model`, `approval_policy`, `sandbox`, `plan_approval_policy`, `plan_sandbox`

## 5) Core LLM provider mode (not external agents)

If you want the main agent runtime itself to use Codex/Claude as LLM provider:

```yaml
runtime:
  llm_provider: "codex"
  llm_model: "gpt-5.2-codex"
  api_key: "${CODEX_API_KEY}"
```

```yaml
runtime:
  llm_provider: "anthropic"
  llm_model: "claude-3-5-sonnet"
  api_key: "${ANTHROPIC_API_KEY}"
```

See `docs/reference/CONFIG.md` for provider precedence and `auto`/`cli` selection behavior.
