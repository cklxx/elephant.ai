# Sandbox 集成指南

Updated: 2026-02-10

本指南说明当前版本下 sandbox 工具链如何接入、切换和排障。

## 1. 当前工具模型

当前 registry 在平台执行能力上只暴露统一工具名：
- `browser_action`
- `read_file`
- `write_file`
- `replace_in_file`
- `shell_exec`
- `execute_code`

实现由 `runtime.toolset` 决定：
- `default`：走 sandbox 后端实现。
- `local`（别名 `lark-local`）：走本地实现。

## 2. 基础配置（YAML）

```yaml
runtime:
  toolset: "default"
  sandbox_base_url: "http://127.0.0.1:8765"
  http_limits:
    sandbox_max_response_bytes: 1048576
```

说明：
- `sandbox_base_url` 只写根地址，不带 `/v1`。
- `toolset: default` 时平台执行工具自动走 sandbox client。

## 3. 本地启动与健康检查

```bash
alex dev sandbox up
alex dev sandbox status
alex dev up
```

手动探活（示例）：

```bash
curl -s http://127.0.0.1:8765/v1/health
```

## 4. 行为边界

- `browser_action` 支持动作列表执行，可选截图（`capture_screenshot`）。
- 文件与命令工具在 sandbox 模式下走远端隔离环境。
- 如需复用本机浏览器登录态或本地文件系统，切换到 `toolset: local`。

## 5. 常见问题

1. `tool not found`：确认是否在当前 profile 下被禁用或未注册。
2. `connection refused`：确认 sandbox 服务已启动且 `sandbox_base_url` 正确。
3. 返回体被截断：提高 `runtime.http_limits.sandbox_max_response_bytes`。
4. 误用旧工具名（如 `sandbox_browser_*`）：请改为统一的 `browser_action`。
