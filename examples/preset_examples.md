# Agent Preset Examples

This document demonstrates how to use ALEX agent presets with practical examples.

## Runtime modes

ALEX now separates tool access by runtime mode:

- **CLI (`alex`)**: `tool_preset` applies. Available presets are `full`, `read-only`, and `safe`.
- **Server (`alex-server`)**: runs in **web mode**. Local filesystem/shell tools are disabled, and `tool_preset` is ignored. All non-local tools are available.

If you need local code access, use the CLI mode.

---

## CLI examples (local code access)

### Example 1: Security Code Review (read-only)

```bash
alex config set tool_preset read-only
alex "Review the authentication code in internal/auth/ for security vulnerabilities. Check for SQL injection, authentication bypasses, and insecure password handling."
```

**What happens:**
- Agent uses security-focused analysis
- Only reads files (no modifications)
- Provides a detailed security report

### Example 2: Performance Optimization (full)

```bash
alex config set tool_preset full
alex "Analyze and optimize the database queries in internal/db/queries.go. Look for N+1 queries and add appropriate indexes."
```

**What happens:**
- Agent analyzes code for performance issues
- Can modify code to implement optimizations
- Adds tests to verify improvements

### Example 3: Safe Code Review (safe)

```bash
alex config set tool_preset safe
alex "Review the pull request changes in /tmp/pr-diff.txt. Analyze code quality, potential bugs, and suggest improvements."
```

**What happens:**
- Agent reviews code changes
- Cannot execute code (safe mode)
- Provides detailed feedback

### Example 4: Codebase Investigation (read-only)

```bash
alex config set tool_preset read-only
alex "Analyze the architecture of internal/agent/. Document the main components, their relationships, and summarize the design patterns used."
```

**What happens:**
- Agent analyzes code structure
- Documents architecture and patterns
- Read-only access prevents modifications

### Example 5: Refactor (full)

```bash
alex config set tool_preset full
alex "Refactor the internal/tools/builtin/file_*.go files to reduce code duplication. Extract common validation logic into shared utilities."
```

**What happens:**
- Agent identifies code duplication
- Extracts common patterns
- Maintains backward compatibility

---

## Server/web mode examples (non-local tools only)

Start the server first:

```bash
./alex-server
```

The server runs on `http://localhost:8080` by default.

### Example A: Technology Research

```bash
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Research and compare different state management solutions for React: Redux, MobX, Zustand, and Jotai. Provide pros/cons and a recommendation.",
    "agent_preset": "researcher"
  }'
```

**What happens:**
- Agent performs systematic research
- Uses web tools to gather information
- No local file system access

### Example B: Deployment Checklist Draft

```bash
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Create a production deployment checklist for running alex-server behind an nginx reverse proxy. Include systemd unit examples, health checks, and required environment variables.",
    "agent_preset": "devops"
  }'
```

**What happens:**
- Agent focuses on operational best practices
- Outputs a checklist + sample configs
- Avoids local filesystem mutations

---

## Monitoring Task Execution (server)

After creating a task, you will receive a response like:

```json
{
  "task_id": "task-abc123",
  "session_id": "sess-xyz789",
  "status": "pending"
}
```

### Check Task Status

```bash
curl http://localhost:8080/api/tasks/task-abc123
```

### Stream Events (SSE)

```bash
curl -N http://localhost:8080/api/events/sess-xyz789
```

---

## Combining Presets Effectively (CLI)

| Task Type | Agent Preset | Tool Preset | Why |
|-----------|--------------|-------------|-----|
| Security Audit | `security-analyst` | `read-only` | Safe analysis without modifications |
| Bug Fix | `code-expert` | `full` | Full capability to fix issues |
| Code Review | `code-expert` | `read-only` | Review without changes |
| Documentation | `researcher` | `read-only` | Analyze and document existing code |
| Untrusted Code Review | `code-expert` | `safe` | Avoid executing untrusted code |

---

## Tips for Best Results

1. **Be Specific**: Provide clear task descriptions
   - ✅ "Review auth.go for SQL injection vulnerabilities"
   - ❌ "Check the code"

2. **Match Preset to Task**: Choose the most relevant preset
   - Security tasks → `security-analyst`
   - Research → `researcher`
   - Infrastructure → `devops`
   - Code quality → `code-expert`

3. **Start Restrictive**: Begin with more restricted tool access
   - Start with `read-only` or `safe`
   - Upgrade to `full` if needed

---

## Advanced Patterns (CLI)

### Two-Phase Workflow

**Phase 1: Analysis**
```bash
alex config set tool_preset read-only
alex "Analyze security vulnerabilities in auth system"
```

**Phase 2: Fix**
```bash
alex config set tool_preset full
alex "Fix the security issues identified in the previous task"
```

---

## Troubleshooting

### Task Fails with "Tool Not Available" (CLI)

**Problem**: Agent tries to use a blocked tool

**Solution**: Use a less restrictive tool preset
```bash
alex config set tool_preset full
```

### Agent Behavior Doesn't Match Preset

**Problem**: Agent doesn't seem to use the preset

**Solution**:
1. Check preset spelling
2. Verify task matches preset expertise
3. Check logs for preset resolution

---

## Example Script (CLI)

Save this as `test-presets.sh`:

```bash
#!/bin/bash

set -euo pipefail

echo "1. Security Audit (read-only)..."
alex config set tool_preset read-only
alex "Audit internal/server/http/ for security issues"

echo "2. Code Review (read-only)..."
alex config set tool_preset read-only
alex "Review internal/tools/builtin/ for code quality"

echo "3. Safe Code Review (safe)..."
alex config set tool_preset safe
alex "Review the pull request changes in /tmp/pr-diff.txt"
```

Make it executable and run:

```bash
chmod +x test-presets.sh
./test-presets.sh
```
