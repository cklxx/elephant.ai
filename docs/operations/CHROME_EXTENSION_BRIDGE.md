# Chrome Extension Bridge（复用日常 Chrome 登录态 / Cookies）

本方案用于 **macOS Chrome**：你在日常 Chrome（默认 profile）里完成登录（例如小红书 XHS），`alex` 通过一个 **Chrome 扩展** 读取同一 session 的 cookies / tabs / localStorage，从而在后续 HTTP 抓取或自动化中复用登录态。

> 对比 `--remote-debugging-port`：CDP 端口通常只能用于 **按指定参数启动的 Chrome 实例**。当 Chrome 已在运行时，macOS 的 `open -a ... --args` 往往不会把参数注入到现有进程；因此 9222 方案不适合“复用我正在用的 Chrome”。

---

## 1) 配置（`runtime.toolset: local`）

在你的 `~/.alex/config.yaml`（或 `ALEX_CONFIG_PATH` 指向的配置文件）增加：

```yaml
runtime:
  toolset: "local"
  browser:
    connector: "chrome_extension"
    bridge_listen_addr: "127.0.0.1:17333"
    # bridge_token: "${ALEX_BROWSER_BRIDGE_TOKEN}" # 可选，建议设置
```

可选 env：

- `ALEX_BROWSER_BRIDGE_TOKEN`：bridge 鉴权 token（推荐设置；extension options 里填同一个）。

---

## 2) 安装扩展（Developer mode）

1. 打开 Chrome → `chrome://extensions/`
2. 右上角开启 **Developer mode**
3. 点击 **Load unpacked**
4. 选择目录：`tools/chrome-extension/elephant-bridge/`
5. 打开扩展的 **Details → Extension options**
   - Bridge URL：保持默认 `ws://127.0.0.1:17333/ws`
   - Token：可空；若你在配置里启用了 token，这里也填同一个

---

## 3) 启动 `alex` 并检查连接

启动你的 `alex`（CLI 或 server 均可；但需要 `toolset: local` 生效）。

然后调用工具：

- `browser_session_status`：应该显示 `connected=true`，并返回 tab 列表
- `browser_cookies(domain="xiaohongshu.com")`：应返回非空 `cookie_header`

若 `connected=false`：

- 确认 `alex` 正在运行，且 bridge 监听地址与扩展 options 中一致
- 确认没有被本机防火墙/代理拦截 localhost WebSocket
- 打开扩展 options 页面保持一会儿（MV3 service worker 可能会被挂起，options 会唤醒它并触发重连）

---

## 4) 用“日常 Chrome”登录 XHS（小红书）

1. 用你平时的 Chrome 打开 `https://www.xiaohongshu.com/` 并按页面流程完成登录（扫码/短信/验证码等）。
2. 如遇 461 / 风控验证页：只能按页面提示完成验证或换网络环境重试（本项目不提供绕过方案）。
3. 登录成功后，调用 `browser_cookies(domain="xiaohongshu.com")` 获取 cookie header，用于后续抓取/请求。

---

## 安全提示

- bridge **仅监听 127.0.0.1**；请不要改成 `0.0.0.0`。
- cookies / localStorage 可能包含会话凭证；推荐设置 `bridge_token` 并避免在日志中明文输出。
- 扩展的 host permissions 默认仅包含 `xiaohongshu.com` 与 localhost；不要随意改成 `<all_urls>`。

