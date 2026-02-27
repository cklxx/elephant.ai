# Bub “无限上下文”实现级调研（Append-only Tape + Anchor/Handoff）

Date: 2026-02-27  
Author: Codex（基于 3 个 sub-agent 并行源码勘查汇总）

## 1. 结论（先看这个）

1. Bub 的“无限上下文”不是扩展单次 LLM context window，而是把会话历史写入 append-only JSONL tape，然后每轮按规则回放/切片。
2. `anchor/handoff` 的核心价值不是“打标签”，而是改变默认历史切片起点（默认取 `LAST_ANCHOR` 之后的内容）。
3. 路由层把命令执行结果结构化为 `<command ...>` block，可回注下一轮模型，形成“失败可恢复”的上下文推进链路。
4. 运行时采用 `fork_tape -> 执行 -> merge`，把一次输入处理隔离在分叉 tape，结束后增量合并回主 tape。
5. 该方案工程上可落地，但当前实现在跨进程并发锁、损坏行可观测性、merge 边界持久化上存在改进空间。

---

## 2. 调研范围与版本基线

- Bub 仓库：`PsiACE/bub`
- 调研 commit：`5cfa3cfdd7ff5738ee799569fe6b4a6d02d71387`
- 重点文件：
  - `src/bub/tape/store.py`
  - `src/bub/tape/service.py`
  - `src/bub/core/router.py`
  - `src/bub/core/agent_loop.py`
  - `src/bub/core/model_runner.py`
  - `src/bub/tools/builtin.py`
  - `tests/test_tape_store.py`
  - `tests/test_tape_service.py`
  - `tests/test_router.py`
- 依赖机制（republic）：
  - `tape/context.py`
  - `clients/chat.py`

说明：本报告所有机制判断均来自源码路径与行号证据，不基于产品宣传语推断。

---

## 3. 数据模型与文件布局

### 3.1 Tape 基本单元

- 数据结构：`TapeEntry(id, kind, payload, meta)`。
- 语义：`id` 是序号，`kind` 是事件类型，`payload/meta` 存内容与附加信息。
- Bub 落盘时会重写 `id`，以文件尾序号为准（外部传入 `id` 不作为最终存储值）。

### 3.2 文件命名与隔离

- 目录：`<home>/tapes`。
- 工作区隔离：`workspace_hash = md5(str(workspace_path.resolve()))`。
- 文件名：`<workspace_hash>__<urlencoded_tape_name>.jsonl`。

这三个规则决定了：
1. 同一 home 下不同 workspace 的 tape 物理隔离。
2. tape 名支持 URL 编码字符。
3. 可从文件名反解 tape 名。

---

## 4. 存储层实现（`store.py`）逐函数拆解

### 4.1 `TapeFile` 运行态字段

- `path`: 当前 tape 文件路径。
- `fork_start_id`: merge 增量起点（仅内存态）。
- `_lock`: 线程内互斥锁。
- `_read_entries`: 已缓存 entry。
- `_read_offset`: 已读文件偏移。

`TapeFile` 本质是“单文件增量读取 + 追加写入 + fork 辅助状态”的最小执行单元。

### 4.2 读取链路：增量读 + 损坏容忍

读取逻辑要点：

1. 若文件不存在：重置缓存并返回空。
2. 若 `file_size < _read_offset`：认为文件被 truncate/替换，清空缓存与 offset。
3. `seek(_read_offset)` 仅读新增行（delta）。
4. 空行跳过；JSON decode 失败跳过；结构不合法跳过。
5. 解析成功 entry 追加到 `_read_entries`，最后更新 `_read_offset = handle.tell()`。

这是一种“可前滚、弱一致、容错优先”的读取策略：
- 优点：不会因单行损坏阻断全局读取。
- 代价：坏行可能被静默吞掉，默认可观测性弱。

### 4.3 写入链路：先同步读，再顺序追加

`_append_many(entries)` 核心步骤：

1. 加锁。
2. 先调用 `_read_locked()`，把缓存/offset 同步到最新。
3. 计算 `next_id`（当前缓存尾部 `id + 1` 或 1）。
4. 打开文件 `a` 追加模式，逐条写 `json.dumps(payload) + "\n"`。
5. 每条写入后将“重编号后的 stored entry”放入 `_read_entries`。
6. 更新 `_read_offset = handle.tell()`。

