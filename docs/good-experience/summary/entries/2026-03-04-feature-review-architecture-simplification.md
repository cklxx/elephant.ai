Summary: 跨层重构先抽共享契约（EventDispatcher + CompressionPolicy + Task 子接口）再迁移调用点，可在不改行为前提下稳定提升简洁性与复用度；并可通过分主题提交与分层门禁快速定位基线失败。

## Metadata
- id: goodsum-2026-03-04-feature-review-architecture-simplification
- tags: [summary, architecture, simplification]
- derived_from:
  - docs/good-experience/entries/2026-03-04-feature-review-architecture-simplification.md
