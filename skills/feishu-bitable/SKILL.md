---
name: feishu-bitable
description: |
  飞书多维表格（Bitable）的创建、查询、编辑和管理工具。包含 27 种字段类型支持、高级筛选、批量操作和视图管理。

  **当以下情况时使用此 Skill**：
  (1) 需要创建或管理飞书多维表格 App
  (2) 需要在多维表格中新增、查询、修改、删除记录（行数据）
  (3) 需要管理字段（列）、数据表
  (4) 用户提到"多维表格"、"bitable"、"数据表"、"记录"、"字段"
  (5) 需要批量导入数据或批量更新多维表格
triggers:
  intent_patterns:
    - "多维表格|bitable|数据表|表格记录"
    - "新增记录|批量导入|表格字段|筛选查询"
  context_signals:
    keywords: ["bitable", "多维表格", "app_token", "table_id", "record"]
  confidence_threshold: 0.5
priority: 9
requires_tools: [bash]
max_tokens: 300
cooldown: 15
---

# Feishu Bitable (多维表格) Skill

## 执行前必读

- **写记录前**：先调用 `field list` 获取字段 type/ui_type，否则格式极易出错
- **默认表的空行坑**：`app create` 自带的默认表中会有空记录！插入数据前先 `record list` + `record batch_delete` 清空
- **人员字段**：默认 open_id（ou_...），值必须是 `[{id:"ou_xxx"}]`（数组对象）
- **日期字段**：毫秒时间戳（例如 `1674206443000`），不是秒
- **单选字段**：字符串（例如 `"选项1"`），不是数组
- **多选字段**：字符串数组（例如 `["选项1", "选项2"]`）
- **附件字段**：必须先上传到当前多维表格，使用返回的 file_token
- **批量上限**：单次 ≤ 500 条，超过需分批
- **并发限制**：同一数据表不支持并发写，需串行调用

---

## 快速索引：意图 → 工具调用

> ⚠️ **CLI 支持的动作**（标注 ✅）可直接用 `python3 skills/feishu-cli/run.py` 调用；未标注的需改用 `api` 原始调用。

| 用户意图 | module | tool_action | 必填参数 | 常用可选 |
|---------|--------|-------------|---------|---------|
| ✅ 查表有哪些字段 | bitable | list_fields | app_token, table_id | - |
| ✅ 查记录 | bitable | list_records | app_token, table_id | filter, sort, field_names |
| ✅ 新增一行 | bitable | create_record | app_token, table_id, fields | - |
| ✅ 更新一行 | bitable | update_record | app_token, table_id, record_id, fields | - |
| ✅ 删除记录 | bitable | delete_record | app_token, table_id, record_id | - |
| ✅ 查看所有表 | bitable | list_tables | app_token | - |
| 🔧 批量导入 | api | POST /bitable/v1/apps/{app_token}/tables/{table_id}/records/batch_create | records (≤500) | - |
| 🔧 批量更新 | api | PUT /bitable/v1/apps/{app_token}/tables/{table_id}/records/batch_update | records (≤500) | - |
| 🔧 创建多维表格 | api | POST /bitable/v1/apps | name | folder_token |
| 🔧 创建数据表 | api | POST /bitable/v1/apps/{app_token}/tables | name | fields |

### 调用方式

```bash
python3 skills/feishu-cli/run.py '{"action":"tool","module":"bitable","tool_action":"list_records","app_token":"S404b...","table_id":"tbl..."}'
```

---

## 核心约束（Schema 未透露的知识）

### 详细参考文档

**当遇到字段配置、记录值格式问题或需要完整示例时，查阅以下文档**：

- **[字段 Property 配置详解](references/field-properties.md)** — 每种字段类型创建/更新时的 `property` 参数结构
- **[记录值数据结构详解](references/record-values.md)** — 每种字段类型在记录中对应的 `fields` 值格式
- **[使用场景完整示例](references/examples.md)** — 完整场景示例

**何时查阅**:
- 创建/更新字段时收到 `125408X` 错误码 → 查 field-properties.md
- 写入记录时收到 `125406X` 错误码 → 查 record-values.md
- 需要完整的操作流程和参数示例 → 查 examples.md

---

### 字段类型与值格式必须严格匹配

| type | ui_type | 字段类型 | 正确格式 | 常见错误 |
|------|---------|----------|---------|----------|
| 11 | User | 人员 | `[{id: "ou_xxx"}]` | 传字符串或 `[{name: "张三"}]` |
| 5 | DateTime | 日期 | `1674206443000`（毫秒） | 传秒时间戳或字符串 |
| 3 | SingleSelect | 单选 | `"选项名"` | 传数组 `["选项名"]` |
| 4 | MultiSelect | 多选 | `["选项1", "选项2"]` | 传字符串 |
| 15 | Url | 超链接 | `{link: "...", text: "..."}` | 只传字符串 URL |
| 17 | Attachment | 附件 | `[{file_token: "..."}]` | 传外部 URL |

**强制流程**：
1. 先调用 `list_fields` 获取字段的 `type` 和 `ui_type`
2. 根据上表或 record-values.md 构造正确格式
3. 错误码 `125406X` 或 `1254015` → 检查字段值格式

---

## 筛选查询（高级筛选）

filter 参数示例：

```json
{
  "conjunction": "and",
  "conditions": [
    {"field_name": "状态", "operator": "is", "value": ["进行中"]},
    {"field_name": "截止日期", "operator": "isLess", "value": ["ExactDate", "1740441600000"]}
  ]
}
```

**filter operator 列表**：

| operator | 含义 | value 要求 |
|----------|------|-----------|
| `is` | 等于 | 单个值 |
| `isNot` | 不等于 | 单个值 |
| `contains` | 包含 | 可多个值 |
| `doesNotContain` | 不包含 | 可多个值 |
| `isEmpty` | 为空 | **必须为 `[]`** |
| `isNotEmpty` | 不为空 | **必须为 `[]`** |
| `isGreater` | 大于 | 单个值 |
| `isLess` | 小于 | 单个值 |

**日期字段特殊值**: `["Today"]`, `["Tomorrow"]`, `["ExactDate", "毫秒时间戳"]`

---

## 常见错误与排查

| 错误码 | 原因 | 解决方案 |
|--------|------|---------|
| 1254064 | 日期格式错误 | 必须用毫秒时间戳，不能用字符串或秒时间戳 |
| 1254068 | 超链接格式错误 | 必须用 `{text, link}` 对象 |
| 1254066 | 人员字段格式错误 | 必须传 `[{id: "ou_xxx"}]` |
| 1254015 | 字段值与类型不匹配 | 先 list_fields，按类型构造 |
| 1254104 | 批量超 500 条 | 分批调用 |
| 1254291 | 并发写冲突 | 串行调用 + 延迟 0.5-1 秒 |
| 1254045 | 字段名不存在 | 检查字段名（包括空格、大小写） |

---

## 资源层级与限制

```
App (多维表格应用)
 ├── Table (数据表) ×100
 │    ├── Record (记录/行) ×20,000
 │    ├── Field (字段/列) ×300
 │    └── View (视图) ×200
 └── Dashboard (仪表盘)
```

| 限制项 | 上限 |
|--------|------|
| 批量创建/更新/删除 | 500（单次） |
| 单元格文本 | 10 万字符 |
| 单选/多选选项 | 20,000/字段 |
