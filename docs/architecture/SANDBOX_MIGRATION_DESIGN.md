# Sandbox Tool Migration - Technical Design & Acceptance Criteria

## Executive Summary

This document describes the migration of file/shell/code tools to sandbox execution for web server mode, while maintaining local execution for CLI mode.

**Key Objectives:**
- ✅ All file/shell/code tools execute in sandbox when running in Web SSE Server mode
- ✅ All tools execute locally when running in CLI mode
- ✅ Tool definitions remain unchanged (transparent to LLM)
- ✅ Sandbox initialized at web server startup
- ✅ Host + sandbox environment snapshots captured at startup, injected into the agent system prompt, and surfaced in the web UI header
- ✅ Zero impact on CLI user experience

---

## 1. Architecture Design

### 1.1 Execution Mode Abstraction

```
┌─────────────────────────────────────────────────────────┐
│                   Tool Interface                         │
│              (ToolExecutor Interface)                    │
└───────────────────┬─────────────────────────────────────┘
                    │
        ┌───────────┴──────────┐
        │                      │
┌───────▼────────┐    ┌───────▼────────┐
│  CLI Process   │    │ Web Server     │
│                │    │                │
│ ExecutionMode: │    │ ExecutionMode: │
│    LOCAL       │    │    SANDBOX     │
└────────────────┘    └────────────────┘
        │                      │
        │                      │
┌───────▼────────┐    ┌───────▼────────┐
│ Local Executor │    │ Sandbox Client │
│ - os.ReadFile  │    │ - file.Read()  │
│ - exec.Command │    │ - shell.Exec() │
│ - filepath.*   │    │ - jupyter.*    │
└────────────────┘    └────────────────┘
```

### 1.2 Dual-Mode Tool Implementation Pattern

Each tool implements both local and sandbox execution paths:

```go
type FileReadTool struct {
    mode          ExecutionMode
    sandboxClient *sandbox.FileClient  // nil in local mode
}

func (t *FileReadTool) Execute(ctx context.Context, call ToolCall) (*ToolResult, error) {
    if t.mode == ExecutionModeSandbox {
        return t.executeSandbox(ctx, call)
    }
    return t.executeLocal(ctx, call)
}
```

**Key Design Principles:**
1. **Single Tool Definition**: ToolDefinition() remains identical for both modes
2. **Transparent Routing**: Mode determined at initialization, not runtime
3. **Shared Interface**: Both modes return *ToolResult with same schema
4. **Graceful Degradation**: Sandbox failures can optionally fall back to local
5. **Explicit Validation**: Execution mode must be validated during startup to avoid undefined behavior

---

## 2. Implementation Plan

### 2.1 Phase 1: Core Infrastructure (Priority: HIGH)

#### 2.1.1 Execution Mode Definition
**File**: `internal/tools/execution_mode.go` (NEW)

```go
package tools

import "fmt"

type ExecutionMode int

const (
    // ExecutionModeUnknown guards against misconfiguration.
    ExecutionModeUnknown ExecutionMode = iota

    // ExecutionModeLocal uses local filesystem and shell
    ExecutionModeLocal

    // ExecutionModeSandbox uses remote sandbox via SDK
    ExecutionModeSandbox
)

func (m ExecutionMode) String() string {
    switch m {
    case ExecutionModeLocal:
        return "local"
    case ExecutionModeSandbox:
        return "sandbox"
    default:
        return "unknown"
    }
}

func (m ExecutionMode) Validate() error {
    if m != ExecutionModeLocal && m != ExecutionModeSandbox {
        return fmt.Errorf("invalid execution mode: %d", m)
    }
    return nil
}
```

#### 2.1.2 Sandbox Manager
**File**: `internal/tools/sandbox_manager.go` (NEW)

```go
package tools

import (
    "bufio"
    "context"
    "fmt"
    "strings"
    "sync"
    "time"

    "github.com/agent-infra/sandbox-sdk-go/file"
    "github.com/agent-infra/sandbox-sdk-go/jupyter"
    "github.com/agent-infra/sandbox-sdk-go/shell"
)

// SandboxManager manages shared sandbox clients
type SandboxManager struct {
    baseURL    string
    fileClient *file.Client
    shellClient *shell.Client
    jupyterClient *jupyter.Client

    envSnapshot map[string]string

    initOnce sync.Once
    initErr  error
}

func NewSandboxManager(baseURL string) *SandboxManager {
    return &SandboxManager{baseURL: baseURL}
}

func (m *SandboxManager) Initialize(ctx context.Context) error {
    m.initOnce.Do(func() {
        if m.baseURL == "" {
            m.initErr = fmt.Errorf("sandbox base URL is required")
            return
        }

        // Initialize all sandbox clients
        m.fileClient = file.NewClient(m.baseURL)
        m.shellClient = shell.NewClient(m.baseURL)
        m.jupyterClient = jupyter.NewClient(m.baseURL)

        // Perform health check
        m.initErr = m.healthCheck(ctx)
    })
    return m.initErr
}

func (m *SandboxManager) healthCheck(ctx context.Context) error {
    // Verify sandbox is reachable
    _, err := m.shellClient.Exec(ctx, shell.ExecRequest{
        Command: "echo 'health_check'",
        Timeout: 5 * time.Second,
    })
    return err
}

func (m *SandboxManager) File() *file.Client { return m.fileClient }
func (m *SandboxManager) Shell() *shell.Client { return m.shellClient }
func (m *SandboxManager) Jupyter() *jupyter.Client { return m.jupyterClient }

func (m *SandboxManager) Environment(ctx context.Context) (map[string]string, error) {
    if err := m.Initialize(ctx); err != nil {
        return nil, err
    }

    if m.envSnapshot != nil {
        return m.envSnapshot, nil
    }

    resp, err := m.shellClient.Exec(ctx, shell.ExecRequest{
        Command: "printenv",
        Timeout: 5 * time.Second,
    })
    if err != nil {
        return nil, err
    }

    m.envSnapshot = parseEnv(resp.Stdout)
    return m.envSnapshot, nil
}

func parseEnv(stdout string) map[string]string {
    vars := map[string]string{}
    scanner := bufio.NewScanner(strings.NewReader(stdout))
    for scanner.Scan() {
        line := scanner.Text()
        if strings.TrimSpace(line) == "" {
            continue
        }
        parts := strings.SplitN(line, "=", 2)
        if len(parts) == 2 {
            vars[parts[0]] = parts[1]
        }
    }
    return vars
}

func (m *SandboxManager) HealthCheck(ctx context.Context) error {
    if err := m.Initialize(ctx); err != nil {
        return err
    }
    return m.healthCheck(ctx)
}
```

