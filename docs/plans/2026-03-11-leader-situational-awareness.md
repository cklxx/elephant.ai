# Leader Agent 态势感知增强方案

**日期**: 2026-03-11  
**分析范围**: `internal/runtime/leader/`, `internal/app/agent/kernel/`, `internal/app/blocker/`, `internal/runtime/hooks/`  
**设计目标**: 提升 Leader Agent 的调度决策质量，降低误判率和决策延迟

---

## 1. 当前架构分析

### 1.1 Leader Agent 决策流程 (PODATA Loop)

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Leader Agent 决策流程                          │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  EventStalled / EventNeedsInput                                     │
│       │                                                             │
│       ▼                                                             │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐ │
│  │   PERCEIVE      │───▶│   buildStall    │───▶│   LLM Decision  │ │
│  │   GetSession    │    │   Prompt        │    │   INJECT/FAIL   │ │
│  │   (ID/Member/   │    │   (仅5个字段)    │    │   /ESCALATE     │ │
│  │   Goal/Status)  │    │                 │    │                 │ │
│  └─────────────────┘    └─────────────────┘    └─────────────────┘ │
│       │                        │                       │            │
│       ▼                        ▼                       ▼            │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐ │
│  │   缺失信号:      │    │   缺失上下文:    │    │   决策类型粗糙   │ │
│  │   - last_tool   │    │   - 无历史决策   │    │   - 无RETRY_TOOL│ │
│  │   - last_error  │    │   - 无session内容│    │   - 无SWITCH_*  │ │
│  │   - token_usage │    │   - 无迭代计数   │    │   - 无REDUCE_*  │ │
│  │   - heartbeat   │    │                 │    │                 │ │
│  └─────────────────┘    └─────────────────┘    └─────────────────┘ │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### 1.2 源码级问题诊断

#### 问题 1: 感知层信号严重缺失

**文件**: `internal/runtime/leader/leader.go:348-377`

```go
// 当前 buildStallPrompt 仅使用 5 个字段
func buildStallPrompt(id, member, goal string, elapsed time.Duration, eventType hooks.EventType, attempt int) string {
    // 缺失信号:
    // - last_tool_call: 最后调用的工具及参数
    // - last_error: 最近的错误详情
    // - iteration_count: ReAct 迭代次数
    // - token_usage: Token 消耗情况
    // - session_tail: 最近的消息摘要
    // - heartbeat_history: 心跳历史模式
}
```

**影响**: LLM 只能给出通用的 "请继续"，无法做针对性决策。

---

#### 问题 2: 决策历史无持久化

**文件**: `internal/runtime/leader/leader.go:51-54`

```go
type Agent struct {
    // stallCounts 仅记录次数，不记录决策内容和结果
    stallCounts   map[string]int
    // 缺失: stall decision history
    // 缺失: decision outcome tracking
}
```

**影响**: 第 N 次 stall 时 Leader 不知前 N-1 次做了什么，可能重复无效决策。

---

#### 问题 3: 决策类型过于粗糙

**文件**: `internal/runtime/leader/leader.go:238-244`

```go
const (
    actionUnknown decisionAction = iota
    actionInject  // 只能发消息
    actionFail    // 只能标记失败
    // 缺失: RETRY_TOOL（重试特定工具）
    // 缺失: SWITCH_STRATEGY（切换策略）
    // 缺失: REDUCE_SCOPE（缩小范围）
)
```

**影响**: 无法对 "某工具频繁超时" 场景指示 "跳过该工具"。

---

#### 问题 4: Stall 检测单一信号源

**文件**: `internal/runtime/hooks/stall_detector.go:15-30`

```go
type StallDetector struct {
    rt        StallScanner
    bus       Bus
    threshold time.Duration  // 仅基于心跳间隔
    interval  time.Duration
    // 缺失: LLM 质量信号
    // 缺失: 工具失败模式信号
    // 缺失: 用户行为信号
}
```

