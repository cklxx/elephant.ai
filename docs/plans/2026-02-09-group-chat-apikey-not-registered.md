# 2026-02-09 群聊链路 `apikey not registered` 排查与修复计划

## 背景
- 现象：群聊消息偶发/持续报错 `apikey 没注册`，单聊正常。
- 目标：定位群聊与单聊在 APIKey 注册/解析上的链路差异并修复，补齐测试防回归。

## 计划
1. [x] 梳理消息处理链路：群聊入口、会话上下文、provider/key 解析与注册调用。
2. [x] 对照单聊链路确认差异点，复现触发条件并锁定根因。
3. [x] 以最小改动修复群聊路径，保持架构边界不变。
4. [x] 采用 TDD 补齐/更新测试，覆盖群聊关键分支。
5. [ ] 执行完整 lint + tests，修正潜在回归。
6. [ ] 更新计划进度与记录，整理提交并合并回 `main`。

## 进度记录
- 2026-02-09 00:00：创建计划，开始链路梳理。
- 2026-02-09 00:20：定位根因：
  - `/model use --chat` 实际按 `chat_id + user_id` 作用域生效，群聊多用户会漏命中已选模型并回落到默认 provider。
  - sender 提取只读 `open_id`，在部分群聊事件中为空会放大作用域命中问题。
- 2026-02-09 00:30：完成修复与测试：
  - SelectionScope 支持 chat 级 key（`channel + chat_id`）并保持 legacy `chat+user` 兼容读取。
  - Lark selection 查询顺序调整为 `chat -> legacy chat+user -> channel`；`--chat` 改为写入 chat 级。
  - sender 提取补充 `user_id/union_id` 回退。
  - 新增/更新单测并通过：
    - `go test ./internal/app/subscription ./internal/delivery/channels/lark`
- 2026-02-09 00:40：执行全量校验：
  - `make check-arch` 通过。
  - `make test` 失败于仓库既有门禁 `internal/shared/config TestNoUnapprovedGetenv`（涉及 `cmd/alex/dev.go` 等与本改动无关文件）。
  - `make fmt` 同样受既有 lint 问题阻塞（`internal/devops/*`, `cmd/alex/dev*.go`）。