The `Environment` helper lazily hydrates and caches the sandbox's environment variables, ensuring we only pay the `printenv` cost once per process lifetime unless the background refresh invalidates the snapshot.

#### 2.1.3 Registry Configuration Update
**File**: `internal/tools/registry.go` (MODIFY)

```go
// Config for tool registry initialization
type Config struct {
    TavilyAPIKey   string
    SandboxBaseURL string

    // NEW: Execution mode for tools
    ExecutionMode ExecutionMode

    // NEW: Shared sandbox manager (nil if mode is Local)
    SandboxManager *SandboxManager
}

func (r *Registry) registerBuiltins(config Config) error {
    if err := config.ExecutionMode.Validate(); err != nil {
        return err
    }

    // File tools
    r.static["file_read"] = builtin.NewFileRead(builtin.FileReadConfig{
        Mode:           config.ExecutionMode,
        SandboxManager: config.SandboxManager,
    })
    r.static["file_write"] = builtin.NewFileWrite(builtin.FileWriteConfig{
        Mode:           config.ExecutionMode,
        SandboxManager: config.SandboxManager,
    })
    r.static["file_edit"] = builtin.NewFileEdit(builtin.FileEditConfig{
        Mode:           config.ExecutionMode,
        SandboxManager: config.SandboxManager,
    })
    r.static["list_files"] = builtin.NewListFiles(builtin.ListFilesConfig{
        Mode:           config.ExecutionMode,
        SandboxManager: config.SandboxManager,
    })

    // Shell tools
    r.static["bash"] = builtin.NewBash(builtin.BashConfig{
        Mode:           config.ExecutionMode,
        SandboxManager: config.SandboxManager,
    })
    r.static["grep"] = builtin.NewGrep(builtin.GrepConfig{
        Mode:           config.ExecutionMode,
        SandboxManager: config.SandboxManager,
    })
    r.static["ripgrep"] = builtin.NewRipgrep(builtin.RipgrepConfig{
        Mode:           config.ExecutionMode,
        SandboxManager: config.SandboxManager,
    })
    r.static["find"] = builtin.NewFind(builtin.FindConfig{
        Mode:           config.ExecutionMode,
        SandboxManager: config.SandboxManager,
    })

    // Code execute already has sandbox support - update config
    r.static["code_execute"] = builtin.NewCodeExecute(builtin.CodeExecuteConfig{
        BaseURL: config.SandboxBaseURL,
        Mode:    config.ExecutionMode,
        SandboxManager: config.SandboxManager,
    })

    // ... other tools remain unchanged

    return nil
}
```

---

### 2.2 Phase 2: Tool Migration (Priority: HIGH)

Each tool needs to be refactored to support dual-mode execution.

#### 2.2.1 Tool Implementation Template

**Pattern for all file/shell tools:**

```go
package builtin

import (
    "context"
    "fmt"

    "github.com/agent-infra/sandbox-sdk-go/file"
    "github.com/yourusername/alex/internal/agent/ports"
    "github.com/yourusername/alex/internal/tools"
)

type FileReadConfig struct {
    Mode           tools.ExecutionMode
    SandboxManager *tools.SandboxManager
}

type FileRead struct {
    mode    tools.ExecutionMode
    sandbox *tools.SandboxManager
}

func NewFileRead(cfg FileReadConfig) *FileRead {
    return &FileRead{
        mode:    cfg.Mode,
        sandbox: cfg.SandboxManager,
    }
}

func (t *FileRead) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
    if t.mode == tools.ExecutionModeSandbox {
        if t.sandbox == nil {
            return nil, fmt.Errorf("sandbox manager is required in sandbox mode")
        }
        return t.executeSandbox(ctx, call)
    }
    return t.executeLocal(ctx, call)
}

func (t *FileRead) Mode() tools.ExecutionMode {
    return t.mode
}

func (t *FileRead) executeLocal(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
    // Existing local implementation
    // ... current code from file_read.go ...
}

func (t *FileRead) executeSandbox(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
    // Extract parameters
    filePath, _ := call.Arguments["file_path"].(string)

    // Call sandbox file client
    resp, err := t.sandbox.File().Read(ctx, file.ReadRequest{
        Path: filePath,
    })
    if err != nil {
        return &ports.ToolResult{
            CallID:  call.ID,
            Success: false,
            Error:   err.Error(),
        }, nil
    }

    return &ports.ToolResult{
        CallID:  call.ID,
        Success: true,
        Output:  resp.Content,
    }, nil
}

func (t *FileRead) Definition() ports.ToolDefinition {
    // UNCHANGED - same definition for both modes
    return ports.ToolDefinition{
        Name:        "file_read",
        Description: "Read the contents of a file at the specified path",
        // ... same as before ...
    }
}
```

