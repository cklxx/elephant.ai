# 企业微信 AI 跟进系统（7天 MVP 技术验证）

## 目录结构

```text
wecom-mvp/
├── README.md
├── requirements.txt
└── src/
    ├── main.py
    ├── config.py
    ├── handler.py
    └── customer.py
```

## 快速启动

```bash
cd ~/.alex/kernel/default/wecom-mvp
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
python src/main.py
```

服务默认监听：`http://127.0.0.1:8080`

健康检查：`GET /health`

## 企业微信自建应用配置

在企业微信管理后台（应用 -> 自建应用 -> 接收消息）配置：

- URL: `https://<你的域名>/wecom/callback`
- Token: 与 `WECOM_TOKEN` 保持一致
- EncodingAESKey: 与 `WECOM_ENCODING_AES_KEY` 保持一致

应用配置对应占位变量在 `src/config.py` 中：

- `WECOM_CORP_ID`
- `WECOM_CORP_SECRET`
- `WECOM_AGENT_ID`
- `WECOM_TOKEN`
- `WECOM_ENCODING_AES_KEY`
- `WECOM_RECEIVE_ID`（通常填 CorpID）

推荐通过环境变量注入：

```bash
export WECOM_CORP_ID="wwxxxxxxxx"
export WECOM_CORP_SECRET="xxxxxxxx"
export WECOM_AGENT_ID="1000002"
export WECOM_TOKEN="your_token"
export WECOM_ENCODING_AES_KEY="43位EncodingAESKey"
export WECOM_RECEIVE_ID="wwxxxxxxxx"
```

## 已实现能力

1. **URL 回调验证**（GET）
   - 校验 `msg_signature`
   - 解密 `echostr`
   - 返回明文完成企业微信配置验证

2. **消息接收与自动回复**（POST）
   - 解密回调 XML
   - 解析文本消息
   - 返回固定回复：`收到您的消息，顾问将尽快联系您`

3. **客户标签管理（内存版）**
   - 基于关键词打标签：
     - 价格/费用/报价 -> `pricing_intent`
     - 试用/体验/demo -> `trial_intent`
     - 购买/下单/合同 -> `purchase_intent`
     - 其他 -> `general`
   - 记录客户互动历史（文本+标签+时间）

## 注意事项（MVP阶段）

- 当前为本地内存存储，进程重启后数据清空。
- 当前以被动回复为主，后续可接入主动发送消息 API。
- 生产环境需补充：签名重放防护、日志脱敏、持久化数据库、异常告警。

