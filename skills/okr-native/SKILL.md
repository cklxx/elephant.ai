---
name: okr-native
description: 飞书原生 OKR 的查询和管理。
triggers:
  intent_patterns:
    - "OKR|目标|关键结果|okr|objective|key result"
  context_signals:
    keywords: ["OKR", "目标", "关键结果", "okr", "objective", "key result", "绩效"]
  confidence_threshold: 0.6
priority: 7
exclusive_group: lark-okr
requires_tools: [channel]
max_tokens: 200
cooldown: 30
---

# okr-native

飞书原生 OKR 管理：查询 OKR 周期、用户 OKR、批量获取 OKR。

## 调用

通过 channel tool 的 action 参数调用：

| Action | 说明 |
|--------|------|
| `list_okr_periods` | 列出 OKR 周期 |
| `list_user_okrs` | 查询用户的 OKR |
| `batch_get_okrs` | 批量获取 OKR 详情 |

## 参数

### list_okr_periods
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| page_size | integer | 否 | 每页数量（默认 20） |
| page_token | string | 否 | 分页 token |

### list_user_okrs
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| user_id | string | 否 | 用户 open_id。**省略时自动使用当前发送者的 open_id**，查询"我的 OKR"时无需提供。仅在查询其他人的 OKR 时才需指定。 |
| period_id | string | 否 | OKR 周期 ID |

### batch_get_okrs
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| okr_ids | array | 是 | OKR ID 列表 |

## 示例

```
查看我的 OKR（自动使用当前用户）
-> channel(action="list_user_okrs")

查看当前 OKR 周期
-> channel(action="list_okr_periods")

查看某人的 OKR（需指定 user_id）
-> channel(action="list_user_okrs", user_id="ou_xxx")

批量获取 OKR
-> channel(action="batch_get_okrs", okr_ids=["okr_1", "okr_2"])
```

## 安全等级

- 所有操作: L1 只读，无需审批
