# Agent Preset Quick Reference
> Last updated: 2025-11-18


## Agent Presets (Personas)

| Preset | Description | Best For |
|--------|-------------|----------|
| `default` | General-purpose coding assistant | Mixed tasks, general development |
| `code-expert` | Code review, debugging, refactoring | Code quality, performance optimization |
| `researcher` | Information gathering, analysis, documentation | Research, codebase analysis, docs |
| `md` | Markdown Architect enforcing Explore/Code/Research/Build | Architecture docs, release notes, runbooks |
| `devops` | Deployment, infrastructure, CI/CD | Infrastructure, automation, deployment |
| `security-analyst` | Security audits, vulnerability detection | Security reviews, threat analysis |
| `designer` | Visual ideation and Seedream prompt engineering | Creative briefs, concept art, marketing visuals |

## Tool Presets (Access Levels)

| Preset | Allowed Tools | Blocked Tools | Best For |
|--------|---------------|---------------|----------|
| `full` | All tools | None | Complete development workflows |
| `read-only` | file_read, grep, ripgrep, find, list_files, web_search, web_fetch, think, todo_read, subagent | file_write, file_edit, bash, code_execute, todo_update | Code review, audits, analysis |
| `code-only` | file_*, grep, ripgrep, find, code_execute, think, todo_*, subagent | web_search, web_fetch, bash | Offline development, local work |
| `web-only` | web_search, web_fetch, think, todo_read | All file and execution tools | Pure research, web lookups |
| `safe` | All except bash and code_execute | bash, code_execute | Untrusted code, extra safety |

## Common Combinations

### Security Audit
```json
{
  "agent_preset": "security-analyst",
  "tool_preset": "read-only"
}
```
**Use**: Security code review without modifications

### Bug Fix
```json
{
  "agent_preset": "code-expert",
  "tool_preset": "full"
}
```
**Use**: Debug and fix issues with full access

### Research
```json
{
  "agent_preset": "researcher",
  "tool_preset": "web-only"
}
```
**Use**: Technology research and information gathering

### Infrastructure
```json
{
  "agent_preset": "devops",
  "tool_preset": "full"
}
```
**Use**: Infrastructure setup and deployment automation

### Code Review
```json
{
  "agent_preset": "code-expert",
  "tool_preset": "read-only"
}
```
**Use**: Review code quality without making changes

### Documentation
```json
{
  "agent_preset": "md",
  "tool_preset": "full"
}
```
**Use**: Author Markdown deliverables with Explore → Code → Research → Build traceability
**Workflow Notes**: Start with an outline/TODOs, cite every fact (file path, URL, or command), and close with the verification commands executed in the Build phase.

### Creative Concepting
```json
{
  "agent_preset": "designer",
  "tool_preset": "safe"
}
```
**Use**: Generate and refine Seedream imagery while avoiding risky execution tools

### Safe Refactoring
```json
{
  "agent_preset": "code-expert",
  "tool_preset": "safe"
}
```
**Use**: Refactor code without executing it

### Codebase Analysis
```json
{
  "agent_preset": "researcher",
  "tool_preset": "read-only"
}
```
**Use**: Deep dive into codebase structure

## API Usage

### Basic Request
```bash
curl -X POST http://localhost:3000/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Your task description",
    "agent_preset": "preset-name",
    "tool_preset": "preset-name"
  }'
```

### With Session
```bash
curl -X POST http://localhost:3000/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Your task description",
    "session_id": "sess-xyz",
    "agent_preset": "preset-name",
    "tool_preset": "preset-name"
  }'
```

## Tips

✅ **DO:**
- Match preset to task type (security → security-analyst)
- Start with restrictive tool access (`read-only`, `safe`)
- Use `read-only` for untrusted code review
- Use `security-analyst` + `read-only` for audits
- Use `researcher` + `web-only` for pure research
- Use `md` + `full` for Markdown deliverables that need Explore/Code/Research/Build traceability
- Capture citations (file paths, URLs, command output) inline when using `md`

❌ **DON'T:**
- Use `full` access for initial security audits
- Use `web-only` for infrastructure tasks
- Use `code-only` for research tasks
- Mix incompatible presets without purpose

## Validation

### Valid Agent Presets
- `default`
- `code-expert`
- `researcher`
- `md`
- `devops`
- `security-analyst`
- `designer`

### Valid Tool Presets
- `full`
- `read-only`
- `code-only`
- `web-only`
- `safe`

## Tool Access Matrix

| Tool | full | read-only | code-only | web-only | safe |
|------|------|-----------|-----------|----------|------|
| file_read | ✅ | ✅ | ✅ | ❌ | ✅ |
| file_write | ✅ | ❌ | ✅ | ❌ | ✅ |
| file_edit | ✅ | ❌ | ✅ | ❌ | ✅ |
| list_files | ✅ | ✅ | ✅ | ❌ | ✅ |
| bash | ✅ | ❌ | ❌ | ❌ | ❌ |
| code_execute | ✅ | ❌ | ✅ | ❌ | ❌ |
| grep | ✅ | ✅ | ✅ | ❌ | ✅ |
| ripgrep | ✅ | ✅ | ✅ | ❌ | ✅ |
| find | ✅ | ✅ | ✅ | ❌ | ✅ |
| web_search | ✅ | ✅ | ❌ | ✅ | ✅ |
| web_fetch | ✅ | ✅ | ❌ | ✅ | ✅ |
| think | ✅ | ✅ | ✅ | ✅ | ✅ |
| todo_read | ✅ | ✅ | ✅ | ✅ | ✅ |
| todo_update | ✅ | ❌ | ✅ | ❌ | ✅ |
| subagent | ✅ | ✅ | ✅ | ❌ | ✅ |

## Examples

### Example 1: Security Audit
```bash
curl -X POST http://localhost:3000/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Audit internal/auth/ for security vulnerabilities",
    "agent_preset": "security-analyst",
    "tool_preset": "read-only"
  }'
```

### Example 2: Performance Optimization
```bash
curl -X POST http://localhost:3000/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Optimize database queries in internal/db/",
    "agent_preset": "code-expert",
    "tool_preset": "full"
  }'
```

### Example 3: Technology Comparison
```bash
curl -X POST http://localhost:3000/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Compare GraphQL vs REST for our API",
    "agent_preset": "researcher",
    "tool_preset": "web-only"
  }'
```

### Example 4: Infrastructure Setup
```bash
curl -X POST http://localhost:3000/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Create Kubernetes deployment manifests",
    "agent_preset": "devops",
    "tool_preset": "full"
  }'
```

## Documentation

- **Full Guide**: `/docs/AGENT_PRESETS.md`
- **Examples**: `/examples/preset_examples.md`
- **Implementation**: `/docs/PRESET_SYSTEM_SUMMARY.md`