#### 2.2.2 Tools to Migrate

| Tool | Priority | Complexity | Estimated Time |
|------|----------|------------|----------------|
| `file_read.go` | HIGH | Low | 2h |
| `file_write.go` | HIGH | Low | 2h |
| `file_edit.go` | HIGH | Medium | 4h |
| `list_files.go` | HIGH | Low | 2h |
| `bash.go` | HIGH | Medium | 3h |
| `grep.go` | MEDIUM | Low | 2h |
| `ripgrep.go` | MEDIUM | Low | 2h |
| `find.go` | MEDIUM | Low | 2h |
| `code_execute.go` | LOW | Low | 1h (refactor only) |

**Total estimated time: 20 hours**

---

### 2.3 Phase 3: DI Container Integration (Priority: HIGH)

#### 2.3.1 CLI Container (No changes)
**File**: `internal/di/container.go` (MODIFY)

```go
// BuildCLIContainer creates container for CLI mode (LOCAL execution)
func BuildCLIContainer(config Config) (*Container, error) {
    // ... existing code ...

    toolRegistry, err := tools.NewRegistry(tools.Config{
        TavilyAPIKey:   config.TavilyAPIKey,
        SandboxBaseURL: config.SandboxBaseURL,
        ExecutionMode:  tools.ExecutionModeLocal,  // NEW: Force local mode
        SandboxManager: nil,                       // NEW: No sandbox in CLI
    })
    if err != nil {
        return nil, err
    }

    // ... rest unchanged ...
}
```

#### 2.3.2 Web Server Container
**File**: `internal/di/container.go` (MODIFY)

```go
// BuildServerContainer creates container for web server mode (SANDBOX execution)
func BuildServerContainer(config Config) (*Container, error) {
    // ... existing code ...

    // NEW: Initialize sandbox manager
    var sandboxManager *tools.SandboxManager
    if config.SandboxBaseURL != "" {
        sandboxManager = tools.NewSandboxManager(config.SandboxBaseURL)

        // Initialize sandbox (with timeout)
        initCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()

        if err := sandboxManager.Initialize(initCtx); err != nil {
            return nil, fmt.Errorf("failed to initialize sandbox: %w", err)
        }
        log.Printf("✓ Sandbox initialized successfully at %s", config.SandboxBaseURL)
    }

    toolRegistry, err := tools.NewRegistry(tools.Config{
        TavilyAPIKey:   config.TavilyAPIKey,
        SandboxBaseURL: config.SandboxBaseURL,
        ExecutionMode:  tools.ExecutionModeSandbox,  // NEW: Force sandbox mode
        SandboxManager: sandboxManager,               // NEW: Shared sandbox clients
    })
    if err != nil {
        return nil, err
    }

    // ... rest unchanged ...
}
```

The container struct gains a `SandboxManager *tools.SandboxManager` field so downstream boot code and diagnostics can reuse the initialized clients without recomputing them.

#### 2.3.3 Server Startup Update
**File**: `cmd/alex-server/main.go` (MODIFY)

```go
func main() {
    // Load configuration
    cfg := config.LoadFromEnv()

    // Capture host environment snapshot for diagnostics
    hostEnv := config.SnapshotProcessEnv()

    // Validate sandbox configuration for server mode
    if cfg.SandboxBaseURL == "" {
        log.Fatal("SANDBOX_BASE_URL is required for server mode")
    }

    // Build server container (with sandbox)
    container, err := di.BuildServerContainer(cfg)
    if err != nil {
        log.Fatalf("Failed to build container: %v", err)
    }

    // Retrieve sandbox environment variables for downstream consumers
    sandboxEnv, err := container.SandboxManager.Environment(context.Background())
    if err != nil {
        log.Fatalf("Failed to read sandbox environment: %v", err)
    }

    diagnostics.PublishEnvironments(diagnostics.EnvironmentPayload{
        Host:     hostEnv,
        Sandbox:  sandboxEnv,
        Captured: time.Now(),
    })

    // Inject the merged environment snapshot into the agent system prompt
    promptEnvSummary := prompts.FormatEnvironmentSummary(hostEnv, sandboxEnv)
    agent := conversation.New(conversation.Config{
        SystemPrompts: []string{
            prompts.BaseSystemPrompt(),
            promptEnvSummary,
        },
    })
    agent.RegisterSandboxManager(container.SandboxManager)

    // ... rest of server startup ...
    log.Printf("Server starting in SANDBOX mode (sandbox: %s)", cfg.SandboxBaseURL)
}
```

#### 2.3.4 Environment Metadata Publication
**File**: `internal/diagnostics/environment.go` (NEW)

```go
package diagnostics

import "time"

type EnvironmentPayload struct {
    Host     map[string]string
    Sandbox  map[string]string
    Captured time.Time
}

func PublishEnvironments(payload EnvironmentPayload) {
    // Fan out to logging, telemetry, and web UI caches.
    // Implementation will inject this payload into the SSE status stream.
}
```

