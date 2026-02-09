# Plan: Claude Code Agent SDK Bridge + 智能消息过滤裁剪

**Status**: In Progress
**Created**: 2026-02-09
**Branch**: `feat/cc-sdk-bridge`

## Goal

Replace noisy stream-json stdout parsing with a Python Agent SDK sidecar that filters/trims events before forwarding to Go. Also add Codex Go-level tool filtering.

## Batches

- [x] Batch 0: Worktree setup
- [ ] Batch 1: Python bridge script + JSONL protocol types
- [ ] Batch 2: Go SDK Bridge Executor
- [ ] Batch 3: Config + Registry integration
- [ ] Batch 4: Codex Go-level tool filtering
- [ ] Batch 5: Lint, test, code review, commit

## Architecture

```
Go SDKBridgeExecutor                    Python Sidecar (cc_bridge.py)
  spawns subprocess ──stdin(config)──→    claude_agent_sdk hooks
  reads stdout JSONL  ←──stdout────←      filter → trim → emit JSONL
```

## JSONL Protocol

```jsonl
{"type":"tool","tool_name":"Write","summary":"file_path=/src/main.go","files":["/src/main.go"],"iter":3}
{"type":"result","answer":"done","tokens":5000,"cost":0.15,"iters":12,"is_error":false}
{"type":"error","message":"API rate limit exceeded"}
```

## Filter Rules

| Tool | Forward? | Trim |
|------|----------|------|
| Write/Edit | Yes | file_path only |
| Bash | Yes | command ≤120 chars |
| WebSearch | Yes | query only |
| NotebookEdit | Yes | notebook_path only |
| Read/Glob/Grep/WebFetch | No | — |
| Task*/Skill | No | — |
