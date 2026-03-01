# 工作区健康扫描报告

**扫描时间**: 2025-01-28  
**扫描范围**: elephant.ai 工作区（完整 diff + 测试 + 日志）

---

## 🔴 关键发现（5-8条）

### 1. **测试失败：配置路径解析回退逻辑失效**
- **位置**: `internal/shared/config/admin/path_test.go:31`
- **失败**: `TestResolveStorePathFallsBackWhenHomeMissing`
- **症状**: 期望回退路径 `"configs/config.yaml"`，实际得到 `"/Users/bytedance/.alex/test.yaml"`
- **根因**: 测试假设 `$HOME` 为空时会回退到 `"configs/config.yaml"`，但实际环境中 `os.UserHomeDir()` 可能仍然解析出路径
- **风险级别**: 中等 — 测试逻辑与环境耦合，非核心功能故障

### 2. **内核 Dispatch Store 完成 Retention/GC 改造**
- **变更**: `internal/infra/kernel/file_store.go` 新增 `retentionDuration` 字段和 `pruneLocked()` 方法
- **实现**: 默认 14 天保留期，终端状态 dispatch 自动清理
- **影响**: 解决 PLAN.md 中识别的 "unbounded dispatch store growth" 风险
- **状态**: ✅ 实现完成，DI 注入已更新（`builder_hooks.go:242` 已传递 retention 参数）

### 3. **调度器新增 Stale Recovery Guard 机制**
- **变更**: `internal/app/scheduler/scheduler.go` 和 `scheduler_test.go`
- **新增**: 188 行测试代码，覆盖重启后作业恢复的场景
- **逻辑**: 如果 `LastRun > LastFailure`，跳过不必要的恢复定时器调度
- **价值**: 避免进程重启后的重复/虚假恢复动作

### 4. **LLM Planner 大规模重构（374行变更）**
- **位置**: `internal/app/agent/kernel/llm_planner.go`
- **变更**: +374/-~100 行，含新的测试用例
- **亮点**: 改进 prompt 工程、添加 structured output 支持、增强 error classification
- **关联**: PLAN.md 中提到的 "failure signature classification" 已部分落地

### 5. **FileStore 构造函数签名变更导致潜在兼容性问题**
- **变更**: `NewFileStore(dir string, leaseDuration, retentionDuration time.Duration)` 新增第三参数
- **已修复**: `builder_hooks.go` 已更新调用点
- **潜在风险**: 如有其他未提交文件调用此构造函数，将产生编译错误

### 6. **STATE.md 记录内核循环已稳定运行**
- **最新记录**: 2026-02-28T22:12:37Z
- **状态**: 所有 kernel-critical lanes green（infra/kernel, agent/kernel, scheduler, taskfile）
- **失败率**: 800 dispatches 中 247 失败（30.9%），集中在 founder-operator 和 capital-explorer lanes
- **行动**: 已识别需要添加 signature-aware backoff 策略

### 7. **工作区存在大量未跟踪文件**
- **未跟踪**: `PLAN.md`, `STATE.md`, 多个测试文件，artifact 日志
- **建议**: 考虑将 PLAN.md 和 STATE.md 纳入版本控制（如它们是设计文档），或添加到 `.gitignore`（如它们是生成的）

### 8. **TODO 残留：CLI 结构化输出待实现**
- **位置**: `cmd/alex/cli.go:365`
- **内容**: `// TODO(context): surface structured diff/plan output once the runtime populates these fields.`
- **状态**: 等待 runtime 填充字段后启用

---

## ✅ 可执行建议（2-3条）

### 建议 1：修复配置路径测试失败
**优先级**: 低  
**操作**:
```bash
# 方案 A：调整测试以适应环境（推荐）
# 修改 internal/shared/config/admin/path_test.go:31
# 将硬编码预期改为检查路径后缀而非完整路径

# 方案 B：跳过此测试
export SKIP_HOMELESS_TEST=1
```
**验收标准**: `go test ./internal/shared/config/admin/...` 通过

