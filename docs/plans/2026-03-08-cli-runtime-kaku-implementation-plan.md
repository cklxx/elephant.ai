# CLI Runtime + Kaku 实施计划（2026-03-08）

状态：Implementation Plan  
目标：把 Kaku 收敛为统一 CLI runtime/panel，让 Codex / Claude Code / Kimi / AnyGen / Colab 等成员以一致方式接入，并支持多 session 并行、hooks 通知、自动调度。

---

## 0. 先说结论

按 **4 个阶段** 做，先做 runtime 骨架，再接入成员，再做调度，再做产品化展示。

推荐顺序：

1. **P0：Runtime Skeleton**
2. **P1：CLI Member Adapter**
3. **P2：Hooks + Scheduler**
4. **P3：Panel / UX / Team Recipe**

原则：
- 先把“能稳定开多个 session 并恢复”做出来
- 再做“agent 自动根据 hooks 调度别人”
- 最后再做复杂 UI 和更多成员扩展

---

## 1. 目标拆分

### P0：Runtime Skeleton（先打地基）

交付：
- `runtime session` 对象模型
- `tape` 持久化
- `kaku panel` 统一命令执行面
- session 生命周期：create/start/stop/resume/cancel/list/status

最小能力：
- 一个 panel 可绑定一个 session
- 一个 runtime 可同时管理多个 session
- 重启后 session 元数据和 tape 可恢复

建议模块：
- `internal/runtime/session/`
- `internal/runtime/tape/`
- `internal/runtime/panel/`
- `internal/runtime/store/`

验收：
- 同时启动 3 个 session
- 可以单独 inject 输入
- 可以停止/恢复某个 session
- 重启后仍能看到 session 列表和最近状态

---

### P1：CLI Member Adapter（接入成员）

交付：
- 通用成员接口：`MemberAdapter`
- 首批适配器：
  - `codex`
  - `claude_code`
  - `kimi`
- 扩展适配器预留：
  - `anygen`
  - `colab`

统一抽象：
- `StartSession`
- `InjectInput`
- `CaptureOutput`
- `Interrupt`
- `Resume`
- `DetectState`

原则：
- 成员都按“人类使用 CLI”的方式接入
- Kaku 负责注入和控制
- adapter 只补每个 CLI 的命令模板、状态识别、hooks 格式

验收：
- 3 个 coding CLI 都能通过同一 runtime 拉起
- 同一套 session 面板能切换不同 member
- adapter 不影响上层调度逻辑

---

### P2：Hooks + Scheduler（跑起来像 runtime）

交付：
- hooks 事件模型
- 轻量监控
- 调度器（按依赖/状态自动推进）

hooks 事件：
- `started`
- `heartbeat`
- `needs_input`
- `completed`
- `failed`
- `stalled`
- `handoff_required`

调度规则：
- A 完成 -> 自动触发 B
- A stalled -> 通知 agent 决策
- A needs_input -> 进入 waiting_input
- 多成员并行执行 -> 汇总状态给上层 agent

验收：
- 两个 session 可串行依赖
- 三个 session 可并行跑
- 某个 session 卡住时能自动上报，不静默
- 完成事件可触发后续 session

---

### P3：Panel / UX / Team Recipe（变成产品）

交付：
- 通用 panel
- session list
- live output
- compact progress checkpoint
- team recipe（把 team 降成 recipe，而不是底层 runtime）

用户可见区块：
- Sessions
- Live Terminal
- Artifacts
- Recent Events
- Intervention

新增能力：
- team recipe = 一组 session 模板
- 支持 coding / research / anygen / colab 混编

验收：
- 用户能在一个界面看多个 session
- 10 秒内看懂谁在跑、谁卡住、谁完成
- team 只是 recipe，不再承担 runtime 本身职责

---

## 2. 里程碑排期

### Milestone 1（3~5 天）
- session/tape/store 骨架
- Kaku panel 统一执行
- session 基础 CLI

CLI 草案：
- `alex runtime session start`
- `alex runtime session list`
- `alex runtime session status`
- `alex runtime session inject`
- `alex runtime session stop`
- `alex runtime session resume`

