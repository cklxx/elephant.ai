# Attachments & Material Registry Design

## 背景
- 现状：attachment catalog 分散在 runtime、工具和前端，缺乏中心化的“物料数据库”，导致难以追踪素材来源、状态与重用。
- 目标：设计一个覆盖所有工具的 Attachment/Material Registry，使所有产出（输入素材、工具中间产物、最终交付物）都带有结构化标签，可在全局范围内被检索、复用、治理。

## 业界最佳实践调研
1. **Dropbox 内容寻址 + 元数据服务**
   - 核心二层架构：大对象走块级内容寻址存储，metadata service 维护文件层次、权限、版本。
   - 启发：将附件二进制与 metadata 解耦，registry 负责逻辑层和属性，底层对象存储负责数据持久。
2. **Figma Artifact Registry**
   - 通过 node graph 描述组件/导出的依赖链；每个导出都有 `source_node`, `variant`, `render_target` 等属性。
   - 启发：记录产物间的派生关系，才能高效地做增量更新与可追溯性。
3. **Notion/Slack File Hub**
   - 每个文件都挂载在特定空间/会话，同时具备全局检索、跨空间引用能力；metadata 中保存 `space_id`, `message_id`, `context_tags`。
   - 启发：附件既要有本地上下文（哪次请求、哪条消息），也要具备全局唯一 key 供跨任务访问。
4. **GitHub Actions Artifact Service**
   - 明确标记 artifact scope（workflow run, job, step）与 retention policy；提供 REST API 查询与下载。
   - 启发：附件生命周期需绑定保留策略、访问控制，以及 API 供自动化流程拉取。

## GitHub Actions & Artifact Service 解读
### GitHub Actions 是什么？
- GitHub Actions 是 GitHub 提供的 **事件驱动 CI/CD 编排平台**：当仓库发生 push/PR/schedule 等事件后，它会按 workflow yaml 描述依次执行 `workflow run → jobs → steps`。
- 每层都会注入丰富的上下文（`workflow`, `run_id`, `job`, `step`, `actor`, `matrix` 等），并提供日志、环境变量、机密、缓存、artifact 等配套能力。
- 我们借鉴的是“**为每次执行提供结构化上下文**”这一理念：Material Registry 里的 `RequestContext` 字段与 GitHub Actions 的 run/job/step 对应，确保任何附件都能被定位到具体的一次请求/迭代/工具调用，而非复刻整个平台。

### Artifact Service 能力 vs 我们的需求
Artifact Service 是 GitHub Actions 中用来在 workflow 各 step 之间传递产物的托管存储。它适合“上传若干文件，设置保留期，供同一 workflow 下载”这一场景，但仍存在明显差距：

| 能力 | Artifact Service | Material Registry 需求 |
| --- | --- | --- |
| 上下文绑定 | 绑定 `workflow run` 与 `job`，无法表达聊天/agent 迭代、工具调用 ID | 需要精确到 `request_id + iteration + tool_call` 并允许跨任务检索 |
| 数据类型 | 主要面向构建产物（tar/zip/logs），缺乏多模态语义 | 需支持图片、音频、矢量、嵌入、HTML 片段等，并附带语义/合规标签 |
| Lineage | 不记录产物之间的派生关系 | 需要 `LineageEdge` 追踪“由哪个工具/参数生成” |
| 事件/实时性 | 只提供拉取接口，不推送事件 | 需要 Event Bus 让 runtime/前端即时感知新附件 |
| 权限模型 | 依赖仓库/Actions 权限 | 需要细粒度的 runtime/tool/user 级 ACL，含临时签名 URL |
| 全局复用 | 主要在单个 workflow 内部共享 | 要求全局物料数据库，可复用旧任务素材、执行全局搜索 |

结论：Artifact Service 解决了“文件上传 + 保留策略”的一部分需求，但在上下文、检索、事件、权限等方面无法满足 agent 体系的复杂性。因此我们只借用其 retention / API 设计思路，把旧附件逻辑统一整合进 Material Registry、Attachment Broker、Storage Mapper 这套栈中。

### 如何把旧逻辑整合进新栈
1. **上下文映射**：
   - 旧有的 `TaskState.Attachments` 中记录的 `iteration`、`tool_name` 映射为 `RequestContext.agent_iteration` 与 `tool_call_id`。
   - 用户消息/工具消息的 `message_id` 转成 `RequestContext.conversation_id`，类似于 GitHub Actions 的 `run_id`。
