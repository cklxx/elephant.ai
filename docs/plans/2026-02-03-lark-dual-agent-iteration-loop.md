# Lark Dual-Agent Iteration Loop (Git-Driven, Self-Healing, Serial Deploy)

Date: 2026-02-03
Status: Draft
Author: cklxx

---

## Problem Statement

目标是“用户只在 Lark 交互”，实现 coding -> test -> auto-fix -> release 的自我迭代闭环；但在 primary 线上持续接单的前提下，不能让 secondary 在同一 worktree 里改代码/commit，否则会出现：

- 测试对象不稳定（测试跑着跑着代码变了）
- `git add -A` 把对方的中间态一起提交
- 自动修复与新需求并发写同一目录，导致冲突、难审计、难回滚

本方案把闭环的“真相源”从“目录状态”切换为“Git commit SHA”，让 secondary 可以大胆自愈，但不污染 primary 的在线工作区。

---

## Core Invariants (Must Hold)

1. 测试/修复对象必须是一个明确的 commit SHA（不可变快照）。
2. secondary 绝不写 primary 的 worktree：所有测试与 auto-fix 在隔离 worktree/clone 中进行。
3. merge & deploy 串行化：可以并行测试多个任务分支，但进入 main/线上发布必须排队。
4. 部署产物与 SHA 绑定：线上运行的二进制可回溯到具体 SHA（可审计/可回滚）。

---

## High-Level Architecture

```text
            Lark Chat (User only talks to Bot A)
                     |
                     v
            Bot A (Primary / Coding)
           - receive task
           - create branch
           - implement + commit + push
           - enqueue (branch + head_sha + chat_id)
                     |
                     v
            Bot B (Secondary / CI + Self-Heal + Release Train)
           - dequeue job
           - git worktree @ head_sha (isolated)
           - lint/test
           - fail: codex auto-edit -> commit -> push -> retest (loop)
           - pass: acquire deploy lock
                 -> rebase/merge to origin/main (ff-only preferred)
                 -> final gate on main head
                 -> build artifact pinned to SHA
                 -> deploy + healthcheck
                 -> notify chat
```

---

## Key Design Decisions

### 1) Branch-per-task (Primary)

- Bot A 每条 Lark 需求创建一个独立分支，避免“同 chat 连续需求”把工作区揉在一起造成并发不可控。
- 推荐命名：`auto/lark/<chat_id_hash>/<yyyymmdd-hhmmss>`（chat_id 取短 hash，避免过长/泄露）

### 2) Isolation via `git worktree` (Secondary)

- secondary 每个 job 使用 `git worktree add` 在 `.worktrees/<job_id>` 创建隔离目录，checkout 到指定分支/sha。
- 优点：快、节省 clone 成本、不会误污染主工作区。

### 3) Queue + Locks

必备：
- Queue（任务队列）：primary enqueue，secondary dequeue。
- Per-branch lock：同一 branch 只允许一个 runner，避免重复跑/重复 push。
- Global deploy lock：确保 merge + deploy 串行，避免线上反复重启/覆盖。

队列实现优先级（从易到难）：
1) 本地文件队列（推荐起步）：`tmp/ci/queue/*.json` + 原子 rename
2) Postgres 队列（多机/可观测）：表 + `FOR UPDATE SKIP LOCKED`

---

## Detailed Workflow

### A. Bot A (Primary) - Coding Phase

输入：用户在 Lark 群里发 “implement X”

Bot A 执行（核心：push 分支 + enqueue job，不做发布）：
1. 创建分支 `auto/lark/...`
2. 在 primary 自己的工作区改代码（允许乱搞，但只在该分支）。
3. （推荐 hard gate）`./dev.sh lint && ./dev.sh test`
4. commit（可多次） -> push 到远端该分支。
5. enqueue job（branch + head_sha + chat_id + trigger_message_id）。
6. 回复用户：已进入自愈/测试/发版队列（附 job_id）。

说明：
- primary 线上持续在线接单，不因为 CI 自愈/发版被反复重启打断。

### B. Bot B (Secondary) - Test + Self-Heal Loop

对每个 job：
1. 读取 job 元数据（branch + sha + chat_id）。
2. 创建隔离 worktree：`.worktrees/<job_id>`。
3. 拉取远端更新，checkout 到 `branch`（或直接 checkout 到 sha）。
4. 运行 gate：
   - `./dev.sh lint`
   - `./dev.sh test`
5. 若失败：
   - 把失败摘要（截断）+ log path 通知到 chat。
   - 在隔离 worktree 中运行 codex auto-edit 修复（约束：只修当前失败）。
   - commit + push（只 push 回该 branch）。
   - 循环：最多 `MAX_CYCLES` 轮，每轮最多 `MAX_RETRIES` 次 codex 重试。
6. 若通过：
   - 通知 “tests passed, entering release train”
   - 进入发版阶段（串行）。

### C. Release Train - Merge + Deploy (Serial)

