# Alex 项目未使用代码删除建议清单

基于 AST 静态分析和依赖关系追踪，以下是可以安全删除的文件和模块清单。

## 🗑️ 可以安全删除的文件（第一优先级）

### 1. 测试文件（42个文件）
所有 `*_test.go` 文件都不影响主干逻辑：
```
cmd/cli_test.go
evaluation/swe_bench/batch_test.go
examples/mcp_demo.go
internal/agent/react_agent_test.go
internal/agent/tool_pairing_test.go
internal/config/manager_test.go
internal/context/message/batch_performance_test.go
internal/context/message/batch_processor_test.go
internal/context/message/compression_test.go
internal/context/message/token_estimator_test.go
internal/llm/factory_test.go
internal/llm/http_client_test.go
internal/llm/session_cache_test.go
internal/llm/streaming_client_test.go
internal/performance/benchmark_test.go
internal/performance/simple_verification_test.go
internal/prompts/loader_test.go
internal/session/async_session_test.go
internal/session/performance_demo_test.go
internal/session/realistic_performance_test.go
internal/session/session_test.go
internal/tools/builtin/file_operations_test.go
internal/tools/builtin/path_resolver_test.go
internal/tools/builtin/web_fetch_tool_test.go
internal/tools/builtin/web_search_tools_test.go
internal/tools/mcp/config_test.go
internal/tools/mcp/protocol/jsonrpc_test.go
internal/tools/mcp/spawner_test.go
internal/utils/logger_test.go
internal/utils/session_helper_test.go
internal/utils/stream_helper_test.go
internal/utils/tool_executor_test.go
internal/utils/version_test.go
pkg/types/message/basic_test.go
```

### 2. verification 模块（整个目录 - 8个文件）
`internal/verification/` 目录完全未被使用：
```
internal/verification/ab_testing.go
internal/verification/decision_engine.go
internal/verification/framework.go
internal/verification/observability.go
internal/verification/performance.go
internal/verification/safety.go
internal/verification/testing.go
internal/verification/validation_phases.go
```

## ⚠️ 需要谨慎评估的文件（第二优先级）

### 1. Async Session 模块（4个文件）
这些文件似乎没有被主干逻辑使用，只在自己的测试中引用：
```
internal/session/async_workers.go
internal/session/async_integration.go  
internal/session/async_session.go
internal/session/enhanced_session.go
internal/session/performance_demo.go
```

### 2. Batch Processing 模块（8个文件）
仅在 `evaluation/swe_bench/monitoring.go` 中有引用，如果不使用 SWE-Bench 评估，可以删除：
```
internal/context/message/batch_processor.go
internal/context/message/batch_workers.go
internal/context/message/batch_integration.go
internal/context/message/batch_optimizations.go
internal/context/message/batch_processing.go
internal/context/message/batch_error_handling.go
```

### 3. Performance 模块
仅被 `cmd/perf/main.go` 使用，如果不需要性能工具，可以考虑删除：
```
internal/performance/abtest.go
internal/performance/benchmark.go
internal/performance/integration.go
internal/performance/monitoring.go
internal/performance/scenarios.go
internal/performance/verification.go
```

## 🔍 具体分析结果

### 主干入口点分析：
1. **主要入口**: `cmd/main.go` → `runCobraCLI()` → `cmd/cobra_cli.go`
2. **性能工具入口**: `cmd/perf/main.go` （独立的性能验证工具）

### 依赖关系追踪结果：
1. **Cobra 命令模块**: 所有在 `cobra_cli.go` 中注册的命令都是被使用的
2. **Agent 核心**: `internal/agent/` 目录下所有非测试文件都被使用
3. **pkg/types**: 被多个核心模块引用，不能删除

### 未被引用的模块：
1. **async_session**: 只在自己的测试和演示中使用
2. **batch_processing**: 只在 SWE-Bench 评估中使用
3. **verification**: 完全未被引用

## 📋 删除建议执行计划

### 第一阶段：安全删除
```bash
# 删除所有测试文件
find . -name "*_test.go" -delete

# 删除 verification 整个目录
rm -rf internal/verification/

# 删除分析工具
rm ast_analyzer.go
```

### 第二阶段：条件删除
根据项目需求判断：

1. **如果不需要异步会话功能**：
   ```bash
   rm internal/session/async_*.go
   rm internal/session/enhanced_session.go
   rm internal/session/performance_demo.go
   ```

2. **如果不需要批处理功能**：
   ```bash
   rm internal/context/message/batch_*.go
   ```

3. **如果不需要性能工具**：
   ```bash
   rm -rf cmd/perf/
   rm -rf internal/performance/
   rm -rf performance/
   rm scripts/performance-verification.sh
   ```

4. **如果不需要 SWE-Bench 评估**：
   ```bash
   rm -rf evaluation/
   ```

## 💡 额外优化建议

1. **清理未使用的导入**: 运行 `goimports` 清理未使用的包导入
2. **检查未使用的函数**: 使用 `deadcode` 工具检查未使用的导出函数
3. **类型清理**: 使用 `structcheck` 检查未使用的结构体字段

## ⚖️ 风险评估

### 安全删除（无风险）：
- 所有测试文件
- verification 目录
- ast_analyzer.go

### 中等风险：
- async session 模块（可能是为了未来功能预留）
- batch processing 模块（SWE-Bench 需要）
- performance 模块（性能监控需要）

### 建议保留（高风险）：
- pkg/types 目录（核心类型定义）
- 所有 cobra 命令模块（CLI 功能需要）

通过删除上述文件，预计可以减少约 **50-60个文件**，显著简化项目结构。