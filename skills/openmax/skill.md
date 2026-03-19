---
name: openmax
description: 任务量大到一个 worker 搞不定，需要拆成多个子任务并行跑（全仓 review、批量分析、多模块重构）时使用。
triggers:
  intent_patterns:
    - "openmax|parallel review|并行|多worker|多任务|code review|批量"
  context_signals:
    keywords: ["openmax", "parallel", "review", "并行", "worktree"]
  confidence_threshold: 0.7
priority: 9
requires_tools: [bash]
max_tokens: 300
cooldown: 30
capabilities: ["code_edit", "code_review", "research", "analysis", "parallel"]
activation_mode: explicit
output:
  format: markdown
  artifacts: true
  artifact_type: document
---

# openMax — 多 Worker 并行编排

在隔离 git worktree 中启动 N 个 CC worker，并行执行任务，汇总 `.openmax/reports/` 下的产出。

## 快速前置

- 需要 `claude` CLI 在 PATH 中
- 工作目录必须是 git 仓库根目录
- briefs 存放于 `.openmax/briefs/<task>.md`

## 命令

```bash
# 1) 基于已有 briefs 并行启动所有 worker
python3 skills/openmax/run.py dispatch --tasks "research-codebase,review-perf,review-arch"

# 2) 指定 goal，自动生成 briefs 并启动
python3 skills/openmax/run.py dispatch --tasks "review-lark,review-eval" --goal "Review the elephant.ai codebase"

# 3) 查看 worker 状态（检查进程 + 报告）
python3 skills/openmax/run.py status

# 4) 汇总已完成的报告
python3 skills/openmax/run.py collect
```

## 参数说明

| 参数 | 说明 |
|------|------|
| `--tasks` | 逗号分隔的任务名列表（对应 `.openmax/briefs/<name>.md`）|
| `--goal` | 可选：补充到每个 brief 的总目标说明 |
| `--brief-dir` | brief 所在目录，默认 `.openmax/briefs` |
| `--report-dir` | 报告输出目录，默认 `.openmax/reports` |
| `--worktree-base` | worktree 根目录，默认 `.openmax-worktrees` |
| `--dry-run` | 只输出计划，不执行 |

## 工作流

1. 读取 `.openmax/briefs/<task>.md`
2. 创建 worktree：`git worktree add -b openmax/<task> .openmax-worktrees/openmax_<task> main`
3. 注入任务上下文到 CLAUDE.md
4. 后台启动：`claude --dangerously-skip-permissions --print <brief>`
5. 所有 worker 完成后，`collect` 汇总 `.openmax/reports/*.md`
