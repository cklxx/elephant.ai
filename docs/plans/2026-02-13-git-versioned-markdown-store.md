# Plan: Git-Versioned Markdown Store + STATE.md Rolling History + kernel_goal Removal

## Context

当前 kernel 存在三个问题：

1. **`kernel_goal` 工具冗余** — `KernelAlignmentContextProvider` 已经在 system prompt 中注入了 GOAL.md 内容（`## Kernel Objective`），agent 每次执行时都能看到。`kernel_goal action=get` 完全多余，`action=set` 可用 `write_file` 替代。
2. **STATE.md 更新机械化** — 每个 cycle 的 runtime block 直接覆盖上一个 cycle 的数据，agent 看不到历史。无 git 版本记录，信息丢失不可追溯。
3. **缺少通用文件版本管理组件** — atomic write + git commit 的模式在 state_file.go、okr/store.go、kernel_goal.go 中重复出现，应抽取为通用组件。

## Solution Overview

1. 新建 `internal/infra/markdown/` 包 — git-versioned markdown store（通用组件）
2. `StateFile` 集成 `VersionedStore`，cycle 边界自动 git commit
3. Runtime block 改为 rolling history（保留最近 N 个 cycle 的摘要表格）
4. 删除 `kernel_goal` 工具，更新 prompt 引用
5. 更新 `config.yaml` prompt（去 kernel_goal 引用，Phase 1 改为读 system prompt）

---

## Batch 1: `internal/infra/markdown/` 通用组件

### 新建 `internal/infra/markdown/git.go`

git CLI 封装，复用 `workspace/manager.go:233-256` 的 `exec.CommandContext` + stderr capture 模式。

```go
type gitOperations struct {
    dir    string
    logger logging.Logger
}

func (g *gitOperations) init(ctx context.Context) error            // git init (idempotent)
func (g *gitOperations) isRepo() bool                              // check .git exists
func (g *gitOperations) add(ctx context.Context, paths ...string) error
func (g *gitOperations) commit(ctx context.Context, msg string) error
func (g *gitOperations) hasChanges(ctx context.Context) (bool, error)  // git status --porcelain
func (g *gitOperations) log(ctx context.Context, file string, n int) ([]CommitEntry, error)
func (g *gitOperations) run(ctx context.Context, args ...string) (string, error)
```

- `init` 用 `git -c user.name=elephant.ai -c user.email=kernel@elephant.ai init`
- `commit` 用 `git -c user.name=... commit -m ...`，避免依赖全局 git config
- `hasChanges` 用 `git status --porcelain`，空输出=clean
- `log` 解析 `git log --format="%H %aI %s" -n N -- file`

### 新建 `internal/infra/markdown/versioned_store.go`

```go
type VersionedStore struct {
    dir        string
    git        *gitOperations
    autoCommit bool          // default true: commit pending changes before each write
    logger     logging.Logger
    mu         sync.Mutex
}

func NewVersionedStore(cfg StoreConfig) *VersionedStore
func (s *VersionedStore) Init(ctx context.Context) error
func (s *VersionedStore) Read(fileName string) (string, error)
func (s *VersionedStore) Write(ctx context.Context, fileName, content string) error
func (s *VersionedStore) Seed(ctx context.Context, fileName, content string) error
func (s *VersionedStore) CommitAll(ctx context.Context, msg string) (bool, error)
func (s *VersionedStore) History(ctx context.Context, fileName string, n int) ([]CommitEntry, error)
```

Write 流程：
1. `autoCommit=true` 且有 uncommitted changes → `CommitAll("auto: pre-write snapshot")`
2. Atomic write: `tmp+rename`
3. `git add <file>`（不立即 commit，等 cycle 边界）

### 新建测试文件

- `internal/infra/markdown/git_test.go` — init idempotency, add+commit, hasChanges, log
- `internal/infra/markdown/versioned_store_test.go` — round-trip, auto-commit, seed, history, concurrent access

---

## Batch 2: StateFile 集成 VersionedStore

### 修改 `internal/app/agent/kernel/state_file.go`

