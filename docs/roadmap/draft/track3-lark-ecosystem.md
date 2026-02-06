# Track 3: 人类集成交互 — Lark 全生态 — OKR-First ROADMAP

> **Parent:** `docs/roadmap/roadmap-lark-native-proactive-assistant.md`
> **Owner:** cklxx
> **Created:** 2026-02-01
> **Last Updated:** 2026-02-06

> **NOTE (2026-02-06):** This draft was written before the implementation audit and is **partially outdated**.
> For the authoritative, up-to-date implementation status, see `docs/roadmap/roadmap.md` (tracks M0/M1/P0+).
>
> Quick status snapshot (2026-02-06):
> - ✅ Calendar/Tasks tools + registration: `internal/tools/builtin/larktools/` + `internal/toolregistry/registry.go`
> - ✅ Typed Lark API layer: `internal/lark/` (calendar/tasks/approval/cards)
> - ✅ Lark interactive cards + rich content: `internal/lark/cards/`, `internal/channels/lark/richcontent/`
> - ✅ Proactive group summary: `internal/lark/summary/`
> - ✅ Message type enrichment (posts, tables, Markdown): `internal/channels/lark/richcontent/`
> - ✅ Proactive scheduler reminder trigger + E2E: `internal/scheduler/`
> - ✅ Lark Approval API: `internal/lark/approval.go`
> - ❌ Deep Lark (Docs/Sheets/Wiki) remains pending: `internal/lark/docs/`, `internal/lark/sheets/`, `internal/lark/wiki/`

---

## Objective & Key Results (O3)

**Objective (O3):** Calendar + Tasks 在 Lark 内闭环，审批与权限可控，支撑 WTCR/TimeSaved 提升。

**Key Results:**
- **KR3.1** Calendar/Tasks 读写能力可用且稳定
- **KR3.2** 写操作全链路审批门禁 + 审计
- **KR3.3** 主动提醒/跟进形成闭环

---

## Roadmap (OKR-driven)

### M0: 日程+任务最小闭环
- Lark API Client 最小封装（认证/重试/错误映射）
- Calendar/Tasks 最小读写能力（查询/创建/更新）
- 写操作审批门禁（IM 交互确认 + 审计记录）

### M1: 主动闭环增强
- 任务/日程意图抽取 → 草案 → 用户确认
- Scheduler 驱动提醒/跟进
- Calendar/Tasks CRUD 完整化

### M2: 深度集成
- 多群联动/权限分级
- Calendar/Tasks 智能化（冲突预警、纪要联动）
- macOS Node Host MVP (D6)

### M3: 协作与治理
- 多用户协作
- 移动端/PWA
- 知识库自动治理

---

## Baseline & Gaps (Current State)

**关键路径：** `internal/channels/lark/` · `internal/lark/` (新增) · `internal/tools/builtin/larktools/` · `web/` · `cmd/alex/`

### 1. Lark IM 消息层

> `internal/channels/lark/`

**现状**
- WebSocket 事件循环、群聊历史、Emoji 进度、审批门禁、富附件、主动发消息 — 已实现

**Milestones (initiatives)**

#### M1: 主动消息交互

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 群聊消息自动感知 | 监听所有消息（不只 @mention），理解群聊动态 | ✅ 已实现 | `channels/lark/gateway.go` |
| 主动摘要 | 群聊长讨论后（按消息量/时间窗口触发）主动提供摘要 | ✅ 已实现 | `internal/lark/summary/` |
| 定时提醒 | 从对话中提取 deadline/约定，到期主动提醒 | ⚙️ 部分 | `internal/scheduler/` |
| 智能卡片交互 | Interactive Card 展示结构化结果 + 按钮操作（审批/选择/反馈） | ✅ 已实现 | `internal/lark/cards/` |
| 消息引用回复 | 引用特定消息回复，保持讨论上下文连贯 | ✅ 已实现 | `channels/lark/` |
| 消息类型丰富化 | 支持发送表格、代码块、Markdown 渲染消息 | ✅ 已实现 | `internal/channels/lark/richcontent/` |

