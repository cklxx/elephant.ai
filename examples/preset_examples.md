# Agent Preset Examples

This document demonstrates how to use ALEX agent presets with practical examples.

## Prerequisites

1. Start the ALEX server:
```bash
./alex server
```

2. The server runs on `http://localhost:3000` by default

## Example 1: Security Code Review

Review code for security vulnerabilities using the security-analyst preset with read-only access.

```bash
curl -X POST http://localhost:3000/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Review the authentication code in internal/auth/ for security vulnerabilities. Check for SQL injection, authentication bypasses, and insecure password handling.",
    "agent_preset": "security-analyst",
    "tool_preset": "read-only"
  }'
```

**What happens:**
- Agent uses security-focused analysis
- Only reads files (no modifications)
- Checks for common vulnerabilities
- Provides detailed security report

## Example 2: Performance Optimization

Optimize code performance using the code-expert preset with full access.

```bash
curl -X POST http://localhost:3000/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Analyze and optimize the database queries in internal/db/queries.go. Look for N+1 queries and add appropriate indexes.",
    "agent_preset": "code-expert",
    "tool_preset": "full"
  }'
```

**What happens:**
- Agent analyzes code for performance issues
- Can modify code to implement optimizations
- Adds tests to verify improvements
- Focuses on maintainability

## Example 3: Technology Research

Research and compare technologies using the researcher preset.

```bash
curl -X POST http://localhost:3000/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Research and compare different state management solutions for React: Redux, MobX, Zustand, and Jotai. Provide pros/cons and recommendation.",
    "agent_preset": "researcher",
    "tool_preset": "web-only"
  }'
```

**What happens:**
- Agent performs systematic research
- Uses web search to gather information
- No file system access (web-only)
- Provides structured comparison with recommendations

## Example 4: Infrastructure Setup

Set up infrastructure using the devops preset.

```bash
curl -X POST http://localhost:3000/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Create a complete Docker Compose setup for our application with PostgreSQL, Redis, and Nginx. Include health checks and proper networking.",
    "agent_preset": "devops",
    "tool_preset": "full"
  }'
```

**What happens:**
- Agent uses DevOps best practices
- Creates docker-compose.yml with all services
- Sets up health checks and monitoring
- Includes documentation for deployment

## Example 5: Codebase Investigation

Investigate a codebase using the researcher preset.

```bash
curl -X POST http://localhost:3000/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Analyze the architecture of the internal/agent/ package. Document the main components, their relationships, and provide a summary of the design patterns used.",
    "agent_preset": "researcher",
    "tool_preset": "read-only"
  }'
```

**What happens:**
- Agent analyzes code structure
- Documents architecture and patterns
- Read-only access prevents modifications
- Creates comprehensive documentation

## Example 6: Code Refactoring

Refactor code using the code-expert preset.

```bash
curl -X POST http://localhost:3000/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Refactor the internal/tools/builtin/file_*.go files to reduce code duplication. Extract common validation logic into shared utilities.",
    "agent_preset": "code-expert",
    "tool_preset": "code-only"
  }'
```

**What happens:**
- Agent identifies code duplication
- Extracts common patterns
- No web access (focused on local code)
- Maintains backward compatibility

## Example 7: CI/CD Pipeline

Create CI/CD pipeline using the devops preset.

```bash
curl -X POST http://localhost:3000/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Create a GitHub Actions workflow for this Go project. Include: linting, testing, building, and Docker image publishing. Add caching for faster builds.",
    "agent_preset": "devops",
    "tool_preset": "full"
  }'
```

**What happens:**
- Agent creates .github/workflows/ci.yml
- Includes all necessary steps
- Optimizes with caching
- Follows CI/CD best practices

## Example 8: Security Audit

Comprehensive security audit using the security-analyst preset.

```bash
curl -X POST http://localhost:3000/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Perform a comprehensive security audit of the entire codebase. Check for: hardcoded secrets, unsafe dependencies, insecure configurations, and potential injection vulnerabilities.",
    "agent_preset": "security-analyst",
    "tool_preset": "read-only"
  }'
```

**What happens:**
- Systematic security analysis
- Checks for common vulnerabilities
- Scans dependencies for CVEs
- Provides prioritized findings

## Example 9: API Documentation

Create API documentation using the Markdown Architect preset so every section cites Explore/Code/Research/Build evidence.

```bash
curl -X POST http://localhost:3000/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Create comprehensive API documentation for all endpoints in internal/server/http/. Include request/response examples, error codes, acceptance criteria, and a changelog of Explore → Code → Research → Build steps.",
    "agent_preset": "md",
    "tool_preset": "full"
  }'
```

