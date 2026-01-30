# 主动性 AI 架构建设方案

> **Author:** cklxx
> **Date:** 2026-01-30
> **Status:** In Progress
> **Scope:** elephant.ai 主动性能力升级 — 从被动响应到主动行动

---

## 实施进度（2026-01-30）

- **已落地：Layer 1 自动记忆** — 通过 Hook Registry + MemoryPolicy 实现任务前自动召回 / 任务后自动捕获，统一在 Coordinator 层注入，配置集中在 `runtime.proactive`。
- **已落地：Layer 2 RAG 融合基础** — 引入 `HybridStore`（关键词 + 向量搜索）与 RRF 融合、metadata 过滤与最小相似度阈值。
- **已落地：Layer 2 自动技能体系基础** — Skill frontmatter 扩展 + Matcher + Chain 解析 + Cache + Feedback Store + 自动注入系统提示词。
- **已落地：迭代间上下文刷新** — ReAct 迭代内注入 recall 结果并发出 `proactive.context.refresh` 事件。
- **已落地：技能学习数据采集** — WorkflowTrace 写入记忆 + SkillLearner 生成建议技能模板。
- **待补充：Layer 2/3 余项** — 记忆生命周期管理、Scheduler、PatternRecognizer、InitiativeEvaluator、AttentionEngine。

## 一、背景与现状分析

### 1.1 当前能力矩阵

elephant.ai 已经具备成熟的 **被动执行** 基础设施：

| 能力 | 状态 | 实现位置 |
|------|------|----------|
| ReAct 循环 (Think → Act → Observe) | ✅ 完成 | `internal/agent/domain/react/` |
| 多 LLM 提供商 (OpenAI, Claude, ARK, DeepSeek, Ollama) | ✅ 完成 | `internal/llm/` |
| 持久化记忆 (File/Postgres) | ✅ 完成 | `internal/memory/` |
| 渠道集成 (Lark, WeChat, CLI, Web) | ✅ 完成 | `internal/channels/`, `internal/server/` |
| Markdown 驱动的技能系统 | ✅ 完成 | `internal/skills/`, `skills/` |
| 审批门控与安全模式 | ✅ 完成 | `internal/toolregistry/`, `internal/agent/presets/` |
| 上下文组装与窗口压缩 | ✅ 完成 | `internal/context/` |
| 后台异步任务与子 Agent | ✅ 完成 | `internal/agent/domain/react/background.go` |
| RAG 检索器 (chromem-go) | ⚠️ 已实现但未集成 | `internal/rag/` |
| 用户画像与驱动力模型 | ⚠️ 仅系统提示词层面 | `internal/agent/ports/user_persona.go` |

### 1.2 核心差距：**推送能力就绪，拉取决策缺失**

当前系统的关键问题是：

```
用户发消息 → 构建上下文 → ReAct 循环 → 输出结果
     ↑                                      ↓
     └──────────── 等待下一条消息 ────────────┘
```

系统是 **事件驱动但不主动**：
- 记忆需要 LLM 手动调用 `memory_recall` / `memory_write` 工具
- 技能在系统提示词中静态索引，不会根据任务动态激活
- 用户驱动力（`TopDrives`, `InitiativeSources`）写在 Persona 中但无运行时评估
- 上下文只在任务开始时构建一次，迭代间不刷新
- 无定时触发、无模式识别、无跨会话学习

**一句话总结：系统有记忆的手，但没有记忆的脑。有技能的库存，但没有技能的直觉。**

---

## 二、目标架构：主动性分层模型

### 2.1 设计原则

1. **主动但安全** — 主动行为必须有明确的信任边界和审批门控
2. **渐进式构建** — 三层递进，每层独立可交付，不依赖后续层
3. **上下文工程优于提示词黑魔法** — 通过结构化的信息注入实现主动性，而非复杂提示词
4. **可观测可回溯** — 每个主动行为产生结构化事件，可审计、可回放
5. **最小侵入** — 利用已有扩展点，避免重写核心循环

### 2.2 三层架构

```
┌───────────────────────────────────────────────────────────────────┐
│ Layer 3: 自主决策 (Autonomous Initiative)                         │
│ - 定时任务触发器 (Scheduled Triggers)                              │
│ - 跨会话模式识别 (Cross-Session Pattern Recognition)               │
│ - 驱动力评估器 (Drive-Based Evaluator)                            │
│ - 主动通知与推荐 (Proactive Notifications)                         │
├───────────────────────────────────────────────────────────────────┤
│ Layer 2: 智能上下文 (Intelligent Context)                         │
│ - 语义记忆检索 (Semantic Memory + RAG Fusion)                     │
│ - 动态技能激活 (Dynamic Skill Activation)                         │
│ - 迭代间上下文刷新 (Mid-Loop Context Refresh)                     │
│ - 自动记忆生命周期 (Auto Memory Lifecycle)                         │
├───────────────────────────────────────────────────────────────────┤
│ Layer 1: 自动记忆 (Automatic Memory)                              │
│ - 任务前自动记忆召回 (Pre-Task Memory Recall)                      │
│ - 任务后自动记忆写入 (Post-Task Memory Write)                      │
│ - 全渠道记忆统一 (All-Channel Memory Unification)                  │
│ - 事件驱动钩子系统 (Event-Driven Hook System)                      │
├───────────────────────────────────────────────────────────────────┤
│ Foundation: 现有基础设施                                           │
│ ReAct Loop │ Memory Store │ Context Manager │ Event System        │
│ Skills │ Tools │ Channels │ Observability                        │
└───────────────────────────────────────────────────────────────────┘
```

---

## 三、Layer 1：自动记忆（最高价值，最低风险）

### 3.1 任务前自动记忆召回

**目标：** 每次任务执行前，系统自动从记忆库中检索相关上下文，注入到系统提示词中。

**扩展点：** `internal/agent/app/preparation/service.go::Prepare()`

**设计：**

```go
// internal/agent/app/preparation/memory_enricher.go

type MemoryEnricher struct {
    memoryService *memory.Service
    maxRecalls    int  // default: 5
}

func (e *MemoryEnricher) Enrich(ctx context.Context, task string, userID string) ([]memory.Entry, error) {
    // 1. 从任务文本提取关键词
    keywords := extractKeywords(task)

    // 2. 查询记忆库
    entries, err := e.memoryService.Recall(ctx, memory.Query{
        UserID:   userID,
        Keywords: keywords,
        Limit:    e.maxRecalls,
    })

    // 3. 格式化为上下文片段
    return entries, err
}
```

**注入位置：** `Prepare()` 中 `BuildWindow()` 之后，将记忆注入 `MetaContext.Memories`

```go
// preparation/service.go::Prepare() 中增加
memories, _ := s.memoryEnricher.Enrich(ctx, task.Input, session.UserID)
if len(memories) > 0 {
    window.MetaContext.Memories = formatMemories(memories)
}
```

**事件：** 发射 `ProactiveMemoryRecalledEvent`，包含召回的记忆数量和关键词匹配度

### 3.2 任务后自动记忆写入

**目标：** 每次任务完成后，自动提取关键决策和学习点，写入记忆库。

**扩展点：** `internal/agent/domain/react/runtime.go` 的 `finalize()` 阶段

**设计：**

```go
// internal/agent/domain/react/memory_capture.go

type MemoryCapturer struct {
    memoryService *memory.Service
    summarizer    llm.Client  // 小模型做摘要
}

func (c *MemoryCapturer) CaptureFromTask(ctx context.Context, state *TaskState) error {
    // 1. 提取任务摘要：目标 + 使用的工具 + 最终结果
    summary := c.extractSummary(state)

    // 2. 识别关键决策和学习点
    keywords := c.extractKeywords(state)
    slots := map[string]string{
        "task_type": classifyTask(state),
        "outcome":   state.FinalStatus,
    }

    // 3. 写入记忆（非阻塞）
    go c.memoryService.Save(ctx, memory.Entry{
        UserID:   state.UserID,
        Content:  summary,
        Keywords: keywords,
        Slots:    slots,
    })

    return nil
}
```

