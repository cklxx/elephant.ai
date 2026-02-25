# Plan: Lark 模型列表支持交互卡片直选

**Date:** 2026-02-06  
**Status:** done  
**Owner:** cklxx + Codex

## Goal
- 在 Lark 中执行 `/model` / `/model list` 时，提供可直接点选模型的交互卡片。
- 点击按钮后复用现有 `/model use <provider>/<model>` 选择链路完成会话级模型切换。

## Non-goals
- 不改动订阅选择存储格式（仍使用 `llm_selection.json`）。
- 不引入新的独立回调协议（优先复用现有 card callback 注入消息逻辑）。

## Approach
1. 调整 Lark model command：list 场景支持返回 `interactive` 或 `text` 两种响应。
2. 新增模型选择卡片构建逻辑（provider 分组 + 模型按钮）。
3. 按钮 value 写入 `text: "/model use ..."`，依赖现有 callback 的 text fallback 注入。
4. 保留文本回退，确保卡片不可用时行为不退化。
5. 添加单测覆盖卡片构建、点击注入映射与回退路径。

## Milestones
- [x] 测试先行（TDD）
- [x] 模型卡片实现
- [x] 回退逻辑确认
- [x] lint + 全量测试

## Progress Log
- 2026-02-06 16: 完成代码路径梳理：`handleModelCommand` 当前固定 text，card callback 已支持从 action.value.text 注入用户输入，可直接复用。
- 2026-02-06 16: 新增测试并先失败：`buildModelListReply` 未实现（符合 TDD 预期）。
- 2026-02-06 16: 完成实现：`/model` list 场景支持 interactive/text 双路径；卡片按钮注入 `/model use <provider>/<model>`。
- 2026-02-06 16: 完成文档更新：`CONFIG` 与 `LARK_CARDS` 增加模型选择卡片说明。
- 2026-02-06 16: 已通过 `./dev.sh lint` 与 `./dev.sh test` 全量回归。
