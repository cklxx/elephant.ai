# Agent 输入输出及日志分析

## 📥 Agent 输入 (Input)

### 1. **Instance 数据结构**
从 `real_instances.json` 输入的每个测试案例包含：
```json
{
  "instance_id": "astropy__astropy-12907",  // 唯一标识
  "repo": "astropy/astropy",                 // 仓库信息
  "base_commit": "d16bfe05a744909de4b27f5875fe0d4ed41ce607",
  "problem_statement": "详细的问题描述...",  // 核心输入
  "hints_text": "提示信息",                  // 可选提示
  "patch": "diff格式的预期修复",            // 参考答案
  "test_patch": "测试补丁",
  "environment": {
    "python": "3.9"
  },
  "metadata": {
    "difficulty": "medium"
  }
}
```

### 2. **配置输入**
从 `config.yaml` 或 `ultra_think_config.yaml` 加载：
```yaml
agent:
  model:
    name: "deepseek/deepseek-r1"    # 模型选择
    temperature: 0.1                 # 生成参数
    max_tokens: 16000
  max_turns: 50                      # 最大对话轮次
  timeout: 900                       # 超时设置
```

## 📤 Agent 输出 (Output)

### 1. **流式输出** (`streaming_results.jsonl`)
每行一个JSON对象，实时记录处理结果：
```json
{
  "task_id": "task_0",
  "instance_id": "astropy__astropy-12907",
  "status": "completed",
  "solution": "# Solution for astropy__astropy-12907\n\n## Problem Analysis\n...",
  "explanation": "解决方案说明",
  "files_changed": ["models.py"],
  "commands": ["python -m unittest discover"],
  "duration": 227501000,
  "tokens_used": 516,
  "cost": 0.000258,
  "trace": [...]  // 思考轨迹
}
```

### 2. **批处理结果** (`batch_results.json`)
完整的批处理执行记录：
```json
{
  "config": {...},           // 使用的配置
  "start_time": "2025-09-13T00:00:04+08:00",
  "end_time": "2025-09-13T00:00:10+08:00",
  "duration": 6146227125,    // 纳秒
  "total_tasks": 3,
  "completed_tasks": 3,
  "failed_tasks": 0,
  "success_rate": 100,
  "results": [...]           // 所有任务结果
}
```

### 3. **详细结果** (`detailed_results.json`)
包含思考轨迹的详细结果：
```json
{
  "trace": [
    {
      "step": 1,
      "action": "analyze_repository",
      "observation": "Analyzed repository structure",
      "thought": "Understanding the codebase structure",
      "timestamp": "2025-09-13T00:00:04+08:00"
    },
    {
      "step": 2,
      "action": "read_problem_statement",
      "observation": "Read and analyzed the problem",
      "thought": "Understanding the specific issue",
      "timestamp": "2025-09-13T00:00:04+08:00"
    },
    {
      "step": 3,
      "action": "identify_root_cause",
      "observation": "Identified potential root cause",
      "thought": "Located the specific code section",
      "timestamp": "2025-09-13T00:00:04+08:00"
    },
    {
      "step": 4,
      "action": "implement_solution",
      "observation": "Implemented the necessary changes",
      "thought": "Applied the fix while ensuring compatibility",
      "timestamp": "2025-09-13T00:00:04+08:00"
    }
  ]
}
```

### 4. **汇总信息** (`summary.json`)
高层次的统计信息：
```json
{
  "timestamp": "2025-09-13T14:47:42+08:00",
  "duration": "4.827541834s",
  "total_tasks": 3,
  "completed_tasks": 3,
  "failed_tasks": 0,
  "success_rate": 100,
  "total_tokens": 1559,
  "total_cost": 0.0008,
  "avg_duration": "219.600263ms",
  "model_name": "deepseek/deepseek-r1"
}
```

## 📊 日志机制 (Logging)

### 1. **控制台输出**
```
2025/09/13 00:00:04 Worker pool started with 1 workers
2025/09/13 00:00:04 Worker 0 started
2025/09/13 00:00:04 Worker 0 processing task task_0 (instance astropy__astropy-12907)
2025/09/13 00:00:04 Worker 0 completed task task_0 in 227.501ms (status: completed)
```

### 2. **监控系统** (`monitoring.go`)
- **ProgressReporter**: 实时进度报告
- **StatusTracker**: 状态跟踪
- **MetricsCollector**: 性能指标收集

### 3. **日志级别**
- **INFO**: 正常操作日志
- **DEBUG**: 详细调试信息（需要启用）
- **ERROR**: 错误和异常
- **TRACE**: 思考轨迹记录

## 🔄 数据流程

```
输入流程:
real_instances.json → BatchProcessor → Worker → Agent
     ↓                      ↓            ↓        ↓
config.yaml          Task Queue    Context   LLM Call

处理流程:
Agent → Think → Act → Observe → Solution
  ↓       ↓      ↓       ↓          ↓
Trace  Step1  Step2   Step3    Final Result

输出流程:
Solution → WorkerResult → BatchResult → Files
    ↓           ↓             ↓          ↓
stream.jsonl  detail.json  batch.json  summary.json
```

## 🎯 关键特性

1. **Ultra Think 模式**
   - 启用深度推理（deepseek-r1模型）
   - 记录完整思考轨迹
   - 多步骤问题分解

2. **并发处理**
   - Worker池管理
   - 任务队列调度
   - 超时控制

3. **成本追踪**
   - Token计数
   - API成本计算
   - 资源使用监控

4. **错误处理**
   - 重试机制
   - 超时保护
   - 失败回滚

## 📝 示例日志分析

成功案例的典型日志模式：
1. **初始化**: Worker启动，加载配置
2. **任务开始**: 接收instance，开始处理
3. **思考步骤**: 4个标准步骤（分析→理解→定位→实施）
4. **完成**: 生成解决方案，记录metrics
5. **清理**: 保存结果，释放资源

失败案例会包含：
- Error类型和消息
- 重试次数
- 失败时的状态快照