**过滤规则：**
- 仅在任务包含工具调用时才自动写入（纯对话不写）
- 去重：与最近 5 条记忆比较，相似度 > 0.8 则合并而非新增
- 可通过配置 `memory.auto_capture: false` 禁用

### 3.3 全渠道记忆统一

**现状：** 仅 Lark 渠道自动存储消息到记忆。CLI/Web 无自动存储。

**方案：** 在 Coordinator 层统一处理，而非各渠道分别实现。

```go
// internal/agent/app/coordinator/coordinator.go::ExecuteTask()

func (c *Coordinator) ExecuteTask(ctx context.Context, task Task) (*Result, error) {
    // 1. 任务前：自动记忆召回
    memories := c.memoryEnricher.Enrich(ctx, task)
    c.injectMemories(task, memories)

    // 2. 执行任务
    result, err := c.engine.SolveTask(ctx, task)

    // 3. 任务后：自动记忆捕获
    c.memoryCapturer.CaptureFromTask(ctx, result)

    return result, err
}
```

**优势：** 一处实现，全渠道生效。Lark 的 `memory.go` 可简化为仅处理渠道特有的消息格式转换。

### 3.4 事件驱动钩子系统

**目标：** 建立统一的事件监听接口，为 Layer 2/3 提供扩展基础。

**设计：**

```go
// internal/agent/domain/hooks/proactive.go

// ProactiveHook 定义主动性钩子接口
type ProactiveHook interface {
    // OnTaskStart 在任务执行前触发
    OnTaskStart(ctx context.Context, task *Task) []Injection

    // OnIterationStart 在每次迭代前触发
    OnIterationStart(ctx context.Context, iteration int, state *IterationState) []Injection

    // OnToolCompleted 在工具执行完成后触发
    OnToolCompleted(ctx context.Context, event *ToolCompletedEvent) []Injection

    // OnTaskCompleted 在任务完成后触发
    OnTaskCompleted(ctx context.Context, result *TaskResult) error
}

// Injection 描述要注入到上下文中的内容
type Injection struct {
    Type    InjectionType  // MemoryRecall, SkillActivation, Suggestion, Warning
    Content string
    Source  string         // 来源钩子名称，用于可观测性
    Priority int           // 优先级，决定注入顺序
}

// HookRegistry 管理所有注册的钩子
type HookRegistry struct {
    hooks []ProactiveHook
}

func (r *HookRegistry) RunOnTaskStart(ctx context.Context, task *Task) []Injection {
    var injections []Injection
    for _, hook := range r.hooks {
        injections = append(injections, hook.OnTaskStart(ctx, task)...)
    }
    sort.Slice(injections, func(i, j int) bool {
        return injections[i].Priority > injections[j].Priority
    })
    return injections
}
```

**注册方式：** 在 DI 容器 (`internal/di/`) 中注册钩子实例

**首批钩子：**
1. `MemoryRecallHook` — 任务前自动召回
2. `MemoryCaptureHook` — 任务后自动写入
3. `MetricsHook` — 主动性行为的可观测性上报

---

## 四、Layer 2：智能上下文

### 4.1 语义记忆检索 (RAG Fusion)

**现状：** `internal/rag/` 已有向量检索实现 (chromem-go)，但未与记忆系统集成。

**目标：** 将 RAG 能力融入记忆召回，从关键词匹配升级到语义搜索。

**设计：**

```go
// internal/memory/hybrid_store.go

type HybridStore struct {
    keywordStore Store           // 现有文件/Postgres 存储
    vectorStore  rag.VectorStore // chromem-go 向量存储
    embedder     rag.Embedder    // 嵌入模型
    alpha        float64         // 混合权重 (0=纯关键词, 1=纯语义)
}

func (s *HybridStore) Recall(ctx context.Context, query Query) ([]Entry, error) {
    // 1. 关键词检索
    keywordResults, _ := s.keywordStore.Recall(ctx, query)

    // 2. 语义检索
    embedding, _ := s.embedder.Embed(ctx, query.Text)
    vectorResults, _ := s.vectorStore.Search(ctx, embedding, query.Limit)

    // 3. 混合排序 (Reciprocal Rank Fusion)
    merged := reciprocalRankFusion(keywordResults, vectorResults, s.alpha)

    return merged[:query.Limit], nil
}
```

**记忆索引：** 在 `Save()` 时同步生成嵌入并写入向量存储

**配置：**
```yaml
memory:
  store: hybrid            # file | postgres | hybrid
  hybrid:
    alpha: 0.6             # 语义搜索权重
    embedder: ark           # 嵌入模型提供商
    min_similarity: 0.7     # 最低相似度阈值
```

### 4.2 自动技能体系（Automatic Skills）

当前技能系统的根本问题：**技能是货架上的商品，用户（LLM）需要自己逛货架挑选**。主动性 AI 需要的是 **智能导购** — 根据场景自动把正确的技能送到手上。

#### 4.2.1 现状分析

| 维度 | 当前状态 | 问题 |
|------|----------|------|
| **发现** | `skills.Load()` 从目录递归扫描 | 每次 `skills` 工具调用都重新加载，无缓存 |
| **索引** | `IndexMarkdown()` 生成目录列表注入系统提示词 | 全部 9 个技能始终展示，浪费 token |
| **匹配** | LLM 自行决定是否调用 `skills({"action":"show"})` | 被动，LLM 经常忽略可用技能 |
| **激活** | LLM 调用 `skills` 工具获取技能内容后自行执行 | 两步调用（先 show 再执行），延迟高 |
| **Frontmatter** | 仅 `name` + `description` 两个字段 | 无触发条件、无前置依赖、无优先级 |
| **反馈** | 无 | 不知道哪些技能被用过、效果如何 |

**数据流现状：**
```
Load(dir) → Library{skills[], byName{}}
    ↓
buildSkillsSection() → IndexMarkdown() → 系统提示词（全量目录）
    ↓
LLM 看到目录 → 可能调用 skills(show) → 获取完整 Body → 自行执行
```

#### 4.2.2 扩展技能 Frontmatter：声明式触发规则

**核心思想：** 将技能从"被动目录条目"升级为"带触发条件的可执行工作流"。

**扩展后的 Frontmatter Schema：**

```yaml
---
name: deep-research
description: "多轮检索、多源验证、证据追踪的深度调研技能"

# === 新增：触发规则 ===
triggers:
  # 意图模式匹配（正则或关键词列表）
  intent_patterns:
    - "调研|研究|分析.*趋势|对比.*方案"
    - "帮我查.*资料|了解.*行业"
    - "research|investigate|analyze"

  # 工具信号：当特定工具被调用时，可能需要此技能
  tool_signals:
    - web_search    # 用户已在搜索，可能需要深度调研
    - web_fetch     # 用户在抓取网页，可能需要结构化调研

  # 上下文信号：当上下文中出现特定 slot/keyword 时触发
  context_signals:
    keywords: ["竞品", "行业", "趋势", "方案对比"]
    slots:
      task_type: ["research", "analysis", "comparison"]

  # 置信度阈值：低于此值不自动激活（0-1）
  confidence_threshold: 0.6

# === 新增：技能元数据 ===
priority: 8              # 优先级 1-10，冲突时高优先级胜出
exclusive_group: research # 同组技能互斥，不同时激活
prerequisites: []         # 前置依赖技能（执行前需先完成）
max_tokens: 2000          # 注入系统提示词的 token 预算上限
cooldown: 300             # 同一会话内再次激活的冷却时间（秒）

# === 新增：输出约束 ===
output:
  format: markdown         # 期望输出格式
  artifacts: true          # 是否产出 artifact
  artifact_type: document  # artifact 类型
---
```

