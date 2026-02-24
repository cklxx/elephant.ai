# 企业微信 AI 跟进系统 MVP

一个 7 天可交付验证的技术原型，提供：
- 企业微信 webhook 消息接收
- 消息持久化（SQLite）
- 基于标签的 SOP 规则匹配
- AI 回复生成（OpenAI/Claude 兼容）
- 日报数据聚合脚本

## 目录结构

```bash
src/
  main.py
  db.py
  models.py
  rule_engine.py
  ai_service.py
config/
  settings.yaml
scripts/
  daily_report.py
docs/
  DEPLOYMENT.md
```

## 快速启动

```bash
cd ~/.alex/kernel/default/mvp/wecom-ai-followup
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
python src/main.py
```

服务默认监听：`0.0.0.0:8000`

## 环境变量

```bash
export AI_PROVIDER=openai   # openai | claude
export AI_API_KEY=your_key
export AI_MODEL=gpt-4o-mini
```

## 核心流程

1. 企业微信 POST webhook 到 `/webhook/wecom`
2. 服务写入消息记录到 SQLite
3. 规则引擎根据客户标签命中 SOP
4. AI 生成回复建议（prompt 模板化）
5. 返回结构化结果，并写入 SOP 执行记录

## 日报脚本

```bash
python scripts/daily_report.py
```

输出指标：
- 当日消息数
- 命中 SOP 次数
- AI 调用次数
- 客户活跃数