一致性语义：
- 进程内线程安全：有锁。
- 跨进程不保证：无文件锁（`flock/fcntl`）。
- 写入不是 fsync 级 durability：崩溃窗口下最后几条可能丢失。

### 4.4 fork/merge 机制：分叉执行，增量回灌

#### fork（`FileTapeStore.fork`）

1. 生成 `new_name = source + "__" + uuid8`。
2. `copy_to` 复制源文件到 fork 文件。
3. fork 侧 `fork_start_id = source._next_id()`。

#### merge（`FileTapeStore.merge`）

1. 从 fork 读取全部 entry。
2. 过滤 `entry.id >= source.fork_start_id` 的增量部分。
3. 追加到目标 tape（目标侧重新分配 id）。
4. 删除 fork 文件并清理缓存映射。

该机制等价于“文件级 copy-on-fork + 逻辑增量 merge”，冲突策略是“按 merge 发生顺序线性追加”，不做语义冲突检测。

### 4.5 archive/reset

- `archive()`：把当前 `.jsonl` rename 成 `.jsonl.<timestamp>.bak`。
- `reset()`：删除主文件并清空缓存。

---

## 5. 运行时链路（router/agent/model）逐步展开

## 5.1 输入总入口与 fork 隔离

运行时处理一次输入时进入 `with tape.fork_tape()`：
1. 在 fork tape 里处理当前输入。
2. 结束后 merge 回主 tape。

这保证了一次处理流程的隔离性，并把最终结果串行化回主会话流。

## 5.2 用户输入与 assistant 输出共享同一命令路由内核

- 用户侧：`route_user()`。
- assistant 侧：`route_assistant()`。
- 两者共用：
  - `_parse_comma_prefixed_command()`
  - `_execute_command()`

区别：
- `origin` 分别为 `human` / `assistant`。
- assistant 额外支持 fenced command 多行聚合执行。

## 5.3 命令失败回注机制（关键）

当命令失败：
- router 生成结构化 `<command ... status="error">...</command>` block。
- 用户侧失败：返回 `enter_model=True` 且 `model_prompt=block`，让模型在下一轮消费真实错误输出。
- assistant 侧命令：也会把 block 放入 `next_prompt`，继续模型回合。

这实现了“命令失败 -> 上下文证据化 -> 模型自纠”闭环，而非简单报错中止。

## 5.4 模型层与 tape 耦合

`ModelRunner` 每步会构建 system prompt（runtime contract + tools view + skills view），然后走 republic chat 客户端。

在 republic 侧：
1. 先按 tape context 取历史消息（默认 `LAST_ANCHOR` 后）。
2. 拼接本轮 user prompt。
3. 模型返回 tool calls 时写入 `tool_call/tool_result/error/run` 事件到 tape。
4. 模型返回自然语言时写入 assistant 事件。

所以“无限上下文”的核心不是 prompt 拼接技巧，而是“每轮事件化写 tape + 下轮按 anchor 切片读取”的持续机制。

---

## 6. Anchor/Handoff 的真实语义

### 6.1 不是标签，而是切片边界

- `tape.handoff` 会写 anchor。
- `default_tape_context()` 未覆写 `anchor` 策略。
- republic `TapeContext.anchor` 默认是 `LAST_ANCHOR`。

因此：
- 写了新 anchor 后，后续默认历史窗口从该 anchor 之后开始。
- 这是会影响模型输入边界的“硬行为”，不是“展示层标记”。

### 6.2 对长会话的价值

1. 显式阶段化（phase boundary）。
2. 减少无关旧上下文进入每轮 token 预算。
3. 可配合 `tape.search/after_anchor/between_anchors` 做按需回溯。

---

## 7. 命令面与参数语法

### 7.1 tape 相关命令（实现层）

- `,tape.handoff name=? summary=? next_steps=?`
- `,tape.anchors`
- `,tape.info`
- `,tape.search query=<required> limit=<default 20, ge=1>`
- `,tape.reset archive=<bool default false>`

### 7.2 参数解析支持

- `key=value`
- `--key value`
- `--key=value`
- `--flag`（布尔）

### 7.3 路由边界

- 仅“行首 `,`”触发命令解析。
- internal command 未命中时按 shell 执行。

---

## 8. 测试覆盖与边界条件

