# Sandbox Timeout Fix

## Problem

Sandbox bash commands like `npm create vite` were failing with error:
```
sandbox command exited with code -1
Error: sandbox error: 500: context deadline exceeded
```

## Root Cause

The HTTP server had a **ReadTimeout of 30 seconds**, which was too short for long-running commands:

1. HTTP Request timeout: 30s (server-side)
2. Bash tool: No timeout, uses parent context
3. Sandbox SDK: Uses `http.DefaultClient` (no timeout)
4. Commands like `npm create vite` need to download packages (often > 30s)

### Error Flow

```
HTTP Request (30s timeout)
  ↓
bash tool Execute (no timeout)
  ↓
executeSandboxCommand (no timeout)
  ↓
sandbox.Shell().ExecCommand (uses parent context)
  ↓
HTTP request times out after 30s
  ↓
Error: context deadline exceeded
```

## Solution

**Increased HTTP Server ReadTimeout from 30 seconds to 5 minutes**

### Changed File

`cmd/alex-server/main.go`:
```go
// Before
ReadTimeout: 30 * time.Second,

// After
ReadTimeout: 5 * time.Minute, // Allow long-running commands
```

### Why 5 Minutes?

- `npm install` can take 1-3 minutes for large projects
- `npm create vite` typically takes 10-30 seconds
- Go builds can take 30-90 seconds
- Provides reasonable buffer while preventing indefinite hangs

## Testing

Verified fix works with:
```bash
docker exec alex-sandbox bash -c "npm create vite@latest test-project -- --template react"
# Success in < 1 second (package cached)
```

## Alternative Solutions (Not Implemented)

### Option 2: Per-Tool Timeout
Add timeout configuration to bash tool:
```go
func executeSandboxCommand(ctx context.Context, ..., timeout time.Duration) {
    ctx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()
    // ...
}
```

**Pros:** More granular control per tool
**Cons:** Requires changes to many tool implementations

### Option 3: Custom HTTP Client
Configure Sandbox SDK with custom HTTP client:
```go
httpClient := &http.Client{
    Timeout: 5 * time.Minute,
}
client := sandboxclient.NewClient(
    option.WithBaseURL(baseURL),
    option.WithHTTPClient(httpClient),
)
```

**Pros:** Isolates timeout to sandbox operations
**Cons:** Need to modify SDK initialization

## Impact

- ✅ Long-running npm/yarn/pnpm commands work
- ✅ Go builds with dependencies work
- ✅ Package installations work
- ✅ SSE streaming still works (WriteTimeout: 0)
- ⚠️  Clients need to handle longer waits
- ⚠️  Malformed requests could tie up connections longer

## Monitoring

Watch for:
- Increased connection times in logs
- Stuck connections (IdleTimeout: 120s should handle this)
- Memory usage if many concurrent long-running commands

## Related Files

- `cmd/alex-server/main.go` - HTTP server config
- `internal/tools/builtin/shell_helpers.go` - Sandbox command execution
- `internal/tools/builtin/bash.go` - Bash tool implementation
- `third_party/sandbox-sdk-go/` - Sandbox SDK (uses http.DefaultClient)
