# ALEX Agent Presets

Agent presets allow you to configure ALEX with specialized personas and tool access levels for different use cases. This enables you to optimize the agent's behavior for specific tasks like code review, research, security analysis, or DevOps operations.

## Overview

The preset system consists of two orthogonal dimensions:

1. **Agent Presets**: Different system prompts that define the agent's persona, expertise, and approach
2. **Tool Presets**: Different tool access levels that control which tools the agent can use

These can be combined independently to create customized agent configurations.

## Agent Presets

Agent presets define the agent's persona and specialized knowledge.

### Available Agent Presets

#### `default` - Default Agent
**Description**: General-purpose coding assistant

**Use Cases**:
- General development tasks
- Mixed workflows requiring versatility
- When you're not sure which preset to use

**Characteristics**:
- Balanced approach to all coding tasks
- Full-stack development support
- No specific domain bias

---

#### `code-expert` - Code Expert
**Description**: Specialized in code review, debugging, and refactoring

**Use Cases**:
- Code review and quality analysis
- Debugging complex issues
- Refactoring and optimization
- Performance analysis

**Characteristics**:
- Focus on code quality metrics
- Systematic bug diagnosis approach
- Emphasis on maintainability and best practices
- Comprehensive review checklists

**Review Checklist**:
- ✅ Correctness: Does the code work as intended?
- ✅ Readability: Is the code clear and well-documented?
- ✅ Performance: Are there any bottlenecks?
- ✅ Security: Are there vulnerabilities?
- ✅ Testing: Is the code adequately tested?
- ✅ Maintainability: Will this be easy to modify?

---

#### `researcher` - Researcher
**Description**: Specialized in information gathering, analysis, and documentation

**Use Cases**:
- Codebase investigation and analysis
- Technology research and evaluation
- Creating comprehensive documentation
- Competitive analysis
- Knowledge synthesis from multiple sources

**Characteristics**:
- Extensive use of search and analysis tools
- Focus on synthesizing information
- Structured documentation output
- Evidence-based recommendations

**Research Methodology**:
1. Define scope and objectives
2. Gather information from multiple sources
3. Analyze patterns and trends
4. Synthesize findings
5. Document results with actionable recommendations

---

#### `devops` - DevOps Engineer
**Description**: Specialized in deployment, infrastructure, and CI/CD

**Use Cases**:
- Deployment automation
- Infrastructure as Code
- CI/CD pipeline design
- Monitoring and observability setup
- Cloud platform configuration
- Container orchestration

**Characteristics**:
- Focus on automation and reliability
- Security-first approach
- Scalability considerations
- Comprehensive monitoring

**DevOps Checklist**:
- ✅ Automation: Minimize manual intervention
- ✅ Reliability: High availability and fault tolerance
- ✅ Security: Least privilege and secret rotation
- ✅ Monitoring: Metrics and alerting
- ✅ Documentation: Deployment procedures
- ✅ Rollback: Failure recovery plans

---

#### `security-analyst` - Security Analyst
**Description**: Specialized in security audits and vulnerability detection

**Use Cases**:
- Security code review
- Vulnerability assessment
- Threat modeling
- Compliance verification
- Security incident investigation
- Dependency auditing

**Characteristics**:
- Focus on security vulnerabilities
- Systematic threat analysis
- Prefer read-only tools by default
- Evidence-based security recommendations

**Security Audit Checklist**:
- ✅ Authentication: Proper user verification
- ✅ Authorization: Correct access control
- ✅ Input Validation: Sanitize all inputs
- ✅ Secrets Management: No hardcoded credentials
- ✅ Encryption: Data protection at rest and in transit
- ✅ Dependencies: Check for CVEs
- ✅ Error Handling: No sensitive info in errors
- ✅ Logging: Security events tracked

**Common Vulnerabilities Checked**:
- SQL Injection, XSS, CSRF
- Path traversal, arbitrary file access
- Insecure deserialization
- Broken authentication/authorization
- Security misconfiguration
- Sensitive data exposure
- Insufficient logging and monitoring

---

## Tool Presets

Tool presets control which tools the agent can access.

### Available Tool Presets

#### `full` - Full Access
**Description**: All tools available - unrestricted access

**Available Tools**: All built-in tools

**Use Cases**:
- Complete development workflows
- When you need maximum flexibility
- Trusted environments

---

#### `read-only` - Read-Only Access
**Description**: Only read operations - no modifications allowed

**Available Tools**:
- `file_read` - Read file contents
- `list_files` - List directory contents
- `grep` - Search in files
- `ripgrep` - Fast text search
- `find` - Find files by name
- `web_search` - Web search
- `web_fetch` - Fetch web content
- `think` - Reasoning tool
- `todo_read` - Read task list
- `subagent` - Delegate to sub-agent

**Blocked Tools**:
- `file_write`, `file_edit` - No file modifications
- `bash`, `code_execute` - No command execution
- `todo_update` - No task modifications

