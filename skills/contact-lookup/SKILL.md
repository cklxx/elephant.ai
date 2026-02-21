---
name: contact-lookup
description: 飞书通讯录的用户和部门查询。
triggers:
  intent_patterns:
    - "通讯录|联系人|部门|who is|组织架构|contact"
  context_signals:
    keywords: ["通讯录", "联系人", "部门", "who is", "组织架构", "contact", "用户"]
  confidence_threshold: 0.6
priority: 7
exclusive_group: lark-contact
requires_tools: [channel]
max_tokens: 200
cooldown: 30
---

# contact-lookup

飞书通讯录查询：查找用户、列出部门成员、查询组织架构。

## 调用

通过 channel tool 的 action 参数调用：

| Action | 说明 |
|--------|------|
| `get_user` | 查询用户信息 |
| `list_users` | 列出部门下的用户 |
| `get_department` | 查询部门信息 |
| `list_departments` | 列出子部门 |

## 参数

### get_user
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| user_id | string | 否 | 用户 open_id。**省略时自动使用当前发送者的 open_id**，查询"我是谁"时无需提供。仅在查询其他人时才需指定。 |
| user_id_type | string | 否 | ID 类型 (open_id/union_id/user_id，默认 open_id) |

### list_users
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| department_id | string | 是 | 部门 ID |
| user_id_type | string | 否 | 返回的用户 ID 类型 |
| page_size | integer | 否 | 每页数量（默认 50） |
| page_token | string | 否 | 分页 token |

### get_department
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| department_id | string | 是 | 部门 ID |

### list_departments
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| parent_department_id | string | 否 | 父部门 ID（默认根部门） |
| page_size | integer | 否 | 每页数量（默认 50） |
| page_token | string | 否 | 分页 token |

## 示例

```
查看我的信息（自动使用当前用户）
-> channel(action="get_user")

查找其他用户信息（需指定 user_id）
-> channel(action="get_user", user_id="ou_xxx")

列出工程部成员
-> channel(action="list_users", department_id="dept_xxx")

查看部门信息
-> channel(action="get_department", department_id="dept_xxx")

列出所有顶级部门
-> channel(action="list_departments")
```

## 安全等级

- 所有操作: L1 只读，无需审批