```go
type StateFile struct {
    dir   string
    store *markdown.VersionedStore  // nil = legacy mode
}

func NewStateFile(dir string) *StateFile                                          // 不变
func NewVersionedStateFile(dir string, store *markdown.VersionedStore) *StateFile  // 新增

func (f *StateFile) CommitCycleBoundary(ctx context.Context, msg string) error    // 新增
```

- `readNamed`/`writeNamed`/`seedNamed` 检查 `f.store != nil`，有则委托，无则走原路径
- `CommitCycleBoundary` 在 `store=nil` 时 no-op
- 所有现有 public API (`Read/Write/Seed/ReadInit/WriteInit/...`) 不变
- 现有测试 `NewStateFile(dir)` 继续走原路径，无任何影响

### 更新 `state_file_test.go`

- 新增 `TestStateFile_VersionedWriteCreatesGitHistory`
- 新增 `TestStateFile_CommitCycleBoundary_NoOpWithoutStore`
- 现有测试全部不变

---

## Batch 3: Rolling History in engine.go

### 修改 `internal/app/agent/kernel/config.go`

```go
type KernelConfig struct {
    // ... existing ...
    MaxCycleHistory int `yaml:"max_cycle_history"` // default: 5
}
```

### 修改 `internal/app/agent/kernel/engine.go`

`persistCycleRuntimeState` 改为：

```
1. CommitCycleBoundary(ctx, "pre-cycle {cycleID}") — git 记录 pre-cycle 快照
2. Read STATE.md
3. parseCycleHistory(content) — 从现有 runtime block 解析 history 表格行
4. 构建新 entry，prepend 到 history
5. 截断到 MaxCycleHistory（默认 5）
6. renderKernelRuntimeBlockWithHistory(...) — 渲染带 rolling history 的 block
7. upsertKernelRuntimeBlock + Write
```

新的 runtime block 格式：

```markdown
<!-- KERNEL_RUNTIME:START -->
## kernel_runtime
- updated_at: 2026-02-13T10:00:00Z
- latest_cycle_id: abc123
- latest_status: success
- latest_dispatched: 2
- latest_succeeded: 2
- latest_failed: 0
- latest_failed_agents: (none)
- latest_agent_summary: agent-a[done]: completed | agent-b[done]: done
- latest_duration_ms: 12340
- latest_error: (none)

### cycle_history
| cycle_id | status | dispatched | succeeded | failed | summary | updated_at |
|----------|--------|------------|-----------|--------|---------|------------|
| abc123 | success | 2 | 2 | 0 | agent-a: completed; agent-b: done | 2026-02-13T10:00:00Z |
| def456 | partial_success | 2 | 1 | 1 | agent-a: ok; agent-b: timeout | 2026-02-13T09:30:00Z |
<!-- KERNEL_RUNTIME:END -->
```

新增内部函数：
- `parseCycleHistory(content string) []cycleHistoryEntry` — 解析 `### cycle_history` 表格
- `renderCycleHistoryTable(entries []cycleHistoryEntry) string` — 渲染 markdown 表格
- `renderKernelRuntimeBlockWithHistory(...)` — 合并 latest + history table
- `buildCycleHistoryEntry(result, err, now)` — 从 CycleResult 构建 entry

向后兼容：旧格式 STATE.md（无 `### cycle_history`）→ `parseCycleHistory` 返回空 slice → 第一个新 cycle 创建表格。

### 更新 `engine_test.go`

- 新增 `TestEngine_RunCycle_RollingHistory` — 3 cycles → 3 history rows
- 新增 `TestEngine_RunCycle_RollingHistoryTruncation` — 7 cycles + max=5 → 5 rows
- 现有 `TestEngine_RunCycle_RuntimeSectionUpsertedOnce` 仍通过（一对 markers）

---

## Batch 4: DI Wiring

### 修改 `internal/app/di/builder_hooks.go:222-304`

在 `buildKernelEngine` 中：

