# External Agents: Codex, Claude Code, Kimi

Updated: 2026-02-24

## Scope
- Run Codex / Claude Code / Kimi as **external agents** (background delegated execution).
- Distinguish external-agent usage from core `llm_provider` selection.
- Support team orchestration with file-based run records.

## 1) Runtime architecture

Current external-agent path is bridge-based:
- Registry: `internal/infra/external/registry.go`
- Executor: `internal/infra/external/bridge/executor.go`
- Permission relay: `internal/infra/external/bridge/permission.go`
- Python bridges:
  - Claude Code: `scripts/cc_bridge/cc_bridge.py`
  - Codex/Kimi: `scripts/codex_bridge/codex_bridge.py`

Delegation entry tools:
- `bg_dispatch`, `bg_status`, `bg_collect`, `ext_reply`, `ext_merge`, `team_dispatch`
- Implemented in `internal/infra/tools/builtin/orchestration/*`

Team run file-based audit:
- `team_dispatch` writes one JSON record per team run.
- Default location: `${session_dir}/_team_runs/*.json`.

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

1. Call `bg_dispatch` with `agent_type: codex` / `claude_code` / `kimi`.
2. Poll with `bg_status`.
3. Collect output with `bg_collect`.
4. For interactive input requests, respond via `ext_reply`.
5. For team workflows, call `team_dispatch`; read run metadata (`team_run_id`, `team_run_record_path`).

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
