# 附件系统全链路分析与鲁棒性重构方案

**Date**: 2026-01-31
**Status**: Implemented (Batches 1-4 complete)
**Author**: cklxx

---

## 1. 现状全链路分析

### 1.1 附件的双重角色

附件系统承担两个关键职责:

**职责 A — 内容交付**: 将 LLM/工具产出的文件(图片、文档、代码)交付给用户(Web/Lark/WeChat/CLI)。

**职责 B — 上下文卸载 (Context Offload)**: 将大块内容从 LLM 消息历史中抽离到外部存储,仅保留轻量引用,从而控制 context window 大小,降低 token 消耗。

```
                附件的双重角色
                =============

  LLM Context                    外部存储
  ┌──────────┐                   ┌──────────┐
  │ Message  │   offload         │ Store    │
  │ History  │ ──────────────►   │ (FS/CDN) │
  │          │   placeholder     │          │
  │ [ref.md] │ ◄──────────────   │ content  │
  └──────────┘   reference       └──────────┘
       │                              │
       ▼                              ▼
  Token Budget                  Content Delivery
  Controlled                    (SSE/Lark/WeChat)
```

### 1.2 附件生命周期总览

```
用户上传 / 工具生成
    ↓
TaskState.Attachments (base64 Data / data URI)    ← 问题: 内容仍在内存
    ↓
finalize() → collectAllToolGeneratedAttachments()
    ↓
decorateFinalResult() → merge A2UI attachments
    ↓
WorkflowResultFinalEvent { Attachments: map[string]ports.Attachment }
    ├─→ SSE Path:  normalizeAttachmentPayload() → CDN URL → 前端
    ├─→ Lark Path: ResolveAttachmentBytes() → 解码 base64 → uploadImage/uploadFile → Lark API
    └─→ WeChat:    ❌ 完全未实现
```

### 1.3 当前上下文卸载机制 (分层)

系统已有多层卸载机制来控制 context window,但附件层存在漏洞:

| 层级 | 机制 | 文件 | 效果 |
|------|------|------|------|
| **L1: 工具参数压缩** | `compactToolCallArguments()` — 将 >256 字符的参数替换为 `{content_len, content_sha256, content_ref}` | `react/tool_args.go:154` | ✅ 历史消息中的工具参数被压缩 |
| **L2: 工具结果摘要** | `summarizeToolResultForWorld()` — 工具输出截取 280 字符预览 | `react/world.go:20` | ✅ WorldState 中只保留摘要 |
| **L3: Thinking 卸载** | `offloadMessageThinking()` — 每轮新输入时清空历史消息的 Thinking 字段 | `react/prepare_context.go:61` | ✅ 扩展推理不累积 |
| **L4: 历史压缩** | `AutoCompact()` — 超过 80% token 预算时压缩为摘要 | `context/manager_compress.go:32` | ✅ 全局兜底 |
| **L5: 附件目录注入** | `buildAttachmentCatalogContent()` — 仅向 LLM 展示名称+描述索引 | `react/attachments.go:394` | ✅ LLM 看到轻量索引 |
| **L6: 附件内容卸载** | ❌ **缺失** — 附件的 base64 Data 始终驻留在 `state.Attachments` 的内存中 | — | ❌ 内存膨胀,序列化膨胀 |

**L6 是缺失的一环**: `artifacts_write` 工具执行后,base64 内容既存在于:
1. `state.Attachments["report.md"].Data` (base64 字符串, 始终在内存中)
2. `state.Messages[n].ToolCalls[m].Arguments["content"]` → 已被 L1 压缩为 hash ✅
3. `state.Messages[n].ToolResults[m].Attachments` → 同样携带完整 base64 ❌

