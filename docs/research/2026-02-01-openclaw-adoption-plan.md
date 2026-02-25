# OpenClaw 借鉴方案 v2（Delta 版）

Updated: 2026-02-01 18:00

## 0. 目标与原则

- **目标**：以最小入侵方式，补齐"工具治理 + 统一事件模型 + 记忆保全 + 调度增强 + 记忆结构化 + macOS 原生集成"六个 gap。
- **非目标**：重述已有能力、引入分布式调度。
- **约束**：`agent/ports` 不引入 memory/RAG 依赖；YAML-only 配置；deny-first 安全语义。

### 与原方案的关系

原方案约 60% 篇幅描述了已有能力。本版仅保留真实 delta，按价值优先级排序。

---

## 1. 现有系统能力基线（不再重复设计）

以下能力已实现，本方案不再覆盖：

| 能力 | 现有实现 | 代码位置 |
|---|---|---|
| Cron 调度 | robfig/cron v3，支持静态 trigger + OKR 动态 trigger + 5min 自动同步 | `internal/scheduler/scheduler.go` |
| Hook 生命周期 | `ProactiveHook` 接口，`OnTaskStart`/`OnTaskCompleted`，priority 排序 | `internal/agent/app/hooks/hooks.go` |
| 5 个内置 Hook | MemoryRecall / MemoryCapture / ConversationCapture / IterationRefresh / OKRContext | `internal/agent/app/hooks/*.go` |
| 文件化记忆 | Markdown + YAML frontmatter，ksuid 主键，retention policy，prune on recall | `internal/memory/file_store.go` |
| 记忆服务 | Save/Recall + 关键词归一 + term 收集 + 过期过滤 | `internal/memory/service.go` |
| 三级工具注册 | static / dynamic / mcp，approval wrapper (Dangerous flag)，ID propagation | `internal/toolregistry/registry.go` |
| 上下文压缩 | `AutoCompact` → `Compress`，保留 system prompt，压缩 compressible 为摘要 | `internal/context/manager_compress.go` |
| 触发执行 | `AgentCoordinator.ExecuteTask`，结果路由到 Lark/Moltbook | `internal/scheduler/executor.go` |

---

## 2. Delta 1：工具治理 — allow/deny profile（优先级最高）

### 2.1 问题

当前 `ToolMetadata` 只有 `Dangerous bool` 和 `Category string`，没有策略层。所有注册工具对所有 session 可见，无法按通道/角色/场景限制工具集。

### 2.2 目标

- 工具按 group 分组，group 在工具 metadata 中声明（复用已有 `Tags []string`）。
- 引入 `ToolPolicy`，支持 deny-first 语义的 allow/deny 规则。
- Policy 按 profile 维度组织，profile 通过 session context 选择。

### 2.3 设计

#### 2.3.1 Group 声明

不新建映射表。复用 `ToolMetadata.Tags` 作为 group 标签：

```go
// 工具注册时声明 group（现有 Tags 字段）
// 约定：tags 中 "group:" 前缀的值为 group 标签
// 例如：Tags: ["group:fs", "group:dangerous", "readonly"]
```

Group 提取逻辑：

```go
func GroupsFromMetadata(meta ports.ToolMetadata) []string {
    var groups []string
    for _, tag := range meta.Tags {
        if strings.HasPrefix(tag, "group:") {
            groups = append(groups, strings.TrimPrefix(tag, "group:"))
        }
    }
    if meta.Category != "" {
        groups = append(groups, meta.Category)
    }
    return groups
}
```

建议 group 分类（对应现有 builtin 目录结构）：

| Group | 工具 |
|---|---|
| `fs` | file_read, file_write, file_edit, list_files |
| `exec` | bash, code_execute, shell_exec, execute_code |
| `search` | grep, ripgrep, find, web_search |
| `memory` | memory_recall, memory_write |
| `browser` | browser_action, browser_info, browser_screenshot, browser_dom |
| `sandbox` | read_file, write_file, list_dir, search_file, replace_in_file, write_attachment |
| `media` | text_to_image, image_to_image, vision_analyze, video_generate, music_play |
| `lark` | lark_chat_history, lark_send_message |
| `okr` | okr_read, okr_write |
| `orchestration` | subagent, explore, bg_dispatch, bg_status, bg_collect |
| `ui` | plan, clarify, request_user, artifacts_write, artifacts_list, artifacts_delete, a2ui_emit |
| `web` | web_fetch, html_edit, douyin_hot |

#### 2.3.2 Policy 模型

```go
// internal/toolregistry/policy.go

type ToolPolicy struct {
    // Deny 优先于 Allow。空 Deny = 不拒绝。空 Allow = 允许全部。
    Deny  []PolicyRule `yaml:"deny"`
    Allow []PolicyRule `yaml:"allow"`
}

type PolicyRule struct {
    Group string `yaml:"group,omitempty"` // "group:fs" 或 group 名
    Tool  string `yaml:"tool,omitempty"`  // 精确工具名
}

// Evaluate 返回工具是否可用。deny-first 语义。
func (p *ToolPolicy) IsAllowed(toolName string, groups []string) bool {
    // 1. 任何 deny 规则命中 → 拒绝
    for _, rule := range p.Deny {
        if rule.Tool == toolName { return false }
        for _, g := range groups {
            if rule.Group == g { return false }
        }
    }
    // 2. 无 allow 规则 → 允许全部
    if len(p.Allow) == 0 { return true }
    // 3. 任何 allow 规则命中 → 允许
    for _, rule := range p.Allow {
        if rule.Tool == toolName { return true }
        for _, g := range groups {
            if rule.Group == g { return true }
        }
    }
    return false
}
```

