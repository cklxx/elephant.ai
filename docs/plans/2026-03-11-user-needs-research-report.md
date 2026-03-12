# Elephant.ai 用户需求调研报告

**报告日期**: 2026-03-11  
**调研范围**: docs/plans/ 下 58 份计划文档、架构文档、Web 前端代码  
**分析人员**: ckl

---

## 一、未关闭计划中对最终用户价值最高的需求

基于 `docs/plans/2026-03-11-user-needs-analysis.md` 及其他相关计划文档分析，以下是按用户价值排序的未完成需求：

### P0 级：阻塞性/基础性需求（用户价值最高）

| 需求 | 用户价值 | 问题描述 | 来源文档 |
|------|----------|----------|----------|
| **Feishu CLI 产品化** | ★★★★★ | 飞书能力面双轨制导致工具选择准确率下降 20-30%，新增飞书能力接入效率低 | `2026-03-06-feishu-cli-canonical-surface.md` |
| **Terminal UX 产品化** | ★★★★★ | Terminal 像开发者调试工具而非用户产品，用户无法直观看到"谁在执行、干到哪、卡没卡、结果在哪" | `2026-03-06-team-terminal-ux-model.md` |
| **Leader Agent Stall 决策历史持久化** | ★★★★☆ | Leader 在第 N 次 stall 时看不到前 N-1 次决策记录，导致重复发同样的 INJECT 消息 | `2026-03-11-leader-agent-improvement.md` |
| **富 Stall Prompt** | ★★★★☆ | 当前 prompt 缺乏 last_tool_call、last_error、iteration_count 等上下文，无法做精准决策 | `2026-03-11-leader-agent-improvement.md` |

### P1 级：重要功能完善（高用户价值）

| 需求 | 用户价值 | 问题描述 | 来源文档 |
|------|----------|----------|----------|
| **飞书文档编辑能力** | ★★★★☆ | 当前 docx 仅支持 create/read/list_blocks，缺少 PATCH 操作导致编辑路径失败 | `2026-03-03-docx-update-block-and-inject-e2e.md` |
| **Handoff 上下文增强** | ★★★★☆ | 缺少 LastToolCall、LastError、SessionTail 等诊断信息，人工诊断时间长 | `2026-03-11-leader-agent-improvement.md` |
| **Handoff Interactive Card** | ★★★★☆ | 当前只能通过自然语言回复，没有 inline button 一键 retry/abort/provide_input | `2026-03-11-leader-agent-improvement.md` |
| **Kaku Runtime Skeleton** | ★★★★☆ | 多 session 管理、生命周期控制、持久化能力缺失，限制复杂任务执行 | `2026-03-08-cli-runtime-kaku-compact-design.md` |

**核心结论**: 当前对用户价值最高的是 **Terminal UX 产品化** 和 **Feishu CLI 产品化**，它们分别解决"任务执行不可观察"和"飞书能力双轨制"两大痛点。

---

## 二、当前架构的优势与不足

基于 `docs/reference/ARCHITECTURE.md` 和 `docs/guides/engineering-workflow.md` 分析：

### 架构优势

| 优势 | 说明 | 证据文件 |
|------|------|----------|
| **清晰的分层模型** | Delivery → Application → Domain → Infrastructure → Shared 五层分离，职责明确 | `docs/reference/ARCHITECTURE.md` 第 9-30 行 |
| **ReAct 引擎核心** | Think → Plan Tools → Execute → Observe 循环，支持 checkpoint 和恢复 | `docs/reference/ARCHITECTURE.md` 第 76-84 行 |
| **统一事件模型** | Domain events 通过 WorkflowEventTranslator 转换，支持 correlation_id/causation_id 追踪 | `docs/reference/ARCHITECTURE.md` 第 140-154 行 |
| **工具链完整** | Registry → SLA → ID propagation → Retry/Circuit breaker → Approval → Validation → Executor | `docs/reference/ARCHITECTURE.md` 第 102-106 行 |
| **工程流程规范** | worktree 开发、TDD、代码审查、渐进式披露等规则明确 | `docs/guides/engineering-workflow.md` 第 9-68 行 |

### 架构不足

| 不足 | 影响 | 证据文件 |
|------|------|----------|
| **枚举驱动扩展** | 每个 provider/channel/tool 需要修改 4-6 个文件的 switch/if，违背开闭原则 | `docs/plans/2026-03-04-architecture-optimization-blueprint.md` 第 8 行 |
| **123 条架构异常** | `make check-arch` 约束从未真正生效，分层边界被持续破坏 | `docs/plans/2026-03-04-architecture-optimization-blueprint.md` 第 8 行 |
| **Domain 层违规** | `internal/domain/agent/react/background.go` 直接 import `os`/`os/exec`/`path/filepath` | `docs/plans/2026-03-04-architecture-optimization-blueprint.md` 第 293-299 行 |
| **状态语义折叠** | `waiting_input` 被折叠为 `running`，前端无法区分等待输入状态 | `docs/plans/2026-03-04-architecture-optimization-blueprint.md` 第 326-340 行 |
| **因果链截断** | `file_event_history_store.go:247` 未持久化 correlationID/causationID | `docs/plans/2026-03-04-architecture-optimization-blueprint.md` 第 341-345 行 |
| **Fat Interface** | ContextManager 8 个方法混合 3 个职责，阻碍单元测试隔离 | `docs/plans/2026-03-04-architecture-optimization-blueprint.md` 第 618-655 行 |
| **代码重复** | 17 个 truncate 函数、60+ 处手写 `strings.ToLower(strings.TrimSpace(x))` | `docs/plans/2026-03-03-global-codebase-simplify.md` |

### 架构改进优先级

