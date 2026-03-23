---
name: config-management
description: When agent config parameters need querying or modification → read/write YAML configuration files.
triggers:
  intent_patterns:
    - "config|配置|设置|setting|参数"
    - "改.*配置|change.*config|修改.*参数|update.*setting"
    - "查看.*配置|show.*config|当前.*设置|current.*setting"
    - "token.*limit|模型.*切换|switch.*model|persona.*设置"
    - "开启|enable|关闭|disable|打开.*功能|turn.*on|turn.*off"
    - "yaml.*配置|环境变量|env.*var|超时|timeout"
  context_signals:
    keywords: ["config", "配置", "设置", "setting", "参数", "yaml", "enable", "disable", "开启", "关闭", "切换", "修改"]
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