**实现：** 扩展 `internal/skills/skills.go` 的 `Skill` 结构体

```go
// internal/skills/skills.go — 扩展 Skill 定义

type Skill struct {
    Name        string     `yaml:"name"`
    Description string     `yaml:"description"`
    Title       string     `yaml:"-"`             // 从 H1 提取
    Body        string     `yaml:"-"`             // Markdown body
    SourcePath  string     `yaml:"-"`

    // 新增：触发规则
    Triggers    *SkillTriggers `yaml:"triggers,omitempty"`

    // 新增：技能元数据
    Priority       int    `yaml:"priority,omitempty"`        // default: 5
    ExclusiveGroup string `yaml:"exclusive_group,omitempty"`
    Prerequisites  []string `yaml:"prerequisites,omitempty"`
    MaxTokens      int    `yaml:"max_tokens,omitempty"`      // default: 2000
    Cooldown       int    `yaml:"cooldown,omitempty"`         // seconds

    // 新增：输出约束
    Output *SkillOutput `yaml:"output,omitempty"`
}

type SkillTriggers struct {
    IntentPatterns      []string            `yaml:"intent_patterns,omitempty"`
    ToolSignals         []string            `yaml:"tool_signals,omitempty"`
    ContextSignals      *ContextSignals     `yaml:"context_signals,omitempty"`
    ConfidenceThreshold float64             `yaml:"confidence_threshold,omitempty"` // default: 0.5
}

type ContextSignals struct {
    Keywords []string            `yaml:"keywords,omitempty"`
    Slots    map[string][]string `yaml:"slots,omitempty"`
}

type SkillOutput struct {
    Format       string `yaml:"format,omitempty"`
    Artifacts    bool   `yaml:"artifacts,omitempty"`
    ArtifactType string `yaml:"artifact_type,omitempty"`
}
```

**向后兼容：** `Triggers` 为 `nil` 时，技能退化为当前的纯目录模式，不参与自动匹配。

#### 4.2.3 技能匹配引擎（SkillMatcher）

**核心架构：** 多信号融合的三阶段匹配

```
任务输入 + 上下文状态
    ↓
Stage 1: 意图模式匹配（正则 + 关键词）→ 候选集
    ↓
Stage 2: 上下文信号增强（工具历史 + slot 匹配）→ 加权评分
    ↓
Stage 3: 冲突解决（互斥组 + 优先级 + 冷却）→ 最终激活集
```

**实现：**

```go
// internal/skills/matcher.go

type SkillMatcher struct {
    library        *Library
    compiledRegex  map[string][]*regexp.Regexp  // 预编译的正则缓存
    cooldownTracker map[string]time.Time         // 技能冷却追踪
    mu             sync.RWMutex
}

type MatchResult struct {
    Skill      Skill
    Score      float64         // 综合匹配分 (0-1)
    Signals    []MatchSignal   // 匹配到的信号来源
    Injected   bool            // 是否已注入上下文
}

type MatchSignal struct {
    Type   string  // "intent_pattern", "tool_signal", "context_keyword", "context_slot"
    Detail string  // 具体匹配内容
    Weight float64 // 该信号的权重贡献
}

func NewSkillMatcher(library *Library) *SkillMatcher {
    m := &SkillMatcher{
        library:        library,
        compiledRegex:  make(map[string][]*regexp.Regexp),
        cooldownTracker: make(map[string]time.Time),
    }
    // 启动时预编译所有正则，避免热路径开销
    for _, skill := range library.List() {
        if skill.Triggers != nil {
            var compiled []*regexp.Regexp
            for _, pattern := range skill.Triggers.IntentPatterns {
                if re, err := regexp.Compile("(?i)" + pattern); err == nil {
                    compiled = append(compiled, re)
                }
            }
            m.compiledRegex[skill.Name] = compiled
        }
    }
    return m
}

// Match 返回与当前任务最匹配的技能列表
func (m *SkillMatcher) Match(ctx MatchContext) []MatchResult {
    var candidates []MatchResult

    for _, skill := range m.library.List() {
        if skill.Triggers == nil {
            continue // 无触发规则的技能不参与自动匹配
        }

        result := m.scoreSkill(skill, ctx)
        threshold := skill.Triggers.ConfidenceThreshold
        if threshold == 0 {
            threshold = 0.5
        }
        if result.Score >= threshold {
            candidates = append(candidates, result)
        }
    }

    // 冲突解决
    resolved := m.resolveConflicts(candidates)

    return resolved
}

// scoreSkill 多信号融合评分
func (m *SkillMatcher) scoreSkill(skill Skill, ctx MatchContext) MatchResult {
    result := MatchResult{Skill: skill}
    var totalWeight float64

    // Stage 1: 意图模式匹配 (权重 0.5)
    if regexes, ok := m.compiledRegex[skill.Name]; ok {
        for _, re := range regexes {
            if re.MatchString(ctx.TaskInput) {
                result.Signals = append(result.Signals, MatchSignal{
                    Type: "intent_pattern", Detail: re.String(), Weight: 0.5,
                })
                totalWeight += 0.5
                break // 一个模式匹配即可
            }
        }
    }

    // Stage 2a: 工具信号匹配 (权重 0.25)
    if skill.Triggers.ToolSignals != nil && len(ctx.RecentTools) > 0 {
        for _, signal := range skill.Triggers.ToolSignals {
            for _, tool := range ctx.RecentTools {
                if tool == signal {
                    result.Signals = append(result.Signals, MatchSignal{
                        Type: "tool_signal", Detail: signal, Weight: 0.25,
                    })
                    totalWeight += 0.25
                    break
                }
            }
        }
    }

    // Stage 2b: 上下文关键词匹配 (权重 0.15)
    if cs := skill.Triggers.ContextSignals; cs != nil {
        matchedKeywords := 0
        for _, kw := range cs.Keywords {
            if strings.Contains(strings.ToLower(ctx.TaskInput), strings.ToLower(kw)) {
                matchedKeywords++
            }
        }
        if len(cs.Keywords) > 0 && matchedKeywords > 0 {
            ratio := float64(matchedKeywords) / float64(len(cs.Keywords))
            weight := 0.15 * ratio
            result.Signals = append(result.Signals, MatchSignal{
                Type: "context_keyword",
                Detail: fmt.Sprintf("%d/%d keywords", matchedKeywords, len(cs.Keywords)),
                Weight: weight,
            })
            totalWeight += weight
        }

        // Stage 2c: Slot 匹配 (权重 0.1)
        if cs.Slots != nil {
            for slotKey, slotValues := range cs.Slots {
                if ctxValue, ok := ctx.Slots[slotKey]; ok {
                    for _, sv := range slotValues {
                        if ctxValue == sv {
                            result.Signals = append(result.Signals, MatchSignal{
                                Type: "context_slot",
                                Detail: fmt.Sprintf("%s=%s", slotKey, sv),
                                Weight: 0.1,
                            })
                            totalWeight += 0.1
                            break
                        }
                    }
                }
            }
        }
    }

    result.Score = min(totalWeight, 1.0)
    return result
}

// resolveConflicts 处理互斥组、优先级和冷却
func (m *SkillMatcher) resolveConflicts(candidates []MatchResult) []MatchResult {
    // 1. 移除冷却中的技能
    m.mu.RLock()
    filtered := make([]MatchResult, 0, len(candidates))
    for _, c := range candidates {
        if lastUsed, ok := m.cooldownTracker[c.Skill.Name]; ok {
            cooldown := time.Duration(c.Skill.Cooldown) * time.Second
            if cooldown > 0 && time.Since(lastUsed) < cooldown {
                continue // 冷却中，跳过
            }
        }
        filtered = append(filtered, c)
    }
    m.mu.RUnlock()

    // 2. 互斥组内保留最高优先级
    groupWinners := make(map[string]MatchResult)
    var noGroup []MatchResult
    for _, c := range filtered {
        if c.Skill.ExclusiveGroup == "" {
            noGroup = append(noGroup, c)
            continue
        }
        existing, ok := groupWinners[c.Skill.ExclusiveGroup]
        if !ok || c.Skill.Priority > existing.Skill.Priority {
            groupWinners[c.Skill.ExclusiveGroup] = c
        }
    }

    result := noGroup
    for _, winner := range groupWinners {
        result = append(result, winner)
    }

    // 3. 按分数排序，限制最多 3 个
    sort.Slice(result, func(i, j int) bool {
        return result[i].Score > result[j].Score
    })
    if len(result) > 3 {
        result = result[:3]
    }

    return result
}

// MatchContext 匹配所需的上下文信息
type MatchContext struct {
    TaskInput   string            // 用户输入的任务文本
    RecentTools []string          // 最近调用过的工具名
    Slots       map[string]string // 当前上下文的 slot 值
    SessionID   string            // 用于冷却追踪
}
```

