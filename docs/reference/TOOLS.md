# Built-in Tool Catalog
> Last updated: 2026-02-01

Source of truth: runtime registry in `internal/toolregistry/registry.go`.
This catalog lists **currently registered** builtin tools; availability is conditional for sandbox/LLM/media features.

---

## 分层（架构视角）

| 层级 | 目的 | 工具 |
| --- | --- | --- |
| L0 编排/协议 | 任务拆分、并行/后台执行、用户交互门控 | `plan`, `clarify`, `request_user`, `subagent`, `explore`, `bg_dispatch`, `bg_status`, `bg_collect`, `ext_reply`, `ext_merge` |
| L1 会话/知识 | 待办、技能、应用清单、记忆检索/读取 | `todo_read`, `todo_update`, `skills`, `apps`, `memory_search`, `memory_get` |
| L2 业务域 | OKR 读写 | `okr_read`, `okr_write` |
| L3 本地资源 | 本地文件/搜索 | `file_read`, `file_write`, `file_edit`, `list_files`, `grep`, `ripgrep`, `find` |
| L4 Web 获取 | Web 检索/抓取/编辑、渠道历史 | `web_search`, `web_fetch`, `html_edit`, `douyin_hot`, `lark_chat_history` |
| L5 Sandbox | 浏览器 + 隔离文件/执行 | `browser_action`, `browser_dom`, `browser_info`, `browser_screenshot`, `read_file`, `write_file`, `list_dir`, `search_file`, `replace_in_file`, `shell_exec`, `execute_code`, `write_attachment` |
| L6 执行/外部 | 本地执行与外部执行器 | `bash`, `code_execute`, `acp_executor` |
| L7 产物/媒体 | 产物与媒体生成 | `artifacts_write`, `artifacts_list`, `artifacts_delete`, `artifact_manifest`, `a2ui_emit`, `pptx_from_images`, `text_to_image`, `image_to_image`, `vision_analyze`, `video_generate`, `music_play` |

---

## 分类（功能视角）

- **编排/交互**：`plan`, `clarify`, `request_user`, `subagent`, `explore`, `bg_dispatch`, `bg_status`, `bg_collect`, `ext_reply`, `ext_merge`
- **会话/记忆**：`todo_read`, `todo_update`, `skills`, `apps`, `memory_search`, `memory_get`
- **业务域**：`okr_read`, `okr_write`
- **本地文件/搜索**：`file_read`, `file_write`, `file_edit`, `list_files`, `grep`, `ripgrep`, `find`
- **Web 获取**：`web_search`, `web_fetch`, `html_edit`, `douyin_hot`, `lark_chat_history`
- **Sandbox 浏览器/IO**：`browser_action`, `browser_dom`, `browser_info`, `browser_screenshot`, `read_file`, `write_file`, `list_dir`, `search_file`, `replace_in_file`, `shell_exec`, `execute_code`, `write_attachment`
- **执行/外部执行器**：`bash`, `code_execute`, `acp_executor`
- **产物与媒体**：`artifacts_write`, `artifacts_list`, `artifacts_delete`, `artifact_manifest`, `a2ui_emit`, `pptx_from_images`, `text_to_image`, `image_to_image`, `vision_analyze`, `video_generate`, `music_play`

---

## 条件可用性（运行时开关）

- **本地执行**：`bash`/`code_execute` 仅在本地执行开启时注册。
- **Sandbox**：`browser_*`、`read_file`/`write_file`/`list_dir`/`search_file`/`replace_in_file`、`shell_exec`、`execute_code`、`write_attachment` 依赖 sandbox 配置。
- **媒体模型**：`text_to_image`/`image_to_image`/`vision_analyze`/`video_generate` 依赖 Seedream 配置。
- **通道**：`lark_chat_history` 依赖 Lark 通道配置。

---

## 功能重叠表（主要重叠点）

| 重叠面 | 工具 | 核心差异 | 默认选择 |
| --- | --- | --- | --- |
| 文件读取 | `file_read` vs `read_file` | 本地仓库直读 vs sandbox 文件系统 | 本地环境优先 `file_read`；隔离环境用 `read_file` |
| 文件写入 | `file_write`/`file_edit` vs `write_file`/`replace_in_file` | 本地文件改动 vs sandbox 文件改动 | 本地改仓库用 `file_*`；sandbox 用 `write_file/replace_in_file` |
| 目录列举 | `list_files` vs `list_dir` | 本地目录 vs sandbox 目录 | 本地用 `list_files`；sandbox 用 `list_dir` |
| 文本/路径搜索 | `grep`/`ripgrep`/`find` vs `search_file` | 本地搜索 vs sandbox 搜索 | 本地用 `ripgrep/grep/find`；sandbox 用 `search_file` |
| Shell 执行 | `bash` vs `shell_exec` | 本地执行 vs sandbox 执行 | 本地用 `bash`；隔离环境用 `shell_exec` |
| 代码执行 | `code_execute` vs `execute_code` | 本地运行时 vs sandbox 运行时 | 本地用 `code_execute`；隔离环境用 `execute_code` |
| Web 获取 | `web_fetch` vs `browser_dom`/`browser_action` | 静态抓取 vs 需要 JS/交互 | 静态页面用 `web_fetch`；交互页面用 `browser_dom`/`browser_action` |
| 浏览器交互 | `browser_dom` vs `browser_action` | DOM 语义操作 vs 坐标操作 | 优先 `browser_dom`；坐标兜底用 `browser_action` |
| 图像理解 | `browser_screenshot` vs `vision_analyze` | 截图采集 vs 通用图像分析 | 先截屏，再 `vision_analyze` 深入理解 |
| 产物输出 | `artifacts_write` vs `write_attachment` | 本地生成文件 vs sandbox 生成附件 | 本地产物用 `artifacts_write`；sandbox 产物用 `write_attachment` |
| 历史/记忆 | `memory_search` vs `lark_chat_history` | Markdown 记忆检索 vs 通道聊天历史 | 优先 `memory_search`；需要原始记录时用 `lark_chat_history` |
| 发现/搜索 | `web_search` vs `douyin_hot` | 通用检索 vs 特定平台热点 | 通用检索用 `web_search`；内容热点用 `douyin_hot` |
