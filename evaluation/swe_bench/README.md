# SWE-Bench Batch Processing

Alex 的 SWE-Bench 批处理模式实现了与 [SWE-Agent](https://swe-agent.com) 兼容的批处理功能，允许在 SWE-Bench 数据集上并行运行多个软件工程任务。

## 概述

SWE-Bench（Software Engineering Benchmark）是一个用于评估大型语言模型在软件工程任务上表现的基准测试。本实现提供了高性能的批处理能力，支持：

- **并行处理**：支持多 worker 并发执行，显著提升处理速度
- **数据集支持**：完整支持 SWE-Bench lite、full 和 verified 数据集
- **灵活配置**：YAML 配置文件和命令行参数双重配置方式
- **结果输出**：标准 SWE-Bench 格式（preds.json）和详细分析报告
- **监控和日志**：实时进度跟踪、性能监控和错误分析

## 快速开始

### 1. 基础使用

```bash
# 构建 Alex
make build

# 运行 SWE-Bench lite 测试（2个实例，1个 worker）
./alex run-batch --dataset.subset lite --dataset.split dev --instance-limit 2 --workers 1 --output ./test_results

# 查看结果
ls -la ./test_results/
```

### 2. 运行完整基准测试

```bash
# SWE-Bench lite（推荐用于初始测试）
./alex run-batch --dataset.subset lite --dataset.split dev --workers 3 --output ./lite_results

# SWE-Bench full（完整数据集）
./alex run-batch --dataset.subset full --dataset.split dev --workers 5 --output ./full_results
```

### 3. 使用配置文件

```bash
# 生成配置模板
make swe-bench-config

# 编辑配置文件
vim swe_bench_config.yaml

# 使用配置文件运行
./alex run-batch --config swe_bench_config.yaml
```

## 配置选项

### 模型配置

```yaml
agent:
  model:
    name: "deepseek/deepseek-chat-v3-0324:free"  # 模型名称
    temperature: 0.1                             # 温度（0.0-2.0）
    max_tokens: 4000                             # 最大 token 数
  max_turns: 20          # 最大对话轮数
  cost_limit: 10.0       # 成本限制（美元）
  timeout: 300           # 超时时间（秒）
```

### 数据集配置

```yaml
instances:
  type: "swe_bench"      # 数据集类型
  subset: "lite"         # 子集：lite、full、verified
  split: "dev"           # 分割：dev、test、train
  
  # 可选过滤选项
  instance_limit: 10              # 限制实例数量
  instance_slice: [0, 50]         # 处理实例范围
  instance_ids: ["id1", "id2"]    # 特定实例 ID
  shuffle: true                   # 随机排序
```

### 执行配置

```yaml
num_workers: 3                    # 并行 worker 数量
output_path: "./batch_results"    # 输出目录
enable_logging: true              # 启用详细日志
fail_fast: false                  # 首次失败时停止
max_retries: 2                    # 最大重试次数
max_delay: 5s                     # 任务间最大延迟
```

## 命令行选项

### 基本选项

```bash
# 模型配置
--model "gpt-4o"                    # 模型名称
--temperature 0.1                   # 模型温度
--max-tokens 4000                   # 最大 token 数

# 数据集配置
--dataset.type swe_bench            # 数据集类型
--dataset.subset lite               # 数据集子集
--dataset.split dev                 # 数据集分割

# 执行配置
--workers 3                         # Worker 数量
--output ./results                  # 输出目录
--timeout 300                       # 超时时间（秒）
```

### 高级选项

```bash
# 实例过滤
--instance-limit 50                 # 限制实例数量
--instance-slice "0,100"            # 实例范围
--instance-ids "id1,id2,id3"        # 特定实例 ID
--shuffle                           # 随机排序

# 执行控制
--fail-fast                         # 首次失败时停止
--max-retries 3                     # 最大重试次数
--resume ./previous_results         # 从之前的结果恢复

# 输出控制
--quiet                             # 静默输出
--verbose                           # 详细输出
--progress                          # 显示进度
```

## 输出格式

批处理完成后，输出目录包含以下文件：

```
batch_results/
├── preds.json              # SWE-Bench 标准格式预测结果
├── batch_results.json      # 完整批处理结果
├── summary.json            # 结果摘要
├── detailed_results.json   # 详细结果（包含 trace）
├── config.yaml             # 使用的配置
└── streaming_results.jsonl # 实时结果流
```

### preds.json 格式

```json
[
  {
    "instance_id": "django__django-12345",
    "solution": "修复代码的详细说明...",
    "explanation": "解决方案的解释...",
    "files_changed": ["path/to/file.py"],
    "commands": ["python manage.py test"],
    "status": "completed",
    "duration_seconds": 45.2,
    "tokens_used": 1250,
    "cost": 0.025
  }
]
```

### 摘要报告示例

```json
{
  "timestamp": "2025-07-11T10:30:00Z",
  "duration": "15m30s",
  "total_tasks": 100,
  "completed_tasks": 85,
  "failed_tasks": 15,
  "success_rate": 85.0,
  "total_tokens": 125000,
  "total_cost": 12.50,
  "avg_duration": "9.3s",
  "error_summary": {
    "timeout_error": 8,
    "processing_error": 5,
    "agent_creation_error": 2
  }
}
```

## 数据集支持

### SWE-Bench 数据集

| 数据集 | 子集 | 分割 | 实例数 | 描述 |
|--------|------|------|--------|------|
| swe_bench | lite | dev | 300 | 精选的高质量实例，推荐用于快速测试 |
| swe_bench | lite | test | 300 | 测试集 |
| swe_bench | full | dev | 2,294 | 完整开发集 |
| swe_bench | full | test | 2,294 | 完整测试集 |
| swe_bench | verified | dev | 500 | 经过验证的高质量实例 |

### 自定义数据集

```bash
# 使用本地文件
./alex run-batch --dataset.type file --dataset.file ./my_instances.json

# 文件格式（JSON 或 JSONL）
[
  {
    "instance_id": "custom_1",
    "repo": "https://github.com/user/repo",
    "base_commit": "abc123",
    "problem_statement": "Fix the bug in...",
    "hints_text": "Check the database connection..."
  }
]
```

## 性能优化

### Worker 配置

```bash
# 根据系统资源调整 worker 数量
./alex run-batch --workers 8    # 高性能机器
./alex run-batch --workers 2    # 资源受限环境
```

### 成本控制

```bash
# 设置每实例成本限制
./alex run-batch --cost-limit 1.0

# 限制实例数量进行测试
./alex run-batch --instance-limit 10
```

### 超时管理

```bash
# 调整超时时间（复杂任务需要更长时间）
./alex run-batch --timeout 600   # 10分钟超时
```

## 故障恢复

### 恢复中断的批处理

```bash
# 从之前的结果目录恢复
./alex run-batch --resume ./batch_results

# 系统会自动跳过已完成的实例
```

### 重试策略

```bash
# 配置重试次数
./alex run-batch --max-retries 3

# 启用快速失败（调试时有用）
./alex run-batch --fail-fast
```

## 监控和调试

### 实时监控

```bash
# 启用详细日志
./alex run-batch --logging --verbose

# 查看进度（默认启用）
./alex run-batch --progress
```

### 日志文件

```bash
# 日志位置
~/.alex/logs/batch_monitor_2025-07-11_10-30-00.log

# 查看实时日志
tail -f ~/.alex/logs/batch_monitor_*.log
```

### 性能分析

```bash
# 查看结果统计
cat ./batch_results/summary.json | jq '.'

# 分析错误类型
cat ./batch_results/batch_results.json | jq '.error_summary'
```

## Makefile 集成

项目提供了便捷的 Makefile 目标：

```bash
# 快速测试
make swe-bench-test

# 运行 lite 基准测试
make swe-bench-lite

# 运行完整基准测试
make swe-bench-full

# 生成配置模板
make swe-bench-config

# 清理结果
make swe-bench-clean
```

## 最佳实践

### 1. 开发和测试

```bash
# 首先使用少量实例测试
./alex run-batch --dataset.subset lite --instance-limit 5 --workers 1

# 验证配置和输出格式
./alex run-batch --config test_config.yaml --instance-limit 1
```

### 2. 生产运行

```bash
# 使用合适的 worker 数量（通常为 CPU 核心数的 0.5-1 倍）
./alex run-batch --workers 4

# 启用日志和监控
./alex run-batch --logging --progress

# 设置合理的超时和重试
./alex run-batch --timeout 600 --max-retries 2
```

### 3. 成本优化

```bash
# 使用免费或低成本模型进行初始测试
./alex run-batch --model "deepseek/deepseek-chat-v3-0324:free"

# 设置成本限制
./alex run-batch --cost-limit 5.0

# 分批处理大数据集
./alex run-batch --instance-slice "0,100"
./alex run-batch --instance-slice "100,200"
```

## 故障排除

### 常见问题

1. **模型 API 错误**
   ```bash
   # 检查 API 密钥配置
   alex config show
   
   # 验证模型名称
   ./alex run-batch --model "openai/gpt-4o" --instance-limit 1
   ```

2. **内存不足**
   ```bash
   # 减少 worker 数量
   ./alex run-batch --workers 1
   
   # 减少最大 token 数
   ./alex run-batch --max-tokens 2000
   ```

3. **网络超时**
   ```bash
   # 增加超时时间
   ./alex run-batch --timeout 900
   
   # 启用重试
   ./alex run-batch --max-retries 3
   ```

### 调试模式

```bash
# 详细输出
./alex run-batch --verbose

# 单实例调试
./alex run-batch --instance-limit 1 --workers 1 --verbose

# 查看配置
./alex run-batch --config config.yaml --verbose | head -20
```

## 与 SWE-Agent 兼容性

本实现与 SWE-Agent 的批处理模式兼容，支持：

- 相同的配置参数格式
- 标准的 `preds.json` 输出格式
- 兼容的命令行选项
- 相似的性能指标

### 迁移自 SWE-Agent

```bash
# SWE-Agent 命令
sweagent run-batch --config config.yaml --instances.subset lite

# 等效的 Alex 命令
alex run-batch --config config.yaml --dataset.subset lite
```

## 开发和贡献

### 架构概览

```
benchmarks/swe_bench/
├── types.go          # 核心数据结构
├── interfaces.go     # 接口定义
├── config.go         # 配置管理
├── batch.go          # 批处理核心逻辑
├── dataset.go        # 数据集加载
├── worker.go         # 并行处理引擎
├── results.go        # 结果输出处理
├── monitoring.go     # 监控和日志
├── agent.go          # Agent 集成
└── batch_test.go     # 测试用例
```

### 运行测试

```bash
# 运行所有测试
go test ./benchmarks/swe_bench/

# 运行性能测试
go test -bench=. ./benchmarks/swe_bench/

# 运行特定测试
go test -run TestDatasetLoader ./benchmarks/swe_bench/
```

### 扩展功能

1. **添加新数据集类型**
   - 实现 `DatasetLoader` 接口
   - 在 `dataset.go` 中添加加载逻辑

2. **自定义输出格式**
   - 扩展 `ResultWriter` 接口
   - 在 `results.go` 中添加格式化方法

3. **增强监控**
   - 扩展 `Monitor` 接口
   - 添加自定义指标和事件

## 许可证

本项目遵循与 Alex 主项目相同的许可证。