# 2026-03-04 - Feature Review 架构简化分阶段落地

## Context
- 目标：提升核心后端的简洁性、复用度、抽象组合质量。
- 约束：不破坏现有行为；可增量提交与回滚。

## What Worked
- 先做“契约提取”，再迁移调用点：
  - `EventDispatcher` 把 coordinator 事件装配集中到单点。
  - `ports.BuildCompressionPlan` 成为 context 压缩策略单一来源，app/react 同步复用。
  - `task.Store` 拆成子接口后，调用方可按最小能力依赖（resumer 仅依赖所需方法）。
- 按主题拆分提交（dispatcher / context policy / task store）提高评审效率。
- 先跑受影响包测试，再跑全量门禁，快速隔离“改动引入”与“基线环境失败”。

## Evidence
- Commits:
  - `a5bf745b` refactor(coordinator): converge event listener assembly via dispatcher
  - `8ae6a491` refactor(context): share compression planning policy across app and react
  - `9c95ba4b` refactor(task): split store interfaces and narrow resumer dependency
- Verification:
  - 受影响包 `go test` 全通过。
  - `make check-arch` 与 `golangci-lint` 通过。
  - `make check-arch-policy` 与 `make test` 失败为可复现基线问题（main 同样失败）。

## Reusable Rule
- 对跨层重构，优先顺序固定为：
  1. 新建共享契约/策略（不改行为）；
  2. 并行迁移调用点；
  3. 删除重复实现；
  4. 用最小接口收口调用依赖。

## Metadata
- id: good-2026-03-04-feature-review-architecture-simplification
- tags: [architecture, simplification, event-pipeline, context-policy, interface-segregation]
- links:
  - docs/plans/2026-03-04-feature-review-architecture-simplification-implementation.md
  - internal/app/agent/coordinator/event_dispatcher.go
  - internal/domain/agent/ports/compression_policy.go
  - internal/domain/task/store.go
