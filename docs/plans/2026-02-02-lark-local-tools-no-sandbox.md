# Plan: Lark 模式本地工具替换 Sandbox + 工作区限制 + 自动附件上传

**Date:** 2026-02-02
**Status:** done
**Owner:** cklxx

## Goal
- Lark channel 不再注册/暴露 sandbox 工具。
- Lark agent 仅在受限工作区内使用本地工具（文件/搜索/执行/浏览器）。
- Web channel 继续使用 sandbox 工具。
- Lark 不暴露附件工具，但能自动上传新建/更新文件的附件。

## Scope
- Lark channel 配置与上下文注入（工作区/浏览器/附件策略）。
- Tool registry 按 tool mode 选择本地或 sandbox 实现，并做工具别名替换。
- 本地浏览器工具（Chrome/CDP）实现与注册。
- 本地文件写入/编辑的自动附件上传。
- 文档与默认配置更新。

## Non-goals
- 不改变 Web UI 的 sandbox 行为或 MCP 工具流程。
- 不改动 Lark 业务逻辑（消息收发、权限校验等）。
- 不做跨仓库或多机部署方案调整。

## Plan
1. **配置与 schema**
   - 为 `channels.lark` 增加：
     - `workspace_dir`（受限工作区根目录）
     - `auto_upload_files`（默认 true）
     - `auto_upload_max_bytes`（默认 2MB，可配置）
     - `auto_upload_allow_ext`（默认常见文档/图片扩展）
     - `browser` 子配置（`cdp_url`/`chrome_path`/`headless`/`user_data_dir`/`timeout_seconds`）
   - 更新 `internal/config/file_config.go`、`internal/config/types.go`、`internal/config/load.go`，并补充 YAML 示例。

2. **Lark 工作区限制注入**
   - 在 `internal/channels/lark/gateway.go` 中构造执行上下文时，调用 `pathutil.WithWorkingDir(execCtx, cfg.WorkspaceDir)`。
   - 统一做路径规范化，空值回退到进程 working dir。
   - 增加单测覆盖：Lark context 下 fileops/shell/code_execute 的路径必须落在 workspace。

3. **工具注册按 mode 分流**
   - 在 `toolregistry.Config` 增加 `ToolMode` 或 `Toolset` 字段（cli/web/lark-local）。
   - `registerBuiltins` 中按 mode 决定：
     - **web**：注册 sandbox 工具（browser_* / read_file / write_file / ...）。
     - **cli / lark-local**：不注册 sandbox 工具，改注册本地实现的同名工具。
   - 在 `internal/server/bootstrap/container.go` 与 `internal/server/bootstrap/lark_gateway.go` 中传递 tool mode 到 container builder。

4. **本地工具别名替换（sandbox 名称 → 本地实现）**
   - 新增 `internal/tools/builtin/aliases`（或同名适配器），为以下 sandbox 名称提供本地实现：
     - `read_file` → fileops `file_read`
     - `write_file` → fileops `file_write`
     - `list_dir` → fileops `list_files`
     - `search_file` → search `ripgrep` / fileops 搜索适配
     - `replace_in_file` → fileops `file_edit`
     - `shell_exec` → execution `bash`
     - `execute_code` → execution `code_execute`
   - 保持 tool definition/metadata 与 sandbox 名称一致，确保系统提示与 UI 格式化一致。

5. **本地浏览器工具（Chrome/CDP）**
   - 新增 `internal/tools/builtin/browser`：实现 `browser_action`/`browser_info`/`browser_screenshot`/`browser_dom`。
   - 复用 `sandbox_browser_dom.go` 的 step 解析/执行逻辑（提取到 shared helper），但使用本地 chromedp：
     - 优先连接 `cdp_url`；若为空则启动本地 Chrome（可配置 `chrome_path` / `headless`）。
   - `browser_screenshot` 返回 `ports.Attachment`，走现有 persister → Lark 上传链路。

6. **Lark 自动附件上传**
   - 为 Lark context 增加 `auto_upload_files` 标识（放在 appcontext 或 tool context）。
   - 在本地 `write_file`/`file_write`、`replace_in_file`/`file_edit` 成功后：
     - 若启用 auto_upload，则读取文件内容，生成 `attachment_mutations.add/update`。
     - 按 `auto_upload_max_bytes` 与扩展名白名单过滤。
   - 对 `shell_exec`/`execute_code` 提供 `output_files` 参数并支持上传（与 sandbox 语义一致）。

7. **Tool preset 调整**
   - 新增 `ToolPresetLarkLocal`（默认给 Lark），允许本地文件/浏览器工具，禁止 sandbox 工具。
   - 更新 `internal/agent/presets/tools.go` 与 Lark 默认配置。

8. **测试与文档**
   - 单测：tool registry 分流、本地别名工具调用、workspace 限制、附件自动上传。
   - 文档：更新 `docs/operations/SANDBOX_INTEGRATION.md`、Lark 配置说明、示例 YAML。
   - 运行 `./dev.sh lint`、`./dev.sh test`、`./dev.sh down && ./dev.sh`。

## Acceptance Criteria
- Lark channel 无 sandbox 工具；`browser_*` 等工具调用使用本地 Chrome。
- Lark 本地文件操作受 `workspace_dir` 限制。
- Lark 新建/更新文件可自动作为附件上传并发送。
- Web channel 保持 sandbox 工具不变。

## Assumptions
- 运行环境可访问本地 Chrome（或提供 `cdp_url`）。
- 附件存储已配置（本地或云端）以承接自动上传。
- Lark 默认 tool_mode 继续为 `cli`，仅 tool_preset 切换为 `lark-local`。

## Result
- Implemented in `94bece4d`.