2. **上传/分发统一化**：
   - 历史工具中直接返回 base64 / data URI 的部分，由 `AttachmentBroker` 拦截并调用 `Storage Mapper`，实现与 CDN 的统一上传分发策略（见下节）。
   - Registry 写入 `material_id` 后，通过事件告知 runtime/前端，取代之前散落的“手动插入 Markdown 链接”流程。
3. **API 兼容层**：
   - 为历史工具暴露一个与 Artifact Service 接口相似的 `UploadLegacyArtifact(request, files[])`，内部调用 Registry API，从而在迁移期保持最小改动。
   - 待工具全部迁移完毕后，逐步弃用旧接口，只保留 Material Registry 的 `Register/List`。

## 总体方案
```
┌───────────────────────────────────────────┐
│ Material Registry Service                │
│  ├─ Catalog API (query/mutate)           │
│  ├─ Policy Engine                        │
│  ├─ Event Stream (Kafka/Webhook)         │
│  └─ Storage Mapper (Object Store, CDN)   │
└───────────────────────────────────────────┘
```
- Registry 作为唯一真相源（SSOT），负责：
  1. 记录全局物料实体、关联关系、上下文。
  2. 提供面向 runtime/工具/前端的标准 API 和订阅流。
  3. 统一执行权限、保留、审计策略。

## 静态存储 & CDN 上传分发
- **统一的 Storage Mapper**：所有附件二进制统一落在静态对象存储（S3/GCS/OSS），Registry 只保存 `storage_key` 与 `content_hash`；`Storage Mapper` 负责把 `RegisterMaterials` 中的内联 `bytes_b64` 上传到对象存储后再写 Catalog，从而复用现有的 `registerMessageAttachments` 上传逻辑。
- **CDN 分发**：上传成功后触发 CDN 刷新或预热，生成 `cdn_public_url` 字段供前端直接访问；对敏感物料可生成带签名的短期 URL，沿用旧流程中的临时 token 机制。
- **旧逻辑整合**：
  - 现有 runtime 中的 `TaskState.Attachments`、`PendingUserAttachments`、`AttachmentIterations` 全部改为 Registry 的缓存层，只保存 `material_id` 与 placeholder；二进制上传和下载统一走 Storage Mapper/CDN。
  - `ensureToolAttachmentReferences`、`ensureAttachmentPlaceholders` 等旧函数不再直接拼 data URI，而是调用 Registry Hydrator 获取 CDN URL 并拼装 Markdown，占位符格式保持兼容，便于逐步替换。
  - Browser/Code Executor 等工具原先直接回传 base64 的逻辑迁移到 `AttachmentBroker`，由 Broker 将内存 buffer 上传到静态存储并返回 `material_id`，保证所有历史工具无需感知存储细节。
- **回源策略**：CDN 命中失败时回源对象存储；对大附件可在 CDN 层配置 Range 请求，并在 Registry 中记录 `cdn_profile` 方便动态选择（视频/图片走不同分发策略）。
- **监控与追踪**：Storage Mapper 输出标准事件（上传成功/失败、CDN 刷新状态），写入 `SystemAttributes`，旧监控面板可以直接订阅这些事件，避免割裂。

## Material Event Bus & 实时同步
- **事件模型**：为 registry 引入 `MaterialEvent` 推送通道（`RegisterMaterials` 后产生 `material` 事件，清理/撤销时推送 `tombstone`），以 request_id 为分区键，保证同一任务/会话内的顺序性。
- **实现**：`internal/materials/events` 提供内存版 `Bus`，在 `Watch(request_id)` 时返回一个 buffered channel，`AttachmentBroker` 完成注册后会调用 `PublishMaterial`，从而把新产物广播给 runtime & 前端。Bus 负责 watcher 生命周期管理（ctx cancel 时关闭 channel）以及对慢消费者的丢弃策略，方便后续替换为 Kafka/NATS。
- **客户端对接**：Runtime 订阅 Bus，刷新本地 TaskState；Web/CLI 复用现有 `attachmentRegistry`，通过 SSE/Websocket 接入 Watch 流，即时展示“新截图/HTML 已到达”的提示。
- **兼容旧链路**：在迁移期内，旧的工具仍可通过 Broker 上传产物，Bus 会在注册成功后发出事件，让前端能够继续感知 attachment 更新，避免“只在最终回答中看到附件”的倒退。

