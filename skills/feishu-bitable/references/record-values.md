# 记录值数据结构详解

创建/更新记录时，`fields` 中每个字段值的格式取决于字段类型。**格式不对会直接报错**。

## 值格式速查表

| type | ui_type | 字段类型 | 正确格式 | 示例 |
|------|---------|----------|---------|------|
| 1 | Text | 文本 | `string` | `"Hello"` |
| 2 | Number | 数字 | `number` | `42`, `3.14` |
| 3 | SingleSelect | 单选 | `string` | `"进行中"` |
| 4 | MultiSelect | 多选 | `string[]` | `["标签A", "标签B"]` |
| 5 | DateTime | 日期 | `number`（毫秒时间戳） | `1674206443000` |
| 7 | Checkbox | 复选框 | `boolean` | `true` |
| 11 | User | 人员 | `[{id: string}]` | `[{id: "ou_xxx"}]` |
| 13 | Phone | 电话 | `string` | `"+8613800138000"` |
| 15 | Url | 超链接 | `{link: string, text: string}` | `{link: "https://...", text: "点击"}` |
| 17 | Attachment | 附件 | `[{file_token: string}]` | `[{file_token: "xxx"}]` |
| 18 | SingleLink | 单向关联 | `[{record_id: string}]` | `[{record_id: "recXXX"}]` |
| 21 | DuplexLink | 双向关联 | `[{record_id: string}]` | `[{record_id: "recXXX"}]` |
| 22 | Lookup | 引用 | **只读** | - |
| 20 | Formula | 公式 | **只读** | - |
| 99001 | Progress | 进度 | `number` (0-100) | `75` |
| 99002 | Currency | 货币 | `number` | `1234.56` |
| 99003 | Rating | 评分 | `number` | `4` |
| 99005 | Location | 地理位置 | `{location: string, ...}` | `{location: "北京市..."}` |

## 详细说明

### 文本 (type=1)

纯文本字符串。支持富文本格式时，可传数组：
```json
[
  {"type": "text", "text": "普通文本"},
  {"type": "mention", "mentionType": "User", "token": "ou_xxx"}
]
```
简单场景直接传字符串即可。

### 人员 (type=11)

**最常出错的字段类型**。

```json
// 正确 ✅
[{"id": "ou_xxx"}]
[{"id": "ou_xxx"}, {"id": "ou_yyy"}]

// 错误 ❌
"ou_xxx"                    // 不是数组
[{"name": "张三"}]          // 不能用 name
{"id": "ou_xxx"}            // 不是数组
[{"id": "ou_xxx", "name": "张三"}]  // 多余字段不影响，但只需 id
```

- 默认使用 `open_id`（以 `ou_` 开头）
- **只能传 id 字段**，不能传 name/email

### 日期 (type=5)

```json
// 正确 ✅
1674206443000      // 毫秒时间戳

// 错误 ❌
1674206443         // 秒时间戳
"2026-03-01"       // 字符串
"2026-03-01T00:00:00Z"  // ISO 格式
```

常用转换：
- Python: `int(datetime.timestamp() * 1000)`
- JavaScript: `Date.now()` 或 `new Date('2026-03-01').getTime()`

### 单选 vs 多选

```json
// 单选 (type=3) — 字符串
"进行中"

// 多选 (type=4) — 字符串数组
["标签A", "标签B"]
```

写入不存在的选项会自动创建该选项。

### 超链接 (type=15)

```json
// 正确 ✅
{"link": "https://example.com", "text": "示例链接"}

// 错误 ❌
"https://example.com"   // 不能只传字符串
```

### 附件 (type=17)

附件必须先上传到当前多维表格，获取 `file_token` 后才能写入：

```json
[{"file_token": "boxcnXXXXXXXXXX"}]
```

不能直接传外部 URL 或本地文件路径。

### 关联字段 (type=18/21)

```json
[{"record_id": "recXXXXXXXX"}]
[{"record_id": "recXXX"}, {"record_id": "recYYY"}]  // 多条关联
```
