# Bitable 使用场景完整示例

## 场景 1: 创建多维表格 + 定义字段（一次性模式）

明确知道表结构时，在创建时一次性定义所有字段，减少 API 调用。

```bash
# 1. 创建 App
python3 skills/feishu-cli/run.py '{"action":"tool","module":"bitable","tool_action":"create_app","name":"客户管理"}'
# 返回: app_token, table_id (默认表)

# 2. 清除默认表的空行
python3 skills/feishu-cli/run.py '{"action":"tool","module":"bitable","tool_action":"list_records","app_token":"S404b...","table_id":"tbl..."}'
# 如果有空记录，批量删除
python3 skills/feishu-cli/run.py '{"action":"tool","module":"bitable","tool_action":"batch_delete_records","app_token":"S404b...","table_id":"tbl...","record_ids":["recXXX"]}'

# 3. 创建字段
python3 skills/feishu-cli/run.py '{"action":"tool","module":"bitable","tool_action":"create_field","app_token":"S404b...","table_id":"tbl...","field_name":"客户名称","type":1}'

python3 skills/feishu-cli/run.py '{"action":"tool","module":"bitable","tool_action":"create_field","app_token":"S404b...","table_id":"tbl...","field_name":"负责人","type":11,"property":{"multiple":false}}'

python3 skills/feishu-cli/run.py '{"action":"tool","module":"bitable","tool_action":"create_field","app_token":"S404b...","table_id":"tbl...","field_name":"状态","type":3,"property":{"options":[{"name":"潜在"},{"name":"跟进中"},{"name":"已签约"},{"name":"已流失"}]}}'

python3 skills/feishu-cli/run.py '{"action":"tool","module":"bitable","tool_action":"create_field","app_token":"S404b...","table_id":"tbl...","field_name":"签约日期","type":5,"property":{"date_formatter":"yyyy-MM-dd"}}'
```

## 场景 2: 批量导入数据

```bash
python3 skills/feishu-cli/run.py '{
  "action": "tool",
  "module": "bitable",
  "tool_action": "batch_create_records",
  "app_token": "S404b...",
  "table_id": "tbl...",
  "records": [
    {
      "fields": {
        "客户名称": "字节跳动",
        "负责人": [{"id": "ou_xxx"}],
        "状态": "跟进中",
        "签约日期": 1674206443000
      }
    },
    {
      "fields": {
        "客户名称": "飞书",
        "负责人": [{"id": "ou_yyy"}],
        "状态": "已签约",
        "签约日期": 1675416243000
      }
    }
  ]
}'
```

**注意**：最多 500 条/次。超过需分批。

## 场景 3: 筛选查询（高级筛选）

查询"状态为进行中"且"截止日期在今天之前"的记录：

```bash
python3 skills/feishu-cli/run.py '{
  "action": "tool",
  "module": "bitable",
  "tool_action": "list_records",
  "app_token": "S404b...",
  "table_id": "tbl...",
  "filter": {
    "conjunction": "and",
    "conditions": [
      {"field_name": "状态", "operator": "is", "value": ["进行中"]},
      {"field_name": "截止日期", "operator": "isLess", "value": ["Today"]}
    ]
  },
  "sort": [{"field_name": "截止日期", "desc": false}]
}'
```

### 日期筛选特殊值

| 值 | 含义 |
|----|------|
| `["Today"]` | 今天 |
| `["Tomorrow"]` | 明天 |
| `["Yesterday"]` | 昨天 |
| `["CurrentWeek"]` | 本周 |
| `["CurrentMonth"]` | 本月 |
| `["ExactDate", "1740441600000"]` | 精确日期（毫秒时间戳） |

## 场景 4: 更新记录

```bash
python3 skills/feishu-cli/run.py '{
  "action": "tool",
  "module": "bitable",
  "tool_action": "update_record",
  "app_token": "S404b...",
  "table_id": "tbl...",
  "record_id": "recXXX",
  "fields": {
    "状态": "已签约",
    "签约日期": 1740441600000
  }
}'
```

只需传要更新的字段，其他字段保持不变。

## 场景 5: 批量更新

```bash
python3 skills/feishu-cli/run.py '{
  "action": "tool",
  "module": "bitable",
  "tool_action": "batch_update_records",
  "app_token": "S404b...",
  "table_id": "tbl...",
  "records": [
    {"record_id": "recAAA", "fields": {"状态": "已签约"}},
    {"record_id": "recBBB", "fields": {"状态": "已流失"}}
  ]
}'
```

## 场景 6: 查字段结构（调试必做）

```bash
python3 skills/feishu-cli/run.py '{"action":"tool","module":"bitable","tool_action":"list_fields","app_token":"S404b...","table_id":"tbl..."}'
```

返回每个字段的 `field_id`、`field_name`、`type`、`ui_type`、`property`。

**关键用途**：
- 确认字段名的精确拼写（包括空格、大小写）
- 确认字段类型以构造正确的值格式
- 获取 field_id（部分高级操作需要）

## 场景 7: 检查空行并清理

```bash
# 列出记录
python3 skills/feishu-cli/run.py '{"action":"tool","module":"bitable","tool_action":"list_records","app_token":"S404b...","table_id":"tbl..."}'

# 找到 fields 为空的记录，提取 record_id，批量删除
python3 skills/feishu-cli/run.py '{"action":"tool","module":"bitable","tool_action":"batch_delete_records","app_token":"S404b...","table_id":"tbl...","record_ids":["recXXX","recYYY"]}'
```