#### 2.3.3 Profile 选择

```go
type PolicyProfile struct {
    Name   string     `yaml:"name"`
    Match  ProfileMatch `yaml:"match"`
    Policy ToolPolicy `yaml:"policy"`
}

type ProfileMatch struct {
    Channels []string `yaml:"channels,omitempty"` // lark, web, cli
    Roles    []string `yaml:"roles,omitempty"`    // admin, user
    Sessions []string `yaml:"sessions,omitempty"` // session prefix 匹配
}
```

Profile resolution 顺序：遍历 profiles，取第一个 match 命中的。无命中则使用 `default` profile。

#### 2.3.4 集成点

在 `Registry.List()` 和 `Registry.Get()` 中加入 policy 过滤层：

```go
// 新增 policyAwareRegistry wrapper（类似现有 filteredRegistry）
type policyAwareRegistry struct {
    parent *Registry
    policy *ToolPolicy
}

func (r *policyAwareRegistry) Get(name string) (tools.ToolExecutor, error) {
    tool, err := r.parent.Get(name)
    if err != nil { return nil, err }
    groups := GroupsFromMetadata(tool.Metadata())
    if !r.policy.IsAllowed(name, groups) {
        return nil, fmt.Errorf("tool denied by policy: %s", name)
    }
    return tool, nil
}
```

在 coordinator 组装 session 时，根据 channel/role 选择 profile，注入 `policyAwareRegistry`。

#### 2.3.5 配置

`configs/tools/policy.yaml`：

```yaml
version: 1
profiles:
  - name: lark_user
    match:
      channels: [lark]
    policy:
      deny:
        - group: exec        # Lark 通道禁止直接执行
        - group: sandbox      # Lark 通道禁止 sandbox
      allow: []               # 其余全部允许

  - name: web_sandbox
    match:
      channels: [web]
    policy:
      deny:
        - tool: bash          # Web 通道禁止本地 bash
      allow: []

  - name: default
    match: {}
    policy:
      deny: []
      allow: []               # CLI 全放开
```

### 2.4 改动范围

| 文件 | 改动 |
|---|---|
| `internal/agent/ports/tools.go` | `ToolMetadata.Tags` 已有，无需改动 |
| `internal/toolregistry/policy.go` | **新增** — ToolPolicy, PolicyProfile, IsAllowed |
| `internal/toolregistry/registry.go` | 新增 `WithPolicy(policy)` 方法返回 policyAwareRegistry |
| 各 builtin tool 的 `Metadata()` | 补充 `group:xxx` 到 Tags |
| `internal/config/file_config.go` | 新增 `ToolPolicyFileConfig` |
| `internal/di/container_builder.go` | policy 加载 + profile 选择注入 |
| `configs/tools/policy.yaml` | **新增** — 默认 policy 配置 |

### 2.5 测试

- 单元：`IsAllowed` 的 deny-first 语义、group 匹配、精确工具匹配。
- 集成：Lark 通道 session 拿到的 tool list 不包含 exec group。
- 边界：空 policy、全 deny、全 allow、deny + allow 同时命中。

---

## 3. Delta 2：统一事件总线（解锁 session/system 级事件）

### 3.1 问题

当前 `ProactiveHook` 接口只有 task 粒度的两个生命周期点（`OnTaskStart` / `OnTaskCompleted`）。原方案提出 `session.started`、`command.*`、`tool.failed`、`gateway.startup` 等系统级事件，但这些无法映射到现有 hook 接口——它们不是 task 级别的。

如果直接给 `ProactiveHook` 加更多方法，会导致每个 hook 实现都要 stub 无关方法，接口膨胀不可控。

### 3.2 目标

引入轻量 event bus，让 task 级 hook 和 system 级事件用同一套发布/订阅机制。现有 hook 平滑迁移为 event subscriber。

### 3.3 设计

#### 3.3.1 Event 定义

```go
// internal/events/event.go

type EventType string

const (
    // Task-level（对应现有 hook 生命周期）
    EventTaskStarted   EventType = "task.started"
    EventTaskCompleted EventType = "task.completed"

    // Session-level（新增）
    EventSessionStarted EventType = "session.started"
    EventSessionEnded   EventType = "session.ended"

    // System-level（新增）
    EventToolFailed     EventType = "tool.failed"
    EventContextCompact EventType = "context.compact"  // flush 触发点
    EventGatewayStartup EventType = "gateway.startup"
)

type Event struct {
    Type      EventType
    Payload   any       // 强类型 payload，按 EventType switch
    SessionID string
    Timestamp time.Time
}
```

#### 3.3.2 Bus 实现

```go
// internal/events/bus.go

type Handler func(ctx context.Context, event Event) error

type Bus struct {
    mu       sync.RWMutex
    handlers map[EventType][]handlerEntry
    logger   logging.Logger
}

type handlerEntry struct {
    name     string
    handler  Handler
    priority int
}

func (b *Bus) Subscribe(eventType EventType, name string, priority int, handler Handler)
func (b *Bus) Publish(ctx context.Context, event Event)  // 同步，按 priority 排序执行
func (b *Bus) PublishAsync(ctx context.Context, event Event) // 异步，不阻塞调用方
```

关键约束：
- `Publish` 中单个 handler 失败不阻塞后续 handler，错误写 observability。
- handler 有 timeout（默认 10s），超时自动取消。
- 支持速率限制：每个 handler 可配置 `maxRate`（次/分钟）。

#### 3.3.3 现有 Hook 迁移