The diagnostics package exposes a subscribe-able feed so that the web layer can render both host and sandbox environments. The payload is stored in-memory with periodic refreshes (default 15 minutes) triggered by a background cron using the same `SandboxManager.Environment` helper to keep the snapshot current.

---

### 2.4 Phase 4: Sandbox SDK Integration (Priority: MEDIUM)

#### 2.4.1 Sandbox SDK API Mapping

**File Operations:**
```go
// Local → Sandbox mapping
os.ReadFile(path)                 → file.Read(ReadRequest{Path: path})
os.WriteFile(path, data, perm)    → file.Write(WriteRequest{Path: path, Content: data})
os.Stat(path)                     → file.Stat(StatRequest{Path: path})
filepath.Walk(root, fn)           → file.List(ListRequest{Path: root, Recursive: true})
```

**Shell Operations:**
```go
// Local → Sandbox mapping
exec.CommandContext(ctx, "bash", "-c", cmd) → shell.Exec(ExecRequest{Command: cmd, Timeout: t})
cmd.CombinedOutput()                        → resp.Stdout + resp.Stderr
```

**Search Operations:**
```go
// Grep/Ripgrep → Sandbox shell
exec.Command("grep", args...)     → shell.Exec(ExecRequest{Command: "grep ..."})
exec.Command("rg", args...)       → shell.Exec(ExecRequest{Command: "rg ..."})
exec.Command("find", args...)     → shell.Exec(ExecRequest{Command: "find ..."})
```

#### 2.4.2 Error Handling Strategy

```go
func (t *FileRead) executeSandbox(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
    resp, err := t.sandbox.File().Read(ctx, req)
    if err != nil {
        // Map sandbox errors to tool errors
        return &ports.ToolResult{
            CallID:  call.ID,
            Success: false,
            Error:   formatSandboxError(err),
        }, nil  // Return nil error - tool execution succeeded, operation failed
    }

    return &ports.ToolResult{
        CallID:  call.ID,
        Success: true,
        Output:  resp.Content,
    }, nil
}

func formatSandboxError(err error) string {
    // Convert sandbox errors to user-friendly messages
    switch {
    case strings.Contains(err.Error(), "connection refused"):
        return "Sandbox unreachable - check SANDBOX_BASE_URL"
    case strings.Contains(err.Error(), "timeout"):
        return "Sandbox operation timed out"
    case strings.Contains(err.Error(), "not found"):
        return "File not found in sandbox"
    default:
        return fmt.Sprintf("Sandbox error: %v", err)
    }
}
```

### 2.5 Web UI Environment Banner (Priority: MEDIUM)

The web console should surface the captured environment details so operators can confirm the sandbox context. The banner renders in muted, small-type text beneath the session header to avoid distracting end-users while still surfacing critical debugging metadata.

**File**: `web/src/components/status/EnvironmentStrip.tsx` (NEW)

```tsx
import { useDiagnostics } from "@/hooks/useDiagnostics";

export function EnvironmentStrip() {
  const { environments } = useDiagnostics();
  if (!environments) {
    return null;
  }

  return (
    <div className="text-xs text-muted-foreground truncate" data-testid="environment-strip">
      Host: {formatEnv(environments.host)} | Sandbox: {formatEnv(environments.sandbox)}
    </div>
  );
}

function formatEnv(env: Record<string, string>): string {
  const whitelist = ["HOSTNAME", "USER", "SANDBOX_BASE_URL"];
  return whitelist
    .map((key) => (env[key] ? `${key}=${env[key]}` : null))
    .filter(Boolean)
    .join(" · ");
}
```

**File**: `web/src/components/layout/AppHeader.tsx` (MODIFY)

```tsx
import { EnvironmentStrip } from "@/components/status/EnvironmentStrip";

export function AppHeader() {
  return (
    <header className="border-b bg-background">
      <div className="flex flex-col gap-1 px-6 py-3">
        <AppBreadcrumbs />
        <EnvironmentStrip />
      </div>
    </header>
  );
}
```

The diagnostics hook reuses the SSE feed populated by `PublishEnvironments`, ensuring the UI stays current without additional API calls. Typography leverages the existing `text-xs` Tailwind utility to satisfy the "small text" display requirement.

---

## 3. Configuration Management

### 3.1 Environment Variables

```bash
# Existing (used by both CLI and server)
ALEX_SANDBOX_BASE_URL=http://localhost:8090

# New (optional - for explicit mode override)
ALEX_EXECUTION_MODE=auto|local|sandbox  # auto = detect from process type
```

### 3.2 Configuration Precedence

1. **CLI Mode**: Always use `ExecutionModeLocal` (hardcoded)
2. **Server Mode**: Always use `ExecutionModeSandbox` if `SANDBOX_BASE_URL` is set
3. **Fallback**: If sandbox URL not set in server mode, fail fast with clear error

### 3.3 Configuration Validation

**File**: `internal/config/runtime_env.go` (MODIFY)

