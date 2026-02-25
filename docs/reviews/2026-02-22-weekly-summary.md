# 周报：2026-02-15 ~ 2026-02-22

> **51 commits · 155 files changed · +25,605 / -3,514 lines**

---

## 一、按主题分类

### 1. FileStore 统一化

**时间：** 02-15, 02-22
**范围：** `internal/filestore/`, stores 层全面迁移

将全局文件 I/O 收敛到 `filestore` 包，完成以下迁移：

| 原实现 | 迁入 filestore |
|---|---|
| Lark `PlanReviewLocalStore` | `filestore.Collection` |
| Lark `ChatSessionBindingLocalStore` | `filestore.Collection` |
| Lark `FileTokenStore` | `filestore.Collection` |
| Kernel `dispatch store` | `filestore` 原语 |
| Server `TaskStore` | `filestore` 原语 |
| Server `EventHistoryStore` | `filestore` 原语 |
| 散布的 `os.WriteFile` | `filestore.AtomicWrite` |

02-22 补充了 `RecoverStaleRunning`，解决 FileStore 上进程异常退出后的 stale running 状态恢复问题。

**关键提交：**
- `578e1aec` ~ `81d27ca9`：FileStore 迁移批次
- `e39c4097`：RecoverStaleRunning

---

### 2. Lark P1 能力大扩展

**时间：** 02-16 ~ 02-17
**范围：** `internal/channels/lark/`, `internal/tools/builtin/lark/`

新增完整的 Lark P1 API 能力矩阵：

| 模块 | 能力 |
|---|---|
| **Docx** | 文档读写 |
| **Wiki** | 知识库操作 |
| **Bitable** | 多维表格 |
| **Drive** | 云文档/文件管理 |
| **Sheets** | 电子表格 |
| **OKR** | 目标管理 |
| **Contact** | 通讯录 |
| **Mail** | 邮件 |
| **VC** | 视频会议 |

关键改进：
- **user_id 自动解析**：从 sender context 自动获取 user_id，不再要求用户手动传入
- **UserIdType 规范化**：统一使用 `user_id` 类型调用 OKR 等 API
- 配套单测覆盖 message handler / attachment handler / task store / plan review

**关键提交：**
- `9ef240fa` ~ `7f88f466`：P1 API wrappers + tool handlers
- `19d19881`, `ca7208e4`：user_id 自动解析
- `e893b111`, `1b0237ea`：单测补全

---

### 3. Agent 编排 & LLMPlanner

**时间：** 02-20 ~ 02-21
**范围：** `internal/agent/`, `internal/orchestration/`

两个核心新能力：

**LLMPlanner（02-20）**
- 用 LLM 动态分析任务，路由到最合适的 agent
- 通过 file config 反序列化注入配置

**team_dispatch（02-21）**
- Agent 团队协作编排工具
- 新增 agent team types 和 config 定义
- 经 coordinator + DI 注入，在 `RegisterSubAgent` 中注册
- 通过 P1 code review，修复了所有 P1 findings

**关键提交：**
- `6fa99d4a`, `f7828b29`：LLMPlanner
- `a49a3eea` ~ `de4a2492`：team_dispatch 全链路

---

### 4. 浏览器工具替换：ChromeDP → Playwright MCP

**时间：** 02-21
**范围：** `internal/tools/builtin/browser/`, `internal/mcp/`

整体替换浏览器自动化方案：

| Before | After |
|---|---|
| ChromeDP `browser_action` | Playwright MCP server |
| 进程内 Chrome 控制 | MCP 协议 + 独立 Playwright 进程 |

新增：
- Playwright MCP config builder
- Registry 注入机制
- Skills 层 progressive disclosure + autonomous execution 策略

**关键提交：**
- `c0281efc`, `1ee75df9`：ChromeDP → Playwright 替换
- `313eec5d`：MCP config builder + registry 注入
- `bf164b90`：skills 自主执行策略

---

