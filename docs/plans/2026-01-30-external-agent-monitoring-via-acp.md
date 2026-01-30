# External Agent Monitoring — Claude Code & Codex Native Protocol Integration

**Date:** 2026-01-30
**Status:** Draft
**Author:** cklxx

---

## 1. Problem Statement

elephant.ai needs to delegate coding tasks to external agents (Claude Code, OpenAI Codex) and monitor them without consuming their full execution stream. The main agent should only be interrupted when external agents need human input (permission approval, clarification). Otherwise, external agents run autonomously and report final results.

### Current State

The codebase already has:
- `ExternalAgentExecutor` interface (`internal/agent/ports/agent/external_agent.go`) — but no concrete implementations
- `BackgroundTaskManager` (`internal/agent/domain/react/background.go`) — routes to `externalExecutor` for non-"internal" agent types
- `bg_dispatch` tool — already documents `"claude_code"`, `"cursor"` as future agent types (note: `"codex"` not yet listed; add it)
- ACP server (`cmd/alex/acp_server.go`) — elephant.ai can serve as ACP server, but cannot yet act as ACP **client**

### What's Missing

1. Concrete `ExternalAgentExecutor` implementations for Claude Code and Codex
2. Input request routing — when external agents need input, forwarding to the main agent
3. Subprocess lifecycle management with filtered event consumption
4. Configuration for external agent binaries, API keys, and execution policies
5. `ExternalAgentRequest` missing `AgentType` and `WorkingDir` is not set during dispatch (`background.go:148`)
6. Coordinator omits `ExternalExecutor` field when constructing `ReactEngineConfig` (`coordinator.go:361-379`)

---

## 2. Research Summary

### Claude Code SDK Protocol

- **Invocation:** `claude -p --output-format stream-json --verbose -- "<prompt>"`
- **Transport:** NDJSON (newline-delimited JSON) over stdio
- **Message types:** `system` (init), `assistant` (response/tool_use), `user` (tool_result), `result` (completion)
- **Permission:** `--permission-prompt-tool <mcp_tool>` allows external permission handler
- **Multi-turn:** `--resume <session-id>` or `--continue`
- **Streaming:** `--include-partial-messages` enables token-level streaming
- **Key flags:** `--allowedTools`, `--dangerously-skip-permissions`, `--max-turns`, `--max-budget-usd`

