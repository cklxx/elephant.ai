# AIO Sandbox 接入与浏览器工具迁移指南
> Last updated: 2025-12-27

本指南覆盖 **AIO Sandbox（agent-infra/sandbox）** 的完整接入流程、运行方式、配置项、工具调用规范，以及从旧版 `browser` 工具迁移到 `sandbox_browser` 系列工具的操作细节。适用于本地开发、服务器部署与运维排障。

---

## 1. 目标与变化总览

### 1.1 为什么迁移

旧版 `browser` 工具依赖 Playwright 本地运行，稳定性和可重复性受环境影响较大（浏览器安装、依赖缺失、运行时状态漂移等）。AIO Sandbox 提供 **统一、隔离、可复用** 的浏览器环境，同时支持 VNC/CDP/MCP 访问，便于多工具协作。

### 1.2 新旧工具映射

| 旧工具 | 新工具 | 变化说明 |
| --- | --- | --- |
| `browser` | `sandbox_browser` | 执行动作列表，动作结构遵循 Sandbox OpenAPI |
| `browser` | `sandbox_browser_info` | 获取 browser info（user agent / viewport / CDP / VNC） |
| `browser` | `sandbox_browser_screenshot` | 仅截图，不执行动作 |

> ✅ 旧工具 `browser` 已删除，**请全部切换到 sandbox 系列工具**。

---

## 2. 运行 Sandbox（必须）

### 2.1 Docker 方式（推荐）

#### 国际版镜像
```bash
docker run --rm -it -p 18086:8080 ghcr.io/agent-infra/sandbox:latest
```

#### 中国大陆镜像
```bash
docker run --rm -it -p 18086:8080 enterprise-public-cn-beijing.cr.volces.com/vefaas-public/all-in-one-sandbox:latest
```

#### 腾讯云国内部署（建议走内网）

若在腾讯云（TKE/自建 Docker）部署，通常会遇到外网镜像拉取慢或被阻断的情况。推荐流程：

1. 在一台可访问 `ghcr.io` 的机器上拉取镜像：  
   `docker pull ghcr.io/agent-infra/sandbox:latest`
2. 推送到腾讯云 TCR（示例域名请替换为你的实例）：  
   `docker tag ghcr.io/agent-infra/sandbox:latest ccr.ccs.tencentyun.com/<namespace>/sandbox:latest`  
   `docker push ccr.ccs.tencentyun.com/<namespace>/sandbox:latest`
3. 在 TKE 或云服务器上使用 TCR 镜像启动，并通过内网 LB/CLB 暴露：  
   `docker run --rm -it -p 18086:8080 ccr.ccs.tencentyun.com/<namespace>/sandbox:latest`

建议将 `SANDBOX_BASE_URL` 指向 **同地域内网地址**（VPC 内 DNS / 内网 LB），并确认安全组放行 18086。
```

### 2.2 健康检查与入口

Sandbox 启动后，确认以下入口可访问：

- 文档首页：`http://localhost:18086/v1/docs`
- OpenAPI：`http://localhost:18086/v1/openapi.json`
- VNC 浏览器：`http://localhost:18086/vnc/index.html?autoconnect=true`
- MCP 服务：`http://localhost:18086/mcp`

若无法访问，优先检查：

1. Docker 容器是否运行：`docker ps`
2. 端口映射是否为 `18086:8080`
3. 本地防火墙 / VPN / 代理是否阻断 `localhost:18086`

---

## 3. ALEX 侧配置（必须）

### 3.1 配置项

| 配置来源 | Key | 说明 |
| --- | --- | --- |
| 环境变量 | `SANDBOX_BASE_URL` | Sandbox API 根地址（**不含 `/v1`**） |
| JSON 配置 | `sandbox_base_url` | 同上，写入 `~/.alex-config.json` |

默认值为 `http://localhost:18086`。

### 3.2 环境变量方式

```bash
export SANDBOX_BASE_URL="http://localhost:18086"
```

### 3.3 JSON 配置方式

`~/.alex-config.json` 示例：

```json
{
  "sandbox_base_url": "http://localhost:18086"
}
```

> 如果同时设置 env + config file，遵循常规优先级规则（overrides > env > file > default）。

### 3.4 dev/deploy 脚本自动启动

`dev.sh` 与 `deploy.sh` 会在本地模式下自动启动 Sandbox，并在 `/v1/docs` 做可用性检测。可通过以下环境变量调整：

- `SANDBOX_PORT`：本地映射端口（默认 `18086`）
- `SANDBOX_IMAGE`：Sandbox 镜像（默认 `ghcr.io/agent-infra/sandbox:latest`）
- `SANDBOX_BASE_URL`：已部署的 Sandbox 地址（若指向非本地地址，将跳过本地容器启动，仅做健康检查）

在腾讯云环境中，建议先将镜像推送到 TCR，再通过 `SANDBOX_IMAGE` 指向 TCR 镜像，以避免 ghcr.io 拉取失败。

---

## 4. sandbox_browser 工具规范（核心）

### 4.1 工具一览

1. **`sandbox_browser`**  
   执行一组动作（actions），可选截图。

2. **`sandbox_browser_info`**  
   查询 Sandbox 的浏览器信息（viewport/CDP/VNC）。