工具参数侧 (L1) 的 `compactToolCallArguments` 做了内容替换,但附件侧的 base64 **没有被卸载到文件系统**。这意味着:
- 一个 10KB 的 artifacts_write 内容,在 state 中以 ~13KB base64 存在
- 多次 artifacts_write 后,state.Attachments 可能持有数百 KB 的 base64 数据
- 这些数据在事件序列化时写入 Postgres,inflating 数据库
- 对 LLM 不可见(仅看到 catalog 索引),但占用进程内存

### 1.4 `artifacts_write` 全流程 (关键路径)

```
LLM 调用 artifacts_write(name="report.md", content="# 大量内容...")
    ↓
artifacts.go:130 → base64.StdEncoding.EncodeToString([]byte(content))
    ↓
创建 Attachment{Data: "base64...", URI: "data:text/markdown;base64,..."}
    ↓
ToolResult.Attachments["report.md"] = Attachment{Data: "base64..."}
ToolResult.Metadata["attachment_mutations"]["add"]["report.md"] = same
    ↓
observe.go:25 → compactToolCallHistory()
    ├── call.Arguments["content"] → 压缩为 {content_len, sha256, ref} ✅
    └── result.Attachments["report.md"].Data → 未处理,base64 保留 ❌
    ↓
attachments.go → applyAttachmentMutations()
    ↓
state.Attachments["report.md"] = Attachment{Data: "base64..."}  ← 内容驻留内存
    ↓
buildAttachmentCatalogContent() → "1. report.md — source: artifacts_write"  ← LLM只看到索引
    ↓
finalize() → result.Attachments = state.Attachments (含完整 base64)
    ↓
事件序列化 → base64 写入 Postgres ← 数据库膨胀
    ↓
SSE normalizeAttachmentPayload() → 此时才写入 Store, 才清除 base64
```

### 1.5 数据结构

```go
// internal/agent/ports/llm.go:109
type Attachment struct {
    Name                string                    // 文件名
    MediaType           string                    // MIME type
    Data                string                    // base64 编码 (可选) ← 卸载前始终填充
    URI                 string                    // CDN/本地 URL (可选) ← 卸载后才填充
    Source              string                    // 来源: tool名, "user_upload"
    Description         string                    // 描述
    Kind                string                    // "attachment" | "artifact"
    Format              string                    // pptx, html 等
    PreviewProfile      string                    // 渲染提示
    PreviewAssets       []AttachmentPreviewAsset  // 衍生预览
    RetentionTTLSeconds uint64                    // 清理周期
}
```

**关键问题**: `Data` 和 `URI` 同时存在但优先级不明确。系统在不同阶段对两者的处理不一致。在 Agent 域内, `Data` 始终被填充, `URI` 仅为 `data:` URI (等效于重复的 base64),直到 SSE 层才转换为真实 URI。

### 1.6 存储层

两个 Provider:
- **Local**: `~/.alex/attachments/` + SHA256 命名 → `/api/attachments/<hash>.<ext>`
- **Cloudflare R2**: S3 兼容存储 → CDN URL 或 Presigned URL (15min TTL)

存储接口: `internal/attachments/store.go` - `StoreBytes(name, mediaType, data) → URI`

**问题**: 存储层仅被 SSE 渲染调用,未被 Agent 域层使用。

---

## 2. 已识别的问题

### 2.1 🔴 P0 — 附件在总结阶段不下发 (Lark/WeChat 通道)

**根因链路**:

```
Tool 执行 → 附件存入 state.Attachments (base64 Data)
    ↓
finalize() → result.Attachments = map[string]Attachment{Data: "base64..."}
    ↓
Lark gateway.sendAttachments()
    ↓
ResolveAttachmentBytes() → 解码 base64 → 原始字节
    ↓
uploadImage() / uploadFile() → Lark API
```

**问题 1 — 归一化只在 SSE 层执行**:

`normalizeAttachmentPayload()` 只在 `sse_render_attachments.go` 中被调用,仅服务于 Web 前端 SSE 推送。Lark 通道直接拿到 `TaskResult.Attachments`,其中的附件仍然是 **base64 二进制数据**,没有经过持久化转 CDN URL。