```go
// internal/agent/app/hooks/adapter.go

// HookBusAdapter 将现有 ProactiveHook 注册为 event bus subscriber
func RegisterHookOnBus(bus *events.Bus, hook ProactiveHook) {
    bus.Subscribe(events.EventTaskStarted, hook.Name(), 0, func(ctx context.Context, e events.Event) error {
        taskInfo := e.Payload.(TaskInfo)
        injections := hook.OnTaskStart(ctx, taskInfo)
        // 将 injections 写入 event 的 response channel 或 context
        return nil
    })
    bus.Subscribe(events.EventTaskCompleted, hook.Name(), 0, func(ctx context.Context, e events.Event) error {
        result := e.Payload.(TaskResultInfo)
        return hook.OnTaskCompleted(ctx, result)
    })
}
```

现有 `Registry.RunOnTaskStart` / `RunOnTaskCompleted` 改为向 bus 发布事件，收集 injections。过渡期保持两套调用兼容，最终 hook registry 只做注册，执行全部走 bus。

#### 3.3.4 新增事件触发点

| 事件 | 触发位置 | 用途 |
|---|---|---|
| `session.started` | `coordinator.NewSession()` | 加载 session 级配置、预热缓存 |
| `session.ended` | `coordinator.EndSession()` | 触发 memory flush、清理资源 |
| `tool.failed` | `solve.go` 工具执行失败时 | 告警、自动降级策略 |
| `context.compact` | `manager_compress.go:AutoCompact` 执行前 | **memory flush 触发点**（见 Delta 3） |
| `gateway.startup` | `cmd/alex-server/main.go` | 系统健康检查、scheduler 状态上报 |

### 3.4 改动范围

| 文件 | 改动 |
|---|---|
| `internal/events/` | **新增包** — event.go, bus.go, bus_test.go |
| `internal/agent/app/hooks/adapter.go` | **新增** — hook → bus 适配 |
| `internal/agent/app/hooks/hooks.go` | `RunOnTaskStart` / `RunOnTaskCompleted` 改为走 bus |
| `internal/agent/app/coordinator/coordinator.go` | 注入 bus，session 生命周期发事件 |
| `internal/context/manager_compress.go` | `AutoCompact` 前发布 `context.compact` 事件 |
| `internal/di/container_builder.go` | 初始化 bus，注册现有 hooks |

### 3.5 测试

- 单元：Bus 的 Subscribe/Publish 顺序、timeout、handler 失败隔离。
- 集成：发布 `task.started` → 现有 MemoryRecallHook 正常收到并返回 injections。
- 回归：确保迁移后所有 hook 行为不变。

---

## 4. Delta 3：Memory Flush-before-Compaction

### 4.1 问题

当前上下文压缩 (`AutoCompact`) 会将 compressible messages 压缩为一行摘要（`[Earlier context compressed]...`），压缩后对话细节永久丢失。如果其中包含重要决策、工具调用结果或关键对话，这些信息就无法被后续 recall。

### 4.2 目标

在 `AutoCompact` 执行**之前**，将即将被压缩的 messages 提取关键信息落盘到 memory store，确保有价值的上下文不随压缩丢失。

### 4.3 设计

#### 4.3.1 触发路径

```
AutoCompact 被调用
  → 发布 EventContextCompact { CompressibleMessages []ports.Message }
  → MemoryFlushHandler 订阅该事件
  → 提取关键信息 → memory.Save()
  → AutoCompact 继续执行压缩
```

这里用 `Publish`（同步），确保 flush 完成后再压缩。

#### 4.3.2 Flush Handler

```go
// internal/agent/app/hooks/memory_flush.go

type MemoryFlushHook struct {
    memService memory.Service
    logger     logging.Logger
}

func (h *MemoryFlushHook) HandleContextCompact(ctx context.Context, event events.Event) error {
    payload := event.Payload.(*ContextCompactPayload)
    entries := h.extractMemories(payload.Messages, payload.SessionID, payload.UserID)
    for _, entry := range entries {
        if _, err := h.memService.Save(ctx, entry); err != nil {
            h.logger.Warn("flush memory failed: %v", err)
            // 不阻塞压缩流程
        }
    }
    return nil
}
```

#### 4.3.3 提取策略

从 compressible messages 中提取值得记忆的内容：

```go
func (h *MemoryFlushHook) extractMemories(msgs []ports.Message, sessionID, userID string) []memory.Entry {
    var entries []memory.Entry

    for _, msg := range msgs {
        // 1. 带工具调用的 assistant 消息 → 记录决策
        if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
            entries = append(entries, memory.Entry{
                UserID:   userID,
                Content:  buildToolDecisionSummary(msg),
                Keywords: extractToolNames(msg.ToolCalls),
                Slots: map[string]string{
                    "type":       "auto_flush",
                    "source":     "context_compact",
                    "session_id": sessionID,
                },
            })
        }

        // 2. 用户消息中的明确指令/偏好 → 记录偏好
        if msg.Role == "user" && looksLikePreference(msg.Content) {
            entries = append(entries, memory.Entry{
                UserID:  userID,
                Content: msg.Content,
                Keywords: []string{"preference", "user_instruction"},
                Slots: map[string]string{
                    "type":       "auto_flush",
                    "source":     "context_compact",
                    "session_id": sessionID,
                },
            })
        }

        // 3. 工具结果中的关键输出 → 记录事实
        for _, tr := range msg.ToolResults {
            if len(tr.Output) > 200 { // 有实质内容的工具输出
                entries = append(entries, memory.Entry{
                    UserID:  userID,
                    Content: truncate(tr.Output, 500),
                    Keywords: []string{"tool_result", tr.ToolName},
                    Slots: map[string]string{
                        "type":       "auto_flush",
                        "source":     "context_compact",
                        "session_id": sessionID,
                    },
                })
            }
        }
    }

    return deduplicateEntries(entries)
}
```