#### 4.2.4 技能自动注入（Context-Aware Injection）

**现状：** `buildSkillsSection()` 调用 `IndexMarkdown()` 输出全量目录，LLM 需自己调 `skills(show)` 获取内容。

**目标：** 匹配成功的技能直接将 **完整 Body** 注入系统提示词，LLM 无需额外工具调用即可执行技能。

**扩展点：** `internal/context/manager_prompt.go::buildSkillsSection()`

**设计：**

```go
// internal/context/manager_prompt.go — 改造 buildSkillsSection

func buildSkillsSection(logger logging.Logger, task string, recentTools []string) string {
    library, err := skills.DefaultLibrary()
    if err != nil {
        logging.OrNop(logger).Warn("Failed to load skills: %v", err)
        return ""
    }

    matcher := skills.NewSkillMatcher(library)
    matches := matcher.Match(skills.MatchContext{
        TaskInput:   task,
        RecentTools: recentTools,
    })

    var sb strings.Builder

    // Part 1: 自动激活的技能（完整 Body 注入）
    if len(matches) > 0 {
        sb.WriteString("# Activated Skills\n\n")
        sb.WriteString("The following skills have been automatically loaded based on your task. ")
        sb.WriteString("Follow their workflow instructions directly.\n\n")

        for _, m := range matches {
            sb.WriteString(fmt.Sprintf("## Skill: %s (confidence: %.0f%%)\n\n",
                m.Skill.Name, m.Score*100))
            sb.WriteString(m.Skill.Body)
            sb.WriteString("\n\n---\n\n")
        }
    }

    // Part 2: 未激活技能的精简目录（仅 name + description）
    remaining := filterOutActivated(library.List(), matches)
    if len(remaining) > 0 {
        sb.WriteString("# Other Available Skills\n\n")
        sb.WriteString("Use `skills({\"action\":\"show\",\"name\":\"...\"})` to load:\n\n")
        for _, s := range remaining {
            sb.WriteString(fmt.Sprintf("- `%s` — %s\n", s.Name, s.Description))
        }
    }

    return sb.String()
}
```

**Token 预算控制：**

```go
// 确保注入的技能内容不超过总 token 预算
func (m *SkillMatcher) fitTokenBudget(matches []MatchResult, budget int) []MatchResult {
    var result []MatchResult
    used := 0
    for _, match := range matches {
        maxTokens := match.Skill.MaxTokens
        if maxTokens == 0 {
            maxTokens = 2000 // default
        }
        bodyTokens := estimateTokens(match.Skill.Body)
        actual := min(bodyTokens, maxTokens)
        if used+actual > budget {
            break // 超预算，停止注入
        }
        used += actual
        result = append(result, match)
    }
    return result
}
```

**配置：**
```yaml
skills:
  auto_activation:
    enabled: true
    max_activated: 3          # 单次最多自动激活技能数
    token_budget: 4000        # 技能注入的总 token 预算
    fallback_to_index: true   # 未匹配的技能仍显示目录
```

#### 4.2.5 技能组合与编排（Skill Composition）

**场景：** 复杂任务需要多个技能协同。例如"调研竞品并做成 PPT" = `deep-research` → `research-briefing` → `ppt-deck`。

**设计：** 声明式技能链（Skill Chain）

```yaml
# skills/competitive-analysis/SKILL.md frontmatter
---
name: competitive-analysis
description: "竞品调研 + 结构化报告 + PPT 输出的端到端工作流"
triggers:
  intent_patterns:
    - "竞品分析|竞品调研|competitor.*analysis"
priority: 9
exclusive_group: research

# 新增：技能链定义
chain:
  - skill: deep-research
    output_as: research_data    # 输出命名，供后续步骤引用
    params:
      depth: comprehensive
      sources: ["web", "academic"]

  - skill: research-briefing
    input_from: research_data   # 引用前一步输出
    output_as: briefing

  - skill: ppt-deck
    input_from: briefing
    params:
      template: competitive_analysis
---
# 竞品分析端到端工作流
...
```

**实现：**

```go
// internal/skills/chain.go

type SkillChain struct {
    Steps []ChainStep `yaml:"chain"`
}

type ChainStep struct {
    SkillName string            `yaml:"skill"`
    InputFrom string            `yaml:"input_from,omitempty"` // 前一步的 output_as
    OutputAs  string            `yaml:"output_as,omitempty"`  // 本步输出命名
    Params    map[string]string `yaml:"params,omitempty"`     // 步骤参数
}

// ResolveChain 将技能链展开为完整的执行指令
func (l *Library) ResolveChain(chain SkillChain) (string, error) {
    var sb strings.Builder
    sb.WriteString("## Multi-Step Workflow\n\n")
    sb.WriteString("Execute the following steps in order. ")
    sb.WriteString("Each step's output feeds into the next.\n\n")

    for i, step := range chain.Steps {
        skill, ok := l.Get(step.SkillName)
        if !ok {
            return "", fmt.Errorf("chain step %d references unknown skill: %s", i, step.SkillName)
        }

        sb.WriteString(fmt.Sprintf("### Step %d: %s\n\n", i+1, skill.Name))

        if step.InputFrom != "" {
            sb.WriteString(fmt.Sprintf("**Input:** Use output from `%s`\n\n", step.InputFrom))
        }
        if step.OutputAs != "" {
            sb.WriteString(fmt.Sprintf("**Output:** Save as `%s` for subsequent steps\n\n", step.OutputAs))
        }

        sb.WriteString(skill.Body)
        sb.WriteString("\n\n---\n\n")
    }

    return sb.String(), nil
}
```

**与钩子系统的集成：**

技能链通过 `OnTaskStart` 钩子解析，展开后的完整工作流注入系统提示词。LLM 收到的是一份结构化的多步执行指令，而非多个独立技能。

#### 4.2.6 技能自动学习与生成（Skill Learning）

**目标：** 从反复出现的工具调用序列中，自动提取并建议新技能。

**这是 Layer 3 模式识别（5.2）中 `workflow_sequence` 模式的具体实现。**

**设计：**

```go
// internal/skills/learner.go

