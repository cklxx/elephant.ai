# User Correction: Remove Legacy Channel Registration When Migrating to CLI

Date: 2026-03-05

## What happened
用户明确要求“不要再注册 `channel` 工具，直接走 CLI”。我在迁移执行中仍保留了 `channel` 作为兼容适配层，偏离了用户对目标态（仅 CLI/skills）的要求。

## Why it happened
- 我把“平滑兼容”误判为优先级高于“目标态收敛”。
- 在用户给出“全部迁移到skills”后，没有把“移除旧入口注册”设为硬约束。

## Preventive rule
当用户明确指定“不要某入口/不要兼容层/直接走新路径”时：
1. 立即把“移除旧入口注册与调用路径”写入执行清单第一优先级；
2. 在代码上删除注册点与调用入口，而不是保留薄适配；
3. 仅在用户明确要求兼容期时，才保留过渡层。

## Enforcement checklist
- [ ] 删除旧工具注册（而非只改实现）
- [ ] 清理与旧入口绑定的测试期望
- [ ] 更新文档与提示词，避免继续推荐旧入口