#### 4.3.4 容量控制

- 每次 flush 最多保存 **5 条** entry，避免低价值信息淹没 memory store。
- Flush 写入的 entry 使用 `type: auto_flush`，retention 使用 `auto_capture_days`（当前配置 90 天）。
- 重复检测：与最近 10 条 memory 做关键词重叠度检查，超过 `dedupe_threshold` 的跳过。

### 4.4 改动范围

| 文件 | 改动 |
|---|---|
| `internal/agent/app/hooks/memory_flush.go` | **新增** — MemoryFlushHook |
| `internal/events/event.go` | 新增 `EventContextCompact` + `ContextCompactPayload` |
| `internal/context/manager_compress.go` | `AutoCompact` 中发布事件（需注入 bus） |
| `internal/context/manager.go` | manager 增加 bus 字段 |
| `internal/di/container_builder.go` | 注册 MemoryFlushHook 到 bus |

### 4.5 测试

- 单元：`extractMemories` 的提取逻辑（工具决策、偏好识别、结果过滤）。
- 集成：构造超长 message 列表 → 触发 AutoCompact → 验证 memory store 中多了 flush entries。
- 边界：空消息列表、全 system prompt（无 compressible）、memory service 不可用时不阻塞压缩。

---

## 5. Delta 4：Scheduler 增强 — Job 持久化 + 执行状态

### 5.1 问题

当前 `Scheduler` 的 trigger 仅来自两个源：YAML 静态配置 + OKR 动态扫描。缺少：
- **运行时动态添加**：用户通过对话说"每天 9 点提醒我"无法持久化。
- **执行状态跟踪**：`executeTrigger` 没有记录 `last_run_at`、成功/失败、重试。
- **冷却与并发控制**：同一 job 的 cron 可能重叠执行。

### 5.2 目标

在现有 scheduler 上最小化扩展，不引入分布式调度。单实例场景下解决动态持久化和执行跟踪。

### 5.3 设计

#### 5.3.1 Job 持久化

复用文件化方案（与 memory file_store 同构），不引入新存储依赖：

```go
// internal/scheduler/job_store.go

type Job struct {
    ID              string        `yaml:"id"`
    Schedule        string        `yaml:"schedule"`
    Timezone        string        `yaml:"timezone,omitempty"`
    Task            string        `yaml:"task"`
    Channel         string        `yaml:"channel"`
    UserID          string        `yaml:"user_id"`
    ChatID          string        `yaml:"chat_id,omitempty"`
    Mode            string        `yaml:"mode"`            // main | isolated
    Status          string        `yaml:"status"`          // active | paused | error
    MaxConcurrency  int           `yaml:"max_concurrency"` // 默认 1
    CooldownSeconds int           `yaml:"cooldown_seconds"`
    RetryPolicy     RetryPolicy   `yaml:"retry_policy"`
    LastRunAt       *time.Time    `yaml:"last_run_at,omitempty"`
    LastRunStatus   string        `yaml:"last_run_status,omitempty"` // success | failed
    ConsecFailures  int           `yaml:"consec_failures"`
    CreatedAt       time.Time     `yaml:"created_at"`
    Source          string        `yaml:"source"` // static | okr | dynamic
}

type RetryPolicy struct {
    MaxRetries     int `yaml:"max_retries"`
    BackoffSeconds int `yaml:"backoff_seconds"`
}

type JobStore struct {
    dir string
    mu  sync.RWMutex
}

func (s *JobStore) Save(job Job) error     // 写 YAML 文件
func (s *JobStore) Load(id string) (Job, error)
func (s *JobStore) List() ([]Job, error)
func (s *JobStore) Delete(id string) error
```

存储路径：`~/.alex/scheduler/jobs/<id>.yaml`

#### 5.3.2 执行状态跟踪

在 `executeTrigger` 中增加状态更新：

```go
func (s *Scheduler) executeTrigger(trigger Trigger) {
    job, _ := s.jobStore.Load(trigger.Name)

    // 冷却检查
    if job.CooldownSeconds > 0 && job.LastRunAt != nil {
        if time.Since(*job.LastRunAt) < time.Duration(job.CooldownSeconds)*time.Second {
            s.logger.Info("Scheduler: trigger %q in cooldown, skipping", trigger.Name)
            return
        }
    }

    // 并发检查（单实例用 mutex 即可）
    if !s.tryAcquire(trigger.Name) {
        s.logger.Info("Scheduler: trigger %q already running, skipping", trigger.Name)
        return
    }
    defer s.release(trigger.Name)

    // 执行
    now := time.Now()
    result, err := s.coordinator.ExecuteTask(ctx, trigger.Task, sessionID, nil)

    // 更新状态
    job.LastRunAt = &now
    if err != nil {
        job.LastRunStatus = "failed"
        job.ConsecFailures++
        // 连续失败 >= 3 次自动暂停
        if job.ConsecFailures >= 3 {
            job.Status = "error"
            s.logger.Warn("Scheduler: trigger %q paused after %d consecutive failures", trigger.Name, job.ConsecFailures)
        }
    } else {
        job.LastRunStatus = "success"
        job.ConsecFailures = 0
    }
    _ = s.jobStore.Save(job)

    // 通知...
}
```

#### 5.3.3 动态 Job 创建

通过新增工具让 agent 可以在对话中创建定时任务：

```go
// internal/tools/builtin/session/scheduler_tool.go

// scheduler_create: 创建定时任务
// scheduler_list:   列出定时任务
// scheduler_delete: 删除定时任务
// scheduler_pause:  暂停/恢复定时任务
```

这些工具直接调用 `Scheduler` 的方法，持久化到 `JobStore`。

#### 5.3.4 isolated 模式