## Lineage & SystemAttributes 持久化实现
- **Schema 设计**：新增 `migrations/materials/001_init.sql`，包含：
  - `materials` 主表：行键为 `material_id`，列覆盖 descriptor、上下文、存储、`system_attributes JSONB`。`tags`、`annotations`、`system_attributes` 均建立 GIN 索引，`request_id + agent_iteration`、`content_hash` 建 BTREE，保证全局检索与去重。
  - `material_lineage`：`parent_material_id + child_material_id` 复合主键，写入 `derivation_type`、`parameters_hash`，并建立 `child_material_id` 索引便于向后/向前追踪。
  - `material_access_bindings`（预留）：与 `materials` 通过外键关联，为后续 ACL 存储打基础。
- **Go Store 实现**：`internal/materials/store/postgres` 暴露 `Store.InsertMaterials`，把 `AttachmentBroker` / 控制面传入的 `store.MaterialRecord` 写入上述 schema，逻辑包含：
  - 校验 descriptor/storage 完整性，将 `tags/annotations/system_attributes` 统一编码为 JSONB。
  - 使用 `ON CONFLICT(material_id)` upsert，保证重复注册幂等且不会破坏 lineage。
  - 循环写入 `material_lineage`，对 parent-child 组合同样 upsert，确保派生链自动更新。
- **SystemAttributes 语义**：字段包括 `domain_tags`、`compliance_tags`、`embeddings_ref`、`vector_index_key`、`extra` map，运行时和治理服务可把风险标签、向量索引引用写入其中，实现“物料数据库 + 治理索引”合一。

## 数据模型
| 实体 | 关键字段 | 说明 |
| --- | --- | --- |
| **Material** | `material_id (ULID)`, `global_key`, `hash`, `mime_type`, `size`, `storage_key`, `status` (`input/intermediate/final`), `retention_ttl`, `visibility` | 物料主体，`global_key` 用 `request_id:iteration:ordinal` 或内容哈希构建，保证全局唯一。 |
| **RequestContext** | `request_id`, `task_id`, `agent_iteration`, `tool_call_id`, `user_id`, `conversation_id` | 描述物料所属的请求/会话。 |
| **LineageEdge** | `parent_material_id`, `child_material_id`, `derivation_type (transform/crop/summary)`, `tool_name`, `parameters_hash` | 记录素材之间的派生关系，便于追溯与缓存。 |
| **SystemAttributes** | `domain_tags`, `compliance_tags`, `embeddings_ref`, `vector_index_key` | 供搜索与治理使用的扩展属性。 |
| **AccessBinding** | `material_id`, `principal`, `scope (runtime/tool/user)`, `capability (read/write/delete)`, `expires_at` | 访问控制条目。 |

## 关键能力
1. **注册与引用**
   - 工具/agent 通过 `RegisterMaterials` API 批量上报附件：
     ```json
     {
       "context": {"request_id": "req_123", "tool_call_id": "tc_5", "agent_iteration": 7},
       "materials": [
         {"name": "browser_example_com.png", "mime_type": "image/png", "bytes_b64": "...", "origin": "tool_output", "status": "intermediate"}
       ]
     }
     ```
   - Registry 生成 `material_id` 与 placeholder `[material:browser_example_com.png]`，并推送事件到 Attachment Event Bus。

2. **查询/快照**
   - Runtime 在执行工具或生成回答前调用 `ListMaterials(request_id, up_to_iteration)`，获得包含 placeholder、storage URI、lineage 的快照。
   - 对前端提供 `GET /materials/{material_id}` 与 `GET /requests/{request_id}/materials` 便于展示。

3. **占位符解析与 Hydration**
   - 引入 `AttachmentBroker`（见上一轮设计）读取 registry 快照，将 `[material:xxx]` 替换为工具可消费的 URI/临时文件。
   - 模型输出校验 `material_id` 是否存在，避免悬空引用。

4. **生命周期与策略**
   - 每个 material 附带 `retention_ttl`（默认 30 天），Policy Engine 根据 `status`（最终交付永久、中间产物短期）、`compliance_tags` 执行归档或删除。
   - 审计日志记录 `who did what to which material`，满足合规。

5. **索引与检索**
   - 将关键属性（`name`, `tags`, `request_id`, `hash`）写入可扩展的索引（Postgres + Elastic/Typesense）。
   - 嵌入式内容（OCR/文本描述）写入向量库，使 agent 可以按语义搜索已有物料。

## 系统集成
1. **Runtime**
   - 启动时订阅 registry 事件，保持 TaskState 附件与 registry 一致。
   - 每个工具调用前后，通过 Broker 自动调用 `RegisterMaterials` / `ListMaterials`。
   - 兼容旧逻辑：保留对历史任务的回溯能力，Runtime 若发现 `legacy_attachment` 结构则调用迁移适配器将其上传到静态存储并回写 `material_id`，确保一次上线即可覆盖所有会话。