### Milestone 2（4~6 天）
- codex / cc / kimi adapter
- 输出抓取
- 状态识别
- 基础 hooks

### Milestone 3（4~6 天）
- scheduler
- 依赖编排
- stalled 检测
- compact progress checkpoint

### Milestone 4（5~7 天）
- panel/UI
- team recipe
- anygen / colab 扩展入口
- 文档和示例完善

---

## 3. 风险与处理

### 风险 1：不同 CLI 的交互模式不一致
处理：adapter 层吸收差异，runtime 不感知具体内核。

### 风险 2：长任务恢复不稳定
处理：先保证 session metadata + tape 恢复，再做完整终端恢复。

### 风险 3：hooks 噪音太多
处理：只保留 6 类核心事件，其他都归并成 event payload。

### 风险 4：team/runtime 抽象打架
处理：明确 runtime 是底座，team 是 recipe。

---

## 4. 建议现在就开的实现项

按优先级：

1. 建 `runtime/session + tape + store` 目录和对象模型
2. 抽 `MemberAdapter` 接口
3. 接 `codex` / `claude_code` 两个首批 adapter
4. 加 `runtime session` CLI
5. 补 hooks 事件总线
6. 再把 `team` 改造成 recipe

---

## 5. 我建议的执行方式

先做 **Milestone 1 + 2**，不要一口气上完整 UI。

也就是先证明这 4 件事：
- 多 session 能开
- 能恢复
- 能 inject
- 不同成员能共用同一个 panel/runtime

这 4 件成立，后面的 scheduler / team recipe / AnyGen / Colab 扩展就顺了。


## 7. Leader Agent / PMO 能力（新增）

这个能力应该作为 runtime 的控制层单独设计，不并入某个 coding agent。

### 定义
主控 agent 不直接承担具体编码，而是持续做三件事：
- 看各 session / agent 当前进度
- 重新分配任务与依赖顺序
- 以“单位时间内项目推进最快”为目标做调度决策

### 角色定位
建议拆成两个逻辑角色，可先由一个 agent 承担：
- **Leader Agent**：面向执行，决定谁继续做、谁暂停、谁接棒、谁并行
- **PMO Agent**：面向项目，盯里程碑、阻塞点、风险、资源利用率

### 需要读取的核心信号
- session 状态：starting/running/stalled/waiting_input/completed/failed
- 最近输出时间 / hooks 心跳
- artifact 产出情况
- 任务 DAG 依赖是否解除
- 各 agent 擅长类型（coding/review/research/doc/tool-use）
- 当前成本与并发占用

### 决策目标
统一目标不是“平均分配任务”，而是：
- **最小化总完工时间**
- **最大化高价值 agent 的有效忙碌时间**
- **减少等待依赖与空转**
- **阻塞出现时尽快切换可并行工作**

### 关键动作
- dispatch：给空闲 agent 分配新任务
- rebalance：某 agent 卡住时，把可拆分子任务转给别的 agent
- unblock：发现等待输入/依赖时，优先补齐缺口
- promote：关键路径任务优先
- handoff：某 agent 完成后自动触发下游 session
- intervene：需要人接手时及时上报

### MVP 做法
第一版不要做复杂优化器，只做规则调度：
1. 优先保证关键路径不断档
2. 有可并行任务时，优先喂给空闲 agent
3. 某 session stalled 超阈值，自动降级/换人/请求人工
4. 某 agent 完成后，自动扫描可启动下游任务
5. 每轮输出 compact checkpoint，避免黑盒

### 产品表现
用户看到的不是抽象“调度器”，而是：
- 当前谁空闲
- 谁在关键路径上
- 谁卡住了
- 下一步系统准备把什么任务交给谁
- 为什么这么分配

### 实现建议
放在 runtime control plane：
- `session runtime` 负责执行
- `leader/pmo controller` 负责调度
- `skills facade` 负责把调度能力暴露给上层 agent

这层后面也能扩展到 AnyGen / Colab / browser worker / notebook worker，不只限 coding agent。