`mode: isolated` 的 job 需要创建独立 session 执行：

```go
// 在 AgentCoordinator 接口中新增：
ExecuteIsolatedTask(ctx context.Context, task string, listener agent.EventListener) (*agent.TaskResult, error)
```

与现有 `ExecuteTask` 的区别：不绑定已有 session，创建临时 session，执行完毕后释放。

### 5.4 配置变更

在现有 `proactive.scheduler` 下扩展：

```yaml
proactive:
  scheduler:
    enabled: true
    jobs_dir: "~/.alex/scheduler/jobs"  # 新增
    max_consec_failures: 3               # 新增：连续失败自动暂停阈值
    triggers:
      - name: daily_summary
        schedule: "0 9 * * *"
        task: "Generate daily summary"
        channel: lark
        user_id: cklxx
        mode: main                       # 新增
        cooldown_seconds: 3600           # 新增
        retry_policy:                    # 新增
          max_retries: 2
          backoff_seconds: 60
```

### 5.5 改动范围

| 文件 | 改动 |
|---|---|
| `internal/scheduler/job_store.go` | **新增** — Job, JobStore |
| `internal/scheduler/scheduler.go` | 集成 JobStore，启动时加载持久化 jobs |
| `internal/scheduler/executor.go` | 增加状态跟踪、冷却、并发控制 |
| `internal/tools/builtin/session/scheduler_tool.go` | **新增** — 对话式创建/管理 job |
| `internal/config/file_config.go` | 扩展 SchedulerFileConfig |

### 5.6 测试

- 单元：JobStore 的 CRUD、冷却逻辑、连续失败暂停。
- 集成：动态创建 job → 重启 → job 仍在运行。
- 边界：并发触发同一 job、cooldown 边界、retry backoff。

---

## 6. Delta 5：记忆目录结构化（日志 + 长期事实分层）

### 6.1 问题

当前 `FileStore` 将所有 memory entry 存为 `~/.alex/memory/<ksuid>.md` 平铺文件。存在三个问题：

1. **不可浏览**：几百个 ksuid 文件名没有语义，人工无法快速定位某天的记忆。
2. **无层级区分**：auto_capture 的临时事实、用户偏好、工具决策混存一起，recall 时无法按"持久性"过滤。
3. **长期事实无汇总**：`docs/memory/long-term.md` 是人工维护的、运行时代码不读取。稳定的长期事实（偏好、架构决策、常用工作流）没有被系统性地从日常 entry 中提炼出来。

OpenClaw 用 `memory/YYYY-MM-DD.md`（日志）+ `MEMORY.md`（长期事实）解决了这个问题。

### 6.2 目标

在现有 FileStore 上引入分层目录结构，同时保持 `Store` 接口不变：
- **日志层**：按日期归档，人可浏览，自动 retention。
- **长期事实层**：从日志中提炼的持久知识，不过期。
- **向后兼容**：现有 ksuid 文件作为原始层保留，迁移期双读。

### 6.3 设计

#### 6.3.1 新目录结构

```
~/.alex/memory/
├── entries/                  # 原始 entry（现有 ksuid.md，迁移后归入此目录）
│   ├── 2abc...xyz.md
│   └── ...
├── daily/                    # 日汇总（自动生成）
│   ├── 2026-02-01.md
│   ├── 2026-01-31.md
│   └── ...
└── MEMORY.md                 # 长期事实（自动提炼 + 人工可编辑）
```

#### 6.3.2 日汇总生成

通过 event bus 的 `session.ended` 事件触发日汇总更新：

```go
// internal/memory/daily_summarizer.go

type DailySummarizer struct {
    store   Store
    dir     string  // ~/.alex/memory/daily/
    logger  logging.Logger
}

// OnSessionEnded 在 session 结束时触发
func (s *DailySummarizer) OnSessionEnded(ctx context.Context, event events.Event) error {
    today := time.Now().Format("2006-01-02")
    todayEntries := s.getEntriesForDate(ctx, today)
    if len(todayEntries) == 0 {
        return nil
    }
    summary := s.buildDailySummary(todayEntries)
    return s.writeDailyFile(today, summary)
}
```

日汇总文件格式：

```markdown
---
date: 2026-02-01
entry_count: 12
sources: [auto_capture, auto_flush, manual]
---

## 事实与决策
- 用户决定采用 deny-first 的工具治理策略
- 完成了 event bus 的 Phase 0 迁移

## 工具使用
- web_search × 5, file_edit × 8, bash × 3

## 关键对话摘要
- 讨论了 OpenClaw 借鉴方案，确定了 6 个 delta 优先级
```

#### 6.3.3 长期事实提炼

通过 cron job（每日一次）从日汇总中提炼长期事实到 `MEMORY.md`：

```go
// internal/memory/longterm_extractor.go

type LongtermExtractor struct {
    dailyDir    string
    memoryFile  string  // ~/.alex/memory/MEMORY.md
    logger      logging.Logger
}

// Extract 读取最近 N 天日汇总，提炼重复出现的事实/偏好
func (e *LongtermExtractor) Extract(ctx context.Context) error {
    dailySummaries := e.readRecentDays(30)
    currentFacts := e.parseMemoryFile()
    newFacts := e.identifyDurableFacts(dailySummaries, currentFacts)
    return e.updateMemoryFile(currentFacts, newFacts)
}
```

`MEMORY.md` 格式：

```markdown
# Long-Term Memory

Updated: 2026-02-01 18:00

## User Preferences
- Config format: YAML only
- Commit strategy: small incremental commits
- Testing: TDD preferred

## Architecture Decisions
- agent/ports must not import memory/RAG
- deny-first tool policy
- Event bus for cross-concern lifecycle events

## Workflow Patterns
- Daily summary at 09:00 via Lark
- OKR review cadence: weekly
```