---

### 2. Lark Open API 封装层（基础设施）

> 新增 `internal/lark/`

**Milestones (initiatives)**

#### M0: 基础封装

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| Lark API Client | 统一的 HTTP client，处理 tenant_access_token 刷新 | ✅ 已实现 | `internal/lark/client.go` |
| 认证管理 | App credentials 管理 + token 自动刷新 | ✅ 已实现 | `internal/lark/auth.go` |
| 限流与重试 | Lark API 限流感知 + 指数退避重试 | ✅ 已实现 | `internal/lark/ratelimit.go` |
| 错误码映射 | Lark 错误码 → 结构化错误 | ✅ 已实现 | `internal/lark/errors.go` |
| 权限声明 | 所需 Lark 权限的声明式管理 | ✅ 已实现 | `internal/lark/permissions.go` |

---

### 3. Lark 日历 (Calendar)

> 新增 `internal/lark/calendar/`

**Milestones (initiatives)**

#### M0: 日历操作

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 日程查询 | 查询用户日程（今天/本周/指定范围） | ✅ 已实现 | `lark/calendar/reader.go` |
| 日程创建 | 创建会议/事件，邀请参与者，设置提醒 | ✅ 已实现 | `lark/calendar/writer.go` |
| 日程修改/取消 | 修改时间/地点/参与者，或取消 | ✅ 已实现 | `lark/calendar/writer.go` |
| 空闲时间查找 | 查找多人共同可用时间段 | ✅ 已实现 | `lark/calendar/finder.go` |

#### M2: 日历智能

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 会议准备助手 | 会议前自动汇总相关文档、上次纪要、待办 | **Library done** | `internal/lark/calendar/meetingprep/` |
| 会议纪要自动生成 | 会议结束后主动生成并推送纪要 | **Skill done** | `skills/meeting-notes/` |
| 日程冲突预警 | 检测并提醒日程冲突 | ❌ 待实现 | `lark/calendar/` |
| 日程建议 | 基于历史模式建议会议时间 | **Library done** | `internal/lark/calendar/suggestions/` |

---

### 4. Lark 任务 (Tasks) 与审批 (Approval)

> 新增 `internal/lark/tasks/` · `internal/lark/approval/`

**Milestones (initiatives)**

#### M0: 任务管理

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 任务查询 | 查询待办/已完成任务列表 | ✅ 已实现 | `lark/tasks/reader.go` |
| 任务创建 | 创建任务 + 指派负责人 + 截止日期 | ✅ 已实现 | `lark/tasks/writer.go` |
| 任务更新 | 标记完成、修改截止日期、添加评论 | ✅ 已实现 | `lark/tasks/writer.go` |
| 逾期跟进 | 逾期/即将到期任务主动 Lark 提醒 | ✅ 已实现 | `lark/tasks/` |

#### M0: 审批流

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 审批查询 | 查询待审批/已审批记录 | ✅ 已实现 | `lark/approval/reader.go` |
| 发起审批 | 通过 Approval API 发起审批流程 | ✅ 已实现 | `lark/approval/writer.go` |
| 状态追踪 | 监听审批状态变更，通知发起人 | ✅ 已实现 | `lark/approval/tracker.go` |

---

### 5. Lark 文档生态 (Docs / Docx)

> 新增 `internal/lark/docs/`

**Milestones (initiatives)**

#### M1: 文档读取

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 文档内容读取 | 通过 Docx API 读取文档全文，富文本 → Markdown 转换 | ❌ 待实现 | `lark/docs/reader.go` |
| 文档元信息 | 标题、作者、创建/修改时间、版本、权限 | ❌ 待实现 | `lark/docs/meta.go` |
| 文档列表搜索 | 按关键词/文件夹/创建者搜索文档 | ❌ 待实现 | `lark/docs/search.go` |
| 文档评论读取 | 读取文档中的评论、批注、@提及 | ❌ 待实现 | `lark/docs/comments.go` |
| 文档块解析 | 解析 Docx Block 结构（段落/表格/图片/代码块） | ❌ 待实现 | `lark/docs/blocks.go` |