根据 `2026-03-04-architecture-optimization-blueprint.md` 第 843-851 行：
1. **Phase 0（基础）**: 消除层违规 + 契约修复
2. **Phase 1（能力注册）**: Provider + Channel 插件化（A06-A10）
3. **Phase 2（接口细化）**: ISP + 抽象后端无关化（A11-A13）

---

## 三、Web 前端交互体验可改进点

基于 `web/` 目录代码分析：

### 3.1 当前已实现能力

| 能力 | 实现位置 |
|------|----------|
| SSE 事件流 + 虚拟化列表 | `web/components/agent/VirtualizedEventList.tsx` |
| 响应式三栏布局（Sidebar/Main/RightPanel） | `web/app/conversation/components/ConversationMainArea.tsx` |
| 附件上传 + 粘贴支持 | `web/components/agent/TaskInput.tsx` 第 290-380 行 |
| 国际化支持 | `web/lib/i18n` |
| Skills 面板 + 附件面板 | `web/components/agent/SkillsPanel.tsx`, `AttachmentPanel.tsx` |

### 3.2 可改进点

| 改进点 | 当前问题 | 建议方案 | 证据文件 |
|--------|----------|----------|----------|
| **任务状态可视化不足** | 当前仅显示 running/completed/failed，缺少 `waiting_input` 状态 | 增加 waiting_input 状态的独立 UI（输入框高亮 + 等待提示） | `web/lib/types/api/task.ts` 需扩展 |
| **Team 执行现场缺失** | Web 端无法展示多 Agent 协作状态（谁在执行、进度如何） | 增加 Team Run 视图：Role Cards + Activity Timeline + Terminal Panel | `docs/plans/2026-03-06-team-terminal-ux-model.md` 第 48-82 行 |
| **右侧边栏利用率低** | SkillsPanel 和 AttachmentPanel 信息密度低 | 整合 Agent 状态、工具执行进度、Artifacts 汇总 | `web/app/conversation/components/ConversationMainArea.tsx` 第 226-238 行 |
| **会话管理功能弱** | Sidebar 仅展示 session ID，无任务状态标识 | 增加状态图标（进行中/已完成/失败）和最后活动时间 | `web/components/layout/Sidebar.tsx` 第 32-77 行 |
| **输入框缺少快捷操作** | 无快速选择 skill 或模板的方式 | 增加 `/` 命令或 `@` 提及的快捷菜单 | `web/components/agent/TaskInput.tsx` |
| **事件流可读性待提升** | 子 Agent 事件折叠不够，工具调用信息过载 | 优化 EventLine 的折叠策略，增加进度条/步骤指示器 | `web/components/agent/VirtualizedEventList.tsx` 第 265-325 行 |

### 3.3 用户体验改进优先级

1. **P0**: 增加 `waiting_input` 状态可视化（后端已支持，前端需适配）
2. **P1**: Team Run 执行现场（Role Cards + Timeline）
3. **P1**: 右侧边栏整合 Agent 状态和进度
4. **P2**: 会话列表状态标识
5. **P2**: 快捷操作菜单

---

## 四、可收敛的核心结论

### 4.1 最高优先级用户需求（排序）

1. **Terminal UX 产品化** - 让用户能直观看到"谁在执行、干到哪、卡没卡"（P0，5-7天）
2. **Feishu CLI 产品化** - 统一飞书能力入口，解决双轨制问题（P0，3-5天）
3. **Handoff Interactive Card** - 一键操作降低用户摩擦（P1，3-4天）
4. **飞书文档编辑能力** - 补全 PATCH 操作（P1，3-5天）
5. **Stall Recovery 效果评估** - 量化改进效果的基础设施（P1，3-4天）

### 4.2 架构层面必须先修复

1. **Domain 层净化** - 移除 `os`/`os/exec`/`path/filepath`/`net/http` 直接引用
2. **状态语义直通** - 取消 `waiting_input` → `running` 折叠
3. **因果链持久化** - 修复 `correlationID`/`causationID` 丢失

### 4.3 Web 前端优先改进

1. **Team Run 视图** - 在 Web 端实现 Role Cards + Activity Timeline
2. **waiting_input 状态 UI** - 独立渲染等待输入状态
3. **右侧边栏重构** - 整合 Agent 状态、进度、Artifacts

---

## 五、参考文档索引

| 文档 | 路径 |
|------|------|
| 需求优先级报告 | `docs/plans/2026-03-11-user-needs-analysis.md` |
| Agent Team 一体化方案 | `docs/plans/2026-03-06-agent-team-feishu-cli-terminal-integration.md` |
| Terminal UX 模型 | `docs/plans/2026-03-06-team-terminal-ux-model.md` |
| Feishu CLI 设计 | `docs/plans/2026-03-06-feishu-cli-canonical-surface.md` |
| Leader Agent 改进 | `docs/plans/2026-03-11-leader-agent-improvement.md` |
| 架构优化蓝图 | `docs/plans/2026-03-04-architecture-optimization-blueprint.md` |
| 架构参考文档 | `docs/reference/ARCHITECTURE.md` |
| 工程流程指南 | `docs/guides/engineering-workflow.md` |
| 对话主页面 | `web/app/conversation/page.tsx` |
| 对话主区域 | `web/app/conversation/components/ConversationMainArea.tsx` |
| 虚拟化事件列表 | `web/components/agent/VirtualizedEventList.tsx` |
| 任务输入组件 | `web/components/agent/TaskInput.tsx` |
| 侧边栏组件 | `web/components/layout/Sidebar.tsx` |

---

*报告完成。建议优先启动 Terminal UX 产品化和 Feishu CLI 产品化两个 P0 需求，同时修复架构层面的状态语义折叠问题。*
