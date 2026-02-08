# Sandbox Container Recreated on Every `alex dev up`

**Date**: 2026-02-08
**Severity**: Medium (performance, not correctness)
**Component**: `internal/devops/services/sandbox.go`

## Symptom

Every `alex dev up` showed:
```
── sandbox ──
⚠ Sandbox container needs recreation
▸ Creating sandbox container alex-sandbox on :18086...
▸ Ensuring Codex + Claude Code inside sandbox...
```

Even though the container was already running and healthy.

## Root Cause

When `ACP_PORT=0` (default, auto-pick), the `Start()` method in `SandboxService`:

1. Calls `ports.Reserve("acp", 0)` which picks a **random** port (e.g. 23456)
2. Sets `s.config.ACPPort = 23456`
3. Calls `needsRecreate()` which checks if container has port 23456 mapped
4. Container has **old** random port (e.g. 31789) from previous creation
5. Mismatch → `needsRecreate()` returns `true` → container destroyed and rebuilt

Each `alex dev up` creates a new `Orchestrator` → new `port.Allocator` with no memory of previous allocations.

## Fix

Before allocating a new random ACP port, inspect the existing container's port mappings and reuse the ACP port that's already mapped:

```go
if exists {
    if existing := s.detectExistingACPPort(ctx); existing > 0 {
        acpPort = existing
    }
}
```

`detectExistingACPPort()` inspects the container and returns the first host port that isn't the sandbox port itself.

## Lesson

When using ephemeral state (random port allocation) to make decisions about persistent state (container port mappings), always check the persistent state first. The allocator is in-memory-only and loses state across process restarts.