对通过的 job（拿全局 deploy lock）：
1. `git fetch origin`
2. 尝试把任务分支变成可 fast-forward 的形态（推荐策略）：
   - 在隔离 worktree 中：`git rebase origin/main`
   - 若 rebase 冲突：停止自动化 -> 通知用户需要人工介入（或让 Bot A 解决冲突后重新 enqueue）。
3. 最终 gate：在 rebase 后 head 上再跑一次 `./dev.sh lint && ./dev.sh test`，保证与 main 兼容。
4. merge：优先 `--ff-only`，并 push `origin main`。
5. build：产物与 SHA 绑定（例如旁写 `VERSION`/`REVISION` 文件，或 `-ldflags` 注入）。
6. deploy：原子替换/滚动更新 + healthcheck。
7. notify：发布完成（附 main sha、变更摘要、回滚指令）。

---

## Notification Strategy (Lark)

目标：用户在 Lark 实时看到 “queued -> testing -> fixing -> passed -> deploying -> done/failed”。

推荐实现（从易到难）：
1) CLI 方式（推荐长期）：新增 `alex lark send --config ~/.alex/config-secondary.yaml --chat-id ... --text ...`，脚本直接调用 CLI，不需要额外 HTTP endpoint。
2) HTTP endpoint（短期可用）：Bot B server 提供 `POST /api/internal/lark/send`，仅监听 `127.0.0.1` + `X-Internal-Token` 鉴权 + chat_id allowlist。

通知内容规范：
- 长日志只发摘要（例如最后 30 行），附带本地日志路径（或上传后给链接）。
- 文本必须做 JSON 转义（禁止手写 `-d "{\"text\":\"$x\"}"` 这种拼接）。

---

## Configuration

### Primary: `~/.alex/config.yaml`

保持现有（线上 primary）。

### Secondary: `~/.alex/config-secondary.yaml`

Bot B 用于“通知 + 自愈/发版”，建议默认关闭 proactive/memory，减少噪声与 token 开销。

```yaml
# ~/.alex/config-secondary.yaml
runtime:
  api_key: ${OPENAI_API_KEY}
  llm_provider: auto
  llm_model: ${SECONDARY_LLM_MODEL}
  llm_small_model: ${SECONDARY_LLM_SMALL_MODEL}
  max_tokens: 12800
  verbose: false
  proactive:
    enabled: false
    memory:
      enabled: false

channels:
  lark:
    enabled: true
    app_id: "${SECONDARY_LARK_APP_ID}"
    app_secret: "${SECONDARY_LARK_APP_SECRET}"
    base_domain: https://open.larkoffice.com
    workspace_dir: /Users/bytedance/code/elephant.ai
    session_prefix: secondary
    allow_direct: true
    allow_groups: true
    memory_enabled: false
    reply_timeout_seconds: 18000
```

---

## Script Layout (Proposed)

```text
.
├── scripts/lark/primary.sh          # start/stop primary bot server
├── scripts/lark/ci-worker.sh        # Bot B worker: dequeue -> test/fix -> release train
├── scripts/lark/ci-enqueue.sh       # Bot A enqueue helper (branch+sha+chat)
├── scripts/lark/notify.sh           # send Lark messages (via CLI or internal endpoint)
├── tmp/ci/queue/                    # job queue items
├── tmp/ci/running/                  # in-progress markers
├── tmp/ci/done/                     # completed markers
├── .worktrees/                      # per-job worktrees (isolated)
└── .locks/deploy.lock               # global merge/deploy lock
```

Queue item format（JSON；脚本用 python 读取）：

```json
{
  "job_id": "20260203-001122-abc123",
  "branch": "auto/lark/xxxx/20260203-001122",
  "head_sha": "abcdef1234...",
  "chat_id": "oc_xxx",
  "trigger_message_id": "om_xxx",
  "created_at": "2026-02-03T00:11:22Z"
}
```

---

## Safeguards

- `MAX_CYCLES`：防止无限自愈循环。
- `MAX_RETRIES`：单轮 codex 修复重试上限。
- `deploy.lock`：确保 main/线上只会被一个 job 更新。
- worktree TTL：成功/失败后清理 `.worktrees/<job_id>`，避免磁盘膨胀。
- ff-only 默认：不允许自动产生 merge commit（更易审计/回滚）。

---

## Failure Modes & Handling

- rebase 冲突：停止自动化 -> 通知用户/primary 处理冲突 -> push 更新后重新 enqueue。
- 持续测试失败：保留分支与日志 -> 通知“需要人工介入”，附失败摘要 + log path。
- deploy 失败：执行 rollback（上一版本 SHA）-> 标记 job failed -> 通知。

---

## Progress Checklist

- [ ] 选择通知通道：CLI `alex lark send` vs 内部 HTTP endpoint
- [ ] 落地 queue（本地文件队列 or Postgres）
- [ ] 实现 `git worktree` 隔离 runner（含 cleanup）
- [ ] 实现 deploy train（deploy lock + ff-only merge + final gate）
- [ ] 更新 `AGENTS.md`/`CLAUDE.md`：primary 的“push 后 enqueue job（而非重启）”规则
- [ ] E2E 演练：并发两个 Lark 任务，验证不会互相污染；发布串行