### 5. DevOps 简化

**时间：** 02-21
**范围：** `dev.sh`, `scripts/`

大幅简化开发环境：
- 移除 Docker sandbox / ACP runtime 相关代码
- `dev.sh` 默认启动模式改为 lark-only
- 移除 `withAuthDB` 死参数，合并 orchestrator builders
- 修复 `mktemp` macOS BSD 兼容性

**关键提交：**
- `44640dab`：移除 Docker/sandbox/ACP
- `33257894`：dev.sh lark-only 默认
- `5ed746ad`：移除死代码
- `78f5cf0e`：mktemp 兼容修复

---

### 6. Prompt 工程重构

**时间：** 02-22
**范围：** `internal/agent/domain/react/`, presets 层

提示词架构化改造：

| Before | After |
|---|---|
| 命令式 section builders | Decision trees + ALWAYS/NEVER rules |
| 7C preset 正向描述 | NEVER patterns（反向约束更精确） |
| 冗长 tool routing suffix | 精简版 |

目标：让 prompt 组装逻辑更可预测、可测试、可组合。

**关键提交：**
- `8933d6a2`：decision trees + ALWAYS/NEVER rules
- `86fd2513`：7C NEVER patterns + tool routing 精简

---

### 7. 测试补全

**时间：** 02-16 ~ 02-17
**范围：** 全项目

密集的一轮测试补充，覆盖此前的空白区域：

| 模块 | 测试内容 |
|---|---|
| `react` core | 核心函数综合单测 |
| `coordinator` | session_manager, config_resolver |
| `llm` | responses parsing, factory, errors, helpers |
| `lark` SDK | response write error checks |
| `agent_eval` | request_user intent e2e 稳定化 |

**关键提交：**
- `e3149271`：react core 单测
- `d498171c`：coordinator 单测
- `49707932`：llm 单测
- `48d6bd62`, `bbbd51ba`：lark SDK + eval 稳定化

---

## 二、趋势分析

### 架构趋势

| 趋势 | 信号 | 影响 |
|---|---|---|
| **去中心化 → 收敛** | FileStore 统一、Docker/ACP 清理、ChromeDP→Playwright | 依赖减少，运维简化 |
| **Prompt 架构化** | Decision tree + ALWAYS/NEVER rules 替代拼字符串 | 可维护性、可测试性大幅提升 |
| **MCP 协议标准化** | Playwright MCP 接入，config builder + registry 模式 | 为更多 MCP 工具接入铺路 |

### 产品趋势

| 趋势 | 信号 | 影响 |
|---|---|---|
| **Lark 生态深入** | P1 批量 API 接入、user_id 自动解析 | Lark 成为最核心交互面 |
| **多 Agent 协作** | team_dispatch + LLMPlanner | 从单 agent 走向 agent 团队 |
| **Skills 自主执行** | progressive disclosure + autonomous execution | Agent 主动性增强 |

### 工程趋势

| 趋势 | 信号 | 影响 |
|---|---|---|
| **测试意识加强** | 一周内多次专门 test commit | 不再"功能先行、测试补票" |
| **基建做减法** | dev.sh 精简、sandbox 移除、死代码清理 | 开发体验改善，认知负担降低 |
| **Code review 制度化** | P1 findings 修复后才合入 | 代码质量守门 |

---

## 三、总结

本周是 **架构收敛 + 能力扩展** 并行推进的一周：

- **做减法**：统一 FileStore、移除 Docker/ACP、简化 dev.sh、精简 prompt
- **做加法**：Lark P1 九大模块、LLMPlanner、team_dispatch、Playwright MCP
- **补欠债**：测试覆盖、code review 修复、脚本兼容性

下一步关注点：
1. team_dispatch + LLMPlanner 的端到端联调和实战验证
2. Playwright MCP 在 skills 中的实际使用效果
3. Prompt decision tree 的测试覆盖和效果评估
4. Lark P1 工具的用户反馈和迭代