**影响**: 仅依赖 heartbeat，无法提前识别 "LLM 能力不足导致的低效循环"。

---

#### 问题 5: Blocker Radar 与 Leader Agent 脱节

**文件**: `internal/app/blocker/radar.go:117-130`

```go
type Radar struct {
    store    task.Store
    notifier notification.Notifier
    config   Config
    // 与 Leader Agent 无直接联动
    // 检测到的 blocker 仅做通知，不触发自动修复
}
```

**影响**: Blocker 检测和 Leader 调度是两条平行线，无协同效应。

---

## 2. 态势感知增强方案

### 2.1 方案概览

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         态势感知增强架构                                      │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌─────────────────────────────────────────────────────────────────────┐  │
│   │                        信号采集层 (Signal Layer)                     │  │
│   ├─────────────────────────────────────────────────────────────────────┤  │
│   │  Session Signals │ Tool Signals │ LLM Signals │ User Signals │ Git   │  │
│   │  ────────────────┼──────────────┼─────────────┼──────────────┼─────  │  │
│   │  - heartbeat     │ - tool_name  │ - token_usage│ - response  │ - PR  │  │
│   │  - state_change  │ - tool_args  │ - retry_count│   latency   │   status│
│   │  - iteration     │ - error_type │ - refusal   │ - stop_freq │ - review│
│   │  - elapsed       │ - duration   │ - hallucinat│ - query_freq│   wait  │
│   └─────────────────────────────────────────────────────────────────────┘  │
│                                    │                                        │
│                                    ▼                                        │
│   ┌─────────────────────────────────────────────────────────────────────┐  │
│   │                       态势聚合层 (Fusion Layer)                      │  │
│   ├─────────────────────────────────────────────────────────────────────┤  │
│   │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐              │  │
│   │  │ Situation    │  │ StallPredict │  │ HealthScore  │              │  │
│   │  │ Snapshot     │  │ (预测性)      │  │ Calculator   │              │  │
│   │  └──────────────┘  └──────────────┘  └──────────────┘              │  │
│   └─────────────────────────────────────────────────────────────────────┘  │
│                                    │                                        │
│                                    ▼                                        │
│   ┌─────────────────────────────────────────────────────────────────────┐  │
│   │                       决策引擎层 (Decision Layer)                    │  │
│   ├─────────────────────────────────────────────────────────────────────┤  │
│   │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐              │  │
│   │  │ Rich Prompt  │  │ Decision     │  │ Recovery     │              │  │
│   │  │ Builder      │  │ History      │  │ Evaluator    │              │  │
│   │  └──────────────┘  └──────────────┘  └──────────────┘              │  │
│   └─────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

### 2.2 详细设计

#### 2.2.1 信号采集层增强

##### A. Session 信号增强

**新增文件**: `internal/runtime/signals/session_signals.go`

```go
// SessionSignals 采集 session 运行时信号
type SessionSignals struct {
    // 基础信号
    SessionID     string
    MemberType    string
    State         string
    StartedAt     time.Time
    LastHeartbeat time.Time
    
    // 新增信号
    IterationCount   int           // ReAct 迭代次数
    LastToolCall     *ToolCallInfo  // 最后工具调用
    LastError        *ErrorInfo     // 最后错误详情
    TokenUsage       TokenUsage     // Token 消耗统计
    RecentMessages   []MessageTail  // 最近消息摘要
}

type ToolCallInfo struct {
    Name      string
    Args      map[string]any
    Duration  time.Duration
    Timestamp time.Time
}

type ErrorInfo struct {
    Type      string  // permission_error / timeout_error / logic_error
    Message   string
    ToolName  string  // 关联的工具
    Count     int     // 同类错误连续出现次数
}
```

---

##### B. LLM 质量信号

**新增文件**: `internal/runtime/signals/llm_signals.go`

