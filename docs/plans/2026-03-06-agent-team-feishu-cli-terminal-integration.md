# Agent Team / Skills CLI / Feishu CLI / 可视化 Terminal 一体化方案

日期：2026-03-06  
状态：Proposal / 可执行方案  
目标：统一 agent team 的产品契约与工程实现，收敛为 CLI-first、skills-aware、用户可见的执行体验。

---

## 0. 结论

建议按 **A → B** 路线推进：

- **A. 先统一 Team/Skills/CLI 的产品契约，并把 terminal 打磨成用户可见现场**
- **B. 再把面向 LLM 的飞书操作面收敛成 canonical Feishu CLI**
- 暂不直接上“大而全的 agent workbench”，避免把收敛问题做成大工程

一句话定义：

> **Team 是执行引擎，Skills 是 LLM 可见能力目录，CLI 是稳定契约，Terminal 是用户可见执行现场。**

---

## 1. 现状判断

### 1.1 已具备基础

#### Team CLI 主链路已在
当前仓库已经有以下命令：

- `alex team run`
- `alex team status`
- `alex team inject`
- `alex team terminal`

文档依据：
- `docs/guides/orchestration.md`
- `skills/team-cli/SKILL.md`
- `cmd/alex/team_cmd.go`
- `cmd/alex/team_status_cmd.go`

这说明 Team 的 CLI-first 路线已经成形，不需要再重造一套执行入口。

#### Skills 披露机制已在
当前已有：
- skills frontmatter 元数据
- `<available_skills>` prompt 注入
- `skills` tool 的 list/show/search
- “canonical entrypoint per product capability domain” 去重原则

文档依据：
- `docs/guides/skills.md`

#### Team runtime / terminal 能力已在
当前已支持：
- runtime status sidecar
- runtime artifacts
- tmux session / pane
- terminal attach / capture / stream

说明“让用户看到 agent 正在工作”已经有工程底座，但还没完全产品化。

### 1.2 当前核心问题

#### 问题 1：产品契约仍有双轨
对外希望讲 CLI-first / Team-first；但历史文档和部分代码叙事仍围绕：
- `run_tasks`
- `reply_agent`

虽然它们已被降级为 internal detail，但项目资料里仍有较多残留，会造成：
- LLM 认知分裂
- 开发者理解不一致
- 用户文档与实际入口不一致

#### 问题 2：飞书能力面仍双轨
当前飞书能力同时存在于：
- channel / Lark delivery / Go tool action matrix
- 未来希望统一的 Feishu CLI 能力面

如果不收敛，长期会出现：
- 两套 schema
- 两套错误语义
- 两套 prompt 说明
- 两套扩展路径

#### 问题 3：terminal 更像调试工具，不像产品体验
现在已有 `attach/capture/stream`，但还停留在“工程可用”。
用户真正需要的是：
- 谁在执行
- 执行到哪一步
- 哪个角色卡住了
- 可以怎么干预
- 结果在哪

---

## 2. 产品目标重述

### 目标 1：把 agent team 功能整合好，用 CLI 调度，skills 披露给 LLM

推荐定义：

- **Team = execution substrate**
- **Skills = semantic affordance to LLM**
- **CLI = concrete execution contract**

也就是：
- LLM 不直接学习复杂 runtime 细节
- LLM 通过 skill 知道何时使用 Team
- 一旦选择 Team，即调用稳定的 `alex team ...` 契约

### 目标 2：用 Feishu CLI 替代面向 LLM 的飞书 Go 工具面

推荐边界：
- **保留 Go 的 delivery / webhook / session / channel 基础设施**
- **把面向 LLM 的飞书对象操作统一成 Feishu CLI**

即：
- 飞书消息入口仍由 Lark channel 层承接
- LLM 要“操作飞书对象”时，统一走 Feishu CLI

### 目标 3：通过 Team 打开的 terminal，让用户直观看到执行现场

推荐定义：
- 不是暴露 tmux 细节
- 而是把 Team execution 变成一个用户能理解、能观察、能干预的“现场”

---

## 3. 统一产品模型

建议把用户可见模型收敛为四类对象：

### 3.1 Team Run
表示一次多 agent 协作执行。

核心字段：
- `team_run_id`
- `session_id`
- `goal`
- `status`
- `started_at`
- `updated_at`
- `roles[]`
- `artifacts[]`
- `recent_events[]`

### 3.2 Role
表示 Team 内一个角色或子任务执行体。

核心字段：
- `role_id`
- `task_id`
- `agent_type`
- `summary`
- `status`
- `terminal_ref`
- `artifact_refs[]`

### 3.3 Terminal View
表示角色实时执行现场。

