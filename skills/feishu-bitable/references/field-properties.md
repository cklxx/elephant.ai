# 字段 Property 配置详解

创建或更新字段时，不同 `type` 需要的 `property` 参数结构不同。

## 常用字段类型 property

### 单选 (type=3)

```json
{
  "options": [
    {"name": "选项1"},
    {"name": "选项2", "color": 1}
  ]
}
```

- `color`: 0-54 整数，颜色索引
- 创建记录时写入不存在的选项会自动创建

### 多选 (type=4)

```json
{
  "options": [
    {"name": "标签A"},
    {"name": "标签B"}
  ]
}
```

### 日期 (type=5)

```json
{
  "date_formatter": "yyyy/MM/dd",
  "auto_fill": false
}
```

- `date_formatter`: `"yyyy/MM/dd"`, `"yyyy-MM-dd"`, `"yyyy/MM/dd HH:mm"`, `"yyyy-MM-dd HH:mm"`
- `auto_fill`: true 时自动填充创建时间

### 数字 (type=2)

```json
{
  "formatter": "0.00"
}
```

- `formatter`: `"0"` 整数, `"0.0"` 1位小数, `"0.00"` 2位小数, `"0%"` 百分比, `"0.00%"` 百分比2位

### 复选框 (type=7)

```json
{}
```

无需 property，或传空对象。

### 人员 (type=11)

```json
{
  "multiple": true
}
```

- `multiple`: 是否允许多人

### 超链接 (type=15)

```json
{}
```

无需特殊 property。

### 附件 (type=17)

```json
{}
```

### 单向关联 (type=18)

```json
{
  "table_id": "tblXXXXXXXX",
  "multiple": true
}
```

- `table_id`: 关联目标表 ID
- `multiple`: 是否允许关联多条记录

### 双向关联 (type=21)

```json
{
  "table_id": "tblXXXXXXXX",
  "back_field_name": "反向关联字段名",
  "multiple": true
}
```

### 进度 (type=99001)

```json
{
  "min": 0,
  "max": 100
}
```

### 货币 (type=99002)

```json
{
  "currency_code": "CNY"
}
```

- 支持: `"CNY"`, `"USD"`, `"EUR"`, `"JPY"`, `"GBP"` 等

### 评分 (type=99003)

```json
{
  "rating": {
    "symbol": "star"
  },
  "min": 0,
  "max": 5
}
```

### 电话 (type=99004)

```json
{}
```

### 地理位置 (type=99005)

```json
{
  "location": {
    "input_type": "only_mobile"
  }
}
```

- `input_type`: `"only_mobile"` 仅移动端, `"not_limit"` 不限制

## 只读字段（不可创建/修改值）

| type | 类型 | 说明 |
|------|------|------|
| 1001 | 创建时间 | 系统自动 |
| 1002 | 修改时间 | 系统自动 |
| 1003 | 创建人 | 系统自动 |
| 1004 | 修改人 | 系统自动 |
| 20 | 公式 | 由公式计算 |
| 22 | 引用 | 从关联表引用 |
| 23 | 自动编号 | 系统自动生成 |

## 创建字段示例

```bash
python3 skills/feishu-cli/run.py '{"action":"tool","module":"bitable","tool_action":"create_field","app_token":"S404b...","table_id":"tbl...","field_name":"负责人","type":11,"property":{"multiple":true}}'
```