type SkillLearner struct {
    memoryService *memory.Service
    library       *Library
    minOccurrence int     // 工作流重复次数阈值 (default: 3)
    minSteps      int     // 最少步骤数 (default: 2)
}

// WorkflowTrace 记录一次任务中的工具调用序列
type WorkflowTrace struct {
    TaskID    string
    UserID    string
    Tools     []ToolStep
    Outcome   string    // success | failure
    CreatedAt time.Time
}

type ToolStep struct {
    Name      string
    Arguments map[string]any
    Duration  time.Duration
    Success   bool
}

// AnalyzePatterns 分析工具调用历史，发现可自动化的工作流
func (l *SkillLearner) AnalyzePatterns(ctx context.Context, userID string) []SkillSuggestion {
    // 1. 从记忆中加载工作流追踪
    traces := l.loadTraces(ctx, userID)

    // 2. 提取工具调用序列的频繁子序列
    sequences := extractFrequentSubsequences(traces, l.minOccurrence, l.minSteps)

    // 3. 为每个频繁序列生成技能建议
    var suggestions []SkillSuggestion
    for _, seq := range sequences {
        // 检查是否已有覆盖此序列的技能
        if l.isAlreadyCovered(seq) {
            continue
        }

        suggestions = append(suggestions, SkillSuggestion{
            Name:         generateSkillName(seq),
            Description:  generateDescription(seq),
            ToolSequence: seq.Tools,
            Occurrences:  seq.Count,
            AvgDuration:  seq.AvgDuration,
            SuccessRate:  seq.SuccessRate,
            Confidence:   calculateConfidence(seq),
        })
    }

    return suggestions
}

type SkillSuggestion struct {
    Name         string
    Description  string
    ToolSequence []string
    Occurrences  int
    AvgDuration  time.Duration
    SuccessRate  float64
    Confidence   float64
}

// GenerateSkillFile 将建议转化为 Markdown 技能文件
func (l *SkillLearner) GenerateSkillFile(suggestion SkillSuggestion) string {
    var sb strings.Builder
    sb.WriteString("---\n")
    sb.WriteString(fmt.Sprintf("name: %s\n", suggestion.Name))
    sb.WriteString(fmt.Sprintf("description: \"%s\"\n", suggestion.Description))
    sb.WriteString("triggers:\n")
    sb.WriteString("  intent_patterns: []\n")  // 需要用户补充
    sb.WriteString("  tool_signals:\n")
    for _, tool := range suggestion.ToolSequence {
        sb.WriteString(fmt.Sprintf("    - %s\n", tool))
    }
    sb.WriteString(fmt.Sprintf("priority: 5\n"))
    sb.WriteString("---\n\n")
    sb.WriteString(fmt.Sprintf("# %s\n\n", suggestion.Name))
    sb.WriteString(fmt.Sprintf("*Auto-generated from %d occurrences (success rate: %.0f%%)*\n\n",
        suggestion.Occurrences, suggestion.SuccessRate*100))
    sb.WriteString("## Workflow Steps\n\n")
    for i, tool := range suggestion.ToolSequence {
        sb.WriteString(fmt.Sprintf("%d. Execute `%s`\n", i+1, tool))
    }
    return sb.String()
}
```

**工作流追踪采集：** 在 `MemoryCaptureHook.OnTaskCompleted()` 中记录

```go
// memory_capture.go — 扩展，记录工具调用序列
func (h *MemoryCaptureHook) captureWorkflowTrace(ctx context.Context, result *TaskResult) {
    if len(result.ToolCalls) < 2 {
        return // 单步任务不记录
    }

    trace := WorkflowTrace{
        TaskID:    result.TaskID,
        UserID:    result.UserID,
        Outcome:   result.Status,
        CreatedAt: time.Now(),
    }
    for _, tc := range result.ToolCalls {
        trace.Tools = append(trace.Tools, ToolStep{
            Name:      tc.Name,
            Arguments: tc.Arguments,
            Duration:  tc.Duration,
            Success:   tc.Error == nil,
        })
    }

    h.memoryService.Save(ctx, memory.Entry{
        UserID:   result.UserID,
        Content:  formatTraceAsContent(trace),
        Keywords: []string{"workflow_trace", trace.Outcome},
        Slots: map[string]string{
            "type":     "workflow_trace",
            "task_id":  result.TaskID,
            "tool_seq": strings.Join(extractToolNames(trace.Tools), "→"),
        },
    })
}
```

**用户交互流程：**

```
1. 系统检测到 "web_search → web_fetch → file_write" 序列出现 4 次
2. 在任务完成后通知用户：
   "检测到你经常执行 [搜索 → 抓取 → 保存] 的工作流（4 次，成功率 100%）。
    是否将其保存为可复用技能？"
3. 用户确认 → 生成 SKILL.md → 写入 skills/ 目录
4. 下次类似意图自动激活
```

#### 4.2.7 技能使用反馈与优化（Skill Feedback Loop）

**目标：** 追踪技能使用情况，持续优化匹配质量。

**数据模型：**

```go
// internal/skills/feedback.go

type SkillUsageRecord struct {
    SkillName    string        `yaml:"skill_name"`
    SessionID    string        `yaml:"session_id"`
    UserID       string        `yaml:"user_id"`
    ActivatedBy  string        `yaml:"activated_by"`  // "auto" | "manual" | "chain"
    Signals      []MatchSignal `yaml:"signals"`       // 匹配信号
    Score        float64       `yaml:"score"`         // 匹配分数
    TaskOutcome  string        `yaml:"task_outcome"`  // success | failure | partial
    ToolsUsed    int           `yaml:"tools_used"`    // 任务中调用的工具数
    UserFeedback string        `yaml:"user_feedback"` // "helpful" | "not_helpful" | ""
    Timestamp    time.Time     `yaml:"timestamp"`
}

type SkillFeedbackStore struct {
    records []SkillUsageRecord
    path    string  // 持久化路径
}

// GetStats 获取技能统计
func (s *SkillFeedbackStore) GetStats(skillName string) SkillStats {
    var stats SkillStats
    for _, r := range s.records {
        if r.SkillName != skillName {
            continue
        }
        stats.TotalActivations++
        if r.ActivatedBy == "auto" {
            stats.AutoActivations++
        }
        if r.TaskOutcome == "success" {
            stats.SuccessCount++
        }
        if r.UserFeedback == "helpful" {
            stats.HelpfulCount++
        }
        if r.UserFeedback == "not_helpful" {
            stats.NotHelpfulCount++
        }
    }
    stats.SuccessRate = float64(stats.SuccessCount) / float64(max(stats.TotalActivations, 1))
    stats.HelpfulRate = float64(stats.HelpfulCount) / float64(max(stats.HelpfulCount+stats.NotHelpfulCount, 1))
    return stats
}

type SkillStats struct {
    TotalActivations int
    AutoActivations  int
    SuccessCount     int
    HelpfulCount     int
    NotHelpfulCount  int
    SuccessRate      float64
    HelpfulRate      float64
}
```

**反馈驱动的自适应匹配：**

```go
// matcher.go — 在 scoreSkill 中引入反馈权重
func (m *SkillMatcher) adjustScoreByFeedback(skill Skill, baseScore float64) float64 {
    stats := m.feedbackStore.GetStats(skill.Name)

    if stats.TotalActivations < 5 {
        return baseScore // 数据不足，不调整
    }

    // 有用率高的技能获得加成，低的被惩罚
    feedbackMultiplier := 1.0
    if stats.HelpfulRate > 0.8 {
        feedbackMultiplier = 1.2  // 加成 20%
    } else if stats.HelpfulRate < 0.3 && stats.NotHelpfulCount >= 3 {
        feedbackMultiplier = 0.6  // 惩罚 40%
    }

    return baseScore * feedbackMultiplier
}
```

**可观测性事件：**

```go
const (
    EventSkillAutoActivated   = "proactive.skill.auto_activated"
    EventSkillManualActivated = "proactive.skill.manual_activated"
    EventSkillChainStarted    = "proactive.skill.chain_started"
    EventSkillChainCompleted  = "proactive.skill.chain_completed"
    EventSkillSuggested       = "proactive.skill.suggested"       // 学习器建议新技能
    EventSkillFeedback        = "proactive.skill.feedback"
)
```

#### 4.2.8 技能缓存与性能

**现状问题：** 每次 `skills` 工具调用都执行 `skills.DefaultLibrary()` 重新加载，涉及文件 I/O 和 YAML 解析。

**方案：** 单例 Library + 文件变更监听

```go
// internal/skills/cache.go

