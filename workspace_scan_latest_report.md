# 工作区状态扫描报告
**扫描时间**: 2026-03-01T15:20:00+08:00  
**工作区**: /Users/bytedance/code/elephant.ai

---

## 🔴 关键发现 (8条)

### 1. Kernel Runtime 失败率 30.86%，最近周期全失败
- **影响**: 核心调度引擎无法正常工作
- **证据**: STATE.md 显示 `failed_ratio: 30.86%`，最新 cycle `run-5ZHwPB80GaXz` 5 dispatch 全部失败
- **根因**: Context cancellation during session acquisition (已定位到 `internal/app/agent/preparation/service.go:149`)
- **澄清**: 代码测试全绿，这是运行时上下文生命周期问题，非代码回归

### 2. Context Cancellation 根因已明，修复待实施
- **发现**: 周期超时配置 930s (900s+30s buffer)，但失败发生在 ~35s，说明 parent context 被上游取消
- **当前状态**: STATE.md 中 next_action 已更新为修复方案：使用独立 context 替代继承 context
- **文件**: `internal/app/agent/preparation/service.go:149` 需 patch

### 3. LLM 限流严重 Anthropic 429，Kimi 400 错误
- **Anthropic**: Rate limit 触发，retry_after ~10000s (~2.7h)，claude-sonnet-4-6 模型频繁失败
- **Kimi**: HTTP 400 `reasoning_content is missing in assistant tool call message` — API 契约变更或不兼容
- **影响**: LLMPlanner fallback 到 static planner，降低调度质量

### 4. Config 配置错误：OpenAI provider 使用 Kimi key
- **日志**: `api key prefix=sk-kimi-... looks moonshot/kimi but base_url="https://api.openai.com/v1" is not moonshot-compatible`
- **影响**: 配置 reload 失败，可能导致 provider 选择错误
- **建议**: 检查 `.env` 或配置文件中的 llm profile 映射

### 5. Sidecar 状态文件 stale：`team-deep_research_multi_agent`
- **状态文件**: `.elephant/tasks/team-deep_research_multi_agent.status.yaml`
- **显示**: pending/blocked (4 pending, 2 blocked)，但可能实际已完成
- **根因**: `ExecuteAndWait` final sync path 中 `StatusWriter` 未 hydrate `sw.file.Tasks`，导致 `SyncOnce` no-op
- **修复**: 需 patch `internal/domain/agent/taskfile/executor.go`，添加 hydration + regression test (PLAN.md 已记录)

### 6. Dispatch Store 无限增长风险 (PLAN.md 识别)
- **问题**: `dispatches.json` 单调增长，无 GC 机制
- **估算**: 48 cycles/day × 30 days = 7200+ records，全部常驻内存
- **影响**: `ListRecentByAgent()` 和 `RecoverStaleRunning()` 每次扫描全部记录，周期启动变慢
- **优先级**: PLAN.md 标记为最高价值修复

### 7. 2 个 commit 未推送，STATE.md 有未提交修改
- **未推送**: 
  - `47725c3b` kernel: context cancellation fix, test coverage additions
  - `591aff39` kernel: update STATE.md with context cancellation investigation findings
- **未提交**: STATE.md 的 next_action 已更新为 context cancellation 修复方案
- **状态**: 工作区有未完成的修复工作

### 8. Integration Test Flake 持续：`TestLarkInject_TeamHappyPath`
- **错误**: `read |0: file already closed` (pipe race)
- **范围**: `alex/internal/infra/integration`
- **历史**: 多次 audit 中标记为已知问题，非 kernel 核心路径
- **风险**: 全量测试 suite 不可靠，影响 CI 信号质量

---

## ✅ 可执行建议 (3条)

### 建议 1: 立即修复 Context Cancellation (优先级 P0)
**目标**: 解决 kernel runtime 失败，恢复调度能力

**具体步骤**:
1. 修改 `internal/app/agent/preparation/service.go:149` 的 `loadSession` 调用
2. 使用独立 context (带 timeout) 替代继承自 parent 的 context：
   ```go
   sessionCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
   defer cancel()
   session, err := s.loadSession(sessionCtx, ...)
   ```
3. 添加 context deadline 日志用于后续调试
4. 推送已有的 2 个 commits，提交新修复
5. 重新部署并监控 cycle 成功率

