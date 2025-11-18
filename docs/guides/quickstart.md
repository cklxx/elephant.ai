# Alex - Quick Start Guide
> Last updated: 2025-11-18


## Overview

Alex (高性能普惠的软件工程助手) is a high-performance, universally accessible AI software engineering assistant that uses advanced ReAct (Reasoning and Acting) architecture with powerful tool calling capabilities. This guide will get you up and running quickly.

## Installation

### Prerequisites

- Go 1.21 or later
- Git
- Make (optional, for using Makefile commands)

### Quick Install

```bash
# Clone the repository
git clone https://github.com/your-org/alex.git
cd alex

# Build the agent
make build

# Or build manually
go build -o alex ./cmd/main.go
```

### Using Install Script

```bash
# Run the installation script
./scripts/install.sh

# This will:
# - Install dependencies
# - Build the binary
# - Set up configuration
# - Optionally add to PATH
```

## Configuration

### Initial Setup

```bash
# Initialize with default configuration
./alex --init

# Or manually configure
./alex config --init
```

### AI Provider Configuration

Configure your AI provider (OpenAI or compatible):

```bash
# Set OpenAI API key
./alex config --set openaiApiKey=your-api-key-here

# Or set ARK API key (ByteDance alternative)
./alex config --set arkApiKey=your-ark-key-here

# Set custom API endpoint (optional)
./alex config --set apiBaseURL=https://your-custom-endpoint.com

# Set model (optional)
./alex config --set apiModel=gpt-4-turbo
```

### Basic Configuration Options

```bash
# Enable ReAct mode (recommended)
./alex config --set reactMode=true

# Set max iterations for complex tasks
./alex config --set reactMaxIterations=10

# Enable thinking process display
./alex config --set reactThinkingEnabled=true

# Configure allowed tools
./alex config --set allowedTools=file_read,file_write,file_list,bash
```

## Basic Usage

### Interactive Mode

Start an interactive session for conversational coding assistance:

```bash
# Start interactive mode
./alex -i

# In interactive mode, you can:
# - Ask questions about your code
# - Request file analysis
# - Get help with debugging
# - Generate code snippets
```

### Single Command Mode

Execute one-off commands:

```bash
# Analyze current directory
./alex "Analyze the project structure and provide insights"

# Read and explain a specific file
./alex "Read main.go and explain what it does"

# Get help with a specific task
./alex "How can I optimize this Go code for better performance?"
```

### JSON Output

Get structured responses for integration with other tools:

```bash
# JSON format output
./alex --format json "List all Go files in this project"

# Pipe to jq for processing
./alex --format json "Analyze code quality" | jq '.data.metrics'
```

## Common Use Cases

### 1. Project Analysis

```bash
# Analyze entire project structure
./alex -i
> "Analyze this project's architecture and suggest improvements"

# Check for code smells
> "Scan the codebase for potential issues and code smells"

# Review test coverage
> "Check test coverage and suggest areas that need more testing"
```

### 2. Code Generation

```bash
# Generate a new feature
> "Create a REST API handler for user authentication in Go"

# Generate tests
> "Generate unit tests for the user service in internal/user/service.go"

# Create documentation
> "Generate API documentation for the endpoints in main.go"
```

### 3. Debugging and Optimization

```bash
# Debug performance issues
> "Analyze the performance of this Go application and suggest optimizations"

# Find and fix bugs
> "Help me debug why this function is not working as expected"

# Code review
> "Review the code in internal/handlers/ and suggest improvements"
```

### 4. File Operations

```bash
# Read specific files
> "Read the README.md file and summarize the project"

# Search in files
> "Search for all TODO comments in Go files"

# List project files
> "List all files in the src directory recursively"
```

### 5. Git and Repository Management

```bash
# Check git status
> "Show me the current git status and any uncommitted changes"

# Analyze git history
> "Analyze recent commits and suggest a changelog entry"

# Help with git workflow
> "Help me create a proper commit message for these changes"
```

## Advanced Features

### ReAct Mode

ReAct (Reasoning and Acting) mode enables the agent to think through problems step by step:

```bash
# Enable ReAct mode with thinking display
./alex config --set reactMode=true --set reactThinkingEnabled=true

# The agent will show its thinking process:
# 🤔 Thinking: The user wants to analyze the project structure...
# 🛠️ Action: I'll use the file_list tool to see the directory structure
# 📋 Observation: Found 15 Go files and 3 directories...
# 🤔 Thinking: Based on the structure, this appears to be a web service...
```

### Tool Restrictions

Control which tools the agent can use for security:

```bash
# Restrict to read-only operations
ALLOWED_TOOLS="file_read,file_list" ./alex "Analyze the codebase"

# Allow file operations but not command execution
ALLOWED_TOOLS="file_read,file_write,file_list" ./alex "Help me refactor this code"
```

### Session Management

```bash
# Start named session
./alex --session-id myproject "Start working on the authentication feature"

# Resume previous session
./alex --session-id myproject "Continue with the authentication work"

# List active sessions
./alex --list-sessions
```

## Configuration Files

### Main Configuration

Location: `~/.alex-config.json`

```json
{
  "aiProvider": "openai",
  "openaiApiKey": "your-key-here",
  "reactMode": true,
  "reactMaxIterations": 10,
  "allowedTools": ["file_read", "file_write", "file_list", "bash"],
  "maxTokens": 2000,
  "temperature": 0.3
}
```

### Tool Configuration

```json
{
  "toolsConfig": {
    "maxConcurrentExecutions": 5,
    "defaultTimeout": 30000,
    "securityConfig": {
      "enableSandbox": false,
      "allowedTools": ["file_read", "file_list", "file_write", "bash"],
      "restrictedTools": ["rm", "format", "dd"]
    }
  }
}
```

## Development Mode

For development and testing:

```bash
# Start the local stack (server + web UI)
./deploy.sh

# Run development workflow (format, vet, build, test)
make dev

# Run tests with coverage
make test

# Format code
make fmt
```

## Troubleshooting

### Common Issues

1. **API Key Not Configured**
   ```bash
   Error: OpenAI API key not configured
   Solution: ./alex config --set openaiApiKey=your-key
   ```

2. **Tool Execution Denied**
   ```bash
   Error: Tool 'bash' is restricted by security policy
   Solution: ./alex config --set allowedTools=file_read,file_list,bash
   ```

3. **File Permission Errors**
   ```bash
   Error: Permission denied accessing /restricted/path
   Solution: Check file permissions or use allowed directories
   ```

### Debug Mode

```bash
# Enable debug logging
./alex --debug "Analyze this file"

# Verbose output
./alex --verbose "Run comprehensive analysis"
```

### Check Configuration

```bash
# View current configuration
./alex config --show

# Validate configuration
./alex config --validate

# Reset to defaults
./alex config --reset
```

## Integration Examples

### CI/CD Integration

```yaml
# GitHub Actions example
- name: Code Analysis
  run: |
    ./alex --format json "Analyze code quality and security" > analysis.json
    
- name: Generate Documentation
  run: |
    ./alex "Generate API documentation" > docs/api.md
```

### IDE Integration

```bash
# VS Code task example
{
  "label": "Deep Coding Analysis",
  "type": "shell",
  "command": "./alex",
  "args": ["Analyze current file and suggest improvements"],
  "group": "build"
}
```

### Git Hooks

```bash
# Pre-commit hook
#!/bin/sh
./alex "Review staged changes for potential issues"
```

## Performance Tips

1. **Use specific commands** instead of broad requests for faster responses
2. **Enable caching** for repeated operations: `--enable-cache`
3. **Limit tool scope** to only what you need: `ALLOWED_TOOLS="file_read,file_list"`
4. **Use JSON output** for programmatic processing: `--format json`
5. **Set reasonable timeouts** for long operations: `--timeout 60`

## Next Steps

- Read the [Architecture Documentation](AGENT_ARCHITECTURE.md) for deeper understanding
- Check the [API Reference](API_REFERENCE.md) for detailed tool documentation
- Explore [Tool Development Guide](TOOL_DEVELOPMENT_GUIDE.md) for creating custom tools
- Review [Security Best Practices](../internal/security/README.md) for production use

## Getting Help

```bash
# Built-in help
./alex --help

# Tool-specific help
./alex tools --help

# Configuration help
./alex config --help

# Interactive help
./alex -i
> "How do I configure the agent for my specific use case?"
```

---

*For more examples and detailed documentation, see the complete documentation in the `docs/` directory.*
