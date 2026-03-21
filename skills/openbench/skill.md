---
name: openbench
description: When prompt, tool, or agent logic changes need regression testing → run eval benchmarks to detect regressions or improvements.
triggers:
  intent_patterns:
    - "openbench|eval|benchmark|基准|评估|测试套件|跑个评测"
  context_signals:
    keywords: ["openbench", "eval", "benchmark", "evaluation", "评估"]
  confidence_threshold: 0.7
priority: 8
requires_tools: [bash]
max_tokens: 200
cooldown: 60
capabilities: ["evaluation", "benchmark"]
activation_mode: explicit
output:
  format: markdown
  artifacts: true
  artifact_type: document
---

# openBench — Eval 基准测试

运行 `evaluation/agent_eval` 评估套件，输出 pass rate、延迟、质量指标。

## 快速前置

- 需在 elephant.ai repo 根目录运行
- 需要 Go toolchain（`go` in PATH）

## 命令

```bash
# 快速评估（默认 10 cases）
python3 skills/openbench/run.py run

# 指定套件和超时
python3 skills/openbench/run.py run --suite foundation --timeout 600

# 列出可用套件/数据集
python3 skills/openbench/run.py list

# 查看最近一次评估结果
python3 skills/openbench/run.py last
```

## 参数

| 参数 | 说明 |
|------|------|
| `--suite` | 评估套件名，默认 `quick`（对应 `scripts/eval-quick.sh`）|
| `--timeout` | 每个 case 超时秒数，默认 300 |
| `--limit` | 最多运行 N 个 case，默认 10 |
| `--output-dir` | 结果输出目录，默认 `.openmax/bench` |

## 套件说明

| 套件 | 命令 | 说明 |
|------|------|------|
| `quick` | `scripts/eval-quick.sh` | 快速回归，~10 cases |
| `full` | `alex dev test ./evaluation/...` | 完整评估，包含 agent_eval |