能力：
- `stream`
- `capture`
- `attach`

但对用户展示时建议统一叫：
- **Live Terminal**
- **Recent Output**
- **Open Interactive View**

### 3.4 Artifact
表示执行产物。

示例：
- 报告 md
- 分析文件
- patch 路径
- 审查结果
- 飞书文档链接

---

## 4. 目标架构

### 4.1 Team 能力架构

#### 用户层
- 用户发起复杂任务
- 用户查看 Team Run 状态
- 用户查看 Live Terminal
- 用户给某个 role 注入 follow-up

#### LLM 层
仅暴露四个心智动作：
- `team.run`
- `team.status`
- `team.inject`
- `team.terminal`

#### CLI 层
实际映射到：
- `alex team run`
- `alex team status`
- `alex team inject`
- `alex team terminal`

#### Runtime 层
继续复用当前已有：
- runtime sidecar
- background task manager
- team runtime artifacts
- tmux session / pane
- status recorder

### 4.2 Feishu 能力架构

#### 保留层
保留 Go 实现：
- Lark message ingress
- webhook / callback
- chat routing
- session binding
- streaming / event delivery
- progress listener

#### 收敛层
新增 canonical `feishu-cli` 能力面，专门服务 LLM 的对象操作：
- message send/history
- calendar query/create/update
- task list/create/update
- doc read/write
- wiki query/create
- drive list/copy/delete
- bitable query/update
- vc / rooms / meetings

#### 适配方式
短期：
- CLI 内部可先调用现有 Go client / infra service
- 先统一调用面与错误语义

中期：
- 工具提示、审批、日志、审计围绕 CLI 收口

---

## 5. 为什么不建议一刀切“全部用 Feishu CLI 替代 Go 代码”

因为当前 Go 代码承载的不只是 API 调用，还包括：
- delivery 生命周期
- bot channel 行为
- session continuity
- progress callback
- 可靠性处理
- 富文本/附件/回调等边界细节

因此建议采用 **分层替代**：

### 保留 Go 的部分
- channel infra
- webhook / callback
- long-lived session / context
- message delivery pipeline

### CLI 替代的部分
- 面向 LLM 的飞书对象操作面
- 高层 job-to-be-done 能力抽象
- 审计和结构化调用契约

这样收益最大、风险最低。

---

## 6. 产品视角下最该打磨的体验

### 6.1 可委托
用户可以明确感知：
- 复杂任务已被拆解
- 多个 agent 正在分工
- 系统不是在“想”，而是在“干活”

### 6.2 可观察
用户一眼能看到：
- 哪些角色在运行
- 当前进度
- 最近行为
- 有无卡住

### 6.3 可干预
用户可以：
- 对指定角色发送 follow-up
- 提供额外上下文
- 改变目标边界
- 拉取更长 terminal capture

### 6.4 可复盘
用户可以回看：
- role 输出摘要
- artifacts
- 关键事件时间线
- 最终结论

---

## 7. Team Terminal 的产品化方案

当前已有能力：
- `attach`
- `capture`
- `stream`

下一步应该从“命令能力”提升为“统一展示模型”。

### 推荐展示结构

#### Header
- Team 名称 / goal
- status
- started_at
- session_id

#### Roles
每个 role 显示：
- role 名称
- 当前状态：running / blocked / completed / failed
- 最近一句摘要

#### Recent Activity
时间线显示最近事件。

#### Live Terminal
展示最近 50~200 行输出。

#### Artifacts
展示：
- 文件路径
- 文档链接
- 结果摘要

#### Intervene
允许用户：
- 对 role follow-up
- 再跑一次 capture
- attach interactive terminal

### 多端呈现建议

#### CLI
- `alex team status --json` 提供结构化数据
- `alex team terminal --mode capture` 提供 terminal 窗口

#### Web
- Team Run 面板 + role tabs + recent output

#### Lark
- Team progress 卡片 + 最近输出节选 + “查看详情/继续跟进”入口

---

## 8. Skills 披露策略

### 8.1 原则
对 LLM 只暴露“能力域”，不要暴露“实现碎片”。

### 8.2 推荐保留的 canonical skills

#### `team-cli`
职责：
- 运行多 agent team
- 查状态
- 注入 follow-up
- 查看 terminal

#### `feishu-cli`
职责：
- 统一飞书对象操作
- 成为面向 LLM 的唯一飞书能力入口

#### `artifact-management`
职责：
- 统一执行产物创建/读取/交付

### 8.3 去重规则
对于同一能力域：
- 只保留一个 canonical entrypoint
- 不再同时暴露多个 wrapper skill
- 历史残留技能只保留内部兼容，不再作为 prompt 主入口