var (
    cachedLibrary *Library
    cacheMu       sync.RWMutex
    cacheTime     time.Time
    cacheTTL      = 5 * time.Minute
)

// CachedLibrary 返回缓存的技能库，TTL 过期后自动刷新
func CachedLibrary() (*Library, error) {
    cacheMu.RLock()
    if cachedLibrary != nil && time.Since(cacheTime) < cacheTTL {
        defer cacheMu.RUnlock()
        return cachedLibrary, nil
    }
    cacheMu.RUnlock()

    cacheMu.Lock()
    defer cacheMu.Unlock()

    // Double-check after acquiring write lock
    if cachedLibrary != nil && time.Since(cacheTime) < cacheTTL {
        return cachedLibrary, nil
    }

    lib, err := DefaultLibrary()
    if err != nil {
        return nil, err
    }

    cachedLibrary = &lib
    cacheTime = time.Now()
    return cachedLibrary, nil
}

// InvalidateCache 手动失效缓存（技能文件变更时调用）
func InvalidateCache() {
    cacheMu.Lock()
    cachedLibrary = nil
    cacheMu.Unlock()
}
```

**SkillMatcher 的正则预编译**已在 4.2.3 中实现，确保热路径零正则编译开销。

### 4.3 迭代间上下文刷新

**现状：** 上下文在任务开始时构建一次，迭代间仅注入后台任务完成通知。

**目标：** 在多步任务中，根据工具结果动态补充相关记忆和知识。

**扩展点：** `internal/agent/domain/react/runtime.go` 迭代循环

**设计：**

```go
// 在 runIteration() 循环中，think() 之前
func (r *reactRuntime) refreshContext(iteration int) {
    if iteration == 0 || iteration % r.refreshInterval != 0 {
        return // 每 N 次迭代刷新一次
    }

    // 1. 提取最近工具结果的关键词
    recentKeywords := r.extractRecentKeywords(r.state.ToolResults)

    // 2. 召回相关记忆
    memories, _ := r.memoryService.Recall(r.ctx, memory.Query{
        Keywords: recentKeywords,
        Limit:    3,
    })

    // 3. 注入为系统消息
    if len(memories) > 0 {
        r.injectSystemMessage(formatRefreshedMemories(memories))
        r.emitEvent(ProactiveContextRefreshEvent{
            Iteration: iteration,
            MemoriesInjected: len(memories),
        })
    }
}
```

**控制：**
- `refresh_interval: 3` — 每 3 次迭代刷新一次
- `max_refresh_tokens: 500` — 刷新内容的 token 上限
- 仅在多步任务（> 3 次迭代）时启用

### 4.4 自动记忆生命周期

**目标：** 记忆不是无限堆积，需要衰减、合并和归档。

**设计：**

```go
// internal/memory/lifecycle.go

type LifecycleManager struct {
    store       Store
    summarizer  llm.Client
    maxAge      time.Duration  // 记忆最大存活时间 (default: 90 days)
    mergeThresh float64        // 合并相似度阈值 (default: 0.85)
}

// RunMaintenance 定期运行的维护任务
func (m *LifecycleManager) RunMaintenance(ctx context.Context, userID string) error {
    entries, _ := m.store.List(ctx, userID)

    // 1. 过期归档
    for _, entry := range entries {
        if time.Since(entry.CreatedAt) > m.maxAge {
            m.store.Archive(ctx, entry.Key)
        }
    }

    // 2. 相似记忆合并
    clusters := clusterBySimilarity(entries, m.mergeThresh)
    for _, cluster := range clusters {
        if len(cluster) > 1 {
            merged := m.summarizer.Summarize(ctx, cluster)
            m.store.Save(ctx, merged)
            for _, old := range cluster[1:] {
                m.store.Archive(ctx, old.Key)
            }
        }
    }

    return nil
}
```

**触发时机：**
- 每日首次会话时自动运行
- 或当记忆数量超过阈值（如 200 条）时触发

---

## 五、Layer 3：自主决策

### 5.1 定时任务触发器

**目标：** 支持系统在无用户输入的情况下，根据时间规则自主发起任务。

**场景：**
- 每日早上推送工作摘要
- 每周五总结本周决策和学习点
- 定时检查待办事项进度

**设计：**

```go
// internal/agent/app/scheduler/scheduler.go

type Scheduler struct {
    coordinator *coordinator.Coordinator
    triggers    []Trigger
    cron        *cron.Cron
}

type Trigger struct {
    Name     string        `yaml:"name"`
    Schedule string        `yaml:"schedule"`   // cron 表达式
    Task     string        `yaml:"task"`       // 任务模板
    Channel  string        `yaml:"channel"`    // 输出渠道 (lark/wechat/web)
    UserID   string        `yaml:"user_id"`    // 目标用户
    Enabled  bool          `yaml:"enabled"`
    ApprovalRequired bool  `yaml:"approval_required"` // 是否需要审批
}

func (s *Scheduler) Start() {
    for _, trigger := range s.triggers {
        t := trigger
        s.cron.AddFunc(t.Schedule, func() {
            ctx := context.Background()

            if t.ApprovalRequired {
                s.requestApproval(ctx, t)
                return
            }

            s.coordinator.ExecuteTask(ctx, Task{
                Input:   t.Task,
                UserID:  t.UserID,
                Channel: t.Channel,
                Source:  "scheduler",
            })
        })
    }
    s.cron.Start()
}
```

**配置示例：**
```yaml
scheduler:
  triggers:
    - name: daily_briefing
      schedule: "0 9 * * 1-5"  # 工作日早 9 点
      task: "生成今日工作摘要，回顾昨天的决策和今天的待办"
      channel: lark
      user_id: cklxx
      enabled: true
      approval_required: false

    - name: weekly_review
      schedule: "0 17 * * 5"   # 周五下午 5 点
      task: "总结本周的关键决策、学习点和未完成事项"
      channel: lark
      user_id: cklxx
      enabled: true
      approval_required: true
```

### 5.2 跨会话模式识别

**目标：** 从历史会话中识别重复模式，主动提出优化建议。

**设计：**

```go
// internal/agent/app/patterns/recognizer.go

type PatternRecognizer struct {
    memoryService *memory.Service
    patterns      []Pattern
}

type Pattern struct {
    Name        string
    Description string
    Detector    func(entries []memory.Entry) *Detection
    Suggestion  func(detection *Detection) string
}

type Detection struct {
    PatternName string
    Confidence  float64
    Evidence    []memory.Entry
    Frequency   int
}

