---
name: config-management
description: When agent config parameters need querying or modification → read/write YAML configuration files.
triggers:
  intent_patterns:
    - "config|配置|设置|setting|参数"
  context_signals:
    keywords: ["config", "配置", "设置", "setting"]
  confidence_threshold: 0.6
priority: 5
requires_tools: [bash]
max_tokens: 200
cooldown: 30
---

# config-management

查询和修改 agent YAML 配置文件。

## 调用

```bash
python3 skills/config-management/run.py get --key llm.model
python3 skills/config-management/run.py set --key llm.model --value gpt-4o
python3 skills/config-management/run.py list
```
