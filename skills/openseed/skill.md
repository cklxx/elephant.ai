---
name: openseed
description: 单任务 openMax 种子 — 写入 brief、创建 worktree、启动单个 CC worker。
triggers:
  intent_patterns:
    - "openseed|seed.*task|单任务|单worker|create.*worktree"
  context_signals:
    keywords: ["openseed", "seed", "brief", "worktree"]
  confidence_threshold: 0.7
priority: 9
requires_tools: [bash]
max_tokens: 200
cooldown: 30
output:
  format: markdown
  artifacts: false
---

# openSeed — 单任务种子

创建一个隔离 worktree 并启动单个 CC worker 执行任务。

## 快速前置

- 需要 `claude` CLI 在 PATH 中
- 工作目录必须是 git 仓库根目录

## 命令

```bash
# 用 brief 文件启动
python3 skills/openseed/run.py seed --task fix-race-condition --brief-file .openmax/briefs/fix-race.md

# 用内联 brief 启动
python3 skills/openseed/run.py seed --task "review-auth" --brief "Review internal/app/auth/ for security issues. Write findings to .openmax/reports/review-auth.md"

# Dry run（只输出计划）
python3 skills/openseed/run.py seed --task "review-auth" --brief "..." --dry-run
```

## 参数

| 参数 | 说明 |
|------|------|
| `--task` | 任务名（字母数字+连字符，会成为 branch/worktree 名的一部分）|
| `--brief` | brief 内容（内联字符串）|
| `--brief-file` | brief 文件路径（与 `--brief` 二选一）|
| `--base-branch` | 基准分支，默认 `main` |
| `--dry-run` | 只输出计划，不创建 worktree 或启动 worker |

## 流程

1. 写入 `.openmax/briefs/<task>.md`
2. `git worktree add -b openmax/<task> .openmax-worktrees/openmax_<task> <base>`
3. 向 worktree 的 CLAUDE.md 追加任务报告模板
4. 后台执行 `claude --dangerously-skip-permissions --print <brief>`