关键路径:
- `internal/server/http/sse_render_attachments.go:227` → 仅 SSE 调用
- `internal/channels/lark/gateway.go:800` → 直接解码 base64

**问题 2 — 事件回放类型丢失**:

`PostgresEventHistoryStore` 序列化事件时使用 `json.Marshal`,反序列化时 `WorkflowEventEnvelope.Payload` 变成 `map[string]any`。

`sse_render.go:156` 的类型断言:
```go
if typedAtts, ok := rawAtts.(map[string]ports.Attachment); ok  // ← 回放后失败!
```

对于从 Postgres 回放的事件,`rawAtts` 实际类型是 `map[string]any`,断言 **静默失败**,导致前端刷新页面时丢失附件。

### 2.2 🔴 P0 — 附件内容未从内存卸载 (Context Offload 缺失)

**根因**: `artifacts_write` / sandbox 工具产出附件后,base64 内容始终驻留在 `state.Attachments` 的内存中。系统虽然有 L1-L5 的卸载机制,但缺少 L6 — 附件内容到文件系统的卸载。

**影响链路**:
```
artifacts_write("report.md", 50KB 内容)
    ↓
state.Attachments["report.md"].Data = ~67KB base64   ← 内存驻留
    ↓
state.Messages[n].ToolResults[m].Attachments 也持有同一份 base64  ← 双重驻留
    ↓
事件序列化 json.Marshal(Payload) → 67KB base64 写入 Postgres     ← DB 膨胀
    ↓
多次 artifacts_write → state 中累积数百 KB base64              ← 内存持续膨胀
    ↓
compactToolCallArguments 只压缩 call.Arguments,不触及 result.Attachments ← 遗漏
```

**具体数据**:
- 一次 `artifacts_write` 10KB 内容 → ~13.3KB base64 Data + ~13.3KB data: URI
- 在 state.Attachments + ToolResult.Attachments 中双重存储 = ~26.6KB
- 10 次 artifacts_write → ~266KB 无用内存占用 (LLM 只看到名称索引)
- 事件持久化时全部写入 Postgres JSONB

**与 L1 压缩的对比**:
- L1 `compactToolCallArguments` 把 `call.Arguments["content"]` 压缩为 `{len, sha256, ref}` ← 正确做法
- 但 `ToolResult.Attachments[name].Data` 和 `state.Attachments[name].Data` **完全没有被压缩或卸载**

### 2.3 🟡 P1 — 附件传输二进制数据而非 CDN 地址

**根因**: 附件在 Agent 域内始终以 base64 `Data` 字段流转。CDN URL 转换仅在 SSE 推送时的 `normalizeAttachmentPayload()` 中执行,属于 **展示层逻辑**,而非 **域逻辑**。

影响:
1. Lark 通道: 每次下发都要先解码 base64 再重新上传,浪费带宽和内存
2. WeChat 通道: 未实现 (但同样会面临此问题)
3. 事件序列化: 大量 base64 数据写入 Postgres,inflating 数据库
4. SSE 流: 首次推送前附件以 base64 形式在内存中传递

### 2.4 🟡 P1 — WeChat 通道完全无附件支持

`internal/channels/wechat/gateway.go` 中没有任何附件处理逻辑。

### 2.5 🟢 P2 — 附件在 SSE 流中间事件中缺失

`emitFinalAnswerStream()` (`runtime.go:767-800`) 发送分块事件时不携带 Attachments:
```go
r.engine.emitEvent(&domain.WorkflowResultFinalEvent{
    // ... 800字符分块
    IsStreaming:    true,
    StreamFinished: false,
    // ← 无 Attachments
})
```

仅最终的 `StreamFinished=true` 事件携带附件。如果前端在流式渲染过程中尝试展示附件,需要等到最终事件才能获取。目前这 **是设计意图**,但不够鲁棒。

