# ALEX Agent Evaluation Framework

一个简化、实用的Agent评估框架，基于现有SWE-Bench系统构建，遵循ALEX项目的简洁性原则。

## 架构概述

本框架采用简化的3层架构设计：

```
┌─────────────────────────────────────────┐
│  Evaluation Manager                     │ ← 任务调度和管理
├─────────────────────────────────────────┤
│  Metrics & Analysis                     │ ← 指标收集和智能分析
├─────────────────────────────────────────┤
│  Enhanced Execution                     │ ← 基于SWE-Bench的执行层
└─────────────────────────────────────────┘
```

## 核心特性

### 🎯 **简化设计**
- 3层架构替代复杂的6层设计
- 规则引擎替代复杂ML组件
- 文件存储替代复杂数据库
- 保持与ALEX项目的简洁性原则一致

### 📊 **全面指标收集**
- **性能指标**: 成功率、执行时间、超时率、重试率
- **质量指标**: 解决方案质量、错误恢复率、一致性评分
- **资源指标**: Token使用、成本分析、内存占用
- **行为指标**: 工具使用模式、错误模式分析
- **注意力指标**: HAM（人类注意力分钟）基线/实际、Attention Saving Ratio、打断频率、审查时长、恢复成本、严重失败率、交付就绪率、信任校准误差

### 🧠 **智能分析**
- 基于规则的建议引擎（12个内置规则）
- 自动洞察生成
- 趋势分析和预测
- 异常检测和警报

### 📝 **详细报告**
- Markdown格式的详细报告
- 执行摘要和关键洞察
- 优先级分类的建议
- 对比分析支持

## 内置通用Agent评测集

默认数据集位于 `evaluation/agent_eval/datasets/general_agent_eval.json`（已内置到二进制，可选指定路径），`DatasetType` 使用 `general_agent`。该集合聚焦通用能力（主要面向 Web Agent 能力），剔除了低价值项，仅保留高信号任务，覆盖：

- 规划与项目管理：跨团队计划、歧义拆解、排期对齐
- 分析与可观察性：日志三角定位、指标归因、故障假设
- 架构与API：最小API设计、治理与工具使用边界
- 安全与运营：威胁建模、运维窗口排程、弹性与测试策略
- 产品与沟通：歧义拆解、优先级梳理

每条任务默认 `surface=web`，可通过 `Metadata["surface"]` 过滤区分；SWE-Bench 仍适用于 CLI/修复类评测。

## Foundation 分层离线评测（当前推荐）

Foundation suite 使用离线 lexical+metadata 路由评估，不依赖模型调用，专注验证：
- 工具可发现性/可用性
- 提示词有效性
- 主动性路由
- 动机感知主动性路由（motivation-aware proactivity）
- 复杂高价值任务首动作选择
- 可用性冲突与降级恢复
- 价值交付工作流
- SWE-bench Verified 风格修复任务路由准备度
- 多步骤编排能力链路（multi-step orchestration）
- 安全边界与策略门控（safety boundary policy）
- context learning hard 模式（latest benchmark inspired）
- 记忆能力专项（memory capabilities）
- 用户习惯 + Soul + 记忆连续性专项
- 任务完成速度专项（task completion speed）
- 冲突收敛高难专项（conflict convergence hard）
- 高难挑战专项（challenge hard v2）
- 复杂任务文件产物交付专项（complex artifact delivery）
- 意图裂解约束矩阵专项（intent decomposition constraint matrix）
- 稀疏线索检索高压专项（sparse-clue retrieval stress）
- 多轮状态承诺边界专项（stateful commitment boundary stress）
- 可复现证据链专项（reproducibility trace evidence stress）
- 多轮 pass@1 易题淘汰与难题替换（easy-case retirement + harder replacements）

新增（2026-02-10）：基础可用工具评测套件（active tools + skills）
- `evaluation/agent_eval/datasets/foundation_eval_suite_basic_active.yaml`
- 仅覆盖当前可用工具与 skills，作为基础回归入口。
- 当前结果（`tmp/foundation-suite-r21-basic-active-20260210-115920`）：
  - Collections: `3/3`
  - Cases: `31/31`
  - N/A: `0`
  - pass@1: `27/31`
  - pass@5: `31/31`

新增（2026-02-10）：系统化端到端 suite（按能力维度分层）
- `evaluation/agent_eval/datasets/foundation_eval_suite_basic_active.yaml`
- 覆盖 `28` 个集合、`344` 个 case（Foundation Core + Stateful/Memory + Delivery + Frontier Benchmark Transfer）。
- 新增 benchmark 映射集合：
  - `industry_benchmark_webarena_verified_webops_hard`
  - `industry_benchmark_agentbench_multidomain_tooluse_hard`
  - `industry_benchmark_browsecomp_sparse_research_hard`
  - `industry_benchmark_agentlongbench_long_context_memory_hard`