#### 6.3.4 Recall 集成

修改 `FileStore.Search` 增加分层 recall 优先级：

```go
func (s *LayeredFileStore) Search(ctx context.Context, query Query) ([]Entry, error) {
    var results []Entry

    // 1. 长期事实优先（MEMORY.md 解析为虚拟 entries）
    if longtermEntry := s.parseLongtermAsEntry(); longtermEntry != nil {
        if matchesQuery(longtermEntry, query) {
            results = append(results, *longtermEntry)
        }
    }

    // 2. 近期日汇总（最近 7 天）
    dailyEntries := s.searchDailySummaries(query, 7)
    results = append(results, dailyEntries...)

    // 3. 原始 entries（兜底）
    rawEntries, err := s.searchRawEntries(ctx, query)
    if err != nil {
        return results, err
    }
    results = append(results, rawEntries...)

    // 去重 + 截断到 limit
    return dedup(results, query.Limit), nil
}
```

#### 6.3.5 迁移策略

1. 启动时检测 `~/.alex/memory/` 下是否有直接的 ksuid.md 文件（旧格式）。
2. 如果有，自动迁移到 `entries/` 子目录。
3. 迁移是 idempotent 的（已迁移的跳过）。
4. 第一次迁移时生成最近 7 天的日汇总 + 初始 MEMORY.md。

### 6.4 改动范围

| 文件 | 改动 |
|---|---|
| `internal/memory/file_store.go` | 重构为 `LayeredFileStore`，支持 entries/ + daily/ + MEMORY.md |
| `internal/memory/daily_summarizer.go` | **新增** — 日汇总生成 |
| `internal/memory/longterm_extractor.go` | **新增** — 长期事实提炼 |
| `internal/memory/migration.go` | **新增** — 旧格式迁移 |
| `internal/di/container_builder.go` | 初始化 summarizer/extractor，注册到 event bus |

### 6.5 测试

- 单元：日汇总生成逻辑、长期事实提炼去重、迁移 idempotent。
- 集成：写入 entries → 触发 session.ended → 验证 daily/ 文件生成。
- 回归：迁移后现有 Search/Insert/Delete 行为不变。
- 边界：空 memory 目录、只有旧格式文件、MEMORY.md 被手工编辑后的合并。

---

## 7. Delta 6：macOS Companion App + 本地 Node Host

### 7.1 问题

当前 elephant.ai 的交付面是 CLI + Web + Lark + WeChat，全部基于 Go 后端 + Next.js 前端。在 macOS 上缺少：

1. **系统级权限入口**：屏幕录制、麦克风、辅助功能（Accessibility）等 TCC 权限没有统一申请和管理的入口。
2. **本地工具执行**：`bash`、`code_execute` 等工具在 server 模式下运行在服务端，用户 macOS 上的文件和应用无法直接操作。
3. **常驻入口**：用户需要手动打开终端或浏览器才能使用 assistant，没有 menu bar 常驻入口。
4. **原生体验**：Web 界面无法调用 macOS 原生 API（Spotlight、Shortcuts、Calendar 等）。

OpenClaw 通过 menu bar companion app + local node host 解决了这些问题。

### 7.2 目标

构建 macOS companion app，作为本地权限中心和工具执行宿主，通过 Gateway API 与现有后端对接。

### 7.3 架构

```
┌─────────────────────────────────────────────┐
│  macOS Companion App (SwiftUI, menu bar)    │
│  ┌───────────────┐  ┌────────────────────┐  │
│  │ Permission    │  │ Node Host (HTTP)   │  │
│  │ Manager       │  │  ├─ system.run     │  │
│  │ (TCC broker)  │  │  ├─ screen_capture │  │
│  │               │  │  ├─ audio_record   │  │
│  └───────────────┘  │  ├─ ui_automation  │  │
│                     │  └─ fs_local       │  │
│                     └────────────────────┘  │
│  ┌───────────────┐  ┌────────────────────┐  │
│  │ WebChat View  │  │ Status & Settings  │  │
│  │ (WKWebView)   │  │ (SwiftUI native)   │  │
│  └───────────────┘  └────────────────────┘  │
└──────────────┬──────────────────────────────┘
               │ HTTP/WebSocket (localhost:PORT)
               │ + Bearer token auth
┌──────────────▼──────────────────────────────┐
│  elephant.ai Gateway (Go server)            │
│  toolregistry → node host tools registered  │
│  as dynamic tools with "node:" prefix       │
└─────────────────────────────────────────────┘
```

### 7.4 设计

#### 7.4.1 Companion App（SwiftUI）

技术栈：Swift + SwiftUI，Xcode 项目独立于 Go codebase，单独仓库或 `macos/` 顶级目录。

核心组件：

| 组件 | 职责 |
|---|---|
| `AppDelegate` | Menu bar 常驻，状态图标，快捷键（全局 hotkey 唤起） |
| `PermissionManager` | 封装 TCC 权限请求（屏幕录制、麦克风、辅助功能、完全磁盘访问） |
| `NodeHostServer` | 本地 HTTP server，暴露 macOS 原生工具 API |
| `GatewayConnector` | 与 Go Gateway 的 WebSocket/HTTP 连接管理 |
| `WebChatView` | WKWebView 嵌入现有 Next.js Web 界面 |
| `SettingsView` | Gateway URL、token、权限状态、自启动配置 |

#### 7.4.2 Node Host API

本地 HTTP server（默认 `localhost:19820`），仅接受来自 Gateway 的已认证请求：