### 2.6 🟢 P2 — Presigned URL 过期

Cloudflare R2 Provider 使用 15分钟 TTL 的 Presigned URL。如果用户在页面上停留超过 15 分钟后点击附件,URL 已过期。

---

## 3. 全链路生命周期设计目标

### 3.1 CDN-First + Eager Offload 架构

```
                     CDN-First + Eager Offload Architecture
                     ======================================

Tool 生成附件 (artifacts_write / sandbox / media)
    ↓
Persist(att) → 立即写入 Store → 获得 CDN URI → 清空 Data
    ↓
state.Attachments[name] = Attachment{URI: "https://cdn.../hash.png", Data: ""}
                                                                     ^^^^^^^^
                                                                  内存已释放 ✅
    ↓
buildAttachmentCatalogContent() → "1. report.md" (名称索引给 LLM) ← 轻量
    ↓
compactToolResultAttachments() → result.Attachments 中的 Data 也被清空 ← 新增 L6
    ↓
finalize() → result.Attachments 已经全是 URI 引用
    ↓
事件序列化 → 只写 URI (几十字节) 而非 base64 (数十KB) ← DB 不膨胀
    ↓
├─→ SSE:   直接推送 URI (无需 normalizeAttachmentPayload 做转换)
├─→ Lark:  HTTP GET URI → bytes → upload to Lark API
├─→ WeChat: HTTP GET URI → bytes → upload to WeChat API
└─→ CLI:   展示 URI / 按需下载
```

### 3.2 核心原则

1. **Write-Through**: 附件一旦产生,立即持久化到 Store,后续全部以 URI 引用流转
2. **Eager Offload**: 持久化后立即清空 `Data` 字段,释放内存 (小型文本附件可保留)
3. **Uniform Reference**: 所有通道 (SSE/Lark/WeChat/CLI) 统一通过 URI 获取内容
4. **Consolidate to Summary**: 所有附件统一汇总到最终总结事件,通道在总结消息中一并展示
5. **Graceful Degradation**: Store 不可用时降级保留 base64,SSE 层 DataCache 兜底

### 3.4 附件汇总到总结消息 (业务要求)

所有在 task 生命周期内产生的附件,必须在最终总结消息中统一汇总展示给用户。各通道行为:

| 通道 | 当前行为 | 目标行为 |
|------|---------|---------|
| **Web** | `WorkflowResultFinalEvent.Attachments` → `TaskCompleteCard` 渲染 | 保持不变,确保 force-include 不被类型断言拦截 |
| **Lark** | 文本回复 + 单独 sendAttachments (分开发送) | 文本回复中追加附件汇总摘要 + 依次发送附件 |
| **WeChat** | ❌ 未实现 | 同 Lark 模式 |
| **CLI** | 仅文本 | 文本 + 附件 URI 列表 |

关键链路:
```
decorateFinalResult(state, result)
    → collectAllToolGeneratedAttachments(state)   // 从 state.Attachments 收集所有非 user_upload
    → merge A2UI attachments
    → result.Attachments = 完整附件集             // 一次性汇总
    ↓
WorkflowResultFinalEvent{Attachments: 完整附件集, StreamFinished: true}
    ↓
├── SSE: force-include all → 前端 TaskCompleteCard 一次性渲染
├── Lark: buildReply(result) 追加附件列表 + sendAttachments() 逐个发送
├── WeChat: 同上
└── CLI: 输出附件列表
```

### 3.3 卸载层级完整闭合

| 层级 | 机制 | 目标 |
|------|------|------|
| L1 | `compactToolCallArguments()` | 工具参数 → hash+ref |
| L2 | `summarizeToolResultForWorld()` | 工具输出 → 280字符预览 |
| L3 | `offloadMessageThinking()` | Thinking → 清空 |
| L4 | `AutoCompact()` | 全历史 → 压缩摘要 |
| L5 | `buildAttachmentCatalogContent()` | 附件 → 名称索引 |
| **L6** | **`persistAndOffload()`** (新增) | **附件 Data → Store URI, 清空 Data** |
| **L7** | **`compactToolResultAttachments()`** (新增) | **ToolResult.Attachments Data → 清空** |

