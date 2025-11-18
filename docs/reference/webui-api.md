# ALEX Web UI API 文档
> Last updated: 2025-11-18


ALEX Web UI 服务器提供 HTTP REST API 和 WebSocket 接口，允许通过 web 浏览器或 HTTP 客户端与 ALEX 进行交互。

## 启动 Web UI 服务器

```bash
# 默认设置（localhost:8080）
./alex webui

# 自定义主机和端口
./alex webui --host 0.0.0.0 --port 3000

# 启用调试模式
./alex webui --debug --cors
```

## API 端点

### 健康检查

**GET** `/api/health`

返回服务器状态信息。

```bash
curl http://localhost:8080/api/health
```

响应示例：
```json
{
  "success": true,
  "data": {
    "status": "ok",
    "version": "0.4.6",
    "timestamp": "2025-09-23T14:21:57Z",
    "uptime": "1m30s"
  }
}
```

### 会话管理

#### 创建会话

**POST** `/api/sessions`

```bash
curl -X POST http://localhost:8080/api/sessions \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "my-session",
    "working_dir": "/path/to/project"
  }'
```

#### 获取会话列表

**GET** `/api/sessions`

```bash
curl http://localhost:8080/api/sessions
```

#### 获取单个会话

**GET** `/api/sessions/{session_id}`

```bash
curl http://localhost:8080/api/sessions/my-session
```

#### 删除会话

**DELETE** `/api/sessions/{session_id}`

```bash
curl -X DELETE http://localhost:8080/api/sessions/my-session
```

### 消息处理

#### 发送消息（非流式）

**POST** `/api/sessions/{session_id}/messages`

```bash
curl -X POST http://localhost:8080/api/sessions/my-session/messages \
  -H "Content-Type: application/json" \
  -d '{
    "content": "分析这个项目的架构",
    "stream_mode": false
  }'
```

#### 获取消息历史

**GET** `/api/sessions/{session_id}/messages`

```bash
curl http://localhost:8080/api/sessions/my-session/messages?limit=10&offset=0
```

### 配置管理

#### 获取配置

**GET** `/api/config`

```bash
curl http://localhost:8080/api/config
```

#### 更新配置（当前未实现）

**PUT** `/api/config`

### 工具管理

#### 获取可用工具列表

**GET** `/api/tools`

```bash
curl http://localhost:8080/api/tools
```

## WebSocket 流式通信

WebSocket 端点：`ws://localhost:8080/api/sessions/{session_id}/stream`

### 连接示例

```javascript
const sessionId = 'my-session';
const ws = new WebSocket(`ws://localhost:8080/api/sessions/${sessionId}/stream`);

ws.onopen = function(event) {
    console.log('WebSocket connected');
};

ws.onmessage = function(event) {
    const message = JSON.parse(event.data);
    console.log('Received:', message);

    if (message.type === 'stream') {
        // 处理流式响应
        console.log('Stream content:', message.data.content);
    }
};

// 发送消息
const messageRequest = {
    type: 'message',
    data: {
        content: '帮我分析这个代码库',
        config: {}
    }
};

ws.send(JSON.stringify(messageRequest));
```

### WebSocket 消息类型

- `connect`: 连接成功
- `disconnect`: 连接断开
- `message`: 用户发送的消息
- `stream`: 流式响应数据
- `error`: 错误消息
- `heartbeat`: 心跳检测
- `complete`: 任务完成

### 流式消息格式

```json
{
  "type": "stream",
  "data": {
    "type": "thinking",
    "content": "正在分析代码结构...",
    "complete": false,
    "metadata": {
      "phase": "analysis"
    },
    "tokens_used": 10,
    "timestamp": "2025-09-23T14:21:57Z"
  },
  "session_id": "my-session",
  "timestamp": "2025-09-23T14:21:57Z"
}
```

## 错误处理

所有 API 响应都使用统一的格式：

```json
{
  "success": true|false,
  "data": {}, // 成功时的数据
  "message": "操作描述", // 可选的消息
  "error": "错误描述" // 失败时的错误信息
}
```

常见错误代码：
- `400 Bad Request`: 请求参数错误
- `404 Not Found`: 资源不存在
- `500 Internal Server Error`: 服务器内部错误
- `501 Not Implemented`: 功能未实现

## 使用示例

### Python 客户端示例

```python
import requests
import json
import websocket

# 创建会话
response = requests.post('http://localhost:8080/api/sessions',
                        json={'session_id': 'python-client'})
print(response.json())

# 发送非流式消息
response = requests.post('http://localhost:8080/api/sessions/python-client/messages',
                        json={'content': '你好，ALEX！', 'stream_mode': False})
print(response.json())

# WebSocket 流式通信
def on_message(ws, message):
    data = json.loads(message)
    print(f"Received: {data}")

def on_open(ws):
    # 发送消息
    message = {
        "type": "message",
        "data": {
            "content": "分析当前目录的代码",
            "config": {}
        }
    }
    ws.send(json.dumps(message))

ws = websocket.WebSocketApp("ws://localhost:8080/api/sessions/python-client/stream",
                           on_message=on_message,
                           on_open=on_open)
ws.run_forever()
```

### cURL 完整工作流程

```bash
# 1. 创建会话
curl -X POST http://localhost:8080/api/sessions \
  -H "Content-Type: application/json" \
  -d '{"session_id": "curl-demo", "working_dir": "/tmp"}'

# 2. 发送消息
curl -X POST http://localhost:8080/api/sessions/curl-demo/messages \
  -H "Content-Type: application/json" \
  -d '{"content": "创建一个简单的 Hello World 程序", "stream_mode": false}'

# 3. 获取消息历史
curl http://localhost:8080/api/sessions/curl-demo/messages

# 4. 获取会话信息
curl http://localhost:8080/api/sessions/curl-demo

# 5. 删除会话
curl -X DELETE http://localhost:8080/api/sessions/curl-demo
```

## 架构说明

Web UI 服务器完全复用现有的 ReactAgent 逻辑，提供以下特性：

- **会话管理**: 完全兼容现有的 session 系统
- **流式响应**: 通过 WebSocket 实现实时流式通信
- **并发支持**: 支持多个并发会话
- **错误处理**: 统一的错误处理和恢复机制
- **配置兼容**: 与现有配置系统完全兼容

Web UI 不会影响现有的 CLI 功能，两者可以同时使用。
