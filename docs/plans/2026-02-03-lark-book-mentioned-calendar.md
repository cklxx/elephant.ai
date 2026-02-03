# Plan: Book @mentioned user calendar (resolve primary by user)

Date: 2026-02-03

## Context
- Users want to “book @某人”的日历，即把日程写入被 @ 的用户主日历。
- Lark Calendar v4 不接受 `calendar_id="primary"`，需要真实 `calendar_id`（如 `cal_...`）。
- Lark 消息的 @mention 在 `post`/`text` 内容中可携带 `user_id`，但当前抽取为纯文本时丢失了 ID，LLM 无法把“@人”映射到可用的 user id。

## Goal
1) LLM 能从用户输入中拿到 @mention 的 user id（优先 open_id）。
2) 工具支持把 `calendar_id="primary"` 解析为“某个指定用户”的主日历 `calendar_id`，并在该日历创建日程。

## Approach
1) `internal/lark` 增加通过 Calendar.Primarys 查询用户主日历的方法（user_id_type + user_ids → calendar_id）。
2) `lark_calendar_create`（以及其他 calendar 工具）增加可选参数：
   - `calendar_owner_id`：要操作的目标用户 ID（通常是 open_id）
   - `calendar_owner_id_type`：默认 `open_id`
   - 当 `calendar_id` 为 `primary` 时优先用 owner 来解析；未提供 owner 则使用当前请求上下文的 user_id（sender open_id）。
3) Lark Gateway 抽取消息内容时保留 mention 的 user id：
   - `post`：`@name(ou_xxx)`
   - `text`：把 `<at user_id="ou_xxx">name</at>` 渲染成 `@name(ou_xxx)`

## Progress
- [x] Review mention payload handling.
- [x] Implement primary calendar lookup by user IDs.
- [x] Extend calendar tools with calendar owner params.
- [x] Expose mention IDs in extracted content.
- [ ] Run `make fmt && make test`; commit.

## Result
- `lark_calendar_create` supports booking an @mentioned user's primary calendar by using:
  - `calendar_id: "primary"`
  - `calendar_owner_id: "<open_id>"` (usually from the mention, e.g. `@Bob(ou_xxx)`)
- Lark Gateway now preserves mention IDs in extracted task text:
  - `post`: `@name(ou_xxx)`
  - `text`: `<at user_id="ou_xxx">name</at>` → `@name(ou_xxx)`