```
POST /api/v1/tools/{tool_name}/execute
Authorization: Bearer <node_token>
Content-Type: application/json

{
  "call_id": "...",
  "arguments": { ... }
}
```

支持的工具：

| 工具 | 描述 | TCC 权限 |
|---|---|---|
| `node:system_run` | 执行本地 shell 命令 | 无（但 Dangerous=true） |
| `node:screen_capture` | 截取当前屏幕 | 屏幕录制 |
| `node:audio_record` | 录制麦克风音频 | 麦克风 |
| `node:ui_automation` | 模拟键鼠操作 | 辅助功能 |
| `node:fs_read` | 读取本地文件 | 完全磁盘访问（可选） |
| `node:fs_write` | 写入本地文件 | 完全磁盘访问（可选） |
| `node:open_app` | 打开指定应用 | 无 |
| `node:clipboard` | 读写剪贴板 | 无 |

#### 7.4.3 Gateway 侧集成

Node host 工具注册为动态工具：

```go
// internal/tools/builtin/nodehost/nodehost.go

type NodeHostConfig struct {
    BaseURL string `yaml:"base_url"` // http://localhost:19820
    Token   string `yaml:"token"`
    Timeout int    `yaml:"timeout_seconds"`
}

// RegisterNodeHostTools 发现并注册 node host 暴露的工具
func RegisterNodeHostTools(registry *toolregistry.Registry, config NodeHostConfig) error {
    // 1. GET /api/v1/tools → 获取 node host 支持的工具列表
    // 2. 为每个工具创建 proxy executor
    // 3. 注册为 dynamic tool，名称前缀 "node:"
    // 4. 标记 Dangerous=true（所有 node 工具需要 approval）
}
```

Node host 工具发现是动态的：Gateway 启动时尝试连接 node host，成功则注册工具，失败则跳过（降级）。连接断开时自动 unregister。

#### 7.4.4 权限管理

```swift
// PermissionManager.swift

class PermissionManager: ObservableObject {
    @Published var screenRecording: PermissionStatus = .unknown
    @Published var microphone: PermissionStatus = .unknown
    @Published var accessibility: PermissionStatus = .unknown
    @Published var fullDiskAccess: PermissionStatus = .unknown

    enum PermissionStatus { case unknown, granted, denied, restricted }

    func requestScreenRecording() { /* CGRequestScreenCaptureAccess() */ }
    func requestMicrophone() { /* AVCaptureDevice.requestAccess(for: .audio) */ }
    func checkAccessibility() { /* AXIsProcessTrusted() */ }
    func checkFullDiskAccess() { /* 尝试读取 ~/Library/Mail 判断 */ }
}
```

权限状态通过 Node Host API 暴露给 Gateway：

```
GET /api/v1/permissions
→ { "screen_recording": "granted", "microphone": "denied", ... }
```

Gateway 在工具调用前检查权限状态，权限缺失时返回明确的错误引导而非静默失败。

#### 7.4.5 降级策略

| 场景 | 降级行为 |
|---|---|
| Node host 未运行 | node:* 工具不可用，不影响其他工具 |
| 权限被拒绝 | 返回错误 + 引导用户到"系统设置 → 隐私与安全" |
| 网络不可达 | 使用本地缓存的上次权限状态，工具调用超时后报错 |
| 旧版 companion app | Gateway 检查 node host API version，不兼容时提示升级 |

#### 7.4.6 安全

- Node host **仅监听 localhost**，不暴露到网络。
- Token 由 companion app 生成，存储在 macOS Keychain，Gateway 侧从配置文件读取。
- 所有 node:* 工具标记 `Dangerous=true`，强制走 approval gate。
- Node host 请求日志写入 companion app 的本地日志目录。

### 7.5 配置

`configs/nodehost.yaml`（Gateway 侧）：

```yaml
version: 1
nodehost:
  enabled: false              # 默认关闭，用户显式开启
  base_url: "http://localhost:19820"
  token_file: "~/.alex/nodehost_token"
  timeout_seconds: 30
  reconnect_interval_seconds: 60
  tools:
    system_run:
      enabled: true
      max_timeout_seconds: 120
    screen_capture:
      enabled: true
    audio_record:
      enabled: true
      max_duration_seconds: 300
    ui_automation:
      enabled: true
    fs_read:
      enabled: true
    fs_write:
      enabled: true
```

### 7.6 改动范围

| 位置 | 改动 |
|---|---|
| `macos/` | **新增顶级目录** — SwiftUI companion app Xcode 项目 |
| `internal/tools/builtin/nodehost/` | **新增** — Node host proxy tools + 动态注册 |
| `internal/toolregistry/registry.go` | 支持 node host 工具动态注册/注销 |
| `internal/config/file_config.go` | 新增 `NodeHostFileConfig` |
| `internal/di/container_builder.go` | 条件初始化 node host connector |
| `configs/nodehost.yaml` | **新增** — 默认配置 |

### 7.7 测试

- **Gateway 侧**：
  - 单元：proxy executor 的请求序列化、错误处理、timeout。
  - 集成：mock node host server → 工具注册 → 执行 → 结果返回。
  - 降级：node host 不可达时工具自动 unregister，恢复后自动 register。
- **Companion app 侧**：
  - UI 测试：权限状态显示、设置界面。
  - 集成：node host server 启动 → Gateway 连接 → 工具调用 e2e。
  - 权限：TCC 权限请求流程（需要真机测试）。

---

## 8. Observability 增量

在现有 observability 框架上增加以下指标（不新建框架）：