**Use Cases**:
- Code review and analysis
- Security audits
- Research and investigation
- When you want to prevent accidental modifications

---

#### `code-only` - Code Operations
**Description**: File operations and code execution - no web access

**Available Tools**:
- All file operations (read, write, edit)
- All search tools (grep, ripgrep, find)
- Code execution (`code_execute`)
- Task management tools
- Reasoning and subagent

**Blocked Tools**:
- `web_search`, `web_fetch` - No web access
- `bash` - No arbitrary shell commands

**Use Cases**:
- Offline development
- Air-gapped environments
- When you want to prevent web access
- Focus on local code operations

---

#### `web-only` - Web Access
**Description**: Web search and fetch only - no file system access

**Available Tools**:
- `web_search` - Web search
- `web_fetch` - Fetch web content
- `think` - Reasoning tool
- `todo_read` - Read tasks

**Blocked Tools**:
- All file operations
- All execution tools
- All git tools
- Task modifications

**Use Cases**:
- Pure research tasks
- When you only need information gathering
- API documentation lookup
- Technology research

---

#### `safe` - Safe Mode
**Description**: Excludes potentially dangerous tools (bash, code execution)

**Available Tools**:
- All file operations
- All search tools
- Git tools
- Web tools
- Task management
- Subagent

**Blocked Tools**:
- `bash` - No arbitrary shell commands
- `code_execute` - No code execution

**Use Cases**:
- Untrusted code review
- When you want extra safety
- Shared environments
- Teaching/demonstration scenarios

---

## Using Presets

### API Usage

When creating a task via the API, you can specify presets:

```json
POST /api/tasks
Content-Type: application/json

{
  "task": "Review this code for security vulnerabilities",
  "agent_preset": "security-analyst",
  "tool_preset": "read-only"
}
```

### Example Combinations

#### Security Code Review
```json
{
  "task": "Audit the authentication system for security issues",
  "agent_preset": "security-analyst",
  "tool_preset": "read-only"
}
```
- Uses security-focused analysis approach
- Read-only access prevents accidental modifications
- Perfect for audits and reviews

#### DevOps Infrastructure Setup
```json
{
  "task": "Set up CI/CD pipeline with GitHub Actions",
  "agent_preset": "devops",
  "tool_preset": "full"
}
```
- Uses DevOps best practices
- Full tool access for creating configs and scripts
- Focus on automation and reliability

#### Technology Research
```json
{
  "task": "Research and compare React state management libraries",
  "agent_preset": "researcher",
  "tool_preset": "web-only"
}
```
- Systematic research approach
- Web-only tools for gathering information
- Structured documentation output

#### Code Refactoring
```json
{
  "task": "Refactor the authentication module for better performance",
  "agent_preset": "code-expert",
  "tool_preset": "code-only"
}
```
- Expert code analysis and optimization
- Code operations only, no web distractions
- Focus on quality and performance

#### General Development
```json
{
  "task": "Add user profile feature with avatar upload",
  "agent_preset": "default",
  "tool_preset": "full"
}
```
- General-purpose approach
- All tools available
- Standard development workflow

---

## Preset Details

### How Presets Work

1. **Agent Presets**: Replace the default system prompt with a specialized one
   - Each preset has a unique persona and approach
   - Defines the agent's expertise and methodology
   - Affects how the agent thinks about and solves problems

2. **Tool Presets**: Filter available tools at the registry level
   - Tools not in the allowed list will not be visible to the agent
   - Prevents accidental use of blocked tools
   - Enforced at the execution layer

3. **Context-Based**: Presets are passed via request context
   - Takes priority over default configuration
   - Per-request customization
   - No global state changes

### Combining Presets

Agent and tool presets are orthogonal and can be combined freely:

| Agent Preset | Tool Preset | Use Case |
|--------------|-------------|----------|
| `security-analyst` | `read-only` | Security audit without modifications |
| `security-analyst` | `code-only` | Security fixes and patches |
| `code-expert` | `read-only` | Code review |
| `code-expert` | `full` | Code review with fixes |
| `researcher` | `web-only` | Pure research |
| `researcher` | `read-only` | Codebase analysis |
| `devops` | `full` | Infrastructure setup |
| `devops` | `code-only` | Config file generation |

---

## Best Practices

### Choosing the Right Preset

1. **Match the Task**: Choose the preset that best matches your task domain
   - Code quality → `code-expert`
   - Investigation → `researcher`
   - Infrastructure → `devops`
   - Security → `security-analyst`

2. **Start Restrictive**: Begin with more restrictive tool access and expand if needed
   - Use `read-only` for reviews and audits
   - Use `safe` when you're unsure
   - Use `full` only when necessary

3. **Combine Thoughtfully**: Consider both agent behavior and tool access
   - Security audits should use `read-only` by default
   - DevOps tasks usually need `full` access
   - Research can often work with `web-only`

