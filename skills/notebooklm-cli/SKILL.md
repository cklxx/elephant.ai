---
name: notebooklm-cli
description: 用 `nlm`（NotebookLM CLI）完成 notebook/source/query/report 的本地操作；先 help 再执行。
triggers:
  intent_patterns:
    - "notebooklm|notebook lm|nlm|音频概览|podcast|research notebook"
  context_signals:
    keywords: ["notebooklm", "nlm", "notebook", "source", "podcast", "report"]
  confidence_threshold: 0.6
priority: 7
requires_tools: [bash]
max_tokens: 200
cooldown: 20
---

# notebooklm-cli

使用 `nlm` 直接操作 NotebookLM，优先走 CLI（`bash`）。

## 渐进式 Help（先看再做）

```bash
nlm --help
nlm notebook --help
nlm source --help
nlm report --help
nlm studio --help
```

## 最小可执行流程

```bash
nlm login
nlm notebook create "Weekly Research"
nlm source add <notebook-id> --url "https://example.com/article"
nlm notebook query <notebook-id> "总结 3 个关键结论"
nlm report create <notebook-id> --confirm
nlm studio status <notebook-id>
```

## 规则

- 删除前必须确认：`nlm notebook delete ... --confirm` / `nlm source delete ... --confirm`。
- 不使用 `nlm chat start`（交互 REPL，不适合 agent 自动执行）。
- 鉴权失败时先执行 `nlm login` 重新登录。