Sources:
- [Claude Code CLI Reference](https://code.claude.com/docs/en/cli-reference)
- [Claude Code Headless Mode](https://code.claude.com/docs/en/headless)
- [Zed ACP Adapter](https://github.com/zed-industries/claude-code-acp)

### OpenAI Codex MCP Server Protocol

- **Invocation:** `codex mcp-server` (spawns MCP server over stdio)
- **Transport:** JSON-RPC 2.0 over stdio
- **Tools exposed:** `codex` (new session) and `codex-reply` (continue session)
- **Parameters:** `prompt`, `approval-policy`, `sandbox`, `model`, `cwd`
- **Multi-turn:** Use `threadId` from response to continue via `codex-reply`
- **Notifications:** `notify` hook for `agent-turn-complete` events

Sources:
- [Codex CLI Reference](https://developers.openai.com/codex/cli/reference/)
- [Codex Agents SDK Guide](https://developers.openai.com/codex/guides/agents-sdk/)

### ACP (Agent Client Protocol — Zed)

- **Transport:** JSON-RPC 2.0 over stdio
- **Layers:** Transport → Protocol → Connection → Session → Application
- **Key methods:** `initialize`, `session/new`, `session/prompt`, `session/request_permission`
- **Bidirectional:** Both client and server can initiate requests (e.g., server requests permission from client)

Sources:
- [ACP Explained](https://codestandup.com/posts/2025/agent-client-protocol-acp-explained/)
- [Zed Blog: Claude Code via ACP](https://zed.dev/blog/claude-code-via-acp)

---

## 3. Architecture Design

### 3.1 Core Principle: Interrupt-Only Monitoring

```
Main Agent (ReAct loop)
    │
    ├── bg_dispatch(agent_type="claude_code", prompt="...")
    │       │
    │       ▼
    │   BackgroundTaskManager
    │       │
    │       ▼
    │   ExternalAgentExecutor.Execute()
    │       │
    │       ├── Spawns subprocess ──► [Claude Code / Codex]
    │       │                              │
    │       │   ◄── Filter stream ─────────┘
    │       │       │
    │       │       ├── Input request? ──► Forward to main agent via InputRequestChannel
    │       │       ├── Progress?       ──► Log only (not forwarded)
    │       │       └── Result?         ──► Collect and return
    │       │
    │       ▼
    │   Return ExternalAgentResult
    │
    ├── [Main agent continues other work...]
    │
    ├── [Input request arrives] ──► Inject system message into ReAct loop
    │   "External agent 'claude_code' task X needs input: ..."
    │   Main agent calls ext_reply(task_id, response) tool
    │
    └── [Task completes] ──► bg_collect as usual
```

### 3.2 Component Design

#### Layer 1: Port Interface Extension

**File:** `internal/agent/ports/agent/external_agent.go`

Extend the existing interface to support interactive input.

**Also modify `ExternalAgentRequest`** (currently missing fields):

```go
type ExternalAgentRequest struct {
    Prompt      string
    AgentType   string            // NEW — "claude_code", "codex"; set by registry or BackgroundTaskManager
    WorkingDir  string            // EXISTING but never set — BackgroundTaskManager must populate this
    Config      map[string]string // EXISTING — use for per-request overrides (mode, budget, allowed tools)
    SessionID   string
    CausationID string
    // OnProgress is called by the executor whenever the external agent's state changes
    // (new iteration, tool start/end, token update). BackgroundTaskManager stores the
    // latest snapshot for dashboard queries. Nil-safe — executors check before calling.
    OnProgress  func(ExternalAgentProgress)
}
```

**New types:**

```go
// ExternalAgentExecutor abstracts execution of external code agent processes.
type ExternalAgentExecutor interface {
    Execute(ctx context.Context, req ExternalAgentRequest) (*ExternalAgentResult, error)
    SupportedTypes() []string
}

// --- New additions ---

// InputRequest represents a request from an external agent for user/main-agent input.
type InputRequest struct {
    TaskID      string            // Which background task is asking
    AgentType   string            // "claude_code" or "codex"
    RequestID   string            // Unique ID for this input request
    Type        InputRequestType  // Permission, clarification, etc.
    Summary     string            // Human-readable description
    ToolCall    *InputToolCall    // If permission request, the tool details
    Options     []InputOption     // Available choices
    Deadline    time.Time         // Auto-reject after this time (zero = no deadline)
}

type InputRequestType string
const (
    InputRequestPermission    InputRequestType = "permission"
    InputRequestClarification InputRequestType = "clarification"
)

type InputToolCall struct {
    Name      string
    Arguments map[string]any
    FilePaths []string
}

type InputOption struct {
    ID          string
    Label       string
    Description string
}

// InputResponse is the main agent's reply to an InputRequest.
type InputResponse struct {
    RequestID string
    Approved  bool    // For permission requests
    OptionID  string  // Selected option ID
    Text      string  // Free-form response for clarification
}

// --- Multi-agent collaboration additions ---

// WorkspaceMode defines how an external agent's workspace is isolated.
type WorkspaceMode string
const (
    // WorkspaceModeShared — no isolation; agent works on current working directory.
    // Suitable for read-only tasks or when only one agent writes at a time.
    WorkspaceModeShared WorkspaceMode = "shared"
    // WorkspaceModeBranch — agent works on a dedicated git branch in the same worktree.
    // Provides logical isolation; sequential writes only (no parallel file mutation).
    WorkspaceModeBranch WorkspaceMode = "branch"
    // WorkspaceModeWorktree — agent gets a separate git worktree directory.
    // Full parallel isolation; multiple agents can write simultaneously without conflict.
    WorkspaceModeWorktree WorkspaceMode = "worktree"
)

// WorkspaceAllocation is created by WorkspaceManager when a task is dispatched.
type WorkspaceAllocation struct {
    Mode        WorkspaceMode
    WorkingDir  string   // Actual directory the agent will use
    Branch      string   // Git branch name (empty for shared mode)
    BaseBranch  string   // Branch the worktree/branch was created from
    FileScope   []string // Advisory: files/dirs the agent is expected to modify
}

// MergeStrategy defines how an agent's branch is integrated back.
type MergeStrategy string
const (
    MergeStrategyAuto   MergeStrategy = "auto"   // git merge --no-edit
    MergeStrategySquash MergeStrategy = "squash"  // git merge --squash
    MergeStrategyRebase MergeStrategy = "rebase"  // git rebase onto base branch
    MergeStrategyReview MergeStrategy = "review"  // Generate diff for main agent to review
)

// MergeResult contains the outcome of merging an agent's work back.
type MergeResult struct {
    TaskID       string
    Branch       string
    Strategy     MergeStrategy
    Success      bool
    CommitHash   string   // Merge commit hash (empty on conflict)
    FilesChanged []string // Files modified by the merge
    Conflicts    []string // Conflicting file paths (empty on success)
    DiffSummary  string   // Abbreviated diff stats
}

// TaskDependency defines ordering between tasks.
// Tasks with unresolved dependencies stay in "blocked" state until all deps complete.
type TaskDependency struct {
    DependsOn     []string // Task IDs that must complete successfully before this starts
    InheritContext bool    // If true, inject completed dependency results as context into this task's prompt
}

// --- Progress tracking additions ---

// ExternalAgentProgress is a real-time snapshot of what an external agent is doing.
// Executors report this via the OnProgress callback in ExternalAgentRequest.
type ExternalAgentProgress struct {
    Iteration    int       // Current iteration number
    MaxIter      int       // Max iterations allowed (0 = unlimited)
    TokensUsed   int       // Tokens consumed so far
    CostUSD      float64   // Estimated cost so far
    CurrentTool  string    // Tool currently being executed (e.g., "Bash", "Edit")
    CurrentArgs  string    // Abbreviated tool arguments (≤120 chars)
    FilesTouched []string  // Files read/written so far (deduplicated)
    LastActivity time.Time // Timestamp of most recent stream event
}

// InputRequestSummary is a lightweight view of a pending input request,
// embedded in BackgroundTaskSummary for dashboard rendering.
type InputRequestSummary struct {
    RequestID string
    Type      InputRequestType
    Summary   string    // Human-readable description
    Since     time.Time // When the request was emitted
}

// InteractiveExternalExecutor extends ExternalAgentExecutor with input handling.
//
// Lifecycle contract:
//   1. Construct executor (channel allocated in constructor, buffered size ≥16).
//   2. Caller reads InputRequests() channel before calling Execute().
//   3. Execute() emits InputRequest to the channel; blocks internally on per-request
//      response channel until Reply() is called.
//   4. On context cancellation, executor drains pending requests and closes the channel.
type InteractiveExternalExecutor interface {
    ExternalAgentExecutor
    // InputRequests returns a channel that emits input requests from running agents.
    // The channel is created at construction time and closed when the executor shuts down.
    InputRequests() <-chan InputRequest
    // Reply sends a response to an input request. Thread-safe for concurrent callers.
    Reply(ctx context.Context, resp InputResponse) error
}
```

#### Layer 2: Subprocess Manager

**File:** `internal/external/subprocess.go` (new package)

Generic subprocess lifecycle manager shared by both Claude Code and Codex:

```go
package external

// SubprocessConfig defines how to spawn and manage an external agent subprocess.
// This is a thin lifecycle wrapper — protocol parsing (NDJSON, JSON-RPC) is owned
// by each executor, not by Subprocess itself.
type SubprocessConfig struct {
    Command     string            // Binary path (e.g., "claude", "codex")
    Args        []string          // CLI arguments
    Env         map[string]string // Additional environment variables
    WorkingDir  string
    Timeout     time.Duration     // Max execution time
}

// Subprocess manages the lifecycle of a single external agent process.
// It provides raw I/O pipes; protocol framing is the caller's responsibility.
type Subprocess struct {
    cmd       *exec.Cmd
    stdin     io.WriteCloser
    stdout    io.ReadCloser      // Raw reader — executor wraps with bufio/scanner as needed
    stderr    io.ReadCloser
    done      chan struct{}
    err       error
    pgid      int                // Process group ID for orphan cleanup
}

func (s *Subprocess) Start(ctx context.Context) error
func (s *Subprocess) Write(data []byte) error       // Write to stdin
func (s *Subprocess) Stdout() io.ReadCloser          // Raw stdout pipe for executor-level parsing
func (s *Subprocess) Stderr() io.ReadCloser          // Raw stderr pipe
func (s *Subprocess) Wait() error                    // Block until process exits
func (s *Subprocess) Stop() error                    // Graceful shutdown: SIGTERM → 5s → SIGKILL (entire process group)
func (s *Subprocess) PID() int                       // For orphan tracking
```

#### Layer 3: Claude Code Executor

**File:** `internal/external/claudecode/executor.go`

```go
package claudecode

// Executor implements agent.InteractiveExternalExecutor for Claude Code CLI.
type Executor struct {
    binaryPath   string           // Path to "claude" binary
    apiKey       string           // ANTHROPIC_API_KEY
    model        string           // Optional model override
    allowedTools []string         // Auto-approved tools
    maxBudget    float64          // Max USD spend per task
    maxTurns     int              // Max agentic turns
    inputCh      chan agent.InputRequest
    pending      sync.Map         // requestID → chan InputResponse
    logger       logging.Logger
}

func New(cfg Config) *Executor

func (e *Executor) Execute(ctx context.Context, req agent.ExternalAgentRequest) (*agent.ExternalAgentResult, error)
func (e *Executor) SupportedTypes() []string  // returns ["claude_code"]
func (e *Executor) InputRequests() <-chan agent.InputRequest
func (e *Executor) Reply(ctx context.Context, resp agent.InputResponse) error
```

**Execution flow:**

1. Build command: `claude -p --output-format stream-json --verbose --permission-prompt-tool <mcp_tool> -- "<prompt>"`
   - OR: Use `--dangerously-skip-permissions` + `--allowedTools` for pre-approved tools
2. Spawn subprocess via `external.Subprocess`
3. Background goroutine reads NDJSON lines:
   - `type: "system"` → Log session init, extract `session_id`
   - `type: "assistant"` with `tool_use` → Log tool invocation (not forwarded to main agent)
   - `type: "user"` with `tool_result` → Log tool result (not forwarded)
   - Permission request message → Convert to `InputRequest`, emit on `inputCh`, block on response channel
   - `type: "result"` → Extract answer, usage, cost; return as `ExternalAgentResult`
4. On context cancellation → `Subprocess.Stop()`

**Permission handling approach:**

Two strategies, configurable per task:

**Strategy A: Pre-approved tools (autonomous mode)**
```bash
claude -p --dangerously-skip-permissions --allowedTools "Read,Edit,Bash(git *)" -- "<prompt>"
```
No input requests needed. Fully autonomous. Best for well-scoped tasks.

**Strategy B: Permission prompt tool (interactive mode)**
```bash
claude -p --output-format stream-json \
  --mcp-config /tmp/elephant-mcp-<task-id>.json \
  --permission-prompt-tool mcp__elephant__approve \
  -- "<prompt>"
```

**Permission MCP server implementation (key detail):**

elephant.ai spawns a co-process MCP server alongside each Claude Code subprocess:

1. **Config generation:** Write a temporary MCP config file per task:
   ```json
   {
     "mcpServers": {
       "elephant": {
         "command": "<elephant-binary>",
         "args": ["mcp-permission-server", "--task-id", "<task-id>", "--sock", "/tmp/elephant-perm-<task-id>.sock"],
         "type": "stdio"
       }
     }
   }
   ```

2. **MCP server binary:** A minimal MCP stdio server (embedded in elephant.ai binary as a subcommand) that:
   - Implements `tools/list` → exposes `approve` tool with `tool_name`, `arguments`, `file_paths` parameters
   - Implements `tools/call` for `approve` → writes `InputRequest` to a Unix domain socket, blocks reading `InputResponse`
   - The executor goroutine listens on the same socket, relays to/from `inputCh`/`pending` map

3. **Lifecycle:** Temp config file and socket are cleaned up when the Claude Code subprocess exits.

4. **Auto-approve filter:** Read-only tools (`Read`, `Grep`, `Glob`, `WebSearch`) are auto-approved within the MCP server without forwarding to the main agent. This is **configurable via `autonomous_allowed_tools`** in the YAML config and avoids flooding the main agent with noise.

Recommended: **Strategy B** as default, with Strategy A configurable per agent_type config.

#### Layer 4: Codex Executor

**File:** `internal/external/codex/executor.go`

```go
package codex

// Executor implements agent.InteractiveExternalExecutor for OpenAI Codex CLI.
type Executor struct {
    binaryPath     string
    apiKey         string           // OPENAI_API_KEY
    model          string           // e.g., "o3", "o4-mini"
    approvalPolicy string           // "untrusted", "on-request", "on-failure", "never"
    sandbox        string           // "read-only", "workspace-write", "danger-full-access"
    inputCh        chan agent.InputRequest
    pending        sync.Map
    logger         logging.Logger
}
```

**Execution flow:**

1. Spawn `codex mcp-server` subprocess
2. Send JSON-RPC `initialize` request
3. Send `tools/call` with tool `codex`:
   ```json
   {
     "jsonrpc": "2.0",
     "method": "tools/call",
     "params": {
       "name": "codex",
       "arguments": {
         "prompt": "<task prompt>",
         "approval-policy": "on-request",
         "sandbox": "workspace-write",
         "cwd": "/path/to/project"
       }
     }
   }
   ```
4. Read response with `threadId` and result content
5. If multi-turn needed, use `codex-reply` with `threadId`
6. On completion, extract result and terminate MCP server

**Note:** Codex MCP server mode is simpler than Claude Code — it's inherently request/response with the MCP protocol handling the lifecycle. Permission prompts are handled by `approval-policy` flag rather than interactive callbacks.

#### Layer 5: Input Request Injection into ReAct Loop

**File:** `internal/agent/domain/react/runtime_external_input.go` (new)

Extend the existing `injectBackgroundNotifications()` pattern:

```go
// injectExternalInputRequests checks for pending input requests from external
// agents and injects them as system messages into the ReAct loop.
func (r *reactRuntime) injectExternalInputRequests() {
    if r.externalInputCh == nil {
        return
    }

    for {
        select {
        case req := <-r.externalInputCh:
            msg := formatInputRequestMessage(req)
            r.state.Messages = append(r.state.Messages, ports.Message{
                Role:    "user",
                Content: msg,
                Source:  ports.MessageSourceSystemPrompt,
            })
            r.engine.emitEvent(&domain.ExternalInputRequestEvent{
                BaseEvent: r.engine.newBaseEvent(r.ctx, r.state.SessionID, r.state.RunID, r.state.ParentRunID),
                TaskID:    req.TaskID,
                AgentType: req.AgentType,
                RequestID: req.RequestID,
                Type:      string(req.Type),
                Summary:   req.Summary,
            })
        default:
            return
        }
    }
}
```

**Injected message format:**
```
[External Agent Input Required] task_id="fix-auth" agent_type="claude_code"
Request: Permission to execute: Bash(rm -rf node_modules && npm install)
Files: /project/package.json
Options: [allow] [reject]
Use ext_reply(task_id="fix-auth", request_id="req-001", approved=true) to respond.
```

#### Layer 6: New Tool — `ext_reply`

**File:** `internal/tools/builtin/orchestration/ext_reply.go` (new)

```go
type extReply struct{}

func (t *extReply) Definition() ports.ToolDefinition {
    return ports.ToolDefinition{
        Name: "ext_reply",
        Description: `Reply to an input request from an external agent (Claude Code, Codex, etc.).
Use this when an external background task requests permission or clarification.`,
        Parameters: ports.ParameterSchema{
            Type: "object",
            Properties: map[string]ports.Property{
                "task_id":    {Type: "string", Description: "The background task ID."},
                "request_id": {Type: "string", Description: "The input request ID from the notification."},
                "approved":   {Type: "boolean", Description: "Whether to approve (for permission requests)."},
                "option_id":  {Type: "string", Description: "Selected option ID (if applicable)."},
                "message":    {Type: "string", Description: "Free-form response text (for clarification requests)."},
            },
            Required: []string{"task_id", "request_id"},
        },
    }
}
```

#### Layer 7: Progress Tracking & Task Dashboard

**Core idea:** External agents run autonomously, but the main agent can proactively query a kanban-like dashboard of all tasks at any time via `bg_status`. Task completion still triggers `injectBackgroundNotifications()` as before.

**7.1 Progress capture in BackgroundTaskManager**

**File:** `internal/agent/domain/react/background.go` (modified)

```go
type backgroundTask struct {
    // ... existing fields ...

    // Progress tracking for external agents.
    progress     *agent.ExternalAgentProgress   // Latest snapshot (nil for internal tasks)
    pendingInput *agent.InputRequestSummary     // Non-nil when blocked on input
}
```

When `BackgroundTaskManager.runTask()` calls `externalExecutor.Execute()`, it passes the `OnProgress` callback:

```go
extResult, err = m.externalExecutor.Execute(ctx, agent.ExternalAgentRequest{
    Prompt:      bt.prompt,
    AgentType:   agentType,
    WorkingDir:  m.workingDir,
    SessionID:   m.sessionID,
    CausationID: bt.causationID,
    OnProgress: func(p agent.ExternalAgentProgress) {
        bt.mu.Lock()
        bt.progress = &p
        bt.mu.Unlock()
    },
})
```

When the executor emits an `InputRequest`, the manager also sets `bt.pendingInput`. When `Reply()` resolves the request, it clears `bt.pendingInput`.

**7.2 Enhanced BackgroundTaskSummary**

**File:** `internal/agent/ports/agent/background.go` (modified)

```go
type BackgroundTaskSummary struct {
    // ... existing fields ...

    // NEW — progress and input state for dashboard rendering.
    Progress     *ExternalAgentProgress   // Non-nil for running external tasks
    PendingInput *InputRequestSummary     // Non-nil when task is waiting for input
    Elapsed      time.Duration            // Wall-clock time since task started
}
```

**7.3 Enhanced `bg_status` tool — kanban rendering**

**File:** `internal/tools/builtin/orchestration/bg_status.go` (modified)

The existing `bg_status` tool output is enhanced to render a grouped dashboard view when external tasks exist:

```
═══ Task Dashboard (5 tasks) ═══

▶ RUNNING (2)
  ┌─ fix-auth [claude_code] 3m42s
  │  ↳ Iteration 12/50 · 8,240 tokens · $0.42
  │  ↳ Current: Bash(npm test -- --coverage)
  │  ↳ Files: src/auth/login.ts (+2 more)
  └─
  ┌─ refactor-db [codex] 1m15s
  │  ↳ Iteration 5 · 3,100 tokens
  │  ↳ Current: Writing migration file
  └─

⏳ WAITING FOR INPUT (1)
  ┌─ deploy-staging [claude_code] 5m10s
  │  ↳ Iteration 18/50 · 12,400 tokens · $0.68
  │  ↳ ⚠ Permission: Bash(kubectl apply -f deploy.yaml)
  │  ↳ request_id="req-003" → use ext_reply() to respond
  └─

✅ COMPLETED (1)
  ┌─ lint-fix [internal] 0m45s · 2,100 tokens
  └─

❌ FAILED (1)
  ┌─ gen-docs [codex] 2m30s
  │  ↳ Error: sandbox timeout after 120s
  └─
```

**Rendering rules:**
- Group by effective state: `RUNNING`, `WAITING FOR INPUT`, `COMPLETED`, `FAILED`, `CANCELLED`, `PENDING`
- "WAITING FOR INPUT" = status is `running` AND `PendingInput != nil`
- Within each group, sort by `StartedAt` ascending (oldest first)
- For running tasks: show iteration, tokens, cost, current tool, top 3 files touched
- For waiting tasks: show the input request summary and `ext_reply()` hint
- For completed/failed: show duration, token count, error if any
- File list truncated: show first 3, then `(+N more)`

**7.4 Domain event for progress**

**File:** `internal/agent/domain/events_external.go` (new — already planned)

```go
// ExternalAgentProgressEvent is emitted periodically for TUI/Web dashboard rendering.
// Not injected into LLM context — only consumed by delivery layer (SSE, TUI).
type ExternalAgentProgressEvent struct {
    BaseEvent
    TaskID      string
    AgentType   string
    Iteration   int
    MaxIter     int
    TokensUsed  int
    CostUSD     float64
    CurrentTool string
    Elapsed     time.Duration
}
```

This event is emitted by `BackgroundTaskManager` when the progress callback fires (throttled to at most once per 2 seconds per task to avoid event spam).

#### Layer 8: Multi-Agent Collaboration

**Core problem:** When multiple external agents work concurrently, they can conflict on files, git state, or shared resources. The collaboration layer provides workspace isolation, task dependencies, file scope management, and result merging.

**8.1 Workspace Manager**

**File:** `internal/external/workspace/manager.go` (new)

```go
package workspace

// Manager handles workspace allocation and cleanup for external agent tasks.
// It creates git worktrees/branches for isolated execution and merges results back.
type Manager struct {
    projectDir  string          // Root project directory
    worktreeDir string          // Parent dir for worktrees: <projectDir>/.elephant/worktrees/
    logger      logging.Logger
    mu          sync.Mutex      // Serialize git operations
}

func NewManager(projectDir string, logger logging.Logger) *Manager

// Allocate creates an isolated workspace for a task based on the requested mode.
//
//   shared:   returns projectDir as-is
//   branch:   git checkout -b elephant/<task-id> from current HEAD
//   worktree: git worktree add .elephant/worktrees/<task-id> -b elephant/<task-id>
//
func (m *Manager) Allocate(ctx context.Context, taskID string, mode agent.WorkspaceMode, fileScope []string) (*agent.WorkspaceAllocation, error)

// Merge integrates an agent's branch back into the base branch.
func (m *Manager) Merge(ctx context.Context, alloc *agent.WorkspaceAllocation, strategy agent.MergeStrategy) (*agent.MergeResult, error)

// Cleanup removes a worktree and optionally deletes the branch.
func (m *Manager) Cleanup(ctx context.Context, alloc *agent.WorkspaceAllocation, deleteBranch bool) error

// ValidateFileScope checks if a task's actual file changes match its declared scope.
// Returns files that were modified outside the declared scope.
func (m *Manager) ValidateFileScope(ctx context.Context, alloc *agent.WorkspaceAllocation) (outOfScope []string, err error)
```

**Branch naming convention:** `elephant/<task-id>` (e.g., `elephant/fix-auth`, `elephant/refactor-db`)

**Worktree layout:**
```
project/
├── .elephant/
│   └── worktrees/
│       ├── fix-auth/        ← full worktree for claude_code task
│       │   ├── src/
│       │   └── ...
│       └── refactor-db/     ← full worktree for codex task
│           ├── src/
│           └── ...
├── src/                     ← main worktree (shared mode tasks use this)
└── ...
```

**8.2 Task Dependency Graph**

**File:** `internal/agent/domain/react/background.go` (modified)

```go
type backgroundTask struct {
    // ... existing fields ...

    // Collaboration state.
    dependsOn    []string                      // Task IDs this task waits for
    workspace    *agent.WorkspaceAllocation     // Isolation allocation (nil for internal)
    fileScope    []string                       // Advisory file scope declaration
}
```

**Dependency resolution in `Dispatch()`:**

```go
func (m *BackgroundTaskManager) Dispatch(ctx context.Context, req DispatchRequest) error {
    // ... existing validation ...

    if len(req.DependsOn) > 0 {
        // Validate: all dependency task IDs exist
        // Validate: no cycles (topological sort check)
        // Set status to "blocked" instead of "pending"
        bt.status = agent.BackgroundTaskStatusBlocked
        bt.dependsOn = req.DependsOn
    }

    // ... store task, launch goroutine ...
}

// runTask now checks dependencies before executing.
func (m *BackgroundTaskManager) runTask(ctx context.Context, bt *backgroundTask, agentType string) {
    // Wait for dependencies to complete.
    if len(bt.dependsOn) > 0 {
        if err := m.awaitDependencies(ctx, bt); err != nil {
            // Dependency failed or cancelled → fail this task too
            bt.mu.Lock()
            bt.status = agent.BackgroundTaskStatusFailed
            bt.err = fmt.Errorf("dependency failed: %w", err)
            bt.mu.Unlock()
            return
        }
        bt.mu.Lock()
        bt.status = agent.BackgroundTaskStatusRunning
        bt.mu.Unlock()
    }

    // If inherit_context, prepend dependency results to prompt.
    prompt := bt.prompt
    if bt.inheritContext {
        prompt = m.buildContextEnrichedPrompt(bt)
    }

    // ... existing execution logic ...
}
```

**New status value:**

```go
const BackgroundTaskStatusBlocked BackgroundTaskStatus = "blocked" // Waiting for dependencies
```

**8.3 Enhanced `bg_dispatch` parameters**

**File:** `internal/tools/builtin/orchestration/bg_dispatch.go` (modified)

```go
"depends_on": {
    Type:        "array",
    Description: "Task IDs that must complete successfully before this task starts. Creates a dependency edge.",
    Items:       &ports.Property{Type: "string"},
},
"workspace_mode": {
    Type:        "string",
    Description: `Workspace isolation mode:
- "shared" (default): Agent works on current directory. Best for read-only or single-writer tasks.
- "branch": Agent works on a dedicated git branch. Sequential writes only.
- "worktree": Agent gets a separate git worktree. Full parallel isolation.`,
},
"file_scope": {
    Type:        "array",
    Description: "Advisory: files/directories this task is expected to modify (e.g., [\"src/auth/\", \"tests/auth/\"]). Used for conflict detection and scope validation.",
    Items:       &ports.Property{Type: "string"},
},
"inherit_context": {
    Type:        "boolean",
    Description: "When true, completed dependency results are prepended to this task's prompt as context.",
},
```

**8.4 New tool — `ext_merge`**

**File:** `internal/tools/builtin/orchestration/ext_merge.go` (new)

```go
type extMerge struct{}

func (t *extMerge) Definition() ports.ToolDefinition {
    return ports.ToolDefinition{
        Name: "ext_merge",
        Description: `Merge an external agent's work branch back into the base branch.
Use after an external task completes to integrate its changes.
Returns merge result with diff summary, or conflict details if merge fails.`,
        Parameters: ports.ParameterSchema{
            Type: "object",
            Properties: map[string]ports.Property{
                "task_id":  {Type: "string", Description: "The completed background task ID."},
                "strategy": {Type: "string", Description: `Merge strategy: "auto" (default), "squash", "rebase", or "review" (returns diff without merging).`},
            },
            Required: []string{"task_id"},
        },
    }
}
```

**`ext_merge` output format:**

```
✅ Merge successful: elephant/fix-auth → main
Strategy: squash
Commit: a1b2c3d
Files changed (4):
  M src/auth/login.ts
  M src/auth/session.ts
  A src/auth/__tests__/login.test.ts
  A src/auth/__tests__/session.test.ts

Diff stats: +142 -38 across 4 files
```

On conflict:
```
❌ Merge conflict: elephant/refactor-db → main
Conflicting files (2):
  C src/db/connection.ts
  C src/db/migrations/002_add_index.ts

Non-conflicting changes (3 files) staged.
Use Bash("git diff --merge") to inspect conflicts, then resolve manually.
```

**8.5 Conflict detection at dispatch time**

When `bg_dispatch` specifies `file_scope`, the manager checks for overlap:

```go
// CheckScopeOverlap detects if a new task's file scope overlaps with any running task.
func (m *Manager) CheckScopeOverlap(newScope []string, runningTasks []backgroundTask) []ScopeConflict

type ScopeConflict struct {
    TaskID       string   // Running task that conflicts
    OverlapPaths []string // Paths that overlap
}
```

If overlap is detected, `bg_dispatch` returns a warning (not an error):

```
⚠ Scope overlap detected:
  Task "fix-auth" (running) also modifies: src/auth/login.ts
  Consider using workspace_mode="worktree" for full isolation.

Task "add-tests" dispatched with workspace_mode="branch".
```

**8.6 Cross-agent context sharing**

When `inherit_context=true`, a task's prompt is enriched with dependency results:

```
[Collaboration Context]
This task depends on completed tasks whose results are provided below.

--- Task "fix-auth" (claude_code) — COMPLETED ---
Result summary: Fixed authentication bug in login.ts by adding token refresh logic.
Modified files: src/auth/login.ts, src/auth/session.ts
Branch: elephant/fix-auth (merged)
---

[Your Task]
<original prompt>
```

**8.7 Dashboard enhancement for collaboration**

The kanban view (Layer 7) is extended with workspace and dependency info:

```
═══ Task Dashboard (4 tasks) ═══

▶ RUNNING (2)
  ┌─ fix-auth [claude_code] 3m42s
  │  ↳ Iteration 12/50 · 8,240 tokens · $0.42
  │  ↳ Workspace: worktree (elephant/fix-auth)
  │  ↳ Scope: src/auth/ (+1 dir)
  │  ↳ Current: Bash(npm test -- --coverage)
  └─
  ┌─ refactor-db [codex] 1m15s
  │  ↳ Workspace: worktree (elephant/refactor-db)
  │  ↳ Scope: src/db/
  │  ↳ Current: Writing migration file
  └─

⏸ BLOCKED (1)
  ┌─ integration-test [claude_code]
  │  ↳ Depends on: fix-auth ▶, refactor-db ▶
  │  ↳ Workspace: branch (pending allocation)
  └─

✅ COMPLETED (1)
  ┌─ lint-fix [internal] 0m45s · 2,100 tokens
  │  ↳ Workspace: shared
  └─
```

Status indicators in dependency lines: `▶` running, `⏳` waiting, `✅` done, `❌` failed, `⏸` blocked.

#### Layer 9: Configuration

**File:** `configs/external_agents.yaml` (new)

```yaml
external_agents:
  claude_code:
    enabled: true
    binary: "claude"                    # Binary name or path
    default_model: ""                   # Empty = use Claude Code's default
    default_mode: "interactive"         # "autonomous" or "interactive"
    autonomous_allowed_tools:           # Tools auto-approved in autonomous mode
      - "Read"
      - "Edit"
      - "Glob"
      - "Grep"
      - "Bash(git *)"
    max_budget_usd: 5.00               # Per-task budget limit
    max_turns: 50                       # Max agentic turns per task
    timeout: 30m                        # Max wall-clock time
    env:
      ANTHROPIC_API_KEY: "${ANTHROPIC_API_KEY}"

  codex:
    enabled: true
    binary: "codex"
    default_model: "o3"
    approval_policy: "on-request"       # "untrusted", "on-request", "on-failure", "never"
    sandbox: "workspace-write"          # "read-only", "workspace-write", "danger-full-access"
    timeout: 30m
    env:
      OPENAI_API_KEY: "${OPENAI_API_KEY}"
```

### 3.3 Data Flow — Complete Sequence

```
1. Main Agent ReAct iteration:
   LLM decides: "I'll delegate the auth fix to Claude Code"
   → Calls bg_dispatch(task_id="fix-auth", agent_type="claude_code", prompt="Fix the auth bug in...")

2. BackgroundTaskManager.Dispatch():
   → agentType="claude_code" → routes to externalExecutor
   → ClaudeCodeExecutor.Execute() called in background goroutine
   → OnProgress callback wired to store snapshots on backgroundTask

3. ClaudeCodeExecutor spawns subprocess:
   $ claude -p --output-format stream-json --permission-prompt-tool ... -- "Fix the auth bug in..."

4. Claude Code runs autonomously:
   - Reads files, analyzes code, writes patches
   - All stream output logged but NOT forwarded to main agent
   - Each tool_use / tool_result → OnProgress(iteration, tokens, currentTool, files)
   - BackgroundTaskManager stores latest snapshot → bg_status can render dashboard

5. [At any point] Main Agent can call bg_status() to see dashboard:
   → Returns kanban view: RUNNING tasks with progress, WAITING tasks with input hint
   → Main agent understands overall execution state at a glance

6. Claude Code hits permission wall:
   - Wants to run: Bash("npm test")
   - Emits permission request in stream

7. ClaudeCodeExecutor detects permission request:
   → Creates InputRequest{TaskID: "fix-auth", Type: Permission, ...}
   → Sets bt.pendingInput (visible in dashboard as WAITING FOR INPUT)
   → Sends to inputCh
   → Blocks waiting for InputResponse

8. Main Agent ReAct next iteration:
   → injectExternalInputRequests() picks up the request
   → Injects system message: "[External Agent Input Required] ..."

9. Main Agent LLM thinks:
   "Claude Code wants to run npm test — that's safe, I'll approve."
   → Calls ext_reply(task_id="fix-auth", request_id="req-001", approved=true)

10. ext_reply tool:
    → Routes response to ClaudeCodeExecutor.Reply()
    → Clears bt.pendingInput (dashboard shows RUNNING again)
    → Unblocks the pending channel
    → Claude Code subprocess receives approval, continues

11. Claude Code finishes:
    → Emits result message
    → ClaudeCodeExecutor returns ExternalAgentResult

12. BackgroundTaskManager signals completion:
    → injectBackgroundNotifications() as usual
    → Dashboard shows task under COMPLETED with final stats
    → Main agent calls bg_collect("fix-auth")
```

### 3.4 Multi-Agent Collaboration Example

```
Main Agent receives: "Fix the auth bug and add comprehensive tests, then update the docs."

1. Main Agent plans and dispatches parallel + sequential tasks:

   bg_dispatch(task_id="fix-auth", agent_type="claude_code",
               prompt="Fix the auth token refresh bug in src/auth/...",
               workspace_mode="worktree", file_scope=["src/auth/"])

   bg_dispatch(task_id="add-tests", agent_type="codex",
               prompt="Add comprehensive tests for the auth module...",
               workspace_mode="worktree", file_scope=["tests/auth/"],
               depends_on=["fix-auth"], inherit_context=true)

   bg_dispatch(task_id="update-docs", agent_type="claude_code",
               prompt="Update API documentation for auth changes...",
               workspace_mode="branch", file_scope=["docs/"],
               depends_on=["fix-auth", "add-tests"], inherit_context=true)

2. Workspace Manager allocates:
   fix-auth   → .elephant/worktrees/fix-auth/   (branch: elephant/fix-auth)
   add-tests  → blocked (waiting for fix-auth)
   update-docs→ blocked (waiting for fix-auth + add-tests)

3. bg_status() shows:
   ▶ RUNNING: fix-auth [claude_code] · worktree · Iteration 8/50
   ⏸ BLOCKED: add-tests [codex] · depends on: fix-auth ▶
   ⏸ BLOCKED: update-docs [claude_code] · depends on: fix-auth ▶, add-tests ⏸

4. fix-auth completes → add-tests unblocks automatically.
   Prompt enriched with: "[Context from fix-auth] Fixed token refresh in login.ts..."
   Workspace allocated: .elephant/worktrees/add-tests/ (branch: elephant/add-tests)

5. add-tests completes → update-docs unblocks.
   Prompt enriched with both results. Branch: elephant/update-docs.

6. All three complete. Main agent merges:
   ext_merge(task_id="fix-auth", strategy="squash")    → ✅ merged
   ext_merge(task_id="add-tests", strategy="squash")   → ✅ merged
   ext_merge(task_id="update-docs", strategy="squash") → ✅ merged

7. Main agent runs final validation: bg_dispatch(task_id="validate", prompt="Run full test suite")
```

### 3.5 Dashboard Interaction Patterns

The main agent uses the dashboard in three modes:

**Mode 1: Passive (default)**
- Completions arrive via `injectBackgroundNotifications()` — no action needed
- Input requests arrive via `injectExternalInputRequests()` — respond with `ext_reply()`

**Mode 2: Proactive check**
- Main agent calls `bg_status()` between other work to see the kanban
- Useful when juggling multiple delegated tasks
- Can decide to wait (`bg_collect(wait=true)`) or continue other work

**Mode 3: Monitoring loop**
- For long-running tasks, the main agent may periodically call `bg_status()` to check progress
- Dashboard shows iteration count, token usage, and estimated cost — main agent can decide to cancel if budget is trending too high

---

## 4. File Changes Summary

### New Files

| File | Purpose |
|------|---------|
| `internal/external/subprocess.go` | Generic subprocess lifecycle manager |
| `internal/external/subprocess_test.go` | Tests |
| `internal/external/claudecode/executor.go` | Claude Code SDK executor |
| `internal/external/claudecode/executor_test.go` | Tests |
| `internal/external/claudecode/messages.go` | Stream-JSON message parsing |
| `internal/external/claudecode/messages_test.go` | Tests |
| `internal/external/claudecode/permission.go` | Permission MCP server: auto-approve filter + Unix socket relay |
| `cmd/alex/mcp_permission_server.go` | Subcommand entry point for permission MCP stdio server |
| `internal/external/codex/executor.go` | Codex MCP server executor |
| `internal/external/codex/executor_test.go` | Tests |
| `internal/external/codex/jsonrpc.go` | JSON-RPC 2.0 client |
| `internal/external/registry.go` | Multi-executor registry |
| `internal/external/config.go` | Configuration types |
| `internal/external/mock/executor.go` | Mock executor for Phase 1 integration tests |
| `internal/external/workspace/manager.go` | Git worktree/branch allocation and merge |
| `internal/external/workspace/manager_test.go` | Tests |
| `internal/tools/builtin/orchestration/ext_reply.go` | ext_reply tool |
| `internal/tools/builtin/orchestration/ext_merge.go` | ext_merge tool for branch/worktree merge |
| `internal/tools/builtin/orchestration/ext_merge_test.go` | Tests |
| `internal/tools/builtin/orchestration/ext_reply_test.go` | Tests |
| `internal/agent/domain/react/runtime_external_input.go` | Input request injection |
| `internal/agent/domain/events_external.go` | New event types |

### Modified Files

| File | Change |
|------|--------|
| `internal/agent/ports/agent/external_agent.go` | Add `AgentType`, `OnProgress` to `ExternalAgentRequest`; add `InputRequest`, `InputResponse`, `InteractiveExternalExecutor`, `WorkspaceMode`, `WorkspaceAllocation`, `MergeStrategy`, `MergeResult`, `TaskDependency` types |
| `internal/agent/ports/agent/background.go` | Add `Progress`, `PendingInput`, `Elapsed` fields to `BackgroundTaskSummary` |
| `internal/agent/domain/react/runtime.go` | Add `externalInputCh` field, call `injectExternalInputRequests()` in loop |
| `internal/agent/domain/react/background.go` | Wire up `InteractiveExternalExecutor` input channel; set `WorkingDir` and `OnProgress` on `ExternalAgentRequest`; store progress/pendingInput on `backgroundTask`; add dependency graph (depends_on, blocked status, awaitDependencies); add workspace allocation field; fix partial-result-with-error data loss |
| `internal/agent/app/coordinator/coordinator.go` | Instantiate external executor registry from config; wire `ExternalExecutor` into `ReactEngineConfig` (currently omitted at line 361-379) |
| `internal/tools/builtin/orchestration/bg_dispatch.go` | Update description to list supported agent types (`"claude_code"`, `"codex"`); add `config`, `depends_on`, `workspace_mode`, `file_scope`, `inherit_context` parameters |
| `internal/tools/builtin/orchestration/bg_status.go` | Kanban-style dashboard rendering with progress, input-waiting state, grouped by effective status |
| `configs/` | Add external agent configuration |

---

## 5. Implementation Phases

### Phase 1: Foundation (Port + Subprocess + Dashboard + Mock Executor)

**1a. Port types & wiring**
1. Extend `ExternalAgentExecutor` port with `InputRequest`/`InputResponse`/`InteractiveExternalExecutor` types
2. Add `ExternalAgentProgress`, `InputRequestSummary` types
3. Add `AgentType`, `OnProgress` to `ExternalAgentRequest`; ensure `WorkingDir` is set in `BackgroundTaskManager.runTask()`
4. Add `Progress`, `PendingInput`, `Elapsed` fields to `BackgroundTaskSummary`
5. Wire `ExternalExecutor` in `coordinator.go` `ReactEngineConfig` construction

**1b. Infrastructure**
6. Build generic `Subprocess` lifecycle manager (no protocol awareness — raw I/O pipes)
7. Build `ext_reply` tool
8. Wire input request channel into ReAct loop's iteration cycle (`runtime_external_input.go`)
9. Add new domain event types (`ExternalInputRequestEvent`, `ExternalInputResponseEvent`, `ExternalAgentProgressEvent`)

**1c. Dashboard**
10. Enhance `bg_status` tool: kanban-style grouped rendering (RUNNING / WAITING FOR INPUT / COMPLETED / FAILED / CANCELLED / PENDING) with progress detail
11. Store progress/pendingInput on `backgroundTask` struct; `OnProgress` callback writes latest snapshot; throttled event emission (≤1 per 2s per task)

**1d. Testability**
12. Build **mock executor** (`internal/external/mock/executor.go`) that emits scripted progress updates, input requests, and canned results — enables Phase 1 integration tests
13. Fix pre-existing issue: `BackgroundTaskManager.runTask()` loses partial results when `extResult.Error` is non-empty (set status to `completed_with_errors` or preserve answer alongside error)

### Phase 2: Claude Code Executor
1. Implement NDJSON stream-json message parser (executor-level, wraps `Subprocess.Stdout()` with `bufio.Scanner`)
2. Implement `ClaudeCodeExecutor` with subprocess lifecycle
3. Implement permission MCP server subcommand (`cmd/alex/mcp_permission_server.go`) + Unix socket relay
4. Implement auto-approve filter for read-only tools within MCP server
5. Integration tests with mock subprocess (scripted NDJSON sequences)

### Phase 3: Codex Executor
1. Implement JSON-RPC 2.0 framing client (executor-level, wraps `Subprocess.Stdout()` with Content-Length parser)
2. Implement `CodexExecutor` with MCP server lifecycle (one `codex mcp-server` per task)
3. Map Codex `approval-policy` to input request routing
4. Integration tests with mock MCP server (scripted JSON-RPC sequences)

### Phase 4: Configuration + Registry
1. Load external agent config from YAML
2. Build executor registry that routes agent types to executors
3. Wire into coordinator's dependency injection
4. End-to-end integration test

### Phase 5: Observability + Safety
1. Structured logging for all external agent events (subprocess lifecycle, stream filtering, input routing)
2. OpenTelemetry spans: `alex.external.execute`, `alex.external.input_request`, `alex.external.reply`
3. Cost tracking: aggregate external agent token usage into session cost
4. Timeout and budget enforcement
5. Graceful shutdown: ensure subprocesses are terminated on main agent cancellation (SIGTERM entire process group)
6. Orphan process protection: write subprocess PIDs to `<data_dir>/external-pids.json`; on startup, kill stale processes; use process groups (`Setpgid`) so children die with parent

### Phase 6: Multi-Agent Collaboration

**6a. Workspace isolation**
1. Implement `WorkspaceManager` with git worktree/branch allocation and cleanup
2. Wire workspace allocation into `BackgroundTaskManager.Dispatch()` — allocate before goroutine launch
3. Pass `WorkspaceAllocation.WorkingDir` as `ExternalAgentRequest.WorkingDir`
4. Implement `ext_merge` tool with auto/squash/rebase/review strategies
5. Implement scope validation: after task completes, check modified files against `file_scope`

**6b. Task dependencies**
6. Add `BackgroundTaskStatusBlocked` status
7. Implement dependency graph validation in `Dispatch()` (existence check + cycle detection via topological sort)
8. Implement `awaitDependencies()` — poll-wait for all dependency tasks to reach `completed` status; fail on dependency failure/cancellation
9. Implement `buildContextEnrichedPrompt()` — prepend completed dependency results when `inherit_context=true`

**6c. Conflict detection**
10. Implement `CheckScopeOverlap()` in workspace manager — advisory overlap warning at dispatch time
11. Add overlap warning to `bg_dispatch` tool output

**6d. Enhanced dashboard**
12. Extend kanban renderer with workspace info (mode, branch), file scope, dependency lines with status indicators

**6e. Testing**
13. Unit tests: workspace manager (allocate/merge/cleanup), dependency graph (cycle detection, ordering)
14. Integration tests: fan-out/fan-in pattern (dispatch N → await all → merge all), pipeline pattern (A → B → C with inherit_context), conflict detection + worktree isolation
15. Git integration tests: real git operations (worktree create, branch, merge, conflict scenarios)

---

## 6. Design Decisions & Trade-offs

### D1: Why not use ACP (Agent Client Protocol) directly?

**Decision:** Use native protocols (stream-json for Claude Code, MCP for Codex) instead of requiring ACP adapters.

**Rationale:**
- Claude Code doesn't natively support ACP; the Zed adapter (`claude-code-acp`) adds an npm dependency and Node.js process
- Codex exposes MCP server mode natively, which is simpler than wrapping in ACP
- Native protocols give us direct access to all features (budget control, tool filtering, session resume)
- The `ExternalAgentExecutor` abstraction already provides our own uniform interface

**Future:** When Claude Code ships native ACP support (per [Feature Request #6686](https://github.com/anthropics/claude-code/issues/6686)), we can add an ACP client adapter as a third executor type.

### D2: Why interrupt-only instead of full stream forwarding?

**Decision:** External agent execution stream is logged but not injected into the main agent's context window.

**Rationale:**
- Context window preservation: a Claude Code session can produce thousands of tokens of tool output
- Main agent's reasoning quality degrades with irrelevant context
- The main agent delegates for a reason — it should only care about the result and input requests
- Progress can still be observed via structured events (for TUI display) without polluting LLM context

### D3: Permission handling — MCP tool vs. stream parsing

**Decision:** Use Claude Code's `--permission-prompt-tool` mechanism (Strategy B) as default.

**Rationale:**
- Official supported mechanism, less likely to break across versions
- Clean separation: Claude Code manages its own permission flow, we just implement the callback
- Falls back to stream parsing only if the MCP tool approach isn't available

### D4: Codex approval — pre-configured vs. interactive

**Decision:** Default to `approval-policy: on-request` with input request routing.

**Rationale:**
- `never` is too permissive for arbitrary tasks
- `on-request` provides the right balance: safe operations proceed, risky ones get routed
- Main agent can make informed approval decisions based on task context

### D5: Dashboard via `bg_status` enhancement vs. separate tool

**Decision:** Enhance existing `bg_status` tool with kanban rendering rather than creating a new `bg_dashboard` tool.

**Rationale:**
- The main agent already uses `bg_status` in its tool vocabulary — adding a second tool for "status but richer" creates confusion
- The rendering is purely a formatting concern: same data, better presentation
- Kanban grouping (RUNNING / WAITING FOR INPUT / COMPLETED / FAILED) is a strict superset of the current flat list
- Progress data is only present for external tasks; internal tasks display the same as before — backward compatible
- `ExternalAgentProgressEvent` is emitted separately for TUI/Web delivery layers; `bg_status` serves the LLM's view

### D6: Progress via callback vs. polling vs. channel

**Decision:** Use `OnProgress` callback on `ExternalAgentRequest` rather than a polling interface or progress channel.

**Rationale:**
- **Callback** (chosen): executor calls `OnProgress(snapshot)` on each meaningful stream event. `BackgroundTaskManager` stores the latest snapshot. Zero allocation when no progress listener is attached (nil-safe check). Simple, synchronous, no extra goroutines.
- **Channel**: adds buffer management complexity and requires a dedicated drain goroutine; the data is write-heavy, read-rare (only when `bg_status` is called).
- **Polling** (`Progress(taskID) *ExternalAgentProgress`): requires the executor to maintain per-task state maps, but the executor shouldn't track task IDs — that's the manager's job.
- Callback is also consistent with how Go's `http.Client` reports progress (e.g., `io.TeeReader` pattern).

### D7: Workspace isolation via git worktree vs. directory copy vs. container

**Decision:** Use `git worktree` for parallel agent isolation.

**Rationale:**
- **Git worktree** (chosen): native git feature; O(1) branch creation; agents share object store; merging is first-class git operation; agents can use standard git commands. Limitation: requires git project.
- **Directory copy**: simple but O(n) file copy; no built-in merge; large repos become expensive; no shared git history.
- **Container/sandbox**: strongest isolation but heaviest; overkill for code editing tasks; adds Docker/nsjail dependency; complicates file ownership.
- Worktree mode is opt-in — `shared` mode (default) adds zero overhead for simple or single-agent workflows.

### D8: Task dependencies — graph in manager vs. LLM sequencing

**Decision:** Add explicit `depends_on` to `bg_dispatch` with graph validation in `BackgroundTaskManager`.

**Rationale:**
- **LLM sequencing** (rejected as sole approach): the main agent can manually wait for A, then dispatch B. Works but wastes LLM iterations on polling, increases cost, and can't express parallelism + ordering in one dispatch batch.
- **Manager graph** (chosen): `bg_dispatch(depends_on=["A"])` is declarative; manager handles blocking, failure propagation, and context inheritance. Main agent dispatches the entire plan in one batch. Dashboard shows dependency DAG at a glance.
- The LLM can still choose manual sequencing for simple cases — `depends_on` is optional.

### D9: Merge strategy — default auto vs. review

**Decision:** Default `ext_merge` strategy is `"review"` (return diff without auto-merging).

**Rationale:**
- External agents may produce unexpected changes; auto-merge gives no opportunity for the main agent to inspect.
- `"review"` mode returns diff stats and lets the main agent decide: approve (switch to `"auto"`), request changes, or discard.
- For trusted pipelines (CI-like), the main agent can explicitly request `"auto"` or `"squash"`.

---

## 7. Risk Mitigation

| Risk | Mitigation |
|------|------------|
| External binary not installed | Check at startup, emit warning, disable agent type gracefully |
| Subprocess hangs | Enforce timeout via context.WithTimeout + SIGKILL fallback |
| Input request not answered (main agent stuck) | Deadline on InputRequest; auto-reject after timeout |
| Claude Code CLI version incompatibility | Parse version from `claude --version`; warn on unsupported versions |
| Cost runaway | Enforce `--max-budget-usd` flag per task; aggregate in session cost tracker |
| Concurrent input requests | Support multiple pending requests via sync.Map keyed by requestID |
| Main agent context bloat | Only inject minimal input request descriptions, not full tool arguments |
| Orphan processes after crash | Use process groups (`Setpgid`); write PIDs to `<data_dir>/external-pids.json`; on startup kill stale processes; `--max-budget-usd` as hard API-level cap |
| Partial result + error data loss | Pre-existing: `BackgroundTaskManager.runTask()` sets `err` when `extResult.Error != ""`, marking task as failed even if `Answer` is populated. Fix: introduce `completed_with_errors` status or always preserve answer |
| Git merge conflict | `ext_merge` returns conflict details; main agent inspects and resolves. Default strategy is `review` (diff only, no auto-merge) |
| Worktree exhaustion | Limit max concurrent worktrees (configurable, default 5); queue excess tasks |
| Agent writes outside declared scope | Post-completion scope validation via `ValidateFileScope()`; warn main agent if out-of-scope changes detected |
| Circular task dependencies | Topological sort check at dispatch time; reject with clear error listing the cycle |
| Dependency task failure cascades | When a dependency fails, all downstream tasks transition to `failed` with `"dependency failed"` reason; dashboard shows cascade |
| Stale worktrees after crash | On startup, scan `.elephant/worktrees/`; remove orphaned worktrees and their branches |
| Non-git project | `WorkspaceMode` defaults to `shared`; `branch`/`worktree` modes require git; return clear error if requested in non-git project |

---

## 8. Testing Strategy

1. **Unit tests:** Message parsers, subprocess manager, input routing, kanban renderer
2. **Mock tests:** Mock subprocess that emits scripted NDJSON/JSON-RPC sequences; mock executor with scripted progress/input sequences
3. **Dashboard tests:** Verify `bg_status` kanban output for combinations of: running tasks with progress, tasks waiting for input, completed/failed/cancelled tasks, mixed internal+external tasks
4. **Integration tests:** End-to-end with `bg_dispatch` → executor → progress updates → `bg_status` → `ext_reply` → `bg_collect`
5. **Collaboration tests:** Fan-out/fan-in (dispatch 3 agents → merge all), pipeline (A → B with context), scope overlap detection, worktree isolation, dependency failure cascade
6. **Git integration tests:** Real git operations: worktree create/remove, branch merge with/without conflicts, scope validation
7. **Manual testing:** Real Claude Code and Codex invocations with live API keys (gated behind env vars)

---

## 9. Open Questions

1. ~~**Should the main agent auto-approve certain tool categories?**~~ **RESOLVED:** Yes — auto-approve read-only tools (`Read`, `Grep`, `Glob`, `WebSearch`) within the permission MCP server by default. Configurable via `autonomous_allowed_tools` in `external_agents.yaml`. This prevents flooding the main agent with noise for safe operations while still routing write/execute tools as input requests.
2. **Should we support multi-turn Codex sessions?** The `codex-reply` tool enables follow-up prompts. We could expose this via the `ext_reply` tool with a `message` field.
4. **Should the main agent auto-plan collaboration?** When given a complex task, should the main agent automatically decompose it into parallel sub-tasks with dependency DAG and workspace isolation? Or should the user explicitly request multi-agent dispatch? Initial approach: let the LLM decide based on task complexity (via `plan()` orchestrator gate).
5. **Max concurrent external agents?** Default limit for parallel worktrees? Proposed: 5 (configurable). Higher values increase git overhead and risk merge complexity.
3. ~~**Should external agent progress be visible in TUI/Web?**~~ **RESOLVED:** Yes — two layers: (a) `bg_status` kanban view for the LLM's proactive queries; (b) `ExternalAgentProgressEvent` domain events for real-time TUI/Web rendering (spinner, latest tool name, iteration count). Events are throttled to ≤1 per 2s per task.

---

## Progress Log

- 2026-01-30: Initial research complete — Claude Code SDK protocol, Codex MCP mode, existing codebase analysis
- 2026-01-30: Architecture design drafted with 5 implementation phases
- 2026-01-30: Added multi-agent collaboration design (Layer 8, Phase 6):
  - Workspace isolation via `git worktree` with 3 modes: shared/branch/worktree
  - Task dependency graph with `depends_on`, blocked status, failure cascade, context inheritance
  - `ext_merge` tool with auto/squash/rebase/review strategies
  - File scope declaration + overlap detection at dispatch time + post-completion validation
  - Dashboard extended with workspace info, branch names, dependency DAG with status indicators
  - New port types: `WorkspaceMode`, `WorkspaceAllocation`, `MergeStrategy`, `MergeResult`, `TaskDependency`
  - D7 (worktree vs copy vs container), D8 (graph vs LLM sequencing), D9 (default review merge)
  - 8 new risk mitigations: merge conflict, worktree exhaustion, scope violation, cycles, cascade, stale worktrees, non-git
- 2026-01-30: Added progress tracking & kanban dashboard design (Layer 7):
  - `ExternalAgentProgress` snapshot type with iteration/tokens/cost/currentTool/files
  - `OnProgress` callback on `ExternalAgentRequest` for real-time progress capture
  - `BackgroundTaskSummary` gains `Progress`, `PendingInput`, `Elapsed` fields
  - `bg_status` enhanced to render grouped kanban view (RUNNING / WAITING FOR INPUT / COMPLETED / FAILED)
  - `ExternalAgentProgressEvent` domain event for TUI/Web delivery (throttled ≤1/2s)
  - Three interaction modes: passive (notifications), proactive check (bg_status), monitoring loop
- 2026-01-30: **Review pass** — 13 issues identified and corrected:
  - Fixed misleading title (plan rejects ACP, uses native protocols)
  - Added missing `AgentType` field to `ExternalAgentRequest`, noted `WorkingDir` wiring gap
  - Detailed permission MCP server implementation (subcommand + Unix socket + auto-approve filter)
  - Simplified `Subprocess` to thin lifecycle wrapper (protocol parsing owned by executors)
  - Added `InteractiveExternalExecutor` channel lifecycle contract
  - Added mock executor to Phase 1 for testability
  - Added orphan process protection to risk table and Phase 5
  - Resolved Open Question 1 (auto-approve read-only tools)
  - Noted pre-existing partial-result-with-error data loss in `BackgroundTaskManager`
  - Clarified coordinator wiring gap (`ExternalExecutor` not in `ReactEngineConfig` construction)