```go
// LLMSignals 采集 LLM 调用质量信号
type LLMSignals struct {
    // 效率指标
    TokensPerIteration  float64  // 单次迭代 Token 消耗
    ResponseLatency     time.Duration
    
    // 质量指标
    RetryCount          int      // 重试次数
    RefusalCount        int      // 拒绝响应次数
    HallucinationFlags  int      // 幻觉检测结果
    
    // 能力指标
    ToolUseAccuracy     float64  // 工具调用准确率
    ContextUtilization  float64  // 上下文利用率
}

// LLMQualityScorer 计算 LLM 质量评分
func (s *LLMSignals) QualityScore() float64 {
    // 综合评分: 0-100
    // - 高 Token 消耗 + 低产出 = 低分
    // - 频繁重试 = 低分
    // - 频繁拒绝 = 低分
}
```

---

##### C. 工具执行信号

**新增文件**: `internal/runtime/signals/tool_signals.go`

```go
// ToolSignals 采集工具执行模式信号
type ToolSignals struct {
    // 执行统计
    TotalCalls    int
    SuccessCalls  int
    FailedCalls   int
    
    // 失败模式
    FailurePatterns map[string]int  // 错误类型分布
    
    // 性能指标
    AvgDuration   time.Duration
    P99Duration   time.Duration
    
    // 热点工具
    HotTools      []ToolUsage  // 高频调用工具
}

type ToolUsage struct {
    Name       string
    CallCount  int
    ErrorCount int
    AvgDuration time.Duration
}

// DetectFailurePattern 检测特定失败模式
func (s *ToolSignals) DetectFailurePattern(pattern string) (int, bool) {
    count, ok := s.FailurePatterns[pattern]
    return count, ok && count > threshold
}
```

---

##### D. 用户行为信号

**新增文件**: `internal/domain/signals/user_signals.go`

```go
// UserBehaviorSignals 采集用户交互行为信号
type UserBehaviorSignals struct {
    UserID string
    
    // 响应模式
    AvgResponseLatency time.Duration  // 平均响应延迟
    ResponseLatencyP95 time.Duration  // P95 响应延迟
    
    // 行为频率
    StopCommandFreq    float64  // 单位时间内 /stop 频率
    StatusQueryFreq    float64  // 单位时间内状态查询频率
    
    // 情绪指标
    CorrectionCount    int      // 连续修正次数
    EscalationRequests int      // 主动请求人工次数
}

// UrgencyFactor 计算用户紧急程度因子 (0-1)
func (s *UserBehaviorSignals) UrgencyFactor() float64 {
    // 响应延迟短 + 查询频繁 + 修正多 = 高紧急度
}
```

---

#### 2.2.2 态势聚合层

##### A. 态势快照 (SituationSnapshot)

**新增文件**: `internal/runtime/situation/snapshot.go`

```go
// SituationSnapshot 是某时刻的完整态势快照
type SituationSnapshot struct {
    SessionID    string
    Timestamp    time.Time
    
    // 多维度信号
    Session  *signals.SessionSignals
    LLM      *signals.LLMSignals
    Tools    *signals.ToolSignals
    User     *signals.UserBehaviorSignals
    
    // 派生指标
    HealthScore   int           // 综合健康评分 0-100
    StallRisk     float64       // stall 风险概率 0-1
    BlockerTypes  []string      // 检测到的 blocker 类型
}

// CalculateHealthScore 计算综合健康评分
func (s *SituationSnapshot) CalculateHealthScore() int {
    score := 100
    
    // LLM 质量扣分
    if s.LLM != nil {
        score -= int((1 - s.LLM.QualityScore()/100) * 30)
    }
    
    // 错误模式扣分
    if s.Tools != nil {
        for pattern, count := range s.Tools.FailurePatterns {
            if count > 3 {
                score -= 10  // 重复错误模式
            }
            if pattern == "permission_error" {
                score -= 20  // 权限错误严重
            }
        }
    }
    
    // 用户紧急度扣分
    if s.User != nil {
        score -= int(s.User.UrgencyFactor() * 20)
    }
    
    return max(0, score)
}
```