**验证**: 观察 `logs/alex-kernel.log` 中 `context canceled` 错误是否消失

---

### 建议 2: 修复 LLM Provider 配置 + 限流处理 (优先级 P1)
**目标**: 恢复 LLMPlanner 正常功能，减少 fallback

**具体步骤**:
1. **Config 修复**: 检查 `.env` 或 `configs/`，修复 openai provider 配置 (移除 kimi key 或调整 base_url)
2. **限流缓解**:
   - Anthropic: 实施更激进的客户端限流 + 更长的 backoff
   - Kimi: 调查 `reasoning_content` 错误，可能是 tool call message 格式问题
3. **降级策略**: 确保 static planner fallback 能稳定工作，作为限流时的保底

**验证**: 观察 `logs/alex-llm.log` 中 429/400 错误频率

---

### 建议 3: 实施 Sidecar 状态同步修复 (优先级 P1)
**目标**: 修复 team-deep_research_multi_agent 状态显示错误

**具体步骤**:
1. 修改 `internal/domain/agent/taskfile/executor.go` 的 `ExecuteAndWait` final sync path
2. 在创建 `StatusWriter` 后，调用 `sw.Rehydrate()` 或手动加载 tasks
3. 确保 `SyncOnce` 操作的是最新状态
4. 添加 regression test 验证状态同步逻辑
5. 手动修复 `team-deep_research_multi_agent.status.yaml` 为正确状态

**验证**: 运行 taskfile executor tests，确认 `SyncOnce` 不再 no-op

---

## ❓ 需要确认的事项 (3项)

### 确认 1: Context Cancellation 修复方案
**问题**: STATE.md 中建议的修复方案是使用独立 context，但这是否足够？是否需要更全面的 context 传播审查？

**选项**:
- A) 仅修复 `preparation/service.go:149` 的 session load (快速修复)
- B) 全面审查 kernel 中所有 context 使用点，统一改为带 buffer 的独立 context (彻底修复)

**建议**: 先实施 A 验证效果，同时记录 B 为技术债务

---

### 确认 2: Anthropic Rate Limit 应对策略
**问题**: 当前 Anthropic 限流 retry_after ~2.7h，是否需要切换模型或降低调用频率？

**选项**:
- A) 切换到 claude-haiku 或其他低限流模型 (可能影响质量)
- B) 实施更激进的客户端限流，避免触发服务端限流
- C) 增加多 provider fallback，限流时自动切换

**数据支持**: `logs/alex-llm.log` 显示过去 24h 内 429 错误频繁，影响 kernel 调度

---

### 确认 3: 未推送 Commits 的状态
**问题**: `47725c3b` 和 `591aff39` 两个 commits 包含什么内容？是否已完成测试？

**需要确认**:
1. `47725c3b` 中的 "context cancellation fix" 是否就是 STATE.md 建议的方案？
2. 这两个 commits 是否可以安全推送？
3. 推送后是否需要立即部署？

**建议操作**:
```bash
git show 47725c3b --stat
git show 591aff39 --stat
git log origin/main..HEAD --oneline
```

---

## 📊 数据摘要

| 指标 | 值 | 状态 |
|------|---|------|
| Kernel 失败率 | 30.86% | 🔴 严重 |
| 最新周期成功率 | 0/5 (0%) | 🔴 失败 |
| 核心测试 | PASS | 🟢 正常 |
| Integration Test | FLAKY | 🟡 不稳定 |
| LLM 429 错误 | 频繁 | 🔴 限流 |
| Sidecar Stale | 1 个 task | 🟡 待修复 |
| 未推送 Commits | 2 | 🟡 待处理 |
| 未提交修改 | STATE.md | 🟡 待提交 |

---

## 🎯 下一步行动建议

1. **立即**: 确认并推送现有的 2 个 commits
2. **今天**: 实施 context cancellation 修复，测试并部署
3. **本周**: 修复 sidecar 状态同步问题
4. **本周**: 修复 LLM provider 配置，调研 Kimi reasoning_content 错误
5. **下周**: 评估 dispatch store GC 方案 (参考 PLAN.md)

---

*报告生成: 2026-03-01*  
*数据来源: STATE.md, PLAN.md, git log, logs/*.log, .elephant/tasks/*, artifacts/*, 代码检查*
