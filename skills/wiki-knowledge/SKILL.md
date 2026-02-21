---
name: wiki-knowledge
description: 飞书知识库的空间和节点管理，支持知识沉淀与归档。
triggers:
  intent_patterns:
    - "知识库|wiki|归档|knowledge|知识空间|知识节点"
  context_signals:
    keywords: ["知识库", "wiki", "归档", "knowledge", "知识空间"]
  confidence_threshold: 0.6
priority: 7
exclusive_group: lark-wiki
requires_tools: [channel]
max_tokens: 200
cooldown: 30
---

# wiki-knowledge

飞书知识库管理：浏览空间、管理节点、创建知识文档。

## 调用

通过 channel tool 的 action 参数调用：

| Action | 说明 |
|--------|------|
| `list_wiki_spaces` | 列出所有知识空间 |
| `list_wiki_nodes` | 列出空间下的节点 |
| `create_wiki_node` | 在空间中创建新节点 |
| `get_wiki_node` | 获取节点详情 |

## 参数

### list_wiki_spaces
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| page_size | integer | 否 | 每页数量 |
| page_token | string | 否 | 分页 token |

### list_wiki_nodes
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| space_id | string | 是 | 知识空间 ID |
| parent_node_token | string | 否 | 父节点 token（为空则列出根节点） |
| page_size | integer | 否 | 每页数量 |
| page_token | string | 否 | 分页 token |

### create_wiki_node
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| space_id | string | 是 | 知识空间 ID |
| obj_type | string | 否 | 对象类型（doc/docx/sheet/bitable），默认 docx |
| parent_node_token | string | 否 | 父节点 token |
| title | string | 否 | 节点标题 |

### get_wiki_node
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| node_token | string | 是 | 节点 token |

## 示例

```
列出所有知识空间
→ channel(action="list_wiki_spaces")

在空间中创建文档节点
→ channel(action="create_wiki_node", space_id="xxx", title="会议纪要归档", obj_type="docx")

查看节点详情
→ channel(action="get_wiki_node", node_token="wikcnXXX")
```

## 自动执行原则

- **零参数查询**：用户说"看看知识库"时，直接调用 `channel(action="list_wiki_spaces")`，不需要任何参数。
- **禁止交互式菜单**：不要给出选项让用户选择知识空间，如果只有一个空间则直接使用。
- **链式自动执行**：列出空间后自动列出根节点，方便用户一次看到完整结构。
- **title 智能推断**：创建节点时如果用户没给标题，根据上下文自动生成。

## 安全等级

- `list_wiki_spaces` / `list_wiki_nodes` / `get_wiki_node`: L1 只读
- `create_wiki_node`: L3 高影响，需要审批

## 链式组合

- 与 `meeting-notes` skill 组合：会议纪要自动归档到知识库
- 与 `deep-research` skill 组合：研究结果沉淀到知识库
