---
name: code-review
description: 编码完成后的多维度代码审查，覆盖 SOLID 架构、安全性、代码质量、边界条件与清理计划，输出结构化审查报告。
triggers:
  intent_patterns:
    - "review|审查|code review|CR|代码审查|review code"
  tool_signals: []
  context_signals:
    keywords: ["review", "审查", "CR", "code review", "代码质量", "merge"]
  confidence_threshold: 0.6
priority: 9
exclusive_group: review
max_tokens: 3000
cooldown: 60
output:
  format: markdown
  artifacts: true
  artifact_type: document
---

# 代码审查（编码完成后多维度 Review）

## When to use this skill
- 编码完成、准备提交或合并前，对变更进行系统性审查。
- 需要从架构、安全、质量、边界条件等多个维度发现潜在问题。
- 作为编码结束的标准收尾动作，确保变更质量。

## 严重等级定义

| 等级 | 含义 | 处理方式 |
|------|------|----------|
| **P0 Critical** | 数据丢失、安全漏洞、生产环境崩溃 | 阻止合并，必须立即修复 |
| **P1 High** | 逻辑错误、竞态条件、错误处理缺失 | 合并前修复 |
| **P2 Medium** | 性能问题、命名不当、缺少测试 | 创建 follow-up 跟踪 |
| **P3 Low** | 风格建议、微小优化 | 可选改进 |

## 工作流

### Step 1 — 确定审查范围
- 运行 `git diff --stat` 和 `git diff` 获取变更范围。
- 若无变更（空 diff），输出 "无变更，跳过审查" 并终止。
- 若变更超过 500 行，按文件/模块分批审查，每批不超过 300 行。
- 记录：变更文件数、新增/删除行数、涉及的包/模块。

### Step 2 — SOLID 与架构审查
- 加载 `references/solid-checklist.md` 中的具体检查项。
- 逐文件检查：单一职责、开闭原则、依赖倒置、接口隔离。
- 识别常见代码坏味道：过长函数、特性嫉妒、数据泥团、散弹式修改、死代码、魔法数字。
- 对于 Go 代码重点关注：接口大小、包职责边界、error wrapping 链。
- 对于 Rust 代码重点关注：ownership 清晰度、trait 设计、生命周期复杂度。

### Step 3 — 安全与可靠性审查
- 加载 `references/security-checklist.md` 中的具体检查项。
- 检查输入验证、注入防护、认证授权、密钥泄露、依赖安全。
- **竞态条件专项**：共享状态同步、TOCTOU、数据库并发（乐观/悲观锁）、分布式锁。
- Go 专项：goroutine 泄露、channel 死锁、context 传播、sync.Mutex 正确使用。
- Rust 专项：unsafe 块审查、Send/Sync trait 边界。

### Step 4 — 代码质量审查
- 加载 `references/code-quality-checklist.md` 中的具体检查项。
- 错误处理：吞掉的错误、过宽的 catch/recover、缺少 error boundary。
- 性能：N+1 查询、热路径上的昂贵操作、缺少缓存、无界集合。
- 边界条件：nil/zero-value 处理、空集合访问、数值溢出、off-by-one。
- 可观测性：关键路径是否有 trace/metric、错误是否包含足够上下文。

### Step 5 — 清理与删除计划
- 加载 `references/removal-plan.md` 中的模板。
- 识别变更引入的死代码、冗余逻辑、被 feature flag 关闭的代码。
- 区分"可立即安全删除"与"需延迟删除（附计划）"。
- 检查是否有遗留的 TODO/FIXME/HACK 未清理。

### Step 6 — 生成审查报告

输出格式：

```markdown
## Code Review Summary
**审查范围**: X 个文件，Y 行变更
**总体评估**: [APPROVE | REQUEST_CHANGES | COMMENT]

## 发现

### P0 - Critical
（无则标注"无 P0 问题"）

### P1 - High
- **[file:line]** 问题标题
  - 问题描述
  - 建议修复方案

### P2 - Medium
- ...

### P3 - Low
- ...

## 清理/删除计划
（如有，列出可删除项与延迟删除项）

## 补充建议
（架构改进、测试补充等非阻塞建议）
```

### Step 7 — 确认与修复
- 展示审查报告，等待用户确认。
- 提供选项：
  1. 修复所有问题
  2. 仅修复 P0/P1
  3. 修复指定问题
  4. 不做修改，仅记录
- 修复完成后重新运行 lint + test 验证。

## 边界情况处理
- **空 diff**：输出"无变更"并终止。
- **超大变更（>500 行）**：分批审查，每批附带该批次的上下文摘要。
- **混合关注点**：若一次变更跨多个不相关功能，按功能分组审查并建议拆分提交。
- **仅配置/文档变更**：简化审查，仅检查格式正确性和敏感信息泄露。

## 最终检查清单
- 所有 P0/P1 问题已修复或明确标注延迟原因。
- 审查报告已保存或记录。
- lint + test 全部通过。
- 无敏感信息（密钥、凭据）出现在变更中。