| 指标 | 类型 | 来源 |
|---|---|---|
| `tool.policy.denied_total` | Counter | policyAwareRegistry.Get 拒绝时 |
| `tool.policy.profile_selected` | Counter + label | profile 选择时 |
| `event.bus.publish_total` | Counter + label(event_type) | bus.Publish |
| `event.bus.handler_error_total` | Counter + label(handler_name) | handler 失败时 |
| `event.bus.handler_latency_seconds` | Histogram | handler 执行时间 |
| `memory.flush.saved_total` | Counter | flush handler 成功写入条数 |
| `memory.flush.skipped_dedup_total` | Counter | 重复跳过条数 |
| `memory.daily_summary.generated` | Counter | 日汇总成功生成次数 |
| `memory.longterm.facts_count` | Gauge | MEMORY.md 中事实条数 |
| `scheduler.job.run_total` | Counter + label(status) | 执行完成时 |
| `scheduler.job.consec_failures` | Gauge + label(job_id) | 状态更新时 |
| `nodehost.connection.status` | Gauge (0/1) | Gateway 与 node host 的连接状态 |
| `nodehost.tool.call_total` | Counter + label(tool_name, status) | node host 工具调用 |
| `nodehost.tool.latency_seconds` | Histogram + label(tool_name) | node host 工具调用延迟 |
| `nodehost.permission.status` | Gauge + label(permission_name) | 各权限状态 (0=denied,1=granted) |

告警规则（建议）：
- `scheduler.job.consec_failures >= 3` → 通知
- `event.bus.handler_error_total` rate > 10/min → 通知
- `memory.flush.saved_total` 连续 24h = 0 且有 compact 事件 → 通知
- `nodehost.connection.status == 0` 持续 > 5min（仅 enabled 时）→ 通知
- `nodehost.permission.status == 0` 且对应工具被调用 → 通知

---

## 9. 路线图

### Phase 0：基础设施（前置）
- 新增 `internal/events/` 包，实现 Bus。
- 现有 5 个 hook 迁移为 bus subscriber（保持行为不变）。
- **退出条件**：所有现有 hook 测试通过，bus 单元测试覆盖。

### Phase 1：工具治理
- 实现 ToolPolicy + policyAwareRegistry。
- 为所有 builtin tools 补充 group tags。
- 配置 default/lark/web 三个 profile。
- **退出条件**：Lark 通道 `tool.policy.denied_total > 0` 且无误拒；CLI 全量通过。

### Phase 2：Memory Flush + 记忆结构化
- 实现 MemoryFlushHook + context.compact 事件。
- 集成到 AutoCompact 路径。
- 实现 LayeredFileStore：entries/ + daily/ + MEMORY.md。
- 旧格式自动迁移。
- **退出条件**：compact 后 `memory.flush.saved_total > 0`；session.ended 后 daily/ 文件生成；recall 优先返回长期事实。

### Phase 3：Scheduler 增强
- 实现 JobStore + 状态跟踪 + 冷却。
- 实现 scheduler_create/list/delete/pause 工具。
- **退出条件**：动态创建的 job 重启后恢复；连续失败自动暂停且有告警。

### Phase 4：稳定化
- 全通道 E2E 测试（Delta 1–4）。
- 负载测试：event bus 高频发布、concurrent job 执行。
- Observability dashboard。
- **退出条件**：全量 lint + test 通过；无 P0 bug 持续 3 天。

### Phase 5：macOS Companion App
- **5a — Node Host 协议**：定义 API 契约、Gateway 侧 proxy executor、mock 测试。
- **5b — Companion App MVP**：menu bar app + node host server + system.run + screen_capture。
- **5c — 权限与降级**：TCC broker、权限状态上报、降级策略。
- **5d — 完整工具集**：audio_record、ui_automation、fs_local、clipboard。
- **退出条件**：Gateway → node host 全链路 e2e 通过；权限拒绝时降级正常；companion app 可通过 notarization。

---

## 10. 风险与缓解

| 风险 | 缓解 |
|---|---|
| Event bus 引入延迟 | 同步 Publish 用于关键路径（flush），其余用 PublishAsync |
| Flush 写入低价值记忆 | 容量上限（5 条/次）+ 重复检测 + auto_flush 专用 retention |
| Policy 误拒工具 | deny-first 语义明确；默认 profile 全放开；误拒写 metric 可追踪 |
| Job 状态文件损坏 | YAML 写入用 atomic write（write-tmp + rename）；启动时校验 |
| Flush handler 阻塞 compact | handler timeout 10s；超时后 compact 继续执行 |
| 现有 hook 迁移回归 | 迁移期保持双通道调用；全部测试通过后移除旧路径 |
| 日汇总/长期提炼质量低 | 提炼逻辑基于规则（重复出现 ≥ 3 次），不依赖 LLM；人工可编辑 MEMORY.md 修正 |
| 旧格式迁移数据丢失 | 迁移是 move（不删除源），失败时保留原文件；启动日志记录迁移结果 |
| Companion app 审核/签名 | Apple Developer Program + notarization；MVP 阶段可用 ad-hoc 签名内部分发 |
| Node host 安全 | 仅 localhost、token auth、所有工具 Dangerous=true、Keychain 存储 token |
| Swift/Go 跨仓库协调 | Node host API 契约版本化（`/api/v1/`）；Gateway 检查 version 兼容性 |

---

## 11. 不在本方案范围

以下内容独立评估，不纳入本轮：

- **分布式调度**（claim + heartbeat）：当前单实例足够，多实例部署时再设计。
- **RAG / 向量检索优化**：现有 hybrid store 已支持，按需开启即可。
- **iOS / iPadOS companion**：先验证 macOS companion 的价值，再考虑移动端。
- **LLM 驱动的记忆提炼**：当前长期事实提炼用规则，后续可引入 LLM 做语义摘要，但成本和质量需要单独评估。