#### M1: 文档写入与修改

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 创建新文档 | 在指定文件夹创建 Lark Doc，支持初始内容 | ❌ 待实现 | `lark/docs/writer.go` |
| 追加内容 | 向已有文档追加段落/表格/图片/代码块 | ❌ 待实现 | `lark/docs/writer.go` |
| 定位修改 | 按 Block ID 定位到具体段落，替换/插入/删除 | ❌ 待实现 | `lark/docs/editor.go` |
| 添加评论 | 在文档指定位置添加评论/批注 | ❌ 待实现 | `lark/docs/comments.go` |
| 回复评论 | 对已有评论进行回复 | ❌ 待实现 | `lark/docs/comments.go` |
| 权限管理 | 设置/修改文档共享权限（查看/编辑/管理） | ❌ 待实现 | `lark/docs/permission.go` |

#### M2: 文档智能

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 文档摘要 | 长文档自动生成摘要 | ❌ 待实现 | `lark/docs/` |
| 版本差异对比 | 比较两个版本的内容差异 | ❌ 待实现 | `lark/docs/` |
| 模板填充 | 根据场景选择模板并自动填充 | ❌ 待实现 | `lark/docs/` |
| 联动更新 | A 文档变更后提醒/更新引用了 A 的 B 文档 | ❌ 待实现 | `lark/docs/` |

---

### 6. Lark 表格 (Sheets / Bitable)

> 新增 `internal/lark/sheets/` · `internal/lark/bitable/`

**Milestones (initiatives)**

#### M1: 表格读写

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| Sheets 读取 | 按范围读取单元格内容（支持格式/公式） | ❌ 待实现 | `lark/sheets/reader.go` |
| Sheets 写入 | 写入/更新单元格、追加行、设置格式 | ❌ 待实现 | `lark/sheets/writer.go` |
| Bitable 读取 | 读取多维表格记录（带筛选/排序/分页） | ❌ 待实现 | `lark/bitable/reader.go` |
| Bitable 写入 | 创建/更新/删除记录、添加字段 | ❌ 待实现 | `lark/bitable/writer.go` |
| 数据分析 | 对表格数据执行统计分析（求和/均值/分组） | ❌ 待实现 | `lark/sheets/analysis.go` |

#### M2: 表格自动化

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 定时数据同步 | 从外部数据源定时拉取更新到 Lark 表格 | ❌ 待实现 | `lark/sheets/` |
| 变更监听 | Webhook 监听表格数据变更，触发 Agent 处理 | ❌ 待实现 | `lark/sheets/` |
| 报表自动生成 | 定时汇总表格数据生成报表文档 | ❌ 待实现 | `lark/sheets/` |
| 图表生成 | 从表格数据生成可视化图表 | ❌ 待实现 | `lark/sheets/` |

---

### 7. Lark 知识库 (Wiki)

> 新增 `internal/lark/wiki/`

**Milestones (initiatives)**

#### M1: 知识库读写

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 空间浏览 | 列出知识库空间、节点树结构 | ❌ 待实现 | `lark/wiki/reader.go` |
| 内容读取 | 读取知识库文档全文 | ❌ 待实现 | `lark/wiki/reader.go` |
| 语义搜索 | 在知识库中语义搜索相关内容 | ❌ 待实现 | `lark/wiki/search.go` |
| 页面创建 | 在知识库中创建新页面 | ❌ 待实现 | `lark/wiki/writer.go` |
| 页面更新 | 修改已有页面内容 | ❌ 待实现 | `lark/wiki/writer.go` |