---

##### B. Stall 预测器 (预测性检测)

**新增文件**: `internal/runtime/situation/predictor.go`

```go
// StallPredictor 基于多信号预测 stall 风险
type StallPredictor struct {
    threshold float64  // 风险阈值
}

// Predict 预测 stall 风险
func (p *StallPredictor) Predict(snapshot *SituationSnapshot) Prediction {
    risk := 0.0
    reasons := []string{}
    
    // 信号 1: LLM 质量下降
    if snapshot.LLM != nil && snapshot.LLM.QualityScore() < 50 {
        risk += 0.3
        reasons = append(reasons, "llm_quality_low")
    }
    
    // 信号 2: 工具频繁失败
    if snapshot.Tools != nil {
        if count, ok := snapshot.Tools.DetectFailurePattern("timeout_error"); ok {
            risk += float64(count) * 0.1
            reasons = append(reasons, "tool_timeout_pattern")
        }
    }
    
    // 信号 3: 迭代次数异常
    if snapshot.Session != nil && snapshot.Session.IterationCount > 20 {
        risk += 0.2
        reasons = append(reasons, "high_iteration_count")
    }
    
    // 信号 4: Token 消耗异常
    if snapshot.LLM != nil && snapshot.LLM.TokensPerIteration > 4000 {
        risk += 0.15
        reasons = append(reasons, "high_token_usage")
    }
    
    return Prediction{
        RiskScore:   min(risk, 1.0),
        Reasons:     reasons,
        ShouldAlert: risk > p.threshold,
    }
}
```

---

#### 2.2.3 决策引擎层增强

##### A. 富 Prompt 构建器

**文件修改**: `internal/runtime/leader/leader.go`

```go
// buildStallPrompt 增强版 - 使用态势快照构建富上下文 prompt
func buildStallPromptV2(snap *situation.SituationSnapshot, attempt int) string {
    var b strings.Builder
    
    // 基础信息
    b.WriteString(fmt.Sprintf(`You are a leader agent managing an AI coding session.

Session ID: %s
Member: %s
Goal: %s
Status: %s
Elapsed: %s
Attempt: %d of %d

`, snap.Session.SessionID, snap.Session.MemberType, snap.Session.Goal, 
   snap.Session.State, time.Since(snap.Session.StartedAt), attempt, maxStallAttempts))
    
    // 迭代上下文
    if snap.Session != nil {
        b.WriteString(fmt.Sprintf("Iteration Count: %d\n", snap.Session.IterationCount))
    }
    
    // Token 效率
    if snap.LLM != nil {
        b.WriteString(fmt.Sprintf("Token Usage: %.0f per iteration\n", snap.LLM.TokensPerIteration))
        b.WriteString(fmt.Sprintf("LLM Quality Score: %.0f/100\n", snap.LLM.QualityScore()))
    }
    
    // 最后工具调用
    if snap.Session != nil && snap.Session.LastToolCall != nil {
        b.WriteString(fmt.Sprintf("\nLast Tool Call:\n"))
        b.WriteString(fmt.Sprintf("  Name: %s\n", snap.Session.LastToolCall.Name))
        b.WriteString(fmt.Sprintf("  Duration: %s\n", snap.Session.LastToolCall.Duration))
    }
    
    // 最后错误
    if snap.Session != nil && snap.Session.LastError != nil {
        b.WriteString(fmt.Sprintf("\nLast Error:\n"))
        b.WriteString(fmt.Sprintf("  Type: %s\n", snap.Session.LastError.Type))
        b.WriteString(fmt.Sprintf("  Message: %s\n", snap.Session.LastError.Message))
        b.WriteString(fmt.Sprintf("  Repeated: %d times\n", snap.Session.LastError.Count))
    }
    
    // 工具失败模式
    if snap.Tools != nil && len(snap.Tools.FailurePatterns) > 0 {
        b.WriteString("\nFailure Patterns:\n")
        for pattern, count := range snap.Tools.FailurePatterns {
            b.WriteString(fmt.Sprintf("  - %s: %d times\n", pattern, count))
        }
    }
    
    // 健康评分
    b.WriteString(fmt.Sprintf("\nOverall Health Score: %d/100\n", snap.HealthScore))
    
    // 决策指令
    b.WriteString(`
