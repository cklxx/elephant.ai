---
name: email-lark
description: 飞书邮件组的查询和管理。
triggers:
  intent_patterns:
    - "邮件|email|mail|邮件组|mailgroup"
  context_signals:
    keywords: ["邮件", "email", "mail", "邮件组", "mailgroup", "邮箱"]
  confidence_threshold: 0.6
priority: 7
exclusive_group: lark-mail
requires_tools: [channel]
max_tokens: 200
cooldown: 30
---

# email-lark

飞书邮件组管理：查询邮件组列表、查看邮件组详情、创建邮件组。

## 调用

通过 channel tool 的 action 参数调用：

| Action | 说明 |
|--------|------|
| `list_mailgroups` | 列出邮件组 |
| `get_mailgroup` | 获取邮件组详情 |
| `create_mailgroup` | 创建新的邮件组 |

## 参数

### list_mailgroups
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| page_size | integer | 否 | 每页数量（默认 20） |
| page_token | string | 否 | 分页 token |

### get_mailgroup
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| mailgroup_id | string | 是 | 邮件组 ID |

### create_mailgroup
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| email | string | 否 | 邮件组地址 |
| name | string | 否 | 邮件组名称 |
| description | string | 否 | 邮件组描述 |

## 示例

```
列出所有邮件组
-> channel(action="list_mailgroups")

查看邮件组详情
-> channel(action="get_mailgroup", mailgroup_id="mg_xxx")

创建邮件组
-> channel(action="create_mailgroup", email="team@company.com", name="Team Group")
```

## 自动执行原则

- **零参数查询**：用户说"看看邮件组"时，直接调用 `channel(action="list_mailgroups")`，不需要任何参数。
- **禁止交互式菜单**：不要给出选项让用户选择，直接执行最合理的操作。
- **链式自动执行**：列出邮件组后，如果用户想看详情，自动提取 mailgroup_id 调用 `get_mailgroup`。

## 安全等级

- `list_mailgroups` / `get_mailgroup`: L1 只读，无需审批
- `create_mailgroup`: L3 高影响，需要审批