```go
import (
    "fmt"
    "net/url"
    "os"
    "strings"
)

func LoadFromEnv() Config {
    cfg := Config{
        // ... existing fields ...
        SandboxBaseURL: getEnv("ALEX_SANDBOX_BASE_URL", ""),
    }

    return cfg
}

func SnapshotProcessEnv() map[string]string {
    vars := map[string]string{}
    for _, kv := range os.Environ() {
        parts := strings.SplitN(kv, "=", 2)
        if len(parts) == 2 {
            vars[parts[0]] = parts[1]
        }
    }
    return vars
}

func ValidateServerConfig(cfg Config) error {
    if cfg.SandboxBaseURL == "" {
        return fmt.Errorf("ALEX_SANDBOX_BASE_URL is required for server mode")
    }

    // Validate URL format
    if _, err := url.Parse(cfg.SandboxBaseURL); err != nil {
        return fmt.Errorf("invalid ALEX_SANDBOX_BASE_URL: %w", err)
    }

    return nil
}
```

#### 2.3.5 System Prompt Environment Injection
**Files**: `internal/prompts/environment_summary.go` (NEW), `internal/conversation/agent.go` (MODIFY)

```go
// internal/prompts/environment_summary.go
package prompts

import (
    "fmt"
    "sort"
    "strings"
)

func FormatEnvironmentSummary(hostEnv, sandboxEnv map[string]string) string {
    keys := make([]string, 0, len(hostEnv)+len(sandboxEnv))
    seen := map[string]struct{}{}

    for k := range hostEnv {
        if _, ok := seen[k]; !ok {
            seen[k] = struct{}{}
            keys = append(keys, k)
        }
    }

    for k := range sandboxEnv {
        if _, ok := seen[k]; !ok {
            seen[k] = struct{}{}
            keys = append(keys, k)
        }
    }

    sort.Strings(keys)

    var builder strings.Builder
    builder.WriteString("Environment context:\n")
    for _, k := range keys {
        hostVal := hostEnv[k]
        sandboxVal := sandboxEnv[k]

        switch {
        case hostVal != "" && sandboxVal != "":
            builder.WriteString(fmt.Sprintf("- %s: host=%q, sandbox=%q\n", k, hostVal, sandboxVal))
        case sandboxVal != "":
            builder.WriteString(fmt.Sprintf("- %s: sandbox=%q\n", k, sandboxVal))
        default:
            builder.WriteString(fmt.Sprintf("- %s: host=%q\n", k, hostVal))
        }
    }

    return builder.String()
}

// internal/conversation/agent.go
func New(config Config) *Agent {
    agent := &Agent{
        systemPrompts: append([]string{}, config.SystemPrompts...),
        // ... existing fields ...
    }

    agent.systemPrompts = append(agent.systemPrompts, config.EnvironmentPrompt)
    return agent
}

func (a *Agent) BootstrapEnvironmentPrompt(summary string) {
    if summary == "" {
        return
    }
    a.systemPrompts = append(a.systemPrompts, summary)
}
```

The agent boot logic appends the formatted environment summary as a dedicated system prompt so every conversation reflects the
currently captured host and sandbox metadata. The helper consolidates duplicate keys across environments, ensuring deterministic
output for caching and tests.

---

## 4. Testing Strategy

### 4.1 Unit Tests

#### 4.1.1 Dual-Mode Tool Tests
**File**: `internal/tools/builtin/file_read_test.go` (EXAMPLE)

```go
func TestFileRead_LocalMode(t *testing.T) {
    // Create temp file
    tmpfile := createTempFile(t, "test content")
    defer os.Remove(tmpfile)

    // Create tool in local mode
    tool := NewFileRead(FileReadConfig{
        Mode: tools.ExecutionModeLocal,
    })

    // Execute
    result, err := tool.Execute(context.Background(), ports.ToolCall{
        ID:   "test-1",
        Name: "file_read",
        Arguments: map[string]any{
            "file_path": tmpfile,
        },
    })

    // Verify
    assert.NoError(t, err)
    assert.True(t, result.Success)
    assert.Equal(t, "test content", result.Output)
}

func TestFileRead_SandboxMode(t *testing.T) {
    // Setup mock sandbox manager
    mockSandbox := setupMockSandbox(t)

    // Create tool in sandbox mode
    tool := NewFileRead(FileReadConfig{
        Mode:           tools.ExecutionModeSandbox,
        SandboxManager: mockSandbox,
    })

    // Mock sandbox response
    mockSandbox.File().On("Read", mock.Anything, mock.Anything).
        Return(&file.ReadResponse{Content: "sandbox content"}, nil)

    // Execute
    result, err := tool.Execute(context.Background(), ports.ToolCall{
        ID:   "test-2",
        Name: "file_read",
        Arguments: map[string]any{
            "file_path": "/test.txt",
        },
    })

    // Verify
    assert.NoError(t, err)
    assert.True(t, result.Success)
    assert.Equal(t, "sandbox content", result.Output)
    mockSandbox.AssertExpectations(t)
}
```

#### 4.1.2 Sandbox Manager Tests
**File**: `internal/tools/sandbox_manager_test.go` (NEW)

```go
func TestSandboxManager_Initialize(t *testing.T) {
    // Test initialization with valid URL
    mgr := NewSandboxManager("http://localhost:8090")
    err := mgr.Initialize(context.Background())
    assert.NoError(t, err)
    assert.NotNil(t, mgr.File())
    assert.NotNil(t, mgr.Shell())
}

func TestSandboxManager_InitializeOnce(t *testing.T) {
    // Test singleton pattern
    mgr := NewSandboxManager("http://localhost:8090")

    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            mgr.Initialize(context.Background())
        }()
    }
    wg.Wait()

    // Should only initialize once
    assert.NotNil(t, mgr.Shell())
}
```