---

## 9. 命名与叙事统一建议

### 用户可见词汇
建议只保留：
- **Team Run**
- **Roles**
- **Live Terminal**
- **Artifacts**
- **Follow-up**

### 内部实现词汇
降级为内部：
- `run_tasks`
- `reply_agent`
- `sidecar`
- `tmux pane`

### 原因
减少：
- 文档负担
- prompt 负担
- 用户理解负担

---

## 10. 路线图

## Phase 1：统一契约（P0）

### 目标
让产品叙事、prompt、技能披露、CLI 帮助信息保持一致。

### 工作项
1. 统一文档到 CLI-first / Team-first 叙事
2. `run_tasks/reply_agent` 明确标记为 internal only
3. 对 Team 相关 prompt/preparation 文案统一为 `alex team ...`
4. 清理产品文档中与当前契约冲突的表述

### 验收标准
- 用户文档与 skill 文档中，Team 的唯一入口是 `alex team ...`
- 搜索产品主文档时，不再把 `run_tasks/reply_agent` 当成主要入口说明

### 风险
- 风险低
- 主要是文档/提示词/帮助信息不一致问题

---

## Phase 2：Terminal 体验产品化（P1）

### 目标
让 Team 执行现场从开发者工具变成用户可见体验。

### 工作项
1. 基于 `team status --json` 定义统一 view model
2. 在 CLI/Web/Lark 上统一 Team Run 展示结构
3. 支持 role-level terminal capture
4. 支持 role-level inject / follow-up
5. 汇总 artifacts

### 验收标准
- 用户能在 10 秒内看懂“谁在做什么、卡没卡、结果在哪”

### 风险
- 中低
- 主要是展示模型统一与多端适配成本

---

## Phase 3：Feishu CLI 收敛（P2）

### 目标
把面向 LLM 的飞书能力面统一成 canonical CLI。

### 工作项
1. 盘点现有 channel action matrix 与 infra client 能力
2. 设计 `feishu-cli` 命令与 skill 契约
3. 优先接入读操作：query/list/read/history
4. 再接入写操作：send/create/update/delete
5. 明确 approval / audit / error contract

### 验收标准
- LLM 对飞书操作优先选择 `feishu-cli`
- 新增飞书能力不再需要扩散到多套工具面

### 风险
- 中
- 主要是审批流、富文本、附件、流式反馈契约设计

---

## Phase 4：高级工作台（P3，可选）

### 目标
将 Team Run 升级为完整 agent workbench。

### 工作项
1. Role pane 交互化
2. 可回放事件时间线
3. Artifact graph
4. 重跑 / 分叉 / 审查模式

### 验收标准
- 用户可把 Team Run 当成一等工作对象使用

### 风险
- 高
- 容易过早做重

---

## 11. 三个候选结论

### 方案 A：先收敛契约 + terminal 产品化
- **指标影响**：理解成本 -40%，多 agent 信任感 +30%，debug 成本 -25%
- **成本**：中
- **风险**：低
- **回滚**：容易，几乎不动底层执行链

### 方案 B：同步推进 Feishu CLI 收口
- **指标影响**：tool selection 准确率 +20~30%，新增飞书能力接入效率 +50%
- **成本**：中高
- **风险**：中
- **回滚**：可通过 adapter 回退到旧 Go tool 面

### 方案 C：直接做 agent workbench
- **指标影响**：用户感知价值最高，差异化最强
- **成本**：高
- **风险**：高
- **回滚**：困难

### 推荐
**优先走 A → B。**

---

## 12. 立刻可执行的下一步

### 本周建议
1. 确认 Team/Skills/CLI 的统一产品词汇
2. 出一份 `feishu-cli` capability map
3. 为 Team Run 定义统一 view model
4. 选一个端先做 Team Run 可视化：优先 Web，其次 Lark 卡片

### 工程上建议先做的 4 件事
1. 清理文档里 `run_tasks/reply_agent` 的用户入口描述
2. 补一份 `feishu-cli` 设计文档
3. 补一份 Team Run view model / JSON contract 文档
4. 将 terminal capture/status/artifacts 统一为同一个展示数据模型

---

## 13. 最终建议

当前最优路径不是重写，而是**收敛**：

- 收敛 Team 的产品入口到 `alex team ...`
- 收敛 Skills 的能力披露到 canonical entrypoint
- 收敛飞书对象操作到 `feishu-cli`
- 收敛 terminal 到用户可感知、可干预的执行现场

如果做对，用户最终感知到的不是“又多了一堆命令”，而是：

> 复杂任务真的能被多个 agent 分工执行，而且我看得见、插得上、拿得到结果。