**What happens:**
- Agent inventories relevant files via `explore`, researches missing context, then edits Markdown in scoped chunks
- Documentation explicitly lists Explore/Code/Research/Build actions and cites file paths plus command outputs
- Verification steps (tests, curl invocations) are captured before closing the task

## Example 10: Safe Code Review

Review untrusted code safely.

```bash
curl -X POST http://localhost:3000/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Review the pull request changes in /tmp/pr-diff.txt. Analyze code quality, potential bugs, and suggest improvements.",
    "agent_preset": "code-expert",
    "tool_preset": "safe"
  }'
```

**What happens:**
- Agent reviews code changes
- Cannot execute code (safe mode)
- Provides detailed feedback
- No risk of running malicious code

## Monitoring Task Execution

After creating a task, you'll receive a response like:

```json
{
  "task_id": "task-abc123",
  "session_id": "sess-xyz789",
  "status": "pending"
}
```

### Check Task Status

```bash
curl http://localhost:3000/api/tasks/task-abc123
```

### Stream Events (SSE)

```bash
curl -N http://localhost:3000/api/events/sess-xyz789
```

This will stream real-time events as the agent works on the task.

## Combining Presets Effectively

### Best Combinations

| Task Type | Agent Preset | Tool Preset | Why |
|-----------|--------------|-------------|-----|
| Security Audit | `security-analyst` | `read-only` | Safe analysis without modifications |
| Bug Fix | `code-expert` | `full` | Full capability to fix issues |
| Research | `researcher` | `web-only` | Focus on information gathering |
| Infrastructure | `devops` | `full` | Need to create configs and scripts |
| Code Review | `code-expert` | `read-only` | Review without changes |
| Documentation | `researcher` | `read-only` | Analyze and document existing code |

### Anti-Patterns (Avoid These)

❌ `security-analyst` + `full` for initial audit (start with `read-only`)
❌ `devops` + `web-only` for infrastructure setup (needs file access)
❌ `researcher` + `code-only` for web research (needs web access)

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

4. **Use Sessions**: Reuse sessions for related tasks
   ```json
   {
     "task": "Now fix the issues found",
     "session_id": "sess-xyz789",
     "agent_preset": "code-expert",
     "tool_preset": "full"
   }
   ```

## Advanced Patterns

### Two-Phase Workflow

**Phase 1: Analysis**
```bash
# First, analyze with read-only access
curl -X POST http://localhost:3000/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Analyze security vulnerabilities in auth system",
    "agent_preset": "security-analyst",
    "tool_preset": "read-only"
  }'
```

**Phase 2: Fix**
```bash
# Then, fix with full access using the same session
curl -X POST http://localhost:3000/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Fix the security issues identified in the previous task",
    "session_id": "sess-from-phase-1",
    "agent_preset": "code-expert",
    "tool_preset": "full"
  }'
```

### Research Then Implement

**Step 1: Research**
```bash
curl -X POST http://localhost:3000/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Research best practices for rate limiting in Go APIs",
    "agent_preset": "researcher",
    "tool_preset": "web-only"
  }'
```

**Step 2: Implement**
```bash
curl -X POST http://localhost:3000/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Implement rate limiting based on the research findings",
    "session_id": "sess-from-research",
    "agent_preset": "code-expert",
    "tool_preset": "full"
  }'
```

## Troubleshooting

### Task Fails with "Tool Not Available"

**Problem**: Agent tries to use a blocked tool

**Solution**: Use a less restrictive tool preset
```bash
# Change from read-only to code-only or full
"tool_preset": "code-only"
```

### Agent Behavior Doesn't Match Preset

**Problem**: Agent doesn't seem to use the preset

**Solution**:
1. Check preset spelling
2. Verify task matches preset expertise
3. Check server logs for errors

### Need More Control

**Problem**: Built-in presets don't fit your needs

**Solution**: Combine presets creatively or request custom preset support

## Example Script

Save this as `test-presets.sh`:

```bash
#!/bin/bash

API_URL="http://localhost:3000/api/tasks"

echo "1. Security Audit (read-only)..."
curl -X POST $API_URL \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Audit internal/server/http/ for security issues",
    "agent_preset": "security-analyst",
    "tool_preset": "read-only"
  }'

echo "\n\n2. Code Review (code-expert)..."
curl -X POST $API_URL \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Review internal/tools/builtin/ for code quality",
    "agent_preset": "code-expert",
    "tool_preset": "read-only"
  }'

echo "\n\n3. Tech Research (researcher)..."
curl -X POST $API_URL \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Research Go testing frameworks: testify vs. ginkgo",
    "agent_preset": "researcher",
    "tool_preset": "web-only"
  }'
```

Make it executable and run:
```bash
chmod +x test-presets.sh
./test-presets.sh
```