### Security Considerations

1. **Sensitive Operations**: Use restrictive presets for sensitive code
   - `read-only` for untrusted code review
   - `safe` to prevent code execution
   - `security-analyst` for security-critical reviews

2. **Production Systems**: Be cautious with full access
   - Prefer `safe` or `read-only` in production
   - Use `code-only` to prevent web access
   - Always review agent actions

3. **Secrets and Credentials**:
   - All presets respect security best practices
   - No preset will expose secrets
   - Use environment variables for sensitive data

---

## API Reference

### CreateTaskRequest

```typescript
interface CreateTaskRequest {
  task: string;                    // Task description
  session_id?: string;             // Optional session ID
  agent_preset?: string;           // Optional agent preset
  tool_preset?: string;            // Optional tool preset
}
```

### Valid Preset Values

**Agent Presets**:
- `default`
- `code-expert`
- `researcher`
- `devops`
- `security-analyst`

**Tool Presets**:
- `full`
- `read-only`
- `code-only`
- `web-only`
- `safe`

### Task Response

The task response includes the presets used:

```typescript
interface Task {
  task_id: string;
  session_id: string;
  status: string;
  task: string;
  agent_preset?: string;           // Agent preset used
  tool_preset?: string;            // Tool preset used
  // ... other fields
}
```

---

## Examples

### Example 1: Security Audit

```bash
curl -X POST http://localhost:3000/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Audit the user authentication code for security vulnerabilities",
    "agent_preset": "security-analyst",
    "tool_preset": "read-only"
  }'
```

**Expected Behavior**:
- Agent uses security-focused analysis
- Checks for common vulnerabilities (SQL injection, XSS, etc.)
- Only reads files, doesn't modify
- Provides detailed security report with recommendations

---

### Example 2: Infrastructure as Code

```bash
curl -X POST http://localhost:3000/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Create a Docker Compose file for our microservices architecture",
    "agent_preset": "devops",
    "tool_preset": "full"
  }'
```

**Expected Behavior**:
- Agent uses DevOps best practices
- Creates docker-compose.yml with proper configuration
- Sets up networking, volumes, health checks
- Includes monitoring and logging setup

---

### Example 3: Technology Research

```bash
curl -X POST http://localhost:3000/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Compare GraphQL vs REST APIs for our new service",
    "agent_preset": "researcher",
    "tool_preset": "web-only"
  }'
```

**Expected Behavior**:
- Agent performs systematic research
- Searches for best practices and comparisons
- Analyzes pros/cons of each approach
- Provides structured recommendation with evidence

---

### Example 4: Code Review and Fix

```bash
curl -X POST http://localhost:3000/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Review and optimize the database query performance in user service",
    "agent_preset": "code-expert",
    "tool_preset": "full"
  }'
```

**Expected Behavior**:
- Agent analyzes code for performance issues
- Identifies inefficient queries and N+1 problems
- Implements optimizations (indexing, query refactoring)
- Adds tests to verify improvements

---

## Troubleshooting

### Preset Not Applied

**Issue**: The agent doesn't seem to use the specified preset

**Solutions**:
1. Verify preset name is spelled correctly
2. Check server logs for preset validation errors
3. Ensure preset is supported in your ALEX version
4. Try with default preset first to isolate issues

### Tool Access Denied

**Issue**: Agent reports tools are not available

**Solutions**:
1. Check if the tool is blocked by your tool preset
2. Review the allowed tools for your preset in this documentation
3. Consider using a less restrictive preset if appropriate
4. Use `full` preset for debugging

### Unexpected Behavior

**Issue**: Agent behavior doesn't match preset description

**Solutions**:
1. Review the task description - unclear tasks may confuse any preset
2. Check if task matches the preset's expertise area
3. Try a different preset combination
4. Review agent logs for insights into decision-making

---

## Advanced Usage

### Custom Presets (Future Enhancement)

In future versions, you'll be able to define custom presets:

```yaml
# .alex/presets/my-preset.yaml
name: my-custom-preset
description: Custom preset for my team
system_prompt: |
  You are a specialized agent for...

allowed_tools:
  - file_read
  - file_write
  - grep
```

### Preset Inheritance (Future Enhancement)

Future versions may support preset inheritance:

```yaml
# Inherit from built-in preset and customize
extends: code-expert
tool_preset: safe
additional_instructions: |
  Focus on TypeScript best practices
```

---

## Contributing

To add new presets to ALEX:

1. Define the agent preset in `/internal/agent/presets/prompts.go`
2. Define the tool preset in `/internal/agent/presets/tools.go`
3. Add documentation in this file
4. Add tests for the new preset
5. Update API documentation

---

## Changelog

### Version 1.0.0
- Initial preset system implementation
- 5 agent presets: default, code-expert, researcher, devops, security-analyst
- 5 tool presets: full, read-only, code-only, web-only, safe
- API integration with task creation
- Comprehensive documentation
