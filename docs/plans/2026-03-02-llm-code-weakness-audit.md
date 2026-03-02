# LLM 代码生成系统性缺陷审计

> Date: 2026-03-02
> Status: Completed

## 一、LLM 写代码的 6 大系统性缺陷

基于 2025-2026 年最新研究综合分析（IEEE Spectrum, arXiv, CodeRabbit, MSR 2025）。

### 缺陷分类体系

| # | 缺陷类别 | AI/人类倍率 | 根因 | 度量方法 |
|---|---------|-----------|------|---------|
| 1 | **静默失败** | — | LLM 优化"看起来对"而非"真正对"，会删除 safety check 或伪造输出格式 | 运行时行为分析 vs 预期规格；变异测试(mutation testing)通过率 |
| 2 | **错误处理缺失** | ~2× | 训练数据中 happy path >> error path | 静态分析：err 路径覆盖率、bare return err 计数、error wrapping 率 |
| 3 | **边界条件遗漏** | 1.75× | 模式匹配到常见 case，跳过 edge case | 边界值测试覆盖率；fuzzing 发现的 panic 数 |
| 4 | **性能低效** | ~8× (I/O) | 生成"能用的代码"而非"高效代码"；对算法复杂度无感知 | profiling: runtime、memory；静态分析：lock 粒度、N+1 I/O 模式 |
| 5 | **一致性差** | 3× (可读性) | 无跨文件上下文，每个代码块独立生成 | lint 规则违反率；命名一致性；重复代码检测(duplication %) |
| 6 | **安全漏洞** | 2.74× | 训练数据中不安全模式的统计放大 | SAST 扫描 (CodeQL/Semgrep)；CWE 类型计数 |

### 根本原因分析

```
LLM 本质 = 统计 token 预测器
  ↓
训练数据分布偏差：
  - happy path (90%) >> error path (10%)
  - 常见 case (80%) >> edge case (20%)
  - "看起来对" >> "真正对" (无执行反馈)
  ↓
系统性失效模式：
  1. 代码"表面正确"但运行时失败 → 静默失败
  2. 缺少错误传播和上下文包装 → 错误处理缺失
  3. 只覆盖主路径忽略边界 → 边界条件遗漏
  4. 无算法复杂度意识 → 性能低效
  5. 每个代码块独立生成 → "10个互不沟通的开发者"效应
  6. 复制训练集中的不安全模式 → 安全漏洞放大
```

### 度量指标体系

| 度量维度 | 指标 | 工具 | 阈值参考 |
|---------|------|------|---------|
| 正确性 | 变异测试 kill rate | go-mutesting | >80% |
| 错误处理 | bare `return err` / total error returns | custom linter | <20% |
| 错误处理 | error wrapping rate (`%w`) | custom grep | >70% |
| 边界条件 | fuzzing panic 数 | go-fuzz | 0 |
| 性能 | lock-held duration P99 | pprof + mutex profile | <1ms |
| 性能 | N+1 I/O patterns | static analysis | 0 |
| 一致性 | duplicate code % | dupl, jscpd | <5% |
| 安全 | SAST findings count | CodeQL, gosec | 0 critical, 0 high |

## 二、本项目审计结果

### 审计范围

- `internal/` 下全部 Go 代码（680 非测试文件、1144 总文件）
- 5 个维度并行审计：错误处理、性能、一致性/架构、安全、边界条件

### 审计结论

**本代码库质量显著高于 LLM 平均水平**，未发现典型 LLM 生成代码的严重系统性缺陷。

具体发现：

#### 错误处理 — 良好
- Error wrapping 使用率高（1839 处 fmt.Errorf，大多数使用 `%w`）
- 少量 bare `return err` 存在于内部函数（设计选择，非遗漏）
- `_ = someFunc()` 集中在 cleanup/Close 路径，均有合理理由

#### 边界条件 — 良好
- type assertion 大多有 ok check 或 type switch default
- slice 访问前有长度检查
- 误报纠正：SplitN()[0] 安全（SplitN 始终返回≥1元素）、Models[0] 已有 len 守卫

#### 性能 — 少量可改进
- `timer/manager.go:174-183`：Add() 中线性扫描计数活跃 timer（O(n) under lock）
- `jobstore_file.go`：持 RLock 做文件 I/O（阻塞其他 goroutine）
- `tool_batch.go:76`：无缓冲 channel（实际影响微乎其微，标准 worker pool 模式）

#### 一致性 — 中等
- Lark 服务层 8 个 Service 有重复错误处理模式（可抽取泛型 helper）
- 部分大文件超过 800 行（background.go 1421、runtime.go 1234）
- 同步原语混用（sync.Map vs RWMutex vs Mutex，无统一标准）

#### 安全 — 良好
- 无硬编码凭据（除测试 fixture）
- SQL 使用参数化查询
- TLS 配置正确（无 InsecureSkipVerify）
- Shell 命令执行使用数组形式参数

### 可优化项（按影响排序）

| 优先级 | 文件 | 问题 | 影响 |
|-------|------|------|------|
| P2 | `timer/manager.go:174-183` | Add() 线性扫描活跃 timer 计数 | 持锁时间 O(n)，timer 多时影响并发 |
| P2 | `jobstore_file.go:103-140` | List() 持 RLock 做 N 次文件读取 | 阻塞并发 Save 操作 |
| P3 | Lark 8 个 Service | 重复的 API 错误处理模板 | 维护成本，非功能性问题 |
| P3 | 大文件 (>800 LOC) | 职责过重 | 可读性，非紧急 |

## 三、结论

LLM 最新最强模型写代码仍然存在的核心缺陷：
1. **统计偏差导致的遗漏** — 训练数据决定了模型"见过什么"
2. **无执行反馈导致的静默失败** — 模型无法验证自己的输出
3. **无全局上下文导致的一致性差** — 每次生成是独立的

本项目代码质量良好，上述缺陷不显著。主要原因：
- 有经验的人类 review 和测试把关
- 项目有 CLAUDE.md 等明确的编码规范约束 LLM 输出
- TDD 实践提供了执行反馈闭环

## 参考来源

- [IEEE Spectrum: AI Coding Degrades: Silent Failures Emerge](https://spectrum.ieee.org/ai-coding-degrades)
- [arXiv: Where Do LLMs Still Struggle? Analysis of Code Generation Benchmarks](https://arxiv.org/html/2511.04355v1)
- [arXiv: Quality Assurance of LLM-generated Code: Non-Functional Quality](https://arxiv.org/html/2511.10271v1)
- [CodeRabbit: AI vs Human Code Generation Report](https://www.coderabbit.ai/blog/state-of-ai-vs-human-code-generation-report)
- [Addy Osmani: My LLM Coding Workflow Going into 2026](https://addyosmani.com/blog/ai-coding-workflow/)
- [Sonar: LLMs for Code Generation - Research Quality Summary](https://www.sonarsource.com/resources/library/llm-code-generation/)
