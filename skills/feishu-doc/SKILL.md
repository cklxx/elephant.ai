---
name: feishu-doc
description: |
  飞书云文档工具。支持文档的创建、读取和更新，包含 Lark 风味 Markdown 语法。

  **当以下情况时使用此 Skill**：
  (1) 需要创建飞书云文档
  (2) 需要读取/获取文档内容
  (3) 需要更新已有文档内容
  (4) 用户提到"文档"、"doc"、"document"、"wiki"
triggers:
  intent_patterns:
    - "云文档|document|docx|创建文档|读取文档|更新文档"
    - "知识库|wiki|知识空间"
    - "文档内容|写文档|编辑文档"
  context_signals:
    keywords: ["doc", "document", "docx", "wiki", "文档", "知识库", "document_id"]
  confidence_threshold: 0.5
priority: 9
requires_tools: [bash]
max_tokens: 300
cooldown: 15
---

# Feishu Doc Skill

## 快速索引

### 文档操作

| 用户意图 | module | tool_action | 必填参数 |
|---------|--------|-------------|---------|
| 创建文档 | doc | create | title |
| 读取文档 | doc | read | document_id |
| 读取文档内容 | doc | read_content | document_id |
| 写入 Markdown | doc | write_markdown | document_id, markdown |
| 列出文档块 | doc | list_blocks | document_id |
| 更新文本块 | doc | update_block_text | document_id, block_id, text |

### Wiki 操作

| 用户意图 | module | tool_action | 必填参数 |
|---------|--------|-------------|---------|
| 列出知识空间 | wiki | list_spaces | - |
| 列出节点 | wiki | list_nodes | space_id |
| 创建节点 | wiki | create_node | space_id, title |
| 获取节点 | wiki | get_node | space_id, node_token |

## 调用示例

### 创建文档

```bash
python3 skills/feishu-cli/run.py '{
  "action": "tool",
  "module": "doc",
  "tool_action": "create",
  "title": "Q1 工作总结",
  "folder_token": "fldcnXXXX"
}'
```

### 读取文档内容

```bash
python3 skills/feishu-cli/run.py '{
  "action": "tool",
  "module": "doc",
  "tool_action": "read_content",
  "document_id": "doccnXXXX"
}'
```

### 写入 Markdown 内容

```bash
python3 skills/feishu-cli/run.py '{
  "action": "tool",
  "module": "doc",
  "tool_action": "write_markdown",
  "document_id": "doccnXXXX",
  "markdown": "## 概述\n\n本季度完成了以下关键目标：\n\n1. 项目 A 上线\n2. 性能优化 30%"
}'
```

### Wiki 操作

```bash
# 列出知识空间
python3 skills/feishu-cli/run.py '{"action":"tool","module":"wiki","tool_action":"list_spaces"}'

# 列出节点
python3 skills/feishu-cli/run.py '{"action":"tool","module":"wiki","tool_action":"list_nodes","space_id":"7xxx"}'
```

## 核心约束

### 文档 ID 来源

- URL 中提取：`https://xxx.feishu.cn/docx/doccnXXXX` → `document_id = "doccnXXXX"`
- Wiki URL：先用 `wiki get_node` 获取 `obj_token`，再作为 `document_id` 使用
- API 返回：创建文档后返回

### Lark Markdown 与标准 Markdown 差异

飞书文档支持的 Markdown 是 **Lark 风味**，与标准 GFM 有一些差异：

- **标题**: 支持 `#` 到 `######`
- **代码块**: 支持 ` ```language ` 语法
- **表格**: 使用 `| col1 | col2 |` 标准表格语法
- **任务列表**: 使用 `- [ ]` 和 `- [x]`
- **引用**: 使用 `> ` 前缀
- **特殊块**: 支持 `callout`、`grid`、`mermaid`、`plantuml` 等

### 权限

- 读取需要 `docx:document:readonly` scope
- 写入需要 `docx:document` scope
- Wiki 操作需要 `wiki:wiki` 相关 scope