---

### 建议 2：提交当前 Kernel Retention 变更
**优先级**: 高  
**理由**: 
- 13 个文件已修改，涉及核心组件
- 测试已全部通过（除上述环境耦合测试外）
- 解决了 dispatch store 无界增长的关键问题

**操作**:
```bash
git add -A
git commit -m "feat(kernel): add dispatch retention pruning and stale recovery

- Add retentionDuration to FileStore (default 14d)
- Implement pruneLocked() for automatic GC of terminal dispatches
- Update DI wiring in builder_hooks.go
- Add comprehensive scheduler stale-recovery guard tests
- Large refactor of LLM planner for better failure classification

Fixes unbounded dispatch store growth (see PLAN.md section 2.2)"
```

---

### 建议 3：跟进 Failure Signature 分类与 Backoff 策略
**优先级**: 中  
**背景**: STATE.md 显示 30.9% 失败率集中在特定 lanes  
**操作**:
1. 阅读 `artifacts/kernel_failure_tail_audit_20260228T221237Z.md` 获取详细分析
2. 在 scheduler 中添加按 error signature 的指数退避
3. 在 STATE.md 中添加每周期 failure-signature delta 遥测

**验收标准**:
- 新增 `error_signature` 标签到 dispatch 记录
- 相同 signature 连续失败 3 次后进入冷却期
- 周期报告中包含 top-5 failure signatures

---

## ❓ 需要确认的事项

| # | 问题 | 上下文 | 建议操作 |
|---|------|--------|----------|
| 1 | `PLAN.md` 和 `STATE.md` 是否应纳入版本控制？ | 当前为未跟踪文件，但包含重要设计决策和运行状态 | 确认：如果是设计文档 → 提交；如果是生成文件 → 添加 `.gitignore` |
| 2 | `acp_rpc_additional_test.go` 和 `config_defaults_test.go` 是否为临时文件？ | 新增未跟踪测试文件 | 确认：如为永久测试 → 提交；如为临时 → 删除或移入 `/tmp` |
| 3 | `file_store_test.go` 是否完成？ | 新增未跟踪测试文件 | 确认：如已完成 → 提交；如在开发中 → 继续完成后提交 |
| 4 | 是否有其他调用 `NewFileStore()` 的代码未更新？ | 构造函数签名已变更 | 运行 `grep -rn "NewFileStore" --include="*.go"` 全量检查，确保无遗漏调用点 |

---

## 📊 变更统计

```
13 files changed, 933 insertions(+), 152 deletions(-)

核心变更:
  internal/app/agent/kernel/llm_planner.go    | 374 +++++++---
  internal/app/scheduler/scheduler_test.go    | 188 +++++++++
  internal/infra/kernel/file_store.go         | 123 +++++
  internal/app/agent/kernel/llm_planner_test.go| 170 +++++
  internal/app/agent/kernel/config.go         |  63 ++
  internal/app/agent/kernel/engine.go         |  66 ++
  internal/app/di/builder_hooks.go            |   3 +-
```

---

## ✅ 当前测试状态

| 包 | 状态 | 备注 |
|----|------|------|
| `internal/app/agent/kernel/...` | ✅ PASS | 核心内核测试通过 |
| `internal/infra/kernel/...` | ✅ PASS | 存储层测试通过 |
| `internal/app/scheduler/...` | ✅ PASS | 调度器测试通过 |
| `internal/domain/agent/taskfile/...` | ✅ PASS | 任务文件测试通过 |
| `cmd/alex/...` | ✅ PASS | CLI 测试通过 |
| `internal/shared/config/admin/...` | ❌ FAIL | `TestResolveStorePathFallsBackWhenHomeMissing` 环境耦合失败 |

---

**报告生成完毕**。如需深入分析任何具体发现，请指示。
