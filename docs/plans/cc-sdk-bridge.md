# Plan: Claude Code Agent SDK Bridge + 智能消息过滤裁剪

**Status**: Complete
**Created**: 2026-02-09
**Merged**: 2026-02-09

## Goal

Replace noisy stream-json stdout parsing with a Python Agent SDK sidecar that filters/trims events before forwarding to Go. Also add Codex Go-level tool filtering.

## Batches

- [x] Batch 0: Worktree setup
- [x] Batch 1: Python bridge script + JSONL protocol types
- [x] Batch 2: Go SDK Bridge Executor
- [x] Batch 3: Registry integration (no config items, auto-detect)
- [x] Batch 4: Codex Go-level tool filtering
- [x] Batch 5: Lint, test, code review, commit + merge
- [x] Integration tests: end-to-end Go→Python→Claude API verified

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

## Verification Results

- Unit tests: 11 pass (7 SDK bridge + 4 existing)
- Integration test: Go→Python→Claude API → answer "4", tokens=8, cost=$0.01
- Tool filtering test: Write forwarded (file_path only), Read suppressed
- Codex filtering: isCodexToolSuppressed for Read/Glob/Grep/WebFetch