最新一次本地端到端评测（`tmp/foundation-suite-e2e-systematic-20260211-230148`）：
- Collections: `28`
- Collections passed (0 failed cases): `27/28`
- Cases: `344`
- Applicable Cases: `202`
- N/A Cases: `142`
- pass@1: `179/202`（`85.5%`）
- pass@5: `200/202`（`96.0%`）
- Failed Cases: `3`
- Deliverable Cases: `20/344`
- Deliverable Good: `0/20`
- Deliverable Bad: `20/20`

说明：评测分数会随工具可用性、数据集演进和路由策略变化而变化，不应将历史 100% 结果作为稳定基线。

新增（2026-02-24）：注意力节省专项 suite
- `evaluation/agent_eval/datasets/attention_eval_suite.yaml`
- 包含三类专项用例：
  - `attention_eval_cases_interruption_control.yaml`
  - `attention_eval_cases_recovery_cost.yaml`
  - `attention_eval_cases_trust_calibration.yaml`
- 支持在 suite collection 上设置 `tags` 与 `attention_weight`，用于注意力导向的加权汇总。

运行命令：

```bash
go run ./cmd/alex eval foundation-suite \
  --suite evaluation/agent_eval/datasets/foundation_eval_suite_basic_active.yaml \
  --output tmp/foundation-suite-speed-v1 \
  --format markdown

go run ./cmd/alex eval foundation-suite \
  --suite evaluation/agent_eval/datasets/foundation_eval_suite_basic_active.yaml \
  --output tmp/foundation-suite-e2e-systematic \
  --format markdown

go run ./cmd/alex eval foundation-suite \
  --suite evaluation/agent_eval/datasets/foundation_eval_suite_basic_active.yaml \
  --output tmp/foundation-suite-basic-active \
  --format markdown
```

## 快速开始

### 1. 基本使用

```go
package main

import (
    "context"
    "log"
    
    "alex/evaluation/agent_eval"
)

func main() {
    // 运行快速评估
    if err := agent_eval.RunQuickEvaluation(); err != nil {
        log.Fatalf("Evaluation failed: %v", err)
    }
}
```

### 2. 自定义评估

```go
package main

import (
    "context"
    "time"
    
    "alex/evaluation/agent_eval"
)

func main() {
    // 创建CLI管理器
    cliManager, err := agent_eval.NewCLIManager("./results")
    if err != nil {
        log.Fatalf("Failed to create CLI manager: %v", err)
    }
    
    // 配置评估选项
    options := &agent_eval.EvaluationOptions{
        DatasetPath:    "", // 留空使用内置general_agent数据集
        InstanceLimit:  50,
        MaxWorkers:     4,
        TimeoutPerTask: 300 * time.Second,
        EnableMetrics:  true,
        OutputDir:      "./evaluation_results",
        ReportFormat:   "markdown",
    }
    
    // 运行评估
    ctx := context.Background()
    job, err := cliManager.RunEvaluation(ctx, options)
    if err != nil {
        log.Fatalf("Evaluation failed: %v", err)
    }
    
    log.Printf("Evaluation completed: %s", job.ID)
}
```

### 3. 配置比较

```go
func compareConfigurations() {
    cliManager, _ := agent_eval.NewCLIManager("./results")
    
    // 基准配置
    baselineConfig := &agent_eval.EvaluationConfig{
        DatasetPath:    "", // 留空使用内置general_agent数据集
        InstanceLimit:  20,
        MaxWorkers:     2,
        TimeoutPerTask: 300 * time.Second,
        EnableMetrics:  true,
        OutputDir:      "./baseline_results",
    }
    
    // 实验配置
    experimentConfig := &agent_eval.EvaluationConfig{
        DatasetPath:    "", // 留空使用内置general_agent数据集
        InstanceLimit:  20,
        MaxWorkers:     4, // 增加worker数量
        TimeoutPerTask: 600 * time.Second, // 增加超时时间
        EnableMetrics:  true,
        OutputDir:      "./experiment_results",
    }
    
    // 运行比较
    ctx := context.Background()
    comparison, err := cliManager.CompareConfigurations(ctx, baselineConfig, experimentConfig)
    if err != nil {
        log.Fatalf("Comparison failed: %v", err)
    }
    
    // 显示结果
    fmt.Printf("Success Rate Delta: %.2f%%\n", comparison.ComparisonMetrics.SuccessRateDelta*100)
    fmt.Printf("Performance Delta: %.2f%%\n", comparison.ComparisonMetrics.PerformanceDelta*100)
    fmt.Printf("Cost Delta: %.2f%%\n", comparison.ComparisonMetrics.CostDelta*100)
}
```

## 配置选项

### EvaluationConfig
```go
type EvaluationConfig struct {
    // 数据集配置
    DatasetType   string        // 数据集类型 (默认: "general_agent")
    DatasetPath   string        // 数据集路径
    InstanceLimit int           // 实例限制 (默认: 10)
    
    // 执行配置
    MaxWorkers    int           // 最大worker数 (默认: 2)
    TimeoutPerTask time.Duration // 任务超时 (默认: 5分钟)
    
    // 指标配置
    EnableMetrics bool          // 启用指标收集 (默认: true)
    MetricsTypes  []string      // 指标类型
    
    // 输出配置
    OutputDir     string        // 输出目录
    ReportFormat  string        // 报告格式 (默认: "markdown")
}
```

