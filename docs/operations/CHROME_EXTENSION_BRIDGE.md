# Chrome Extension Bridge（实验性）

Updated: 2026-02-10

该方案用于在 `toolset: local` 下尝试复用日常 Chrome 登录态（cookies/localStorage）。

## 当前状态

- 配置字段 `runtime.browser.connector=chrome_extension`、`browser.bridge_*` 已纳入 runtime schema。
- 当前公开工具面仍以 `browser_action` 为主，不提供独立的 `browser_session_status` / `browser_cookies` 等工具。
- 因此该方案定位为**实验性桥接能力**；生产链路优先使用 `cdp` 方案（见 `docs/operations/LOCAL_BROWSER_CDP.md`）。

## 配置示例（YAML）

```yaml
runtime:
  toolset: "local"
  browser:
    connector: "chrome_extension"
    bridge_listen_addr: "127.0.0.1:17333"
    # bridge_token: "${ALEX_BROWSER_BRIDGE_TOKEN}"
```

## 扩展安装（开发者模式）

1. 打开 `chrome://extensions/`
2. 开启 Developer mode
3. Load unpacked: `tools/chrome-extension/elephant-bridge/`
4. 在扩展 options 中确认 bridge URL/token 与配置一致

## 验证建议

由于没有独立状态工具，建议直接通过任务验证：
- 启动 `alex`（`toolset: local`）
- 执行依赖登录态的 `browser_action` 流程
- 观察是否可直接访问需要登录的页面

如果失败：
- 优先回退到 CDP 方案（`browser.cdp_url`）
- 检查 localhost websocket 连通性、token 一致性、扩展是否活跃

## 安全提示

- bridge 仅监听 `127.0.0.1`。
- 若启用 token，避免在日志中输出明文。
- 不要扩大扩展 host permissions 到不必要域名。