#### M2: 知识自动化

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 自动沉淀 | 从群聊讨论/会议纪要中提取知识，自动写入 Wiki | ❌ 待实现 | `lark/wiki/` |
| 过时检测 | 检测长期未更新/被引用的知识页面 | ❌ 待实现 | `lark/wiki/` |
| 自动归档 | 将过时内容归档到指定区域 | ❌ 待实现 | `lark/wiki/` |

---

### 8. Lark 统一工具封装

> `internal/tools/builtin/larktools/`

**Milestones (initiatives)**

#### M0/M1: 工具注册

| 工具名 | 能力 | 状态 |
|--------|------|------|
| `lark_send_message` | 发送消息到 Lark 群/私聊 | ✅ 已实现 |
| `lark_chat_history` | 获取群聊历史消息 | ✅ 已实现 |
| `lark_doc_read` | 读取 Lark 文档内容 | ❌ 待实现 |
| `lark_doc_write` | 创建/修改 Lark 文档 | ❌ 待实现 |
| `lark_doc_comment` | 添加/读取/回复文档评论 | ❌ 待实现 |
| `lark_sheet_read` | 读取电子表格/多维表格 | ❌ 待实现 |
| `lark_sheet_write` | 写入电子表格/多维表格 | ❌ 待实现 |
| `lark_wiki_search` | 搜索知识库内容 | ❌ 待实现 |
| `lark_wiki_write` | 创建/修改知识库页面 | ❌ 待实现 |
| `lark_calendar_query` | 查询日程安排 | ✅ 已实现 |
| `lark_calendar_create` | 创建日程/会议 | ✅ 已实现 |
| `lark_task_manage` | 创建/查询/更新任务 | ✅ 已实现 |
| `lark_approval_submit` | 发起审批流程 | ✅ 已实现 |

---

### 9. Web Dashboard

> `web/`

**现状**
- SSE 流式渲染、对话界面、附件/工具可视化、会话管理、成本追踪 — 全部已实现。

**Milestones (initiatives)**

#### M1: 增强体验

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 执行过程回放 | 逐步回放 Agent 执行过程 | ❌ 待实现 | `web/components/agent/` |
| 执行时间线 | 甘特图式展示工具执行时序 | ❌ 待实现 | `web/components/agent/` |
| 子 Agent 执行树 | 可视化多 Agent 委派关系 | ⚙️ 部分 | `web/components/agent/` |
| 用户纠错入口 | 中断 → 修改目标 → 重试 | ❌ 待实现 | `web/components/agent/` |

---

### 10. CLI 界面

> `cmd/alex/` · `internal/cli/`

**现状**
- TUI 交互、审批、会话持久化 — 全部已实现。

**Milestones (initiatives)**

#### M1: CLI 增强

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 管道模式 | stdin/stdout 管道，与 shell 工具链集成 | ❌ 待实现 | `cmd/alex/` |
| 后台守护 | CLI 作为后台服务运行 | ❌ 待实现 | `cmd/alex/` |
| 快捷指令 | 常用操作别名与快捷键 | ❌ 待实现 | `cmd/alex/` |

---

### 11. 跨交互面一致性

#### M1: 统一体验

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 跨面会话同步 | Lark/Web/CLI 无缝切换同一会话 | ❌ 待实现 | `internal/session/` |
| 统一通知中心 | 推送到用户偏好渠道 | ❌ 待实现 | `internal/notification/` |
| 跨面审批 | 任意面发起/完成审批 | ⚙️ 部分 | `internal/agent/domain/` |
| 统一附件管理 | 所有面共享附件存储 | ⚙️ 部分 | `internal/attachments/` |

---

### 12. macOS Companion App + 本地 Node Host (OpenClaw D6)

> 新增 `macos/` (SwiftUI) · `internal/tools/builtin/nodehost/` (Gateway 侧)

**Milestones (initiatives)**

