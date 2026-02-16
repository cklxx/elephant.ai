---
name: sheets-report
description: 飞书电子表格的创建、查询和工作表管理。
triggers:
  intent_patterns:
    - "表格|spreadsheet|报表|数据分析|电子表格|sheet"
  context_signals:
    keywords: ["表格", "spreadsheet", "报表", "电子表格", "sheet", "数据分析"]
  confidence_threshold: 0.6
priority: 7
exclusive_group: lark-sheets
requires_tools: [channel]
max_tokens: 200
cooldown: 30
---

# sheets-report

飞书电子表格管理：创建电子表格、查询表格元信息、列出工作表。

## 调用

通过 channel tool 的 action 参数调用：

| Action | 说明 |
|--------|------|
| `create_spreadsheet` | 创建新的飞书电子表格 |
| `get_spreadsheet` | 获取电子表格元信息 |
| `list_sheets` | 列出电子表格中的所有工作表 |

## 参数

### create_spreadsheet
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| title | string | 否 | 表格标题 |
| folder_token | string | 否 | 目标文件夹 token |

### get_spreadsheet / list_sheets
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| spreadsheet_token | string | 是 | 电子表格 token |

## 示例

```
创建数据报表
-> channel(action="create_spreadsheet", title="月度数据报表")

查看表格信息
-> channel(action="get_spreadsheet", spreadsheet_token="shtcnXXX")

列出所有工作表
-> channel(action="list_sheets", spreadsheet_token="shtcnXXX")
```

## 安全等级

- `get_spreadsheet` / `list_sheets`: L1 只读，无需审批
- `create_spreadsheet`: L3 高影响，需要审批
