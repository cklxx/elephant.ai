# Team Terminal UX Model (Draft)

日期：2026-03-06  
状态：Draft / implementation-ready

## 目标

把当前 `alex team terminal --mode attach|capture|stream` 从开发者调试能力，打磨成用户能理解的执行现场。

一句话：

> 用户看到的不是 tmux pane，而是「谁在干活、干到哪、卡没卡、结果在哪、我能不能插话」。

## 用户可见对象模型

### 1. Team Run
- goal
- overall status
- started_at / updated_at
- session_id
- roles[]
- artifacts[]
- recent_events[]

### 2. Role Card
- role name
- selected agent
- status: pending/running/blocked/completed/failed
- short summary
- last activity time
- open terminal
- inject follow-up

### 3. Live Terminal
对外用词建议：
- Live Terminal
- Recent Output
- Open Interactive View

不要把 `tmux pane` 作为产品文案主词。

### 4. Artifacts
- report/doc
- changed files
- review result
- links

## 推荐展示结构

### Header
- Goal
- Team status
- Session ID
- Start time

### Role Grid
每个 role 一张卡：
- 角色名
- agent 类型
- 当前状态
- 最近一句进展
- 打开 terminal / 发送 follow-up

### Activity Timeline
- bootstrap completed
- role started
- role waiting input
- role completed
- artifact generated

### Terminal Panel
默认显示最近 80~120 行 capture。
支持：
- expand
- stream
- attach（高级入口）

### Artifacts Panel
- 文件路径
- 飞书文档链接
- 报告摘要

## CLI / UI 统一 view model

底层继续复用：
- `alex team status --json`
- `alex team terminal --mode capture`
- `alex team inject`

上层统一映射成：
- TeamRunView
- RoleView
- TerminalView
- ArtifactView

## 交互原则

- 默认展示 capture，不默认 attach
- 有多个 role 时，优先展示异常/运行中的 role
- 注入 follow-up 时，按 role 发，不按底层 pane 发
- completed 后保留 terminal 与 artifacts 回看入口

## 成功标准

- 用户 10 秒内能回答：谁在执行？卡住没？结果在哪？
- 用户无需理解 tmux、runtime root、sidecar 等内部概念。
- Lark/Web/CLI 三端展示复用同一套 view model。