#### M1: 协议与 Gateway 集成

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| Node Host API 契约 | `/api/v1/tools/{name}/execute` REST 协议 + 工具发现端点 + 权限查询端点 | ❌ 待实现 | `internal/tools/builtin/nodehost/` |
| Gateway proxy executor | 为每个 node host 工具创建 proxy executor，动态注册为 `node:` 前缀工具 | ❌ 待实现 | `nodehost/nodehost.go` |
| 动态注册/注销 | Gateway 启动时尝试连接 node host；断开时 auto-unregister；恢复后 auto-register | ❌ 待实现 | `nodehost/connector.go` |
| NodeHost 配置 | `configs/nodehost.yaml`：enabled/base_url/token_file/timeout/per-tool 开关 | ❌ 待实现 | `internal/config/file_config.go` |

#### M2: Companion App MVP

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| Menu bar 常驻 | SwiftUI AppDelegate，状态图标，全局 hotkey 唤起 | ❌ 待实现 | `macos/ElephantCompanion/` |
| Node Host Server | 本地 HTTP server (localhost:19820)，Bearer token 认证 | ❌ 待实现 | `macos/ElephantCompanion/NodeHost/` |
| TCC Permission Manager | 屏幕录制/麦克风/辅助功能/完全磁盘访问权限请求与状态管理 | ❌ 待实现 | `macos/ElephantCompanion/Permissions/` |
| 核心工具实现 | node:system_run + node:screen_capture（MVP 最小集） | ❌ 待实现 | `macos/ElephantCompanion/Tools/` |
| 权限状态上报 | `GET /api/v1/permissions` 暴露给 Gateway，权限缺失时返回引导错误 | ❌ 待实现 | `macos/ElephantCompanion/NodeHost/` |
| WebChat View | WKWebView 嵌入现有 Next.js Web 界面 | ❌ 待实现 | `macos/ElephantCompanion/Views/` |
| 降级策略 | node host 未运行时 node:* 不可用；权限被拒时返回引导；旧版检查 API version | ❌ 待实现 | `nodehost/connector.go` |

#### M2: 完整工具集

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| audio_record | 麦克风录音（需 TCC 权限） | ❌ 待实现 | `macos/ElephantCompanion/Tools/` |
| ui_automation | 键鼠模拟（需辅助功能权限） | ❌ 待实现 | `macos/ElephantCompanion/Tools/` |
| fs_read / fs_write | 本地文件读写（可选完全磁盘访问） | ❌ 待实现 | `macos/ElephantCompanion/Tools/` |
| open_app / clipboard | 打开应用 / 读写剪贴板 | ❌ 待实现 | `macos/ElephantCompanion/Tools/` |
| Notarization | Apple Developer Program 签名 + notarization 发布 | ❌ 待实现 | `macos/` |

---

## 进度追踪

| 日期 | 模块 | 更新 |
|------|------|------|
| 2026-02-01 | All | Track 3 详细 ROADMAP 创建。Lark IM / Web / CLI 基础已实现，核心缺口在 Lark 文档/表格/知识库/日历/任务/审批全生态接入。 |
| 2026-02-01 | Lark IM | 实现审计修正：群聊消息自动感知 ❌→✅（已支持非 @mention 监听）；消息引用回复 ❌→✅；定时提醒 ❌→⚙️（scheduler 基础实现）。 |
| 2026-02-01 | macOS | OpenClaw D6 集成：新增 §12 macOS Companion App + 本地 Node Host 章节。Gateway 侧协议定义 M1，Companion App MVP + 完整工具集 M2。 |
| 2026-02-01 | All | Roadmap 重构为 OKR-First，围绕 O3/KR3.* 重新组织章节并优先 Calendar+Tasks。 |
| 2026-02-06 | Lark IM/API | 智能卡片交互、主动摘要、消息丰富化标记 ✅; Lark API Client 基础封装 ✅; Calendar/Tasks/Approval M0 全部 ✅; 工具注册更新。 |
