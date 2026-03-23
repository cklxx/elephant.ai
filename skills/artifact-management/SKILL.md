---
name: artifact-management
description: When you need to persist files (reports, docs, evidence) beyond the session → create/query/delete durable artifacts.
triggers:
  intent_patterns:
    - "artifact|工件|产物|报告保存|deliverable|attachment"
    - "保存.*报告|save.*report|存.*文件|persist.*file"
    - "之前.*生成的|previously.*generated|找.*产物|find.*output"
    - "导出.*结果|export.*result|下载.*文件|download.*file"
    - "删除.*工件|delete.*artifact|清理.*产物|cleanup"
    - "列出.*工件|list.*artifacts|看看.*保存了.*什么|what.*saved"
  context_signals:
    keywords: ["artifact", "工件", "产物", "报告", "deliverable", "保存", "导出", "下载", "persist", "export", "文件"]
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
