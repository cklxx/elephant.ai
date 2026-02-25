# 2026-02-21 代码量与功能实现映射报告（全仓导出版）

## 1. 结论摘要
- 本仓库代码量大，主要是因为它同时承载了 **多交付端（Web/Lark/CLI）+ 多层架构（Domain/App/Delivery/Infra）+ 大量工具系统 + 评测体系 + 文档/技能资产**。
- 以 `git ls-tree -r --name-only HEAD` 口径统计：`2735` 个 tracked 文件；代码文件 `1709`；代码总行数 `328,716`（二进制文件 `10` 个，行数置 0）。
- 从“代码量与功能”对应关系看，主干能力是清晰的，但存在几类 **不合理复杂度**：
  - 安全边界缺口（HTTP 鉴权、Web 内部能力暴露、CSRF/XSS 风险）。
  - 层间依赖泄漏（infra 反向依赖 app，app 直接依赖 infra 具体实现）。
  - 若干超大文件/大对象模块（运行时、事件翻译、server app、web dev 页面）。
  - 多租户隔离与持久化策略不一致（memory/user root、task store/event history）。

## 2. 导出口径与产物

### 2.1 统计口径
- 覆盖范围：`git ls-tree -r --name-only HEAD`（基于当前 `HEAD` 快照，避免导出文件自污染）。
- 行数统计：文本文件按换行计数；二进制文件标记 `is_binary=true` 且 `line_count=0`。
- 模块归属：按路径前缀分层（domain/app/delivery/infra/web/...）并细分到 `module_l2`。
- 功能映射：路径前缀优先 + 关键词兜底启发式映射（在 CSV 中用 `capability_source` 标注来源）。

### 2.2 导出文件
- `docs/analysis/2026-02-21-repo-file-catalog.csv`：全仓文件清单（`2736` 行，含表头）。
- `docs/analysis/2026-02-21-code-file-capability-mapping.csv`：代码文件能力映射（`1710` 行，含表头）。
- `docs/analysis/2026-02-21-module-capability-summary.csv`：模块级聚合映射（`307` 行，含表头，`306` 个 module_l2）。
- `docs/analysis/2026-02-21-code-footprint-summary.json`：汇总统计与热点文件。
- 生成脚本：`scripts/analysis/generate_code_footprint_mapping.py`。

## 3. 为什么这么复杂（复杂度来源拆解）
1. 多交付端并存：`web/`、`internal/delivery/channels/lark/`、`cmd/` 同时演进。
2. 领域运行时复杂：`internal/domain/agent/react/` 承担 ReAct、工具批处理、后台任务、状态转换。
3. 工具与外部系统集成多：`internal/infra/tools/`、`internal/infra/lark/`、`internal/infra/llm/`、`internal/infra/external/`。
4. Server 侧承担聚合能力：`internal/delivery/server/` 既有 API 路由又有 app service（任务/快照/广播）。
5. 评测体系独立且体量不小：`evaluation/`。
6. 文档/技能资产沉淀深：`docs/`、`skills/` 文件数高。

## 4. 代码量分布（按层）

| layer | code_files | code_lines | 占比 |
|---|---:|---:|---:|
| infra | 368 | 71,690 | 21.8% |
| web | 322 | 56,880 | 17.3% |
| delivery | 221 | 47,801 | 14.6% |
| app | 181 | 40,577 | 12.4% |
| domain | 128 | 23,219 | 7.1% |
| evaluation | 63 | 21,321 | 6.5% |
| shared | 91 | 18,994 | 5.8% |
| cmd | 77 | 13,990 | 4.3% |
| scripts | 68 | 11,209 | 3.4% |
| 其他层合计 | 190 | 23,035 | 7.0% |

> 注：完整精确分层统计见 `docs/analysis/2026-02-21-code-footprint-summary.json`。

## 5. 模块内模块映射（Top 25，完整映射见 CSV）

| layer | module_l2 | code_files | code_lines | 主能力 |
|---|---|---:|---:|---|
| delivery | `internal/delivery/server` | 126 | 23,889 | HTTP API 路由、中间件与会话接口 |
| infra | `internal/infra/tools` | 113 | 22,207 | 内建工具实现（文件、记忆、日历、子代理等） |
| domain | `internal/domain/agent` | 116 | 21,484 | ReAct 推理循环、工具编排与后台任务运行时 |
| delivery | `internal/delivery/channels` | 75 | 19,903 | Lark 渠道网关、适配与会话桥接 |
| app | `internal/app/agent` | 91 | 19,743 | 应用层会话编排与事件翻译 |
| web | `web/components/agent` | 73 | 14,892 | Agent 事件与卡片 UI 组件 |
| evaluation | `evaluation/agent_eval` | 30 | 13,929 | 评测数据、脚本与结果 |
| cmd | `cmd/alex` | 73 | 13,758 | 命令行程序入口 |
| infra | `internal/infra/lark` | 50 | 10,867 | Lark 基础设施客户端与 OAuth/日历能力 |
| shared | `internal/shared/config` | 48 | 9,760 | 跨层共享基础能力 |
| infra | `internal/infra/llm` | 50 | 9,377 | LLM 提供方适配与调用基础设施 |
| app | `internal/app/context` | 34 | 8,007 | 上下文预算与策略管理 |
| web | `web/app/dev` | 11 | 7,203 | Web 运维/调试界面 |
| web | `web/lib` | 28 | 6,447 | Web API 客户端与共享库 |
| evaluation | `evaluation/swe_bench` | 17 | 4,802 | 评测数据、脚本与结果 |
| infra | `internal/infra/external` | 18 | 4,450 | 外部代理/子进程桥接能力 |
| infra | `internal/infra/skills` | 16 | 4,162 | 基础设施层实现 |
| scripts | `scripts/lark` | 9 | 3,779 | 自动化脚本与研发工具链 |
| app | `internal/app/scheduler` | 13 | 3,451 | 计划调度应用逻辑 |
| infra | `internal/infra/mcp` | 12 | 3,241 | 基础设施层实现 |
| app | `internal/app/toolregistry` | 13 | 3,082 | 工具注册与生命周期管理 |
| infra | `internal/infra/memory` | 15 | 2,525 | 长期记忆存储、索引与检索 |
| web | `web/app/conversation` | 16 | 2,456 | Web 会话主界面 |
| web | `web/components/ui` | 27 | 2,447 | Web 通用 UI 组件 |
| infra | `internal/infra/observability` | 10 | 2,130 | 日志、指标、追踪可观测性基础设施 |