Decide what to do next. Reply with EXACTLY one of:

INJECT <message> — Send a message to unblock the session
FAIL <reason> — Give up on this session
ESCALATE — Escalate to human operator

Advanced actions (use when appropriate):
RETRY_TOOL <tool_name> — Retry a specific tool that failed
SWITCH_STRATEGY <hint> — Suggest a different approach
REDUCE_SCOPE <hint> — Reduce the task scope

Reply only with one of the above. No explanation.`)
    
    return b.String()
}
```

---

##### B. 决策历史与效果评估

**新增文件**: `internal/runtime/leader/history.go`

```go
// DecisionRecord 记录单次决策
type DecisionRecord struct {
    Timestamp   time.Time
    Attempt     int
    Decision    string  // INJECT/FAIL/ESCALATE/RETRY_TOOL/etc
    Argument    string  // 决策参数
    ContextHash string  // 上下文的哈希（用于比较相似情况）
    
    // 结果跟踪
    Outcome     string  // recovered / still_stalled / failed / unknown
    OutcomeAt   time.Time
    Effectiveness float64  // 效果评分 0-1
}

// DecisionHistory 管理某 session 的决策历史
type DecisionHistory struct {
    records []DecisionRecord
    mu      sync.RWMutex
}

// Add 添加决策记录
func (h *DecisionHistory) Add(r DecisionRecord) {
    h.mu.Lock()
    defer h.mu.Unlock()
    h.records = append(h.records, r)
}

// FindSimilarContext 查找相似上下文的历史决策
func (h *DecisionHistory) FindSimilarContext(contextHash string, n int) []DecisionRecord {
    h.mu.RLock()
    defer h.mu.RUnlock()
    
    var matches []DecisionRecord
    for i := len(h.records) - 1; i >= 0 && len(matches) < n; i-- {
        if h.records[i].ContextHash == contextHash {
            matches = append(matches, h.records[i])
        }
    }
    return matches
}

// CalculateEffectiveness 计算某类决策的有效性
func (h *DecisionHistory) CalculateEffectiveness(decisionType string) float64 {
    h.mu.RLock()
    defer h.mu.RUnlock()
    
    var total, success float64
    for _, r := range h.records {
        if r.Decision == decisionType && r.Outcome != "" {
            total++
            if r.Outcome == "recovered" {
                success++
            }
        }
    }
    if total == 0 {
        return -1  // 无数据
    }
    return success / total
}

// SummaryForPrompt 生成用于 prompt 的历史摘要
func (h *DecisionHistory) SummaryForPrompt(n int) string {
    h.mu.RLock()
    defer h.mu.RUnlock()
    
    if len(h.records) == 0 {
        return "No previous decisions for this session."
    }
    
    var b strings.Builder
    b.WriteString("Previous Decisions:\n")
    
    start := max(0, len(h.records)-n)
    for i := start; i < len(h.records); i++ {
        r := h.records[i]
        b.WriteString(fmt.Sprintf("  Attempt %d: %s → %s\n", r.Attempt, r.Decision, r.Outcome))
    }
    
    return b.String()
}
```

---

##### C. 细粒度决策类型

**文件修改**: `internal/runtime/leader/leader.go`