L6+L7 补齐了唯一缺失的卸载环节,使内容从产生到消费的全链路上不再有 base64 膨胀。

---

## 4. 方案设计

### 4.1 Phase 1 — 域层 CDN-First 持久化 + Eager Offload (解决 P0 全部 + P1)

**目标**: 将附件持久化从 SSE 展示层下沉到 Agent 域层。同时实现 L6/L7 卸载,释放内存。

#### 4.1.1 新增 `AttachmentPersister` 端口

```go
// internal/agent/ports/attachment_store.go (新文件)
package ports

// AttachmentPersister 负责将附件持久化并返回可访问的 URI。
// 这是一个端口(port),具体实现由基础设施层提供。
type AttachmentPersister interface {
    // Persist 将附件的 inline 数据 (Data/data URI) 持久化到存储层,
    // 返回更新后的附件 (URI 已填充, Data 已清空)。
    // 如果附件已有外部 URI 且无 inline 数据,原样返回。
    // 对于小型文本附件 (markdown/json <4KB), Data 可选保留用于前端快速预览。
    Persist(att Attachment) (Attachment, error)
}
```

#### 4.1.2 在 ReactEngine 中注入持久化能力

修改 `ReactEngine` 构造,注入 `AttachmentPersister`:

```go
// internal/agent/domain/react/engine.go
type ReactEngine struct {
    // ... 现有字段
    attachmentPersister ports.AttachmentPersister // 新增
}
```

#### 4.1.3 附件变更时立即持久化 + 卸载 (L6)

修改 `applyAttachmentMutations()` (`internal/agent/domain/react/attachments.go`),在 add/replace/update 操作时立即调用 `Persist`:

```go
func (e *ReactEngine) persistAttachment(att ports.Attachment) ports.Attachment {
    if e.attachmentPersister == nil {
        return att
    }
    // 只处理有 inline 数据的附件
    if att.Data == "" && !strings.HasPrefix(att.URI, "data:") {
        return att
    }
    persisted, err := e.attachmentPersister.Persist(att)
    if err != nil {
        e.logger.Warn("attachment persist failed (%s): %v", att.Name, err)
        return att // 降级: 保留原始 base64
    }
    return persisted
    // 此时: persisted.URI = "https://cdn.../hash.ext"
    //       persisted.Data = "" (已清空,内存释放)
    //       除非是小型文本附件 (<4KB text/markdown 等) Data 保留
}
```

调用点:
- `attachmentMutations.apply()` 中的 add/replace/update 分支
- 用户上传附件注入到 `state.PendingUserAttachments` 时
- `prepareUserTaskContext()` 中 `registerMessageAttachments()` 时

#### 4.1.4 ToolResult 附件卸载 (L7)

在 `observeToolResults()` 中,除了现有的 `compactToolCallHistory()`,新增附件卸载:

```go
// internal/agent/domain/react/observe.go — 新增
func (e *ReactEngine) compactToolResultAttachments(state *TaskState, results []ToolResult) {
    if e.attachmentPersister == nil || state == nil {
        return
    }
    // 对最新一批 ToolResult 中的附件:
    // 如果 state.Attachments 已持有该附件的 URI 版本,
    // 则清空 ToolResult.Attachments[name].Data,只保留 URI 引用。
    // 这避免了同一份 base64 在 state.Attachments 和 ToolResult.Attachments 中双重驻留。
    for i, result := range results {
        if len(result.Attachments) == 0 {
            continue
        }
        compacted := make(map[string]ports.Attachment, len(result.Attachments))
        for name, att := range result.Attachments {
            if stateAtt, ok := state.Attachments[name]; ok && stateAtt.URI != "" {
                // state 中已有 URI 版本,ToolResult 中只保留引用
                att.Data = ""
                att.URI = stateAtt.URI
            }
            compacted[name] = att
        }
        results[i].Attachments = compacted
    }
    // 同样清理历史消息中的 ToolResult.Attachments
    for idx := range state.Messages {
        msg := &state.Messages[idx]
        for j := range msg.ToolResults {
            tr := &msg.ToolResults[j]
            for name, att := range tr.Attachments {
                if stateAtt, ok := state.Attachments[name]; ok && stateAtt.URI != "" {
                    att.Data = ""
                    att.URI = stateAtt.URI
                    tr.Attachments[name] = att
                }
            }
        }
    }
}
```