### 4.2 Integration Tests

#### 4.2.1 CLI Integration Test
**File**: `tests/integration/cli_local_execution_test.go` (NEW)

```go
func TestCLI_UsesLocalExecution(t *testing.T) {
    // Build CLI container
    container, err := di.BuildCLIContainer(config.Config{
        SandboxBaseURL: "http://should-not-be-used:9999",
    })
    require.NoError(t, err)

    // Verify tool registry uses local mode
    registry := container.ToolRegistry
    fileReadTool, _ := registry.Get("file_read")

    // Type assert and verify mode
    fileRead, ok := fileReadTool.(*builtin.FileRead)
    require.True(t, ok)
    assert.Equal(t, tools.ExecutionModeLocal, fileRead.Mode())
}
```

#### 4.2.2 Server Integration Test
**File**: `tests/integration/server_sandbox_execution_test.go` (NEW)

```go
func TestServer_UsesSandboxExecution(t *testing.T) {
    // Setup test sandbox server
    sandboxURL := setupTestSandboxServer(t)

    // Build server container
    container, err := di.BuildServerContainer(config.Config{
        SandboxBaseURL: sandboxURL,
    })
    require.NoError(t, err)

    // Verify tool registry uses sandbox mode
    registry := container.ToolRegistry
    fileReadTool, _ := registry.Get("file_read")

    // Type assert and verify mode
    fileRead, ok := fileReadTool.(*builtin.FileRead)
    require.True(t, ok)
    assert.Equal(t, tools.ExecutionModeSandbox, fileRead.Mode())
}
```

### 4.3 End-to-End Tests

#### 4.3.1 CLI E2E Test
```bash
# Test file operations work locally
$ ./alex "Create a file test.txt with content 'hello world' and read it back"

# Verify file exists on local filesystem
$ cat test.txt
hello world
```

#### 4.3.2 Server E2E Test
```bash
# Start server with sandbox
$ ALEX_SANDBOX_BASE_URL=http://localhost:8090 ./alex-server

# Submit task via API
$ curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{"task": "Create file test.txt with content hello", "session_id": "test"}'

# Verify file created in sandbox (not local)
$ ls test.txt
ls: test.txt: No such file or directory  # Should NOT exist locally
```

---

## 5. Rollout Plan

### 5.1 Development Phases

**Phase 1: Foundation (Week 1)**
- ✅ Create `ExecutionMode` enum
- ✅ Create `SandboxManager` with initialization
- ✅ Update `tools.Config` with mode fields
- ✅ Update DI container for CLI (local mode)
- ✅ Update DI container for server (sandbox mode)
- ✅ Write unit tests for core infrastructure

**Phase 2: Tool Migration (Week 2)**
- ✅ Migrate `file_read` tool (with tests)
- ✅ Migrate `file_write` tool (with tests)
- ✅ Migrate `file_edit` tool (with tests)
- ✅ Migrate `list_files` tool (with tests)
- ✅ Migrate `bash` tool (with tests)
- ✅ Integration test: Verify CLI still works locally

**Phase 3: Search Tools (Week 3)**
- ✅ Migrate `grep` tool (with tests)
- ✅ Migrate `ripgrep` tool (with tests)
- ✅ Migrate `find` tool (with tests)
- ✅ Refactor `code_execute` to use shared manager
- ✅ Integration test: Verify server uses sandbox

**Phase 4: Validation & Documentation (Week 4)**
- ✅ End-to-end tests (CLI + Server)
- ✅ Performance benchmarks
- ✅ Update user documentation
- ✅ Update deployment scripts
- ✅ Create migration guide

### 5.2 Deployment Strategy

1. **Local Development**: Deploy to dev environment first
2. **Canary Deployment**: Roll out to 10% of servers
3. **Monitoring**: Watch for errors, latency increases
4. **Full Rollout**: Deploy to all servers if canary successful
5. **Rollback Plan**: Keep old binaries for 1 week

---

## 6. Acceptance Criteria

### 6.1 Functional Requirements

| ID | Requirement | Verification Method | Status |
|----|-------------|---------------------|--------|
| F1 | All file tools (read/write/edit/list) execute in sandbox when server mode | E2E test: File created in sandbox, not locally | ❌ |
| F2 | All shell tools (bash/grep/ripgrep/find) execute in sandbox when server mode | E2E test: Command runs in sandbox | ❌ |
| F3 | All tools execute locally in CLI mode | E2E test: File created locally | ❌ |
| F4 | Tool definitions remain unchanged | Unit test: ToolDefinition() returns same output | ❌ |
| F5 | Sandbox initialized at server startup | Server logs show "Sandbox initialized" | ❌ |
| F6 | Server fails fast if sandbox unreachable | Server exits with error on startup | ❌ |
| F7 | CLI works without sandbox URL configured | CLI executes tasks successfully | ❌ |
| F8 | Error messages are user-friendly | Manual test: Verify error readability | ❌ |

### 6.2 Performance Requirements

| ID | Requirement | Target | Verification Method | Status |
|----|-------------|--------|---------------------|--------|
| P1 | Sandbox initialization time | < 3 seconds | Benchmark test | ❌ |
| P2 | File read operation (sandbox vs local) | < 2x latency | Benchmark test | ❌ |
| P3 | Shell command execution (sandbox vs local) | < 2x latency | Benchmark test | ❌ |
| P4 | No memory leaks in sandbox manager | 0 leaks | Memory profiler | ❌ |
| P5 | Concurrent sandbox operations | 10+ parallel | Load test | ❌ |

