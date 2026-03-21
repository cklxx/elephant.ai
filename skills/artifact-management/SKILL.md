---
name: artifact-management
description: When you need to persist files (reports, docs, evidence) beyond the session → create/query/delete durable artifacts.
triggers:
  intent_patterns:
    - "artifact|工件|产物|报告保存|deliverable|attachment"
  context_signals:
    keywords: ["artifact", "工件", "产物", "报告", "deliverable"]
  confidence_threshold: 0.6
priority: 5
requires_tools: [bash]
max_tokens: 200
cooldown: 30
---

# artifact-management

管理持久化工件：创建、列表、读取、删除。

## 调用

```bash
python3 skills/artifact-management/run.py create --name report.md --content '# Report'
python3 skills/artifact-management/run.py list
python3 skills/artifact-management/run.py read --name report.md
python3 skills/artifact-management/run.py delete --name report.md
```
