---
name: email-drafting
description: 快速拉齐受众、目的、语气和 CTA 的邮件写作 SOP，覆盖新建邮件与回复线程两种常见场景。
triggers:
  intent_patterns:
    - "email|邮件|写信|reply|回复邮件"
  context_signals:
    keywords: ["email", "邮件", "回复", "draft", "subject"]
  confidence_threshold: 0.6
priority: 6
requires_tools: [bash]
max_tokens: 200
cooldown: 60
---

# email-drafting

Draft or reply to work emails with structured SOP: audience alignment, purpose, tone, CTA, and subject line best practices. Supports both new emails and thread replies in Chinese/English. All drafting workflows and templates are handled by run.py.

## 调用

```bash
python3 skills/email-drafting/run.py '{"action":"draft","purpose":"request decision","audience":"team lead","tone":"formal"}'
```