## 指标说明

### 性能指标
- **成功率**: 成功完成的任务百分比
- **平均执行时间**: 任务平均执行时间
- **超时率**: 超时任务的百分比
- **重试率**: 需要重试的任务百分比

### 质量指标
- **解决方案质量**: 基于解决方案特征的质量评分
- **错误恢复率**: 从错误中恢复的能力
- **一致性评分**: 类似任务间的一致性表现
- **复杂性处理**: 处理复杂任务的能力

### 资源指标
- **Token使用**: 总Token使用量和平均使用量
- **成本分析**: 总成本和每任务平均成本
- **内存使用**: 系统内存占用

### 行为指标
- **工具调用**: 平均工具调用次数
- **工具使用模式**: 各工具使用频率分析
- **常见失败**: 失败模式统计
- **错误模式**: 错误类型分析

## 规则引擎

内置12个评估规则，涵盖：

### 性能规则 (PERF_*)
- **PERF_001**: 低成功率检测
- **PERF_002**: 高超时率检测  
- **PERF_003**: 慢执行时间检测

### 质量规则 (QUAL_*)
- **QUAL_001**: 低解决方案质量检测
- **QUAL_002**: 差错误恢复检测
- **QUAL_003**: 不一致性能检测

### 效率规则 (EFF_*)
- **EFF_001**: 过度工具使用检测
- **EFF_002**: 高Token消耗检测

### 成本规则 (COST_*)
- **COST_001**: 高评估成本检测
- **COST_002**: 高单任务成本检测

### 可靠性规则 (REL_*)
- **REL_001**: 高重试率检测
- **REL_002**: 内存使用警告

## 报告格式

生成的Markdown报告包含：

1. **执行摘要**: 总体评分、等级、优势劣势
2. **性能分析**: 详细的性能指标表格和洞察
3. **质量分析**: 质量指标评估和建议
4. **资源使用**: 成本和资源效率分析
5. **行为分析**: 工具使用模式和错误分析
6. **关键洞察**: 自动生成的洞察和发现
7. **建议**: 按优先级分类的改进建议
8. **警报**: 关键问题和警告信息

## 集成现有系统

本框架基于现有的SWE-Bench评估系统构建，完全兼容：

```go
// 现有的SWE-Bench组件
import "alex/evaluation/swe_bench"

// 新的Agent评估框架
import "alex/evaluation/agent_eval"

// 无缝集成使用
manager := agent_eval.NewEvaluationManager(config)
```

## 性能特性

- **轻量级**: 内存使用 <50MB per session
- **高效**: 处理能力 100+ evaluations/hour
- **可扩展**: 支持并发评估和批量处理
- **稳定**: 非侵入式集成，零风险部署

## 目录结构

```
evaluation/agent_eval/
├── evaluation_manager.go    # 第1层：评估管理器
├── metrics.go              # 第2层：指标收集
├── analyzer.go             # 第2层：分析引擎
├── rules.go                # 第2层：规则引擎
├── reporter.go             # 第2层：报告生成
├── cli.go                  # CLI接口
├── types.go                # 类型定义
├── example_test.go         # 示例测试
└── README.md               # 本文件
```

## 测试

运行测试：
```bash
cd evaluation/agent_eval
go test -v
```

## 故障排除

### 常见问题

1. **数据集文件不存在**
   ```
   Error: dataset file does not exist: ./evaluation/agent_eval/datasets/general_agent_eval.json
   ```
   若需要自定义文件路径，确保评测数据集文件存在且路径正确；默认留空将自动使用内置的general_agent数据。

2. **内存不足**
   ```
   Warning: High memory usage detected
   ```
   减少`InstanceLimit`或`MaxWorkers`参数。

3. **超时过多**
   ```
   Alert: High timeout rate detected
   ```
   增加`TimeoutPerTask`时间或检查任务复杂性。

### 调试模式

启用详细日志：
```go
options := agent_eval.DefaultEvaluationOptions()
options.Verbose = true
```

## 最佳实践

1. **从小规模开始**: 使用少量实例测试配置
2. **监控资源使用**: 关注内存和成本指标
3. **定期比较**: 使用A/B测试验证改进
4. **保存基准**: 建立性能基准线用于对比
5. **关注警报**: 及时处理系统生成的警报

## 技术规格

- **Go版本**: 1.19+
- **架构**: 3层简化设计
- **存储**: 文件系统JSON格式
- **报告**: Markdown格式
- **集成**: 基于现有SWE-Bench系统
- **依赖**: 最小化外部依赖

## 许可证

本项目遵循与ALEX项目相同的许可证。

## 贡献

欢迎贡献代码和建议。请遵循项目的代码规范和简洁性原则。