### 8.1 覆盖到的关键行为

1. `store`：隔离、归档、增量读、truncate、ID 连续、fork/merge 顺序。
2. `service`：reset 后 bootstrap anchor、search fuzzy typo、exact+limit。
3. `router`：命令边界、成功短路、失败回注、assistant 同规则、fenced command。

### 8.2 高价值边界条件（已在代码中体现）

1. 坏 JSON 行被静默跳过（不抛错）。
2. 非法结构行被丢弃。
3. truncate 导致 offset 回退时重置缓存。
4. append 在未 read 场景下仍保证序号连续（基于预同步读）。
5. 并发仅覆盖进程内线程，不覆盖多进程。
6. merge 冲突无语义检测，按 merge 顺序追加。
7. `tape.search` 的 `limit` 是“每 tape 上限”，`all_tapes` 聚合可超过 limit。
8. fuzzy 仅在 query 长度 >= 3 生效，阈值固定（score cutoff 80，候选最多 128）。

---

## 9. 文档与实现一致性审计

### 9.1 一致项

1. 仅行首 `,` 命令模式。
2. 命令成功直接返回。
3. 命令失败结构化回注模型。

### 9.2 偏差项（需要注意）

1. README/Features 用 `,handoff` / `,anchors`，实现注册的是 `,tape.handoff` / `,tape.anchors`。
2. README 示例含 `,skills.describe`，实现中未注册该命令。
3. 文档写 tools+skills 一个 registry，代码中是 tools registry + skills discover 的双通道装配。

---

## 10. 性能与一致性语义评估

### 10.1 时间复杂度（按实现推导）

1. `read`：首读 `O(N)`，后续增量 `O(ΔN)`。
2. `append(k)`：`O(ΔN + k)`（前置同步读 + 写 k 条）。
3. `fork`：文件复制 `O(file_size)`。
4. `merge`：大致 `O(source_delta)`。

### 10.2 一致性语义（当前状态）

1. 进程内线程安全：是。
2. 跨进程安全：否（无 OS 文件锁）。
3. 崩溃恢复：部分（基于 append-only + offset 重建），但无 fsync 与 merge 元数据持久化。
4. 可观测性：中等偏弱（坏行静默跳过）。

---

## 11. 风险清单与改进建议（代码级）

1. **跨进程并发写风险**  
   建议：写入路径增加 `flock/fcntl` 或单写者进程模型。

2. **`_tape_file` 懒加载未加全局锁**  
   建议：在 `_tape_file()` 用 store 级互斥保护 map 初始化。

3. **`fork_start_id` 仅内存态**  
   建议：将 fork 元信息持久化（sidecar 或 metadata entry），避免崩溃后 merge 边界丢失。

4. **坏行静默吞掉**  
   建议：增加错误计数/告警日志；对尾部坏行采用“暂不推进 offset”策略。

5. **无 fsync**  
   建议：新增可配置 durability 级别（`best_effort` / `fsync_on_append`）。

6. **命名规则潜在歧义**（`__`）  
   建议：创建 tape 名时禁止 `__` 或调整 `list_tapes` 过滤策略。

7. **文档示例与实现命令名偏差**  
   建议：同步文档，或提供兼容 alias（`handoff -> tape.handoff`，`anchors -> tape.anchors`）。

---

## 12. 可直接落地的代码实现草案（Go 版本）

下面给的是“按 Bub 机制复刻”的最小实现骨架，可直接用于 elephant.ai 设计讨论与 PoC。

### 12.1 核心类型

```go
type TapeEntry struct {
	ID      int64                  `json:"id"`
	Kind    string                 `json:"kind"`
	Payload map[string]any         `json:"payload"`
	Meta    map[string]any         `json:"meta"`
}

type TapeStore interface {
	ListTapes(ctx context.Context) ([]string, error)
	Read(ctx context.Context, tape string) ([]TapeEntry, error)
	Append(ctx context.Context, tape string, entries []TapeEntry) error
	Reset(ctx context.Context, tape string) error
	Archive(ctx context.Context, tape string) (string, error)
	Fork(ctx context.Context, source string) (string, error)
	Merge(ctx context.Context, source, target string) error
}
```

### 12.2 单文件执行态（增量读 + append）

