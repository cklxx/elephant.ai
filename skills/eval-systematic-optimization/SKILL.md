---
name: eval-systematic-optimization
description: Run foundation-suite baseline, cluster failures, and optimize pass@1 systematically.
triggers:
  intent_patterns:
    - "评测.*优化|pass@1|pass@5|失败case|failure case|系统性评测|benchmark.*optimi"
  context_signals:
    keywords: ["foundation-suite", "pass@1", "pass@5", "x/x", "goodcase", "badcase", "冲突"]
  confidence_threshold: 0.7
priority: 8
requires_tools: [bash]
max_tokens: 200
cooldown: 90
---

# eval-systematic-optimization

Run baseline evaluation and failure clustering for foundation-suite.

## Requirements
- Go toolchain available (`go` in PATH).
- Repo root as working directory (or pass `cwd`).

## Constraints
- Baseline command timeout: 600s.
- Default baseline output path: `/tmp/foundation-suite-<tag>-baseline`.
- `analyze` requires a valid JSON result file path.
- Focus is conflict-family optimization, not single-case overfitting.

## Usage

```bash
# Run baseline
python3 skills/eval-systematic-optimization/run.py '{"action":"baseline","tag":"r12"}'

# Analyze failures
python3 skills/eval-systematic-optimization/run.py '{"action":"analyze","result_file":"/tmp/foundation-suite-r12-baseline/foundation_suite_cases.json"}'
```
