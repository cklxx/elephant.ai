# Artifact System Design

## 背景
- 现有 Attachment/Material Registry 已经统一了上传、事件与治理，但针对高价值交付物（PPT、HTML、Markdown 等）仍缺乏明确的“Artifact”概念，导致它们与一次性附件生命周期混杂，难以在前端做富预览与跨任务复用。
- Artifact 系统的目标是在 Attachment 能力之上，补齐 **多格式富预览**、**占位符渲染**、**长生命周期治理** 与 **lineage 追踪**，让 PPT/HTML/MD 这类文档能在客户端直接浏览、被 pin/共享并具备上下文。

## 业界最佳实践
1. **Dropbox & GitHub Artifact**：大文件与 metadata 解耦、保留策略+TTL、事件驱动同步是构建统一素材库的关键。
2. **Figma Artifact Lineage**：使用 node graph 记录派生关系，支撑多视图预览与版本 diff，对我们实现 preview_assets/preview_profile 有直接启发。
3. **Notion/Slack File Hub**：上下文标签 + 全局检索 + inline preview，证明“占位符 + 富预览”是提升协作体验的核心。
4. **浏览器/云文档渲染服务**：HTML iframe sandbox、Markdown 渲染缓存、PPT→PNG/PDF 等转换都需要异步作业与 lineage 关联，避免重复生成。

## 目标能力
- **多格式 Artifact**：支持 `ppt/pptx`, `html`, `markdown`, `pdf`, `csv` 等格式，统一通过 `format` 字段描述，同时保留底层 `mime_type`。
- **占位符 & 富预览**：新增 `MaterialKind` 与 `preview_profile/preview_assets`，让前端能够决定以分页图片、iframe 或 Markdown 渲染展示 Artifact；占位符如 `[artifact:quarterly-plan]` 在回答中可直接渲染。
- **生命周期治理**：Artifact 默认更长 TTL，可被 pin/unpin、共享到 workspace/global scope。系统会区分 attachment vs artifact，从而应用不同清理、限流策略。
- **Lineage & 追溯**：Artifact 上传与转换（例如 PPT→PNG、HTML→截图）都会写入 `material_lineage`，借助 `preview_assets` 描述派生的预览资产，实现“原件 + 预览”协同管理。

## 技术方案
### 数据模型扩展
- Proto/Go API 新增 `MaterialKind`（`attachment`/`artifact`）、`format`、`preview_profile`、`PreviewAsset`（label、mime、cdn_url、preview_type），并在 Postgres schema 中增加对应列。
- `AttachmentBroker` 默认写入 `attachment` kind；PPT/HTML/MD Artifact 上传路径则可以显式设置 `artifact` kind 与格式，Broker/工具根据 MIME 推导 format。
- `preview_assets` 存储 JSON 数组（面向前端），内容来源于离线转换 job 的输出，如 PPT 每页图片、HTML 截图等。

### 存储与派生
1. 上传链路不变，仍由 Storage Mapper 落在对象存储/CDN。
2. ArtifactProcessor 根据 MIME 调度转换：
   - PPT/PPTX：调用 headless office → PNG/PDF，写入 `preview_assets` 并将派生物料挂在 `material_lineage`。
   - HTML：生成 DOM 压缩包 + 截图，iframe 渲染时从 `preview_profile=html.sandbox` 读取 CSP。
   - Markdown：直接渲染成 HTML 缓存，`preview_profile=markdown.document`。
3. 所有派生结果记录在 `preview_assets` + `material_lineage`，便于前端根据 profile 选择展示方式。

### 事件与前端渲染
- `MaterialEvent` 沿用现有结构但携带新的 descriptor 字段；Web `attachmentRegistry` 根据 `kind/format/preview_profile` 判定渲染：
  - `artifact+pptx` → DocumentCanvas 分页图 + “下载原件”。
  - `artifact+html` → iframe sandbox/源码 tab。
  - `artifact+markdown` → Markdown renderer + toc。
- SSE 事件在 Artifact 注册或 preview 更新时推送，placeholder `[artifact:name]` 解析由 `parseContentSegments` 扩展。

### 权限与生命周期
- AccessBinding 支持 workspace/global scope，Artifact 默认 TTL 90 天，可 pin 为 `retention_ttl_seconds=0`（永久）。
- Policy Engine 根据 `kind=artifact` 应用更严格的审批/合规检查，Janitor Tombstone 事件携带 `kind` 方便前端清晰提示。

## TODO
1. **数据模型**
   - [x] proto/materials 与 Go API 添加 `MaterialKind`、`format`、`preview_profile`、`preview_assets`。
   - [x] Registry service/Store/CLI/Front-end 读取新字段，确保写入/查询/事件链路可见。
2. **存储 & 派生**
   - [x] 新增 ArtifactProcessor，按 MIME 触发 PPT/HTML/MD（目前已覆盖 HTML/Markdown）的预览生成作业，并写回 `preview_assets`。
   - [x] Storage Mapper 扩展多副本/预热策略，保证派生 HTML/Markdown 预览在 CDN 上的首次打开体验。
3. **前端渲染**
   - [x] `web/lib/attachments.ts` & DocumentCanvas 支持 `artifact` 占位符与多格式预览，添加分页、全屏、下载等交互。
   - [x] Task 回答/历史面板按照 `kind`/`format` 过滤、搜索。
4. **生命周期治理**
   - [x] Policy Engine/Janitor 根据 `kind=artifact` 选择 TTL/清理策略，支持 pin/unpin 及审计日志。
5. **工具链接入**
   - [x] 工具 descriptor 声明可产出 Artifact 的格式；Task Input UI 提供“上传为 Artifact”选项并写入 longer TTL。

Browser/WebFetch 等工具的 `material_capabilities` 已新增 `produces_artifacts`，明确它们会生成 `html/markdown` 等长生命周期文档。前端 Task Input 允许用户在上传文件后切换“附件/Artifact”模式，并在选择 Artifact 时自动写入 90 天 TTL，确保注册入库时带上正确的 kind/format 元数据。后台 ArtifactProcessor 已经在注册阶段为 HTML/Markdown 自动生成富预览：HTML 直接引用上传文件的 CDN URL，Markdown 渲染为主题化 HTML、重新上传并在 Storage Mapper 中预热，保证 DocumentCanvas 与 ArtifactPreviewCard 能秒开预览。

通过以上步骤，Artifact 系统将在 Attachment Registry 之上提供统一的多格式富预览体验，支持 PPT/HTML/Markdown 等高价值产物的占位符展示、点击浏览、跨任务引用与治理。

生命周期治理部分现已落地：`internal/materials/policy` 提供统一的 Policy Engine，暴露 `PinMaterial`/`UnpinMaterial` 能力以便 CLI/后台任务将 Artifact 设为永久或恢复默认 TTL，所有操作都会写入审计日志（可复用内存 logger 或 slog 输出）。Postgres store 新增 `UpdateRetention` API，Janitor 在 `internal/materials/policy` 覆盖的 mock 数据 e2e 测试中能够识别被 pin 的 Artifact、在解锁后再次删除，并持续推送 tombstone 事件与存储清理，确保整个生命周期策略被端到端验证。