// 内置模式
var builtinPatterns = []Pattern{
    {
        Name: "repeated_question",
        Description: "用户多次询问相同类型的问题",
        Detector: func(entries []memory.Entry) *Detection {
            // 检测关键词重复频率
            clusters := clusterByKeywords(entries)
            for _, c := range clusters {
                if len(c) >= 3 { // 同一主题出现 3 次以上
                    return &Detection{
                        PatternName: "repeated_question",
                        Confidence:  float64(len(c)) / float64(len(entries)),
                        Evidence:    c,
                        Frequency:   len(c),
                    }
                }
            }
            return nil
        },
        Suggestion: func(d *Detection) string {
            return fmt.Sprintf("检测到你在 %q 主题上有 %d 次相关询问，是否需要我创建一个专项技能来自动化处理？",
                d.Evidence[0].Keywords[0], d.Frequency)
        },
    },
    {
        Name: "tool_failure_pattern",
        Description: "特定工具反复失败",
        // ...
    },
    {
        Name: "workflow_sequence",
        Description: "重复的工具调用序列 — 可自动化为技能",
        // ...
    },
}
```

**触发时机：** 每次任务完成后的 `OnTaskCompleted` 钩子中运行

### 5.3 驱动力评估器

**目标：** 根据用户画像中的驱动力和目标，主动建议行动方向。

**扩展点：** `internal/agent/ports/user_persona.go` 中已有 `InitiativeSources`, `TopDrives`, `Goals`

**设计：**

```go
// internal/agent/app/initiative/evaluator.go

type InitiativeEvaluator struct {
    persona  *UserPersonaProfile
    memories *memory.Service
}

type Initiative struct {
    Type       string  // "goal_alignment", "drive_nudge", "pattern_suggestion"
    Message    string
    Confidence float64
    DriveRef   string  // 关联的驱动力
}

func (e *InitiativeEvaluator) Evaluate(ctx context.Context, task string) []Initiative {
    var initiatives []Initiative

    // 1. 目标对齐检查
    for _, goal := range e.persona.Goals.CurrentFocus {
        if semanticOverlap(task, goal) > 0.6 {
            initiatives = append(initiatives, Initiative{
                Type:       "goal_alignment",
                Message:    fmt.Sprintf("这个任务与你的当前目标 [%s] 直接相关", goal),
                Confidence: 0.8,
            })
        }
    }

    // 2. 驱动力提醒
    for _, drive := range e.persona.CoreDrives {
        if drive.Intensity >= 4 && isRelevant(task, drive.Name) {
            initiatives = append(initiatives, Initiative{
                Type:     "drive_nudge",
                Message:  fmt.Sprintf("基于你的 [%s] 驱动力 (强度 %d/5)，建议关注...", drive.Name, drive.Intensity),
                DriveRef: drive.Name,
            })
        }
    }

    return initiatives
}
```

### 5.4 主动通知引擎

**目标：** 统一管理主动通知的时机、频率和渠道路由。

**核心问题：** 主动性的最大风险是 **过度打扰**。需要注意力评分机制。

**设计：**

```go
// internal/agent/app/attention/engine.go

type AttentionEngine struct {
    config    AttentionConfig
    history   []Notification
    cooldowns map[string]time.Time  // 每种通知类型的冷却时间
}

type AttentionConfig struct {
    MaxDailyNotifications int           `yaml:"max_daily_notifications"` // default: 5
    MinInterval           time.Duration `yaml:"min_interval"`            // default: 30m
    QuietHours            [2]int        `yaml:"quiet_hours"`             // [22, 8] = 晚 10 到早 8 不打扰
    PriorityThreshold     float64       `yaml:"priority_threshold"`      // default: 0.6
}

type Notification struct {
    Type     string
    Priority float64   // 0-1
    Channel  string
    Content  string
    Source   string    // 来自哪个钩子/触发器
}

func (e *AttentionEngine) ShouldNotify(n Notification) bool {
    // 1. 安静时间检查
    hour := time.Now().Hour()
    if hour >= e.config.QuietHours[0] || hour < e.config.QuietHours[1] {
        return false
    }

    // 2. 优先级阈值
    if n.Priority < e.config.PriorityThreshold {
        return false
    }

    // 3. 频率限制
    todayCount := e.countToday()
    if todayCount >= e.config.MaxDailyNotifications {
        return false
    }

    // 4. 冷却时间
    if last, ok := e.cooldowns[n.Type]; ok {
        if time.Since(last) < e.config.MinInterval {
            return false
        }
    }

    return true
}
```

**配置：**
```yaml
attention:
  max_daily_notifications: 5
  min_interval: 30m
  quiet_hours: [22, 8]
  priority_threshold: 0.6
  channels:
    lark:
      enabled: true
      priority_boost: 0.1   # Lark 通知优先级加成
    wechat:
      enabled: false
