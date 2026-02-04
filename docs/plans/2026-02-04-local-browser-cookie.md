# Plan: 本机浏览器接管（复用 Cookie / 已登录态）

**Date:** 2026-02-04  
**Status:** in progress  
**Owner:** cklxx

## Goal
- 让 agent 在**同一台机器**上接管本地 Chromium（优先 Chrome，其次 Atlas），并复用用户已登录态（Cookie/Session）。
- 支持读写操作（导航/点击/输入/截图/DOM automation）。
- 不引入审批/限制（按 cklxx 当前需求），但保持默认安全姿势：仅本机接入、文档明确风险。

## Non-goals
- 不实现 Chrome Extension / openclaw 式 attach 到“当前运行且未开启远程调试”的 Chrome（后续可加）。
- 不做跨机器/远程浏览器接管（后续可加）。

## Approach
1. **Runtime 配置可选启用本地 toolset**
   - 新增 `runtime.toolset`（默认 `default`，可选 `lark-local` / `local`）。
   - 新增 `runtime.browser`（`cdp_url` / `chrome_path` / `headless` / `user_data_dir` / `timeout_seconds`）。
   - 允许 CLI/Web server 使用本地 `browser_*` 工具，而非 sandbox browser。

2. **CDP URL 友好解析**
   - `cdp_url` 支持传：
     - 直接 `ws://...`（webSocketDebuggerUrl）
     - 或 `http://127.0.0.1:9222`（DevTools HTTP endpoint），自动解析 `/json/version` 得到 websocket URL。

3. **本机启动脚本（Chrome / Atlas）**
   - 提供脚本启动 app 并开启 `--remote-debugging-port`，并输出可配置到 `cdp_url` 的地址。
   - 说明：要复用默认 Cookie，通常需要先退出已有 Chrome 实例再启动带远程调试的实例。

## Milestones
- [x] 配置 schema + loader + 文档（runtime.toolset/runtime.browser）
- [x] CDP URL 解析逻辑 + 单测
- [x] 启动脚本（Chrome/Atlas）+ 使用文档
- [ ] 全量 lint + test

## Acceptance Criteria
- 可在 `config.yaml` 里启用本地 toolset，并通过 `runtime.browser.cdp_url` 连接到本机 Chrome/Atlas 的远程调试端口。
- `browser_action/browser_dom/browser_screenshot/browser_info` 在 CLI/Web 模式可用并能操作已登录页面。
- 单测覆盖 `cdp_url` 解析分支；`./dev.sh lint` 与 `./dev.sh test` 通过。