```go
// decisionAction 扩展决策类型
type decisionAction int

const (
    actionUnknown decisionAction = iota
    actionInject
    actionFail
    actionEscalate
    actionRetryTool      // 新增: 重试特定工具
    actionSwitchStrategy // 新增: 切换策略
    actionReduceScope    // 新增: 缩小范围
)

// parseDecisionV2 扩展决策解析
func parseDecisionV2(raw string) (decisionAction, string, string) {
    upper := strings.ToUpper(strings.TrimSpace(raw))
    parts := strings.Fields(upper)
    if len(parts) == 0 {
        return actionUnknown, "", raw
    }
    
    action := parts[0]
    arg := ""
    if len(parts) > 1 {
        arg = strings.Join(parts[1:], " ")
    }
    
    switch action {
    case "INJECT":
        if arg == "" {
            arg = "Please continue with the task."
        }
        return actionInject, arg, raw
    case "FAIL":
        return actionFail, arg, raw
    case "ESCALATE":
        return actionEscalate, "", raw
    case "RETRY_TOOL":
        return actionRetryTool, arg, raw
    case "SWITCH_STRATEGY":
        return actionSwitchStrategy, arg, raw
    case "REDUCE_SCOPE":
        return actionReduceScope, arg, raw
    default:
        return actionUnknown, "", raw
    }
}

// applyDecisionV2 应用扩展决策
func (a *Agent) applyDecisionV2(ctx context.Context, sessionID string, action decisionAction, arg string) {
    switch action {
    case actionInject:
        _ = a.rt.InjectText(ctx, sessionID, arg)
        
    case actionFail:
        if arg == "" {
            arg = "leader agent: session abandoned after stall"
        }
        a.markFailedWithRetry(sessionID, arg)
        
    case actionRetryTool:
        // 构造特定工具重试指令
        msg := fmt.Sprintf("The tool '%s' appears to have failed. Please retry with a different approach or parameters.", arg)
        _ = a.rt.InjectText(ctx, sessionID, msg)
        
    case actionSwitchStrategy:
        msg := fmt.Sprintf("The current approach doesn't seem to be working. Try a different strategy: %s", arg)
        _ = a.rt.InjectText(ctx, sessionID, msg)
        
    case actionReduceScope:
        msg := fmt.Sprintf("The task may be too complex. Consider reducing the scope: %s", arg)
        _ = a.rt.InjectText(ctx, sessionID, msg)
        
    default:
        a.escalate(sessionID, "leader agent: escalating to human operator")
    }
}
```

---

##### D. Recovery 效果评估

**新增文件**: `internal/runtime/leader/evaluator.go`

```go
// RecoveryEvaluator 评估决策恢复效果
type RecoveryEvaluator struct {
    evaluations map[string]*Evaluation  // sessionID -> evaluation
    mu          sync.Mutex
}

type Evaluation struct {
    SessionID      string
    DecisionAt     time.Time
    DecisionType   string
    CheckAt        time.Time
    Result         string  // success / failure
}

// StartEvaluation 启动效果评估
func (e *RecoveryEvaluator) StartEvaluation(sessionID, decisionType string) {
    e.mu.Lock()
    defer e.mu.Unlock()
    
    e.evaluations[sessionID] = &Evaluation{
        SessionID:    sessionID,
        DecisionAt:   time.Now(),
        DecisionType: decisionType,
        CheckAt:      time.Now().Add(30 * time.Second),  // 30秒后评估
    }
}

// Evaluate 执行评估
func (e *RecoveryEvaluator) Evaluate(sessionID string, rt RuntimeReader) string {
    e.mu.Lock()
    eval, ok := e.evaluations[sessionID]
    if !ok {
        e.mu.Unlock()
        return ""
    }
    delete(e.evaluations, sessionID)
    e.mu.Unlock()
    
    snap, ok := rt.GetSession(sessionID)
    if !ok {
        return "session_gone"
    }
    
    // 评估成功标准:
    // 1. session 状态为 running
    // 2. 最近有心跳
    // 3. 有新的迭代进展
    
    if snap.State == "running" && time.Since(snap.LastHeartbeat) < 60*time.Second {
        return "recovered"
    }
    
    return "still_stalled"
}

// ScheduleEvaluation 调度延迟评估
func (a *Agent) ScheduleEvaluation(sessionID, decisionType string) {
    if a.evaluator == nil {
        return
    }
    
    a.evaluator.StartEvaluation(sessionID, decisionType)
    
    // 30秒后评估
    time.AfterFunc(30*time.Second, func() {
        result := a.evaluator.Evaluate(sessionID, a.rt)
        a.recordOutcome(sessionID, decisionType, result)
    })
}
```

