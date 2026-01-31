# Plan: 无状态工具重构 — 移除 ID 依赖与强制协议

**Created**: 2026-01-31
**Status**: Completed (2026-02-01)
**Author**: cklxx + Claude

## 问题描述

当前工具设计存在严重的 UX 和效率问题：

### 症状
用户说 "帮我制定 OKR"，LLM 回复：
> "Got it, let's start by following the OKR management skill workflow. First, I need to clarify the main Objective..."

然后调用 `clarify(run_id="run-x4fJluAIsAH4", task_id="okr-1", needs_user_input=true, question_to_user="请描述你的目标")` 而不是直接开始制定 OKR。

### 根因分析

**三层耦合导致了这个问题：**

#### 1. ReAct Runtime 强制协议门控 (`runtime.go:253-311`)
```
enforceOrchestratorGates() 强制执行：
  任何 action tool 调用 → 必须先 plan() → planEmitted = true
  如果 complexity="complex" → 每个 task 必须先 clarify() → clarifyEmitted[taskID] = true
  plan/clarify/request_user 必须单独调用（不能与其他工具并行）
```

**后果**：每个请求至少浪费 1-2 个 LLM round trip 在协议工具上。

#### 2. 工具要求 LLM 提供上下文已有的 ID
- `plan()`: 必须参数 `run_id`（与 context 校验）
- `clarify()`: 必须参数 `run_id` + `task_id`
- System prompt 注入: `"run_id: {value}\n- Use this exact run_id for plan() (and clarify() if used)."`

**后果**：LLM 需要在 system prompt 中找到 run_id 并复制到参数中，这纯粹是浪费 token。

#### 3. Skill 设计为多轮表单向导
OKR skill 显式指导 LLM 用 `request_user` 逐步收集：
```
Step 1: request_user → 问 Objective
Step 2: request_user → 逐个问 KR
Step 3: request_user → 问 review cadence
Step 4: request_user → 确认
```

**后果**：即使用户已经说了 "月收入提升 30%"，LLM 仍然进入向导模式。

### 受影响的工具/模块清单

| 组件 | 问题 | 严重程度 |
|------|------|----------|
| `plan` tool | `run_id` 是 required 参数，与 context 重复 | 高 |
| `clarify` tool | `run_id` + `task_id` 是 required 参数 | 高 |
| `enforceOrchestratorGates()` | 强制 plan→clarify→act 顺序 | 高 |
| `preparation/service.go:247-251` | System prompt 注入 run_id 指令 | 高 |
| OKR skill | 多轮 request_user 向导 | 中 |
| `bg_dispatch` tool | `task_id` 是 required 参数 | 低 |
| `todo_read/update` | sessionID 从 context 获取（已无状态）| 无（ok）|
| `okr_read/write` | goal_id 是业务标识（已无状态）| 无（ok）|
| `request_user` tool | 无 ID 依赖 | 无（ok）|

## 设计原则

1. **工具从 context 自取 ID，不要求 LLM 传入** — run_id、session_id 等由 Go context 提供
2. **取消强制协议** — plan/clarify 变为可选 UI 提示，不做门控
3. **单次完成优先** — Skill 应尽量用已有信息完成任务，只在真正缺少关键信息时才问
4. **ID 自动生成** — bg_dispatch 的 task_id 可选，自动生成可读标识

## 具体改动

### Batch 1: 移除 run_id 参数依赖

**文件**: `internal/tools/builtin/ui/plan.go`
- `run_id` 从 Required 中移除
- Execute() 内部通过 `id.RunIDFromContext(ctx)` 获取，忽略 LLM 传入的值
- 保留 metadata 中写入 run_id（供前端渲染用）

**文件**: `internal/tools/builtin/ui/clarify.go`
- `run_id` 从 Required 中移除
- `task_id` 从 Required 中移除，如果不传则自动生成 `task-{seq}`
- Execute() 内部自取 context run_id

**文件**: `internal/agent/app/preparation/service.go:247-251`
- 移除 system prompt 中的 "Runtime Identifiers" 段落
- 不再告诉 LLM run_id 值

### Batch 2: 移除强制 plan→clarify 门控

**文件**: `internal/agent/domain/react/runtime.go`
- 删除 `enforceOrchestratorGates()` 中的 plan gate（line 300-301）
- 删除 clarify gate（line 304-309）
- 保留 plan/clarify/request_user 必须单独调用的约束（防止并行冲突）
- 保留 `updateOrchestratorState()` — 如果 LLM 选择调用 plan/clarify，仍然正确跟踪状态
- `planGatePrompt()` 和 `clarifyGatePrompt()` 删除

**文件**: `internal/agent/app/preparation/service.go:31`
- 修改 DefaultSystemPrompt：从 "Always call plan()" 改为 plan/clarify 是可选的
- 新措辞：plan() 用于设定 UI 目标标题（可选）；clarify() 用于需要暂停等待用户输入时（可选）

### Batch 3: bg_dispatch task_id 改为可选

**文件**: `internal/tools/builtin/orchestration/bg_dispatch.go`
- `task_id` 从 Required 移除
- 如果 LLM 不传 task_id，用 `description` 自动生成 slug（如 "research-market-data"）
- 返回结果中始终包含最终使用的 task_id

### Batch 4: 重构 OKR 等 skill 为单次完成模式

**文件**: `skills/okr-management/SKILL.md`
- 重写为 "intelligent fill" 模式：分析用户输入，提取已有信息，一次性创建 OKR
- 只在真正缺少核心信息（如完全没说目标是什么）时才用 request_user
- 移除逐步 request_user 向导流程

**其他 skills**: 按同样原则审查，确保不强制多轮

### Batch 5: 更新测试

- `coordinator_workflow_tool_test.go`: 移除测试中对 plan() 的强制调用
- 新增测试：验证 LLM 可以直接调用 action tool 而不先 plan()
- 新增测试：plan/clarify 的 run_id 参数可选
- 新增测试：bg_dispatch 不传 task_id 时自动生成

## 不改动的部分

- `todo_read/update`: sessionID 从 context 获取，已经是无状态设计
- `okr_read/write`: goal_id 是业务标识符，不是会话状态
- `request_user`: 无 ID 依赖，已经无状态
- `bg_status/bg_collect`: 它们查询的 task_id 是 bg_dispatch 返回的标识，属于正常的引用关系
- `subagent/explore`: 无 ID 依赖

## 风险评估

| 风险 | 缓解 |
|------|------|
| 前端依赖 plan metadata 渲染 UI | plan 仍可被调用，metadata 格式不变；只是不强制 |
| LLM 失去结构化思考 | system prompt 继续鼓励 plan/clarify，但不强制 |
| Plan review 功能失效 | 保留 updateOrchestratorState，如果 LLM 调 plan(complex) 仍触发 review |
| 测试覆盖 | 现有 plan 相关测试需调整，新增无 plan 直接执行的测试 |

## 进度

- [ ] Batch 1: 移除 run_id 参数依赖
- [ ] Batch 2: 移除强制 plan→clarify 门控
- [ ] Batch 3: bg_dispatch task_id 改为可选
- [ ] Batch 4: 重构 OKR skill
- [ ] Batch 5: 更新测试
- [ ] 全量 lint + test 验证