```go
type TapeFile struct {
	Path         string
	ForkStartID  int64
	mu           sync.Mutex
	readEntries  []TapeEntry
	readOffset   int64
}

// 关键语义：
// 1) ReadLocked: seek(readOffset) 只读增量
// 2) AppendMany: 先 ReadLocked 同步，再从 tail+1 分配 ID
// 3) Truncate 检测: fileSize < readOffset 时 reset cache
```

### 12.3 fork/merge 语义复刻

```go
// Fork:
// 1) 复制 source 文件到 fork 文件
// 2) fork.ForkStartID = source.nextID()
//
// Merge:
// 1) 读取 source 所有 entry
// 2) 过滤 entry.ID >= source.ForkStartID
// 3) append 到 target（target 侧重编号）
// 4) 删除 source fork 文件
```

### 12.4 路由与模型耦合接口

```go
type RouteResult struct {
	HandledDirectly bool
	Reply           string
	EnterModel      bool
	ModelPrompt     string // command block when command failed
	ContinuePrompt  string // follow-up for assistant command execution
}

// 关键行为：
// - 用户与 assistant 共用 parse+execute
// - command failure => structured block reinjection
// - 每轮模型后将 user/assistant/tool_call/tool_result/error 写入 tape
```

### 12.5 与 elephant.ai 现有机制的兼容建议

当前 elephant.ai 已有：
- `CTX_PLACEHOLDER` + compaction artifact
- `context_checkpoint` + archive

建议融合方式：
1. 保留 `CTX_PLACEHOLDER` 作为“硬预算超限救火”。
2. 新增 tape 流作为“常态化长会话记忆层”。
3. 采用 `anchor/handoff` 做阶段边界，把“救火型压缩”升级为“阶段型切片”。
4. 在 think 前上下文装配阶段：默认取 `last_anchor` 后窗口，必要时工具检索补充历史段。
5. 最终形成“双层机制”：
   - Layer A：tape 阶段化回放（常态）
   - Layer B：artifact compaction（异常/高压态兜底）

---

## 13. 迁移实施步骤（可执行）

1. 先实现 `TapeStore`（JSONL + offset + lock）与单元测试。
2. 接入 runtime：每轮事件写 tape；history 构建改为按 `last_anchor` 切片。
3. 增加内建工具：
   - `tape_handoff`
   - `tape_info`
   - `tape_search`
   - `tape_reset`
4. 与现有 `context_checkpoint` 合并策略对齐（避免双重摘要冲突）。
5. 压测场景：
   - 10k+ entry 会话
   - 命令失败高频回注
   - fork/merge 高频并发
6. 稳定后再考虑 durability 强化（fsync 策略 + merge 元信息持久化）。

---

## 14. 证据索引（核心）

以下为本报告最关键的证据入口（文件与行号）：

- 存储层：`src/bub/tape/store.py`
  - 读取增量与 truncate：`L77-L97`
  - 追加写入：`L129-L143`
  - fork/merge：`L174-L187`
  - 命名与 workspace hash：`L206-L217`
- 服务层：`src/bub/tape/service.py`
  - `fork_tape`：`L62-L70`
  - `reset/archive`：`L103-L113`
  - `search` 与 fuzzy：`L142-L200`
- 路由层：`src/bub/core/router.py`
  - 逗号命令解析与执行入口：`L212-L229`
  - 失败 block 回注：`L98`, `L175-L184`
  - command event 记录：`L334`
- 运行时与模型：
  - `src/bub/core/agent_loop.py`：`route_user -> model_runner`
  - `src/bub/core/model_runner.py`：assistant 侧路由、system prompt 重建
- 命令定义：`src/bub/tools/builtin.py`
  - `tape.handoff/anchors/info/search/reset`
- 测试：
  - `tests/test_tape_store.py`
  - `tests/test_tape_service.py`
  - `tests/test_router.py`

---

## 15. 最终判断

从代码实现看，Bub 的“无限上下文”方案本质是：

`append-only tape（事实记录） + anchor/handoff（阶段切片） + command block reinjection（失败证据回注）`

这条路径相对 prompt hack 更工程化，适合迁移到 elephant.ai，但建议在落地时优先补齐三项：

1. 跨进程并发锁。  
2. 损坏行可观测性。  
3. merge 边界持久化。  

这三项决定方案能否从“好用”升级到“可长期稳定运行”。
