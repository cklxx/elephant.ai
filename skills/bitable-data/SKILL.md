---
name: bitable-data
description: 飞书多维表格的表、记录、字段管理，支持数据查询和写入。
triggers:
  intent_patterns:
    - "多维表格|bitable|base|数据表|表格数据|记录"
  context_signals:
    keywords: ["多维表格", "bitable", "base", "数据表", "记录", "字段"]
  confidence_threshold: 0.6
priority: 7
exclusive_group: lark-bitable
requires_tools: [channel]
max_tokens: 200
cooldown: 30
---

# bitable-data

飞书多维表格管理：表 CRUD、记录查询与写入、字段查看。

## 调用

通过 channel tool 的 action 参数调用：

| Action | 说明 |
|--------|------|
| `list_bitable_tables` | 列出应用下的所有数据表 |
| `list_bitable_records` | 查询表中的记录 |
| `create_bitable_record` | 创建新记录 |
| `update_bitable_record` | 更新已有记录 |
| `delete_bitable_record` | 删除记录 |
| `list_bitable_fields` | 列出表的字段定义 |

## 参数

### list_bitable_tables
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| app_token | string | 是 | 多维表格应用 token |
| page_size | integer | 否 | 每页数量 |
| page_token | string | 否 | 分页 token |

### list_bitable_records
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| app_token | string | 是 | 应用 token |
| table_id | string | 是 | 表 ID |
| page_size | integer | 否 | 每页数量 |
| page_token | string | 否 | 分页 token |

### create_bitable_record
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| app_token | string | 是 | 应用 token |
| table_id | string | 是 | 表 ID |
| fields | object | 是 | 字段 key-value 映射 |

### update_bitable_record
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| app_token | string | 是 | 应用 token |
| table_id | string | 是 | 表 ID |
| record_id | string | 是 | 记录 ID |
| fields | object | 是 | 要更新的字段 |

### delete_bitable_record
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| app_token | string | 是 | 应用 token |
| table_id | string | 是 | 表 ID |
| record_id | string | 是 | 记录 ID |

### list_bitable_fields
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| app_token | string | 是 | 应用 token |
| table_id | string | 是 | 表 ID |

## 示例

```
查看表的字段定义
→ channel(action="list_bitable_fields", app_token="appXXX", table_id="tblXXX")

查询所有记录
→ channel(action="list_bitable_records", app_token="appXXX", table_id="tblXXX")

创建一条记录
→ channel(action="create_bitable_record", app_token="appXXX", table_id="tblXXX", fields={"Name":"Alice","Score":"95"})
```

## 安全等级

- `list_bitable_tables` / `list_bitable_records` / `list_bitable_fields`: L1 只读
- `create_bitable_record` / `update_bitable_record`: L3 高影响
- `delete_bitable_record`: L4 不可逆

## 链式组合

- 与 `deep-research` skill 组合：研究数据写入多维表格进行可视化
- 与 `okr-management` skill 组合：OKR 进度数据同步到多维表格看板