```

---

## 六、实施路线图

### Phase 1: 自动记忆 (Layer 1)

**交付物：**
1. `MemoryEnricher` — 任务前自动召回
2. `MemoryCapturer` — 任务后自动写入
3. `ProactiveHook` 接口 + `HookRegistry`
4. Coordinator 层统一记忆处理（取代渠道分散实现）
5. 配置开关 `proactive.memory.auto_recall` / `auto_capture`

**技术要点：**
- 在 `internal/agent/app/preparation/` 增加 `memory_enricher.go`
- 在 `internal/agent/domain/react/` 增加 `memory_capture.go`
- 在 `internal/agent/domain/hooks/` 新增钩子包
- 修改 `coordinator.go` 的 `ExecuteTask()` 流程
- 不修改 ReAct 核心循环

**验证标准：**
- [ ] 每次任务执行前自动召回 top-5 相关记忆
- [ ] 每次含工具调用的任务完成后自动写入摘要
- [ ] CLI / Web / Lark 三个渠道行为一致
- [ ] 有 `ProactiveMemoryRecalledEvent` 和 `ProactiveMemoryCapturedEvent` 事件
- [ ] 全量测试通过，无回归

### Phase 2: 智能上下文 (Layer 2)

**交付物：**
1. `HybridStore` — 关键词 + 语义混合检索
2. **自动技能体系：**
   - 2a. 扩展 `Skill` Frontmatter（触发规则、优先级、互斥组、冷却）
   - 2b. `SkillMatcher` — 多信号融合匹配引擎（意图模式 + 工具信号 + 上下文信号）
   - 2c. `buildSkillsSection()` 改造 — 自动注入匹配技能 Body + 精简剩余目录
   - 2d. `SkillChain` — 声明式技能链组合与编排
   - 2e. `CachedLibrary` + 正则预编译 — 技能缓存与性能优化
3. 迭代间上下文刷新（`refreshContext()`）
4. `LifecycleManager` — 记忆衰减与合并

**依赖：** Phase 1 完成 + 嵌入模型可用

**验证标准：**
- [ ] 语义检索的 Recall@5 比纯关键词提升 > 30%
- [ ] 系统提示词 token 消耗减少（仅加载相关技能，非全量）
- [ ] 技能自动激活准确率 > 80%（对有触发规则的技能）
- [ ] 技能匹配延迟 < 10ms（正则预编译 + 缓存）
- [ ] 互斥组和冷却逻辑正确（同组仅激活一个，冷却期内不重复）
- [ ] 技能链能正确解析和展开多步工作流
- [ ] 记忆库不无限膨胀（有效的衰减和合并）
- [ ] 迭代间刷新不影响 ReAct 循环性能（< 200ms 延迟）

### Phase 2.5: 技能学习与反馈（Layer 2 → Layer 3 过渡）

**交付物：**
1. `SkillLearner` — 从工具调用序列中发现可自动化的工作流
2. `WorkflowTrace` 采集 — 在 MemoryCaptureHook 中记录工具调用序列
3. `SkillFeedbackStore` — 技能使用记录与统计
4. 反馈驱动的自适应匹配（使用率/有用率影响匹配分数）
5. `GenerateSkillFile()` — 将建议转化为 SKILL.md 文件

**依赖：** Phase 2 技能匹配引擎完成

**验证标准：**
- [ ] 工具调用序列被正确记录到记忆（含 slot `type=workflow_trace`）
- [ ] 出现 ≥3 次的工具序列被检测并生成建议
- [ ] 已有技能覆盖的序列不重复建议
- [ ] 用户反馈（helpful/not_helpful）能影响后续匹配分数
- [ ] 生成的 SKILL.md 文件格式正确，可被 Library 加载

### Phase 3: 自主决策 (Layer 3)

**交付物：**
1. `Scheduler` — 定时任务触发器
2. `PatternRecognizer` — 跨会话模式识别
3. `InitiativeEvaluator` — 驱动力评估
4. `AttentionEngine` — 通知频率与渠道控制

**依赖：** Phase 2 完成 + 用户画像数据充足

**验证标准：**
- [ ] 定时任务按 cron 表达式准确触发
- [ ] 模式识别能发现 3 种以上重复模式
- [ ] 注意力引擎有效控制通知频率（不过度打扰）
- [ ] 所有主动行为有事件追踪和审计日志

---

## 七、架构决策记录

### ADR-1: 钩子系统 vs 中间件管道

**选择：** 钩子系统 (Hook Registry)

**理由：**
- 钩子更灵活，可以选择性注册和注销
- 中间件管道是线性的，钩子可以并行处理
- 与现有事件系统 (`internal/agent/domain/events.go`) 风格一致
- 钩子可以返回注入内容，中间件通常只做 pass-through

**权衡：** 钩子间没有明确的执行顺序保证，需要通过 Priority 字段排序

### ADR-2: 记忆召回位置 — Preparation vs ReAct 循环内

**选择：** Preparation 层（任务开始前）

**理由：**
- 在 Coordinator 层统一处理，避免侵入 ReAct 核心循环
- 记忆召回是 I/O 操作，在准备阶段做不影响循环内的延迟
- 可以通过 Layer 2 的迭代间刷新补充循环内的记忆需求

**权衡：** 首次召回可能不够精准（任务刚开始，上下文不充分），通过 Layer 2 的 `refreshContext` 弥补

### ADR-3: 注意力引擎 — 规则引擎 vs ML 模型

**选择：** 规则引擎（Phase 3），ML 模型预留扩展

**理由：**
- 规则可解释、可调试、可配置
- 初期数据不足以训练有效的 ML 模型
- 规则引擎足以覆盖 80% 的场景
- 预留 `Scorer` 接口，后续可替换为 ML 模型

### ADR-4: 技能激活策略 — 全量注入 vs 按需激活 vs 混合模式

**选择：** 混合模式（匹配技能 Body 注入 + 剩余技能目录索引）

**理由：**
- 全量注入（当前状态）浪费 token，LLM 上下文有限
- 纯按需激活（仅匹配技能）可能遗漏用户需要的非预期技能
- 混合模式：匹配技能直接注入 Body（零额外工具调用），剩余技能仅展示目录（LLM 可手动调 `skills(show)`）
- Token 预算控制确保不超限

**权衡：** 匹配不准确时可能注入无关技能内容。通过 `confidence_threshold` 和 `max_activated: 3` 限制风险。反馈回路持续优化匹配质量。

### ADR-5: 技能链 — 运行时编排 vs 声明式展开

**选择：** 声明式展开（将链解析为完整指令注入系统提示词，由 LLM 按顺序执行）

**理由：**
- 运行时编排需要引入有限状态机和步骤间数据传递机制，复杂度高
- 声明式展开复用已有 ReAct 循环，LLM 按指令逐步执行
- 与 Markdown 驱动的技能设计哲学一致（技能是指令，不是代码）
- 后续可渐进升级为运行时编排（`chain` 字段已预留数据结构）

**权衡：** LLM 可能不严格按步骤顺序执行，或在某一步失败后不知如何恢复。可通过在展开的指令中添加明确的"完成检查点"和"失败处理"段落缓解。

### ADR-6: 语义记忆 — 独立向量库 vs 混合存储

**选择：** 混合存储 (`HybridStore`)

**理由：**
- 关键词匹配在精确查询场景仍然更快更准
- 语义搜索在模糊查询和跨领域联想上更强
- Reciprocal Rank Fusion 是成熟的混合排序算法
- 复用现有 `chromem-go` 实现，无需引入外部向量数据库

---

## 八、风险与缓解

| 风险 | 影响 | 缓解策略 |
|------|------|----------|
| 自动记忆写入产生大量低质量记忆 | 记忆库膨胀，召回质量下降 | 写入前过滤（仅含工具调用的任务）+ 去重 + 定期清理 |
| 主动通知过度打扰用户 | 用户关闭通知，信任度下降 | AttentionEngine 限频 + 安静时间 + 优先级阈值 |
| 语义检索延迟影响任务启动速度 | 用户感知卡顿 | 设置超时 (200ms)，超时 fallback 到纯关键词 |
| 驱动力评估产生不相关建议 | 干扰用户工作流 | Confidence 阈值 + 用户反馈回路（有用/无用标记） |
| 定时任务在无人值守时产生错误 | 错误操作无法及时纠正 | 高风险任务强制 `approval_required: true` |
| 技能自动激活匹配错误技能 | 注入无关内容浪费 token | `confidence_threshold` + `max_activated: 3` + 反馈回路 |
| 技能链中间步骤失败导致整体卡住 | 多步工作流中断 | 展开指令中包含失败处理段 + LLM 自主判断跳过/重试 |
| 技能学习器产生低质量建议 | 用户疲劳于审批建议 | `minOccurrence: 3` + `successRate > 0.7` 过滤 + 冷却期 |

---

## 九、可观测性设计

每个主动性行为产生结构化事件：

```go
// 主动性事件类型
const (
    EventProactiveMemoryRecalled  = "proactive.memory.recalled"
    EventProactiveMemoryCaptured  = "proactive.memory.captured"
    EventProactiveContextRefresh  = "proactive.context.refresh"
    EventProactiveSkillActivated  = "proactive.skill.activated"
    EventProactivePatternDetected = "proactive.pattern.detected"
    EventProactiveInitiative      = "proactive.initiative"
    EventProactiveNotification    = "proactive.notification"
    EventProactiveNotifySuppressed = "proactive.notification.suppressed"
)
```

**Dashboard 指标：**
- 每日主动记忆召回次数与命中率
- 主动通知发送 vs 抑制比率
- 模式识别检测频率与准确率
- 主动行为对任务完成率的影响（A/B 对比）

---

## 十、总结

elephant.ai 已具备成熟的执行基础设施。从 **被动响应** 到 **主动行动** 的跨越，核心不在于重写系统，而在于：

1. **让记忆自动流动**（Layer 1）— 不再等 LLM 手动调用，系统自动召回和捕获
2. **让上下文更聪明**（Layer 2）— 语义检索、动态技能、迭代间刷新
3. **让系统学会发起**（Layer 3）— 定时触发、模式识别、驱动力评估、注意力控制

三层递进，每层独立可交付。Layer 1 是最高优先级，可以立即开始实施。

**自动技能体系** 是 Layer 2 的核心亮点，分为五个递进子模块：

| 子模块 | 价值 | 复杂度 |
|--------|------|--------|
| Frontmatter 扩展 + SkillMatcher | 让技能从货架走向用户 | 中 |
| Context-Aware Injection | 消除 LLM 的二次工具调用 | 低 |
| Skill Chain | 支持端到端复合工作流 | 中 |
| Skill Learning | 从行为中发现新技能 | 高 |
| Feedback Loop | 持续优化匹配质量 | 中 |