3. **`sandbox_browser_screenshot`**  
   仅截图，不执行动作。

---

### 4.2 `sandbox_browser` 参数结构

```json
{
  "actions": [ ... ],              // 必填，动作列表
  "capture_screenshot": true,      // 可选，是否截图
  "screenshot_name": "page.png"    // 可选，截图文件名
}
```

#### 4.2.1 动作结构（actions）

动作遵循 Sandbox OpenAPI（`/v1/browser/actions`），关键 `action_type` 如下：

| action_type | 主要字段 | 说明 |
| --- | --- | --- |
| `MOVE_TO` | `x`, `y` | 鼠标移动到坐标 |
| `CLICK` | `x`, `y`, `button`, `num_clicks` | 点击（坐标可空，表示当前焦点） |
| `MOUSE_DOWN` | `button` | 鼠标按下 |
| `MOUSE_UP` | `button` | 鼠标抬起 |
| `RIGHT_CLICK` | `x`, `y` | 右键点击 |
| `DOUBLE_CLICK` | `x`, `y` | 双击 |
| `DRAG_TO` | `x`, `y` | 拖拽到坐标 |
| `SCROLL` | `dx`, `dy` | 滚动 |
| `TYPING` | `text`, `use_clipboard` | 输入文本 |
| `PRESS` | `key` | 按键（单键） |
| `KEY_DOWN` | `key` | 按键按下 |
| `KEY_UP` | `key` | 按键抬起 |
| `HOTKEY` | `keys` | 组合键（数组） |

> 字段是否必填以 Sandbox OpenAPI 为准。建议先通过 `sandbox_browser_info` 获取 viewport，再使用坐标类动作。

---

### 4.3 示例：打开页面 + 点击 + 截图

```json
{
  "actions": [
    { "action_type": "HOTKEY", "keys": ["CTRL", "L"] },
    { "action_type": "TYPING", "text": "https://example.com" },
    { "action_type": "PRESS", "key": "ENTER" },
    { "action_type": "CLICK", "x": 200, "y": 320 }
  ],
  "capture_screenshot": true,
  "screenshot_name": "example.png"
}
```

返回结果会包含：

- `content`：动作执行摘要
- `metadata.sandbox_browser.responses`：每个动作的执行结果
- 附件：截图 PNG（若 capture_screenshot=true）

> ALEX 会在请求头注入 `X-Session-ID`（来自工具调用的 session），以便 Sandbox 侧做会话级隔离与复用。
> 若需强绑定（例如多租户隔离），请确保上游调用总是传递一致的 session ID。

### 4.4 Session 绑定建议

当多用户/多任务共享同一 Sandbox 服务时，务必确保：

1. 上游每次调用都传入稳定的 session ID（同一会话一致，不同会话不同）。
2. Sandbox 侧保持按 `X-Session-ID` 的隔离策略，避免不同会话互相污染。

为避免同一 session 长时间闲置导致资源漂移，ALEX 会在客户端侧维护 **session → sandbox 实例** 映射：

- 新的 session 会分配新的 sandbox 会话 ID。
- 若同一 session 空闲超过 10 分钟，下一次调用会自动重置为新的 sandbox 会话 ID。
- 并发请求会复用同一 sandbox 会话 ID（在超时前）。

---

## 5. `sandbox_browser_info` 使用说明

调用后返回 JSON，包含：

```json
{
  "user_agent": "...",
  "cdp_url": "ws://...",
  "vnc_url": "http://...",
  "viewport": { "width": 1280, "height": 720 }
}
```

使用场景：

1. 获取 viewport，确保坐标动作在可见范围内。
2. 使用 CDP URL 进行高级自动化（如 Playwright 接管）。
3. 通过 VNC URL 进行人工视觉检查。

---

## 6. `sandbox_browser_screenshot` 使用说明

最简调用（无需参数）：

```json
{}
```

可选 `name` 指定附件名称：

```json
{ "name": "current.png" }
```

---

## 7. 排障清单

### 7.1 典型错误与处理

| 错误现象 | 可能原因 | 解决方式 |
| --- | --- | --- |
| `sandbox request failed: connection refused` | Sandbox 未启动或端口未映射 | 检查容器是否运行、端口是否映射 |
| `/v1/browser/actions` 404 | `SANDBOX_BASE_URL` 配置错误（含 `/v1`） | 确保 base_url 不含 `/v1` |
| 行为执行成功但页面未变化 | 坐标不正确或元素未聚焦 | 先 `sandbox_browser_info` 获取 viewport，再发 `MOVE_TO`/`CLICK` |

### 7.2 日志定位

若在 `alex-server` 内运行，可查看：

- Server 日志中工具执行报错
- Docker 容器日志（`docker logs <sandbox-container>`）

---

## 8. 迁移 Checklist（强制）

1. ✅ 启动 AIO Sandbox 容器。
2. ✅ 设置 `SANDBOX_BASE_URL`（或写入 config）。
3. ✅ 将所有 `browser` 调用替换为 `sandbox_browser` / `sandbox_browser_info` / `sandbox_browser_screenshot`。
4. ✅ 运行完整 lint + test。

---

## 9. 参考链接

- AIO Sandbox 代码与说明：<https://github.com/agent-infra/sandbox>
- OpenAPI JSON：`http://localhost:18086/v1/openapi.json`