2. **工具生态**
   - Tool descriptor 增加 `produces_material_types`, `consumes_material_types`，用于运行时检查。`ports.ToolDefinition` 与 `ToolMetadata` 已新增 `material_capabilities` 字段，LLM 描述与运行时元数据会同步携带“可消费 / 可产出”的 MIME 列表。
   - Browser/WebFetch/Seedream（文生图、图生图、视频、视觉分析）等内建工具现已声明对应的 `material_capabilities`，在注册时即可知道哪些工具会写入图片/视频，哪些工具需要读取历史图片作为输入。
   - 标准库工具（browser, code executor, image generator）迁移到 registry API，并通过 Broker 自动使用 CDN URL。
3. **前端/CLI**
   - UI 读取 `material_id` 与 placeholder，点击即可下载；支持过滤 `status`, `tool_name`, `iteration`。
   - 当最终回答包含 `[material:foo.png]` 时，客户端可直接命中 registry 缓存或 CDN URL，避免前端重复下载。

## 安全/合规 & AccessBinding 策略
- **AccessBinding Schema**：每个 material 可携带多条 `AccessBinding` 记录（`principal`, `scope`, `capability`, `expires_at`）。`principal` 表达主体（如 `runtime:agent`, `tool:browser`, `user:123`），`scope` 标记上下文（`request`, `workspace`, `global`），`capability` 枚举 `read/write/delete/share`。这些绑定在注册阶段直接落入 `material_access_bindings` 表，后续所有读取都要经过绑定校验。
- **Token 签发与 CDN 访问**：Registry/AttachmentBroker 会根据 AccessBinding 生成短期访问 token（JWT/HMAC），拼入 CDN 签名 URL 或 API 响应 header。运行时/前端收到 token 后在访问 CDN/Storage 时附带，存储网关验证 token 中的 `material_id` 与 `capability` 是否匹配，防止 URL 被滥用。对共享/公共物料可以生成长时效 token，其它情况默认 15 分钟。
- **加密策略**：Storage Mapper 上传对象时默认启用 KMS 客户端加密（SSE-KMS），并把 `kms_key_id` 写入 `SystemAttributes.extra`，方便审计。Registry 将 AccessBinding、SystemAttributes 等敏感 JSON 字段存储在 Postgres JSONB 中并开启列级透明加密；传输层统一走 mTLS。
- **治理钩子**：当绑定过期或被撤销时，Registry 发布 `tombstone` 事件提醒前端删除缓存。Policy Engine 也会扫描 `AccessBinding` + `system_attributes.compliance_tags`，自动触发额外审批（例如 `pii` 标签必须绑定 `user:*` 而非 `public`）。

## Retention & Cleanup Automation
- **Retention 字段落地**：`proto`、Go API 与 Postgres `materials` 表新增 `retention_ttl_seconds` 字段，`AttachmentBroker` 根据物料状态（输入 30 天、中间产物 7 天、最终产物永久）或调用方传入的覆盖值写入 TTL。Registry 的行记录因此具备了“何时可清理”的确定性数据源。
- **Janitor 任务**：`internal/materials/cleanup.Janitor` 封装清理逻辑，周期性地调用 `Store.DeleteExpiredMaterials`，批量删除 TTL 过期且状态在 allowlist 内的物料，并在成功删除后：
  - 调用 `storage.Mapper.Delete/Refresh` 清理静态存储对象并刷新 CDN；
  - 通过 `events.Bus.PublishTombstone` 广播删除事件，让 runtime/前端同步移除附件占位符。
- **数据库支持**：Postgres Store 在删除时使用 `WITH expired ... FOR UPDATE SKIP LOCKED` 保证多实例安全，`DeleteExpiredMaterials` 同时返回 `material_id/request_id/storage_key` 以驱动存储回收与事件通知。

## Legacy Attachment Migration Adapter
- **BrokerMigrator 适配层**：`internal/materials/legacy` 引入 `BrokerMigrator`，在 runtime 侧检测旧格式附件（base64/data URI/缺少 URI），通过 AttachmentBroker 上传到静态存储/CDN，并把 CDN URI 与 `[material:...]` 占位符回写给调用者。
- **ReactEngine 接入**：`ReactEngine` 新增 `AttachmentMigrator` 依赖，用户上传和工具产出都会在写入 `TaskState` 前调用 `legacy.Migrator.Normalize`，因此旧链路也会透明获得 CDN URI、Registry 记录与占位符校验，无需改动每个工具。
- **上下文绑定**：迁移请求会注入 `RequestContext`（task_id、iteration、tool_call_id），确保所有导入的历史附件同样具备 lineage/上下文，可被 registry/清理任务识别。

