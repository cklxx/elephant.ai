---
name: self-test
description: When Lark integration tests need running → execute test suite, analyze failures, auto-iterate fixes by severity.
triggers:
  intent_patterns:
    - "self.?test|自测|场景测试|scenario.?test|run.?test|测试套件"
    - "跑.*测试|run.*tests|执行.*测试|execute.*tests"
    - "飞书.*测试|lark.*test|集成.*测试|integration.*test"
    - "测试.*失败.*修|fix.*failing|修复.*测试|repair.*test"
    - "全量.*测试|full.*suite|冒烟.*测试|smoke.*test"
  context_signals:
    keywords: ["test", "scenario", "lark", "测试", "自测", "集成", "冒烟", "suite", "失败", "修复"]
  confidence_threshold: 0.7
priority: 7
requires_tools: [bash]
max_tokens: 200
cooldown: 120
---

# self-test

Run the Lark scenario test suite, analyze failures by root cause (test_drift, prompt_issue, tool_bug, etc.), apply tiered auto-fixes (up to 3 rounds), and generate a structured report. All test execution, failure analysis, and iterative repair logic are handled by run.py.

## 调用

```bash
python3 skills/self-test/run.py run
```