---

### 2.3 集成方案

#### 2.3.1 与 Blocker Radar 联动

```go
// BlockerRadarAdapter 将 Radar 检测到的 blocker 转化为态势信号
type BlockerRadarAdapter struct {
    radar  *blocker.Radar
    bus    hooks.Bus
}

// OnBlockerDetected 当 radar 检测到 blocker 时触发
func (a *BlockerRadarAdapter) OnBlockerDetected(alert blocker.Alert) {
    // 发布 blocker 信号到 event bus
    a.bus.Publish(alert.Task.TaskID, hooks.Event{
        Type:      hooks.EventBlockerDetected,
        SessionID: alert.Task.TaskID,
        At:        time.Now(),
        Payload: map[string]any{
            "reason": string(alert.Reason),
            "detail": alert.Detail,
            "age":    alert.Age.Seconds(),
        },
    })
    
    // 高优先级 blocker 直接触发 Leader Agent 介入
    if alert.Reason == blocker.ReasonGitReviewBlock && alert.Age > 2*time.Hour {
        a.bus.Publish(alert.Task.TaskID, hooks.Event{
            Type:      hooks.EventHandoffRequired,
            SessionID: alert.Task.TaskID,
            At:        time.Now(),
            Payload:   map[string]any{"reason": "urgent_git_review_block"},
        })
    }
}
```

---

#### 2.3.2 Event Bus 扩展

**文件修改**: `internal/runtime/hooks/bus.go`

```go
const (
    // 现有事件类型
    EventHeartbeat       EventType = "heartbeat"
    EventStarted         EventType = "started"
    EventCompleted       EventType = "completed"
    EventFailed          EventType = "failed"
    EventStalled         EventType = "stalled"
    EventNeedsInput      EventType = "needs_input"
    EventHandoffRequired EventType = "handoff_required"
    EventChildCompleted  EventType = "child_completed"
    
    // 新增事件类型
    EventBlockerDetected EventType = "blocker_detected"  // Blocker 检测
    EventSituationUpdate EventType = "situation_update"  // 态势更新
    EventQualityAlert    EventType = "quality_alert"      // 质量警报
)
```

---

## 3. 实施计划

### Phase 1: 信号采集层 (Week 1-2)

| 任务 | 文件 | 工作量 | 依赖 |
|------|------|--------|------|
| Session 信号采集增强 | `internal/runtime/signals/session_signals.go` | 2d | - |
| LLM 质量信号采集 | `internal/runtime/signals/llm_signals.go` | 2d | - |
| 工具执行信号采集 | `internal/runtime/signals/tool_signals.go` | 1d | - |
| 信号收集器集成到 Runtime | `internal/runtime/runtime.go` | 1d | 前三项 |

### Phase 2: 态势聚合层 (Week 2-3)

| 任务 | 文件 | 工作量 | 依赖 |
|------|------|--------|------|
| 态势快照实现 | `internal/runtime/situation/snapshot.go` | 2d | Phase 1 |
| Stall 预测器 | `internal/runtime/situation/predictor.go` | 2d | 态势快照 |
| 健康评分计算器 | `internal/runtime/situation/health.go` | 1d | 态势快照 |