调用时机 — 在 `observeToolResults()` 末尾,紧接 `compactToolCallHistory()`:

```go
func (e *ReactEngine) observeToolResults(state *TaskState, iteration int, results []ToolResult) {
    // ... 现有逻辑 ...
    e.compactToolCallHistory(state, results)
    e.compactToolResultAttachments(state, results)  // 新增 L7
    e.appendFeedbackSignals(state, results)
}
```

#### 4.1.5 `AttachmentPersister` 的基础设施实现

```go
// internal/attachments/persister.go (新文件)
package attachments

// inlineRetentionLimit 控制哪些小型文本附件在持久化后仍保留 Data 字段。
// 低于此限制的 text/*, markdown, json 附件保留 inline 数据用于前端快速预览。
const inlineRetentionLimit = 4096

type StorePersister struct {
    store *Store
}

func NewStorePersister(store *Store) *StorePersister {
    return &StorePersister{store: store}
}

func (p *StorePersister) Persist(att ports.Attachment) (ports.Attachment, error) {
    // 1. 如果已有外部 URI (非 data:) 且无 inline 数据 → 原样返回
    if att.Data == "" && !isDataURI(att.URI) && att.URI != "" {
        return att, nil
    }

    // 2. 解码 inline 数据
    payload, mediaType := decodeAttachmentPayload(att)
    if len(payload) == 0 {
        return att, nil // 无内容可持久化
    }

    // 3. 写入 Store
    uri, err := p.store.StoreBytes(att.Name, mediaType, payload)
    if err != nil {
        return att, err
    }

    // 4. 更新附件: URI 填充, Data 按策略清空
    att.URI = uri
    if att.MediaType == "" {
        att.MediaType = mediaType
    }

    // 5. 小型文本保留 inline 数据
    if shouldRetainInline(att.MediaType, len(payload)) {
        // 保留 Data 用于前端快速预览
    } else {
        att.Data = "" // 释放内存
    }

    return att, nil
}
```

#### 4.1.6 简化 SSE normalizeAttachmentPayload

`normalizeAttachmentPayload` 不再需要做持久化,简化为:
- 检查是否已有 URI → 直接返回
- 降级处理: 如果仍有 base64 (Store 不可用场景) → 缓存到 DataCache
- HTML 预览增强保持不变

### 4.2 Phase 2 — 修复事件回放类型断言 (解决 P0)

#### 4.2.1 修复 `sse_render.go` 的 force-include 逻辑

```go
// sse_render.go:153-167 修改
if envelope.Event == "workflow.result.final" {
    if finished, _ := envelope.Payload["stream_finished"].(bool); finished {
        if rawAtts, ok := envelope.Payload["attachments"]; ok && rawAtts != nil {
            var typedAtts map[string]ports.Attachment

            switch v := rawAtts.(type) {
            case map[string]ports.Attachment:
                typedAtts = v
            case map[string]any:
                // 从 JSON 反序列化恢复类型
                typedAtts = attachmentsFromUntypedMap(v)
            }

            if len(typedAtts) > 0 {
                forced := sanitizeAttachmentsForStream(typedAtts, sentAttachments, h.dataCache, h.attachmentStore, true)
                if len(forced) > 0 {
                    if payload == nil {
                        payload = make(map[string]any)
                    }
                    payload["attachments"] = forced
                }
            }
        }
    }
}
```

