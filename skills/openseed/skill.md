---
name: openseed
description: When the user wants to research academic papers → search ArXiv, extract PDFs, summarize/review/Q&A, generate experiment code.
triggers:
  intent_patterns:
    - "openseed|论文|paper|arxiv|学术|research.*paper|文献|调研.*论文|survey"
  context_signals:
    keywords: ["openseed", "paper", "arxiv", "论文", "文献", "survey", "research"]
  confidence_threshold: 0.7
priority: 9
requires_tools: [bash]
max_tokens: 200
cooldown: 30
capabilities: ["research", "paper_search", "summarization", "analysis"]
activation_mode: explicit
output:
  format: markdown
  artifacts: false
---

# OpenSeed — AI 论文调研

AI-powered research CLI — 论文检索、摘要、评审、问答、实验代码生成。

## 典型场景

- 搜索某方向的论文：`openseed paper search "diffusion models"`
- 深度调研 + 趋势分析：`openseed agent search "multi-agent systems"`
- 一键流水线（搜→选→分析→入库）：`openseed agent pipeline "ViT image classification"`
- 论文摘要/评审：`openseed agent summarize <id>` / `openseed agent review <id>`
- 自由研究问答：`openseed agent ask "What is RLHF?"`
- 生成实验代码：`openseed agent codegen <id>`

## 命令

```bash
# 搜索论文（按引用数排序）
openseed paper search "attention" --count 20

# 添加论文到本地库
openseed paper add https://arxiv.org/abs/1706.03762

# AI 摘要（支持 --cn 中文）
openseed agent summarize <id>

# 研究问答
openseed agent ask "What is RLHF?"

# 一键调研流水线
openseed agent pipeline "ViT image classification"
```

## 前置

- Python 3.11+，`pip install openseed`
- `ANTHROPIC_API_KEY` 或 `claude setup-token`
- `openseed doctor` 检查环境，`openseed setup` 配置认证