### 6.3 Security Requirements

| ID | Requirement | Verification Method | Status |
|----|-------------|---------------------|--------|
| S1 | Sandbox prevents access to host filesystem | Security test: Attempt path traversal | ❌ |
| S2 | Sandbox enforces timeout limits | Security test: Long-running command | ❌ |
| S3 | Sandbox isolates sessions | Security test: Cross-session file access | ❌ |
| S4 | No credentials logged in sandbox operations | Code review + log audit | ❌ |

### 6.4 Backward Compatibility

| ID | Requirement | Verification Method | Status |
|----|-------------|---------------------|--------|
| B1 | Existing CLI sessions continue to work | Regression test suite | ❌ |
| B2 | Existing tool call formats remain valid | API compatibility test | ❌ |
| B3 | Session file format unchanged | Session load test | ❌ |
| B4 | No breaking changes to public APIs | API diff check | ❌ |

---

## 7. Testing Checklist

### 7.1 Manual Testing

**CLI Mode (Local Execution):**
```bash
# Test 1: File operations
$ ./alex "Create file test.txt with 'hello world', then read it"
✓ File created locally at ./test.txt
✓ Content matches

# Test 2: Shell commands
$ ./alex "List all .go files in current directory"
✓ Output shows local files
✓ Command executed locally

# Test 3: Code execution
$ ./alex "Write a Python script that prints 'test' and execute it"
✓ Script executed locally
✓ Output captured

# Test 4: Search operations
$ ./alex "Find all occurrences of 'func main' in this directory"
✓ Grep/ripgrep executed locally
✓ Results accurate
```

**Server Mode (Sandbox Execution):**
```bash
# Test 1: Start server with sandbox
$ ALEX_SANDBOX_BASE_URL=http://localhost:8090 ./alex-server
✓ Server starts successfully
✓ Logs show "Sandbox initialized successfully"
✓ Health check passes

# Test 2: File operations via API
$ curl -X POST http://localhost:8080/api/tasks \
  -d '{"task": "Create file test.txt with content test", "session_id": "s1"}'
✓ Task executes successfully
✓ File NOT created locally
✓ File exists in sandbox (verify via sandbox API)

# Test 3: Shell commands via API
$ curl -X POST http://localhost:8080/api/tasks \
  -d '{"task": "Run command ls -la", "session_id": "s1"}'
✓ Command executed in sandbox
✓ Output shows sandbox filesystem

# Test 4: Error handling
$ # Stop sandbox service
$ curl -X POST http://localhost:8080/api/tasks \
  -d '{"task": "Create file test.txt", "session_id": "s1"}'
✓ Returns user-friendly error
✓ Error logged with details
```

### 7.2 Automated Testing

```bash
# Unit tests
$ go test ./internal/tools/... -v
✓ All dual-mode tools pass
✓ Sandbox manager tests pass
✓ 95%+ code coverage

# Integration tests
$ go test ./tests/integration/... -v
✓ CLI container uses local mode
✓ Server container uses sandbox mode
✓ Mode detection works correctly

# E2E tests
$ go test ./tests/e2e/... -v
✓ CLI E2E tests pass
✓ Server E2E tests pass
✓ No regression in existing features
```

---

## 8. Monitoring & Observability

### 8.1 Metrics to Track

```go
// Add to internal/tools/metrics.go
var (
    // Execution mode distribution
    toolExecutionMode = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "alex_tool_execution_mode_total",
            Help: "Total tool executions by mode",
        },
        []string{"mode", "tool_name"},
    )

    // Sandbox operation latency
    sandboxOperationDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "alex_sandbox_operation_duration_seconds",
            Help: "Sandbox operation duration",
            Buckets: prometheus.DefBuckets,
        },
        []string{"operation", "tool_name"},
    )

    // Sandbox errors
    sandboxErrors = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "alex_sandbox_errors_total",
            Help: "Total sandbox errors",
        },
        []string{"error_type", "tool_name"},
    )
)
```

### 8.2 Logging Strategy

```go
// Structured logging for sandbox operations
log.WithFields(log.Fields{
    "mode":      "sandbox",
    "tool":      "file_read",
    "operation": "read",
    "path":      filePath,
    "duration":  duration,
}).Info("Sandbox operation completed")
```

### 8.3 Health Checks

```go
// Add to server health check endpoint
func (h *HealthHandler) Check(ctx context.Context) error {
    // Check sandbox connectivity
    if err := h.sandboxManager.HealthCheck(ctx); err != nil {
        return fmt.Errorf("sandbox unhealthy: %w", err)
    }
    return nil
}
```

---

## 9. Risk Assessment & Mitigation

| Risk | Severity | Probability | Mitigation |
|------|----------|-------------|------------|
| Sandbox unavailable at startup | HIGH | MEDIUM | Fail fast with clear error, provide fallback instructions |
| Increased latency | MEDIUM | HIGH | Benchmark and optimize, set clear performance targets |
| Breaking changes to tool behavior | HIGH | LOW | Comprehensive testing, gradual rollout |
| Sandbox SDK bugs | MEDIUM | MEDIUM | Wrap SDK calls with error handling, version pinning |
| Memory leaks in long-running server | HIGH | LOW | Regular health checks, memory profiling |
| Session isolation issues | HIGH | LOW | Thorough security testing, sandbox audit |

---