新增辅助函数:
```go
func attachmentsFromUntypedMap(raw map[string]any) map[string]ports.Attachment {
    result := make(map[string]ports.Attachment, len(raw))
    for key, value := range raw {
        entryMap, ok := value.(map[string]any)
        if !ok {
            continue
        }
        att := attachmentFromMap(entryMap) // 复用已有函数
        if att.Name == "" {
            att.Name = key
        }
        result[key] = att
    }
    if len(result) == 0 {
        return nil
    }
    return result
}
```

### 4.3 Phase 3 — Lark 通道优化 (解决 P1 下发二进制)

在 Phase 1 完成后, `result.Attachments` 已全部持有 CDN URI。Lark 通道的 `sendAttachments` 流程变为:

```
result.Attachments[name].URI = "https://cdn.../hash.png"
    ↓
ResolveAttachmentBytes() → HTTP GET CDN URL → 原始字节
    ↓
uploadImage() / uploadFile() → Lark API
```

这比之前的流程更高效:
- 不再在 Agent 域内传递大量 base64
- CDN 通常有更好的网络路径
- 可以利用 CDN 缓存

进一步优化 (可选): 如果 Lark 支持从 URL 下载资源,可以直接传 CDN URL 避免中间下载。

### 4.4 Phase 4 — WeChat 通道附件支持 (解决 P1)

参照 Lark 通道的实现模式:
```go
func (g *WeChatGateway) sendAttachments(ctx context.Context, result *agent.TaskResult) {
    for name, att := range result.Attachments {
        // 1. 通过 URI 获取字节 (CDN-first)
        // 2. 根据 MediaType 选择: 图片/文件/视频
        // 3. 上传到 WeChat 临时素材 API
        // 4. 发送消息
    }
}
```

### 4.5 Phase 5 — Presigned URL 续期 (解决 P2)

#### 方案 A: 延长 TTL + 前端 lazy refresh (推荐)

1. 将默认 `PresignTTL` 从 15 分钟提升到 **4 小时**
2. 前端在附件点击时检查 URL 是否即将过期 (通过 query param 中的 `X-Amz-Expires`)
3. 如果即将过期,通过 `/api/attachments/<hash>` 代理获取新 URL

#### 方案 B: 始终使用 Public CDN URL

配置 `CloudflarePublicBaseURL` 后,所有附件使用永久 CDN URL,无过期问题。推荐在生产环境使用。

---

## 5. 实施计划

### Batch 1: 基础设施 — AttachmentPersister
1. 创建 `internal/agent/ports/attachment_store.go` (端口定义)
2. 创建 `internal/attachments/persister.go` (基础设施实现)
3. 单元测试: 各种 payload 格式 (base64, data URI, 空, 超大), 降级场景

### Batch 2: 域层集成 — CDN-First 持久化 + L6 卸载
1. `ReactEngine` 注入 `AttachmentPersister`
2. `applyAttachmentMutations()` 中调用 `persistAttachment()` (L6)
3. `prepareUserTaskContext()` 中用户上传注入时持久化
4. 单元测试 + 集成测试: 验证 state.Attachments 中 Data 已清空, URI 已填充

### Batch 3: L7 卸载 — ToolResult 附件压缩
1. 新增 `compactToolResultAttachments()` 函数
2. 在 `observeToolResults()` 中调用
3. 验证历史消息中 ToolResult.Attachments 的 Data 已被清空
4. 单元测试: 多轮工具调用后内存中不残留 base64

### Batch 4: SSE 修复 — 事件回放类型安全
1. 修复 `sse_render.go` 的 force-include 类型断言 (支持 `map[string]any`)
2. 新增 `attachmentsFromUntypedMap()` 复用已有 `attachmentFromMap()`
3. 单元测试: JSON 序列化/反序列化往返后附件仍可正确提取