```go
// 现有代码 line 237-238:
stateRoot := resolveStorageDir("", kernelagent.DefaultStateRootDir)
// stateFile := kernelagent.NewStateFile(filepath.Join(stateRoot, cfg.KernelID))

// 替换为:
stateDir := filepath.Join(stateRoot, cfg.KernelID)
versionedStore := markdown.NewVersionedStore(markdown.StoreConfig{
    Dir:        stateDir,
    AutoCommit: true,
    Logger:     logging.NewKernelLogger("KernelVersionedStore"),
})
if err := versionedStore.Init(context.Background()); err != nil {
    b.logger.Warn("Kernel versioned store init failed: %v (falling back to unversioned)", err)
}
stateFile := kernelagent.NewVersionedStateFile(stateDir, versionedStore)
```

在 `KernelConfig` 构造中加入 `MaxCycleHistory`:

```go
engine := kernelagent.NewEngine(
    kernelagent.KernelConfig{
        // ... existing ...
        MaxCycleHistory: 5,
    },
    ...
)
```

---

## Batch 5: 删除 kernel_goal 工具

| Action | File | Detail |
|--------|------|--------|
| DELETE | `internal/infra/tools/builtin/session/kernel_goal.go` | 整个文件 |
| DELETE | `internal/infra/tools/builtin/session/kernel_goal_test.go` | 整个文件 |
| EDIT | `internal/app/toolregistry/registry_builtins.go:32` | 删除 `r.static["kernel_goal"] = sessiontools.NewKernelGoal()` |
| EDIT | `internal/app/toolregistry/registry_test.go:66-68` | 计数 16→15，注释去 `kernel_goal` |
| EDIT | `internal/app/toolregistry/registry_test.go:324` | 列表去 `"kernel_goal"` |

**不需改动**：`kernel_alignment_provider.go` 直接读文件，不依赖工具。

---

## Batch 6: 更新 config.yaml Prompt

`~/.alex/config.yaml` 中 capital-explorer prompt 修改：

**Phase 1** 从：
```
Use `kernel_goal` (action=get) to read ~/.alex/kernel/default/GOAL.md.
```
改为：
```
Your system prompt already contains GOAL.md under "## Kernel Objective".
Parse your current priorities and knowledge gaps from there.
Use `read_file` on ~/.alex/kernel/default/GOAL.md only if you need the raw file.
```

**Phase 4** 从：
```
Use `kernel_goal` (action=set) to update GOAL.md
```
改为：
```
Use `write_file` to update ~/.alex/kernel/default/GOAL.md
```

---

## File Summary

| Action | File |
|--------|------|
| CREATE | `internal/infra/markdown/git.go` |
| CREATE | `internal/infra/markdown/git_test.go` |
| CREATE | `internal/infra/markdown/versioned_store.go` |
| CREATE | `internal/infra/markdown/versioned_store_test.go` |
| MODIFY | `internal/app/agent/kernel/state_file.go` |
| MODIFY | `internal/app/agent/kernel/state_file_test.go` |
| MODIFY | `internal/app/agent/kernel/config.go` |
| MODIFY | `internal/app/agent/kernel/engine.go` |
| MODIFY | `internal/app/agent/kernel/engine_test.go` |
| MODIFY | `internal/app/di/builder_hooks.go` |
| MODIFY | `internal/app/toolregistry/registry_builtins.go` |
| MODIFY | `internal/app/toolregistry/registry_test.go` |
| DELETE | `internal/infra/tools/builtin/session/kernel_goal.go` |
| DELETE | `internal/infra/tools/builtin/session/kernel_goal_test.go` |
| MODIFY | `~/.alex/config.yaml` (runtime) |

## Verification

1. `go test ./internal/infra/markdown/...` — 新组件测试
2. `go test ./internal/app/agent/kernel/...` — engine + state_file（含 E2E）
3. `go test ./internal/app/toolregistry/...` — 工具注册（kernel_goal 已删）
4. `go test ./internal/infra/tools/builtin/session/...` — session 工具（kernel_goal 测试已删）
5. `go test ./internal/app/agent/preparation/...` — alignment provider 不受影响
6. `golangci-lint run ./internal/infra/markdown/... ./internal/app/agent/kernel/... ./internal/app/toolregistry/...`
7. `go vet ./...`
8. Manual: 触发一个 kernel cycle，验证 STATE.md 包含 `### cycle_history` 表格 + git log 有 commit 记录