## CDN Upload/Prewarm/Refresh Monitoring
- **Mapper 扩展**：`storage.Mapper` 新增 `Delete/Prewarm/Refresh`，所有上传后的对象都会先预热，再在清理时刷新 CDN。InMemory 实现补全了新方法用于测试。
- **Observed & Retrying Mapper**：
  - `storage.ObservedMapper` + `PrometheusObserver` 在上传/删除/预热/刷新时记录延迟、错误次数与总上传字节，为监控面板提供指标。
  - `storage.RetryingMapper` 使用指数回退自动重试 delete/prewarm/refresh，满足“失败重试任务”要求，避免 CDN 抖动导致附件状态不一致。
- **Broker/Janitor 集成**：AttachmentBroker 上传完毕后立即调用 `Prewarm`，Janitor 删除时顺序执行 `Delete → Refresh` 并把失败冒泡，保证 CDN 状态与 Registry 一致。

## TODO
- [x] 定义 `Material Registry Service` 的 gRPC/REST proto，覆盖注册、查询、订阅接口。
- [x] 落地 `AttachmentBroker`，让 runtime 以中间件形式统一调用 registry。
- [x] 设计并实现 `LineageEdge` & `SystemAttributes` 的存储方案（建议 Postgres + JSONB + 索引）。
- [x] 接入事件总线（Kafka/NATS）并扩展前端 `attachmentRegistry` 监听 material 事件。
- [x] 梳理安全/合规需求：AccessBinding schema、token 签发、数据加密策略。
- [x] 建立自动化清理任务，按 `status` + `retention_ttl` 清理中间产物。
- [x] 把旧的 attachment 上传/渲染链路全部迁移到静态存储 + CDN，通过迁移适配器确保历史任务与新任务一致。
- [x] 为 CDN 上传/预热/刷新建立监控面板和失败重试任务，纳入 Storage Mapper 统一管理。

### TODO 进展
- `proto/materials/v1/material_registry.proto` + `internal/materials/api` 提供统一的消息定义，为未来的 gRPC/REST 服务与生成代码打下基础。
- `internal/materials/broker` 引入可测试的 `AttachmentBroker`，调用 Storage Mapper → Registry Client 的标准链路，把工具的 base64/data URI 产物转成 CDN URL。
- `internal/materials/storage` 暂时提供 `Mapper` 接口与 `InMemoryMapper`，方便 runtime 或测试构造上传逻辑，后续可替换为 S3/GCS 实现。
- `internal/materials/store/postgres` + `migrations/materials/001_init.sql` 为 Material Registry 的持久层落地了 Postgres schema：`materials` 表保存 descriptor/上下文/存储字段，`material_lineage` 记录 parent-child 派生链，`system_attributes`/`tags` 列使用 JSONB + GIN 索引，便于在全球范围内按 domain/compliance/tag 做检索。
- `internal/materials/events` + AttachmentBroker EventPublisher 负责把注册成功的物料广播给 runtime/前端；Bus 支持 Watch/Publish/Tombstone，作为未来 Kafka/NATS 的内存实现基线。
- Registry 层新增 AccessBinding 落库逻辑，AttachmentBroker/Store 能够把 `principal/scope/capability` 元数据和过期时间一起写入 `material_access_bindings`，并由 Policy Engine 结合 SystemAttributes 执行治理校验。
- `internal/materials/cleanup` + Postgres Store 的 `DeleteExpiredMaterials` 实现了按状态+TTL 的后台清理，并向 storage mapper/事件总线广播 tombstone。
- `internal/materials/legacy` 的 `BrokerMigrator` 让 ReactEngine 在注册用户附件、工具附件前自动落库并替换成 CDN URI，旧链路无需改动。
- `internal/materials/storage` 的 `ObservedMapper`/`RetryingMapper` 和 Prometheus 观测器提供上传/预热/刷新/删除的指标与重试机制，配合 Janitor/AttachmentBroker 覆盖整个 CDN 生命周期。

- `internal/agent/ports` 新增 `ToolMaterialCapabilities`，Browser/WebFetch/Seedream 等会读写附件的工具均在 `ToolDefinition` / `ToolMetadata`
  中声明 `consumes` / `produces` MIME 列表，使 registry、LLM 及策略引擎能够识别其物料读写范围。

