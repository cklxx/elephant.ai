---
name: doc-management
description: 飞书云文档的创建、读取和内容获取。
triggers:
  intent_patterns:
    - "文档|document|doc|写文档|创建文档|读文档|docx"
  context_signals:
    keywords: ["文档", "document", "doc", "docx", "写文档", "创建文档"]
  confidence_threshold: 0.6
priority: 7
exclusive_group: lark-docs
requires_tools: [channel]
max_tokens: 200
cooldown: 30
---

# doc-management

飞书云文档管理：创建文档、查看文档元信息、获取文档内容。

## 调用

通过 channel tool 的 action 参数调用：

| Action | 说明 |
|--------|------|
| `create_doc` | 创建新的飞书云文档 |
| `read_doc` | 获取文档元信息（标题、修订版本等） |
| `read_doc_content` | 获取文档的原始文本内容 |

## 参数

### create_doc
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| title | string | 否 | 文档标题 |
| folder_token | string | 否 | 目标文件夹 token |

### read_doc / read_doc_content
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| document_id | string | 是 | 文档 ID |

## 示例

```
创建一个名为"周报"的文档
→ channel(action="create_doc", title="周报")

读取文档内容
→ channel(action="read_doc_content", document_id="doxcnXXX")
```

## 安全等级

- `read_doc` / `read_doc_content`: L1 只读，无需审批
- `create_doc`: L3 高影响，需要审批