### Phase 3: 决策引擎增强 (Week 3-4)

| 任务 | 文件 | 工作量 | 依赖 |
|------|------|--------|------|
| 富 Prompt 构建器 | `internal/runtime/leader/prompt.go` | 2d | Phase 2 |
| 决策历史管理 | `internal/runtime/leader/history.go` | 2d | - |
| 细粒度决策类型 | `internal/runtime/leader/leader.go` | 2d | - |
| Recovery 效果评估 | `internal/runtime/leader/evaluator.go` | 1d | 决策历史 |

### Phase 4: 集成与验证 (Week 4-5)

| 任务 | 文件 | 工作量 | 依赖 |
|------|------|--------|------|
| Blocker Radar 联动 | `internal/app/blocker/adapter.go` | 1d | - |
| Event Bus 扩展 | `internal/runtime/hooks/bus.go` | 1d | - |
| 集成测试 | `internal/runtime/leader/*_test.go` | 2d | 全部 |
| 效果评估 | 数据分析 | 1d | 集成测试 |

---

## 4. 度量指标

### 4.1 技术指标

| 指标 | 当前值 | 目标值 | 测量方式 |
|------|--------|--------|----------|
| Stall 检测准确率 | 未知 | > 85% | 人工标注样本评估 |
| INJECT 决策成功率 | 未知 | > 60% | RecoveryEvaluator 自动统计 |
| 决策延迟 | threshold + LLM latency | < 30s | 事件日志时间戳 |
| Prompt Token 数 | ~150 | ~500 (可控) | 实际 prompt 长度 |
| 细粒度决策占比 | 0% | > 30% | 决策类型统计 |

### 4.2 业务指标

| 指标 | 当前值 | 目标值 | 测量方式 |
|------|--------|--------|----------|
| 用户 /stop 频率 | 基线 | 下降 > 20% | 命令日志统计 |
| 状态查询频率 | 基线 | 下降 > 30% | 查询日志统计 |
| 人工介入率 | 基线 | 下降 > 25% | Handoff 事件统计 |
| 任务完成率 | 基线 | 提升 > 15% | 任务状态统计 |

---

## 5. 风险与缓解

| 风险 | 影响 | 概率 | 缓解策略 |
|------|------|------|----------|
| Prompt 过长导致成本上升 | 高 | 中 | 摘要算法控制长度，分层 prompt |
| 多信号源导致误判增加 | 高 | 中 | 渐进式启用信号，A/B 测试验证 |
| 历史记录内存占用 | 中 | 低 | 设置上限，持久化到 store |
| LLM 不遵循新决策类型 | 中 | 中 | fuzzy parsing + fallback，训练样本优化 |
| 与现有 Blocker Radar 冲突 | 中 | 低 | 明确职责边界，Radar 专注检测，Leader 专注决策 |

---

## 6. 附录：关键代码变更摘要

### 6.1 新增文件列表

```
internal/runtime/signals/
├── session_signals.go   # Session 信号定义
├── llm_signals.go       # LLM 质量信号
├── tool_signals.go      # 工具执行信号
└── user_signals.go      # 用户行为信号

internal/runtime/situation/
├── snapshot.go          # 态势快照
├── predictor.go         # Stall 预测器
└── health.go            # 健康评分

internal/runtime/leader/
├── prompt.go            # 富 Prompt 构建器
├── history.go           # 决策历史管理
└── evaluator.go         # Recovery 效果评估

internal/app/blocker/
└── adapter.go           # Radar 与 Leader 联动适配器
```

### 6.2 修改文件列表

```
internal/runtime/leader/leader.go      # 集成新决策流程
internal/runtime/hooks/bus.go          # 扩展事件类型
internal/runtime/runtime.go            # 集成信号采集
```

---

*文档完成。建议召开技术评审会议确认 Phase 1 实施细节。*
