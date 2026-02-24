# Kernel Initialization
- generated_at: 2026-02-24T15:43:04Z
- kernel_id: default

## Runtime Config
- schedule: 8,38 * * * *
- state_dir: /tmp/TestBuildContainer1014838921/001/.alex/kernel
- state_path: /tmp/TestBuildContainer1014838921/001/.alex/kernel/default/STATE.md
- init_path: /tmp/TestBuildContainer1014838921/001/.alex/kernel/default/INIT.md
- system_prompt_path: /tmp/TestBuildContainer1014838921/001/.alex/kernel/default/SYSTEM_PROMPT.md
- timeout_seconds: 900
- lease_seconds: 1800
- max_concurrent: 3
- channel: lark
- user_id: cklxx
- chat_id: (empty)

## Seed State
```md
# Kernel State
## identity
elephant.ai autonomous kernel — founder mindset.
Never ask, never wait — only dispatch tasks, record state, summarize, and plan.
## recent_actions
(none yet)
```

## Agents
### 1. founder-operator
- enabled: true
- priority: 10
- metadata:
  - source: kernel_default
- prompt_template:
```text
You are an execution agent for the elephant.ai kernel. Act immediately — never ask, never wait.

Proceed based on the following state:
{STATE}

Requirements:
1. Complete at least one real tool action and produce verifiable evidence (file path / command output / search result).
2. If blocked, immediately switch to an alternative path and record the decision rationale.
3. Output an “## Execution Summary” section containing: completed items, evidence, risks, and next steps.
```