## 10. Documentation Updates

### 10.1 User-Facing Documentation

**Files to update:**
- `README.md` - Add sandbox configuration section
- `docs/deployment/SERVER_DEPLOYMENT.md` - Add sandbox setup instructions
- `docs/configuration/ENVIRONMENT_VARIABLES.md` - Document `ALEX_SANDBOX_BASE_URL`

**New documentation:**
- `docs/architecture/SANDBOX_ARCHITECTURE.md` - Technical architecture overview
- `docs/troubleshooting/SANDBOX_ISSUES.md` - Common issues and solutions

### 10.2 Developer Documentation

**Files to update:**
- `CLAUDE.md` - Add sandbox migration section
- `docs/architecture/SPRINT_1-4_ARCHITECTURE.md` - Update tool execution flow
- `docs/development/TESTING_GUIDE.md` - Add sandbox testing guidelines

---

## 11. Success Metrics

### 11.1 Launch Metrics (Week 1)

- ✅ Server starts successfully with sandbox initialization
- ✅ 0 critical bugs reported
- ✅ All automated tests passing
- ✅ CLI continues to work without issues

### 11.2 Adoption Metrics (Month 1)

- ✅ 100% of web server traffic uses sandbox
- ✅ < 5% increase in average latency
- ✅ 0 security incidents related to sandbox
- ✅ < 1% error rate for sandbox operations

### 11.3 Long-term Metrics (Quarter 1)

- ✅ Sandbox enables new features (multi-tenancy, resource limits)
- ✅ Reduced security incidents by 50%
- ✅ Support for 10+ concurrent sessions per server
- ✅ Positive user feedback on isolation

---

## 12. Rollback Plan

### 12.1 Rollback Triggers

- Critical bugs affecting > 10% of users
- Performance degradation > 3x baseline
- Security vulnerability discovered
- Sandbox service unavailable > 1 hour

### 12.2 Rollback Procedure

1. **Immediate**: Deploy previous version binaries
2. **Short-term**: Disable sandbox mode via feature flag
3. **Long-term**: Fix issues and re-deploy gradually

### 12.3 Feature Flag Implementation

```go
// Add to config
type Config struct {
    // ... existing fields ...

    // ForceSandboxMode overrides automatic mode detection
    ForceSandboxMode *bool  // nil = auto, true = force sandbox, false = force local
}

// In DI container
func determineExecutionMode(config Config, processType string) tools.ExecutionMode {
    if config.ForceSandboxMode != nil {
        if *config.ForceSandboxMode {
            return tools.ExecutionModeSandbox
        }
        return tools.ExecutionModeLocal
    }

    // Auto-detect based on process type
    if processType == "server" && config.SandboxBaseURL != "" {
        return tools.ExecutionModeSandbox
    }
    return tools.ExecutionModeLocal
}
```

---

## 13. Timeline Summary

| Week | Focus | Deliverables | Risk |
|------|-------|--------------|------|
| W1 | Infrastructure | ExecutionMode, SandboxManager, DI updates | Low |
| W2 | File tools | file_read/write/edit/list migration | Medium |
| W3 | Shell tools | bash/grep/ripgrep/find migration | Medium |
| W4 | Testing & docs | E2E tests, documentation, benchmarks | Low |

**Total Duration**: 4 weeks
**Required Team**: 1 senior engineer
**Dependencies**: Stable sandbox service, SDK documentation

---

## 14. Open Questions

1. **Q**: Should we support fallback to local execution if sandbox fails?
   **A**: No for server mode (fail fast), N/A for CLI (always local)

2. **Q**: How do we handle sandbox session cleanup?
   **A**: Implement session lifecycle hooks in SandboxManager

3. **Q**: Do we need sandbox pooling for performance?
   **A**: Not in v1, monitor metrics and optimize in v2

4. **Q**: Should CLI support optional sandbox mode for testing?
   **A**: Yes, add `--sandbox` flag for advanced users (optional feature)

---

## Checklist (Completed)

All migration tasks were completed during the initial rollout and remain current.

- [x] Implement the `ExecutionMode` validation helper and ensure all constructors fail fast on invalid values.
- [x] Update the tool registry and DI wiring to propagate errors from `NewRegistry` instead of panicking.
- [x] Extend `SandboxManager` with the exported `HealthCheck` helper and ensure timeouts are expressed with `time.Duration`.
- [x] Guard sandbox-mode tool execution paths against missing managers and add corresponding unit tests.
- [x] Refresh integration tests to cover the new `Mode()` accessor used by diagnostics.
- [x] Document the health-check usage in server observability docs once implementation lands. Added guidance for invoking `SandboxManager.HealthCheck` within `docs/operations/monitoring_and_metrics.md`.
- [x] Wire up environment snapshots at startup (host + sandbox), inject them into the system prompt, and expose them through the diagnostics publisher.
- [x] Render the environment strip in the web header with `text-xs` styling and integration tests covering the banner.

---

## 15. References

- Sandbox SDK Documentation: `third_party/sandbox-sdk-go/README.md`
- Existing Architecture: `docs/architecture/SPRINT_1-4_ARCHITECTURE.md`
- Tool System: `internal/tools/README.md` (to be created)
- DI Container: `internal/di/container.go`
- Project Instructions: `CLAUDE.md`

---

**Document Version**: 1.0
**Last Updated**: 2025-10-28
**Author**: Claude Code
**Status**: DRAFT - Awaiting Approval