## 6. 合理性分析

### 6.1 合理部分（与产品能力匹配）
- 架构分层和能力聚类总体可解释：Domain/App/Delivery/Infra 主体代码量分布相对均衡。
- 核心能力（Agent runtime、工具系统、交付层、Web 交互）占比高，符合“主动式 AI 助手平台”定位。
- 测试覆盖比例在若干后端层不低（app/delivery/infra 约 35%~45% 文件包含测试文件），说明质量意识存在。

### 6.2 不合理部分（可优化复杂度）
- **安全复杂度债务** 高于当前体量应有水平：鉴权边界、内部接口暴露、CSRF/XSS 风险并存。
- **文件级复杂度集中**：非测试文件 `>=800` 行有 `24` 个，`>=1000` 行有 `9` 个；变更风险高。
- **层间耦合不纯**：存在 infra→app 反向依赖与 app→infra 具体实现依赖，削弱可替换性。
- **存储与隔离策略不一致**：memory 多租户隔离语义与实现不一致；task/event 默认持久化策略偏弱。

## 7. Subagent 扫描架构问题（汇总）

### P0（必须优先修复）
1. HTTP API 鉴权缺失：`internal/delivery/server/http/router.go`。
2. hooks bridge 可在空 token 下工作：`internal/delivery/server/hooks_bridge.go`。
3. tracing 初始化失败后可能空指针 panic：
   - `internal/infra/observability/observability.go`
   - `internal/infra/observability/tracing.go`

### P1（短期修复）
1. memory user 隔离语义与实现不一致：
   - `internal/infra/memory/paths.go`
   - `internal/infra/memory/indexer.go`
2. 内部/开发接口仅靠环境变量门控而非鉴权：
   - `internal/delivery/server/http/router.go`
   - `internal/delivery/server/http/api_handler.go`
3. Web dev 页面与内部配置更新入口暴露：
   - `web/app/dev/**`
   - `web/lib/api.ts`
4. rate limit 信任未验证 `X-Forwarded-For`：
   - `internal/delivery/server/http/http_util.go`
   - `internal/delivery/server/http/middleware_rate_limit.go`
5. Domain 层含外部 HTTP 与 logging 实现：`internal/domain/materials/attachment_migrator.go`

### P2（中期治理）
1. 层间依赖泄漏：
   - infra 依赖 app：`internal/infra/tools/builtin/orchestration/subagent.go`
   - app 依赖 infra 具体：`internal/app/agent/coordinator/options.go`
2. event history 默认无限会话/TTL：`internal/delivery/server/app/event_broadcaster.go`
3. snapshot replay 清空后重写策略风险：`internal/delivery/server/app/snapshot_service.go`
4. Web markdown 原始 HTML + 协议放宽：`web/components/ui/markdown/MarkdownRenderer.tsx`
5. state-changing 接口 CSRF 防护不足：`web/lib/api.ts`

## 8. 优化点（按优先级）

### 8.1 0-7 天（安全止血）
1. 恢复并强制 API 鉴权中间件；内外部路由分级授权。
2. hooks bridge 强制 token，不满足即禁用入口。
3. Web `dev` 路由服务端门禁 + 后端 RBAC；生产默认关闭。
4. markdown 渲染禁用 `rehypeRaw` 或收紧协议白名单；补充 XSS 测试。
5. 增加 CSRF 机制（双提交 Cookie 或 token header）。

### 8.2 2-4 周（结构去耦 + 稳定性）
1. 修复 observability nil tracer 路径，统一 noop fallback。
2. memory 引入用户维度 root/index 分区，清理跨用户混用风险。
3. 移除 infra→app / app→infra 直接依赖，改用 domain/app 定义接口。
4. 为 task/event store 设置生产默认上限与持久化后端。
5. 拆分大文件（优先：`background.go`、`runtime.go`、`workflow_event_translator.go`、`task_execution_service.go`）。

### 8.3 1-2 个月（规模化治理）
1. 设立复杂度预算：新增/改动文件 LOC 上限、圈复杂度上限、模块依赖门禁。
2. 对 Top 20 模块建立 owner + 变更模板 + 风险清单。
3. 将“架构扫描 + 导出统计 + 安全检查”纳入 CI 周期任务。

## 9. 量化目标（建议）
- P0 问题在 1 个迭代内清零。
- 非测试文件 `>=1000` 行：`9 -> <=3`。
- 非测试文件 `>=800` 行：`24 -> <=10`。
- Top 5 模块代码占比：`32.7% -> <30%`（通过拆分与职责下沉）。

## 10. 参考实践（用于评估合理性）
- DDD/Clean Architecture：依赖方向单向（Domain 不依赖 Infra 细节）。
- SOLID：单一职责与依赖倒置，降低大文件和大对象风险。
- OWASP ASVS：鉴权、CSRF、XSS 与内部接口最小暴露。
- Google SRE/可观测性实践：故障路径必须 fail-safe（如 tracing init fallback）。
- Twelve-Factor App：配置与运行环境隔离、可替换持久化后端。