### Batch 5: SSE 简化 — normalizeAttachmentPayload
1. 简化 `normalizeAttachmentPayload` (去除持久化职责, 域层已完成)
2. 保留降级路径 (DataCache fallback for Store 不可用场景)
3. HTML 预览增强保持不变
4. 回归测试: 确保现有 SSE 推送行为不变

### Batch 6: 渠道优化
1. Lark 通道: 验证 CDN URI 流程工作 (ResolveAttachmentBytes 自动适配)
2. WeChat 通道: 实现附件下发
3. E2E 测试: 全链路验证 (tool → persist → state → event → SSE/Lark)

---

## 6. 架构对比

### Before (当前):
```
工具生成 → base64 in state ──────────────────→ base64 in event → SSE normalize → CDN URL
              ↑ 内存膨胀                              ↑ DB 膨胀
              │                                       │
              └── ToolResult.Attachments 也持有 base64 ┘  (双重驻留)

              → Lark: decode base64 → upload → Lark
              → WeChat: ❌
              → 事件 Postgres: 存储完整 base64 JSONB
```

**问题**:
1. 持久化是展示层关注点,非 Web 通道被遗漏
2. base64 Data 在内存中双重驻留 (state + ToolResult)
3. 事件序列化将完整 base64 写入 Postgres
4. L6/L7 卸载缺失,上下文管理存在漏洞

### After (目标):
```
工具生成 → Persist → CDN URI in state (Data="") → URI in event → SSE: 直接传递
                         ↑ 内存释放 ✅                  ↑ DB 精简 ✅
                         │
                         └── compactToolResultAttachments: ToolResult 中也只有 URI

              → Lark: HTTP GET URI → bytes → upload → Lark
              → WeChat: HTTP GET URI → bytes → upload → WeChat
              → CLI: 展示 URI / 按需下载
              → 事件 Postgres: 仅存储 URI 字符串 (~50 bytes vs ~13KB)
```

**优势**:
1. 持久化是域层关注点,所有通道统一使用 URI 引用
2. L6+L7 补齐卸载链路,内存不膨胀
3. 事件序列化体积降低 99% (URI vs base64)
4. 上下文管理 L1-L7 完整闭合

---

## 7. 风险与降级策略

| 风险 | 降级策略 |
|------|---------|
| 存储不可用 | 保留 base64 Data 不清空, SSE 层 DataCache 兜底 |
| CDN URL 不可达 | Lark/WeChat 通道 fallback 到 base64 解码 |
| 迁移期新旧附件混合 | `normalizeAttachmentPayload` 保留对 base64 的处理能力 |
| Presigned URL 过期 | `/api/attachments/` 代理端点重新生成 URL |

---

## 8. 测试策略

- **单元测试**: `AttachmentPersister` 各实现 + edge case (空数据, 超大文件, 无效 MIME)
- **卸载验证测试**: 多轮 artifacts_write 后验证:
  - `state.Attachments[name].Data == ""` (L6 卸载生效)
  - `state.Attachments[name].URI` 指向有效 Store URI
  - 历史 `Message.ToolResults[n].Attachments[name].Data == ""` (L7 卸载生效)
  - 事件序列化后的 JSONB 大小 (应仅包含 URI 字符串)
- **内存基准测试**: 10 次 artifacts_write (每次 50KB) 后,state 的内存占用应稳定 (不随附件数量线性增长)
- **集成测试**: 端到端附件流转 (tool → persist → state → event → SSE/Lark)
- **回归测试**: 现有 SSE 附件推送行为不变 (normalizeAttachmentPayload 降级路径)
- **类型安全测试**: 事件 JSON 序列化/反序列化后类型断言验证 (attachmentsFromUntypedMap)
- **降级测试**: Store 不可用时,base64 保留、SSE DataCache 兜底
