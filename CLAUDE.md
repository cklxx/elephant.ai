# CLAUDE.md

## Project Overview

**ALEX - Agile Light Easy Xpert Code Agent v1.0** is a production-ready AI code agent built in Go with complete ReAct agent architecture, full MCP protocol implementation, intelligent memory management, SWE-Bench evaluation framework, and enterprise-grade security features.

## Essential Development Commands

### Building and Testing
```bash
# Build Alex
make build                    # Builds ./alex binary

# Development workflow
make dev                      # Format, vet, build, and test functionality
make test                     # Run all tests
make fmt                      # Format Go code
make vet                      # Run go vet
```

### Alex Usage
```bash
# Interactive mode (auto-detects TTY)
./alex                        # Auto-enters interactive mode
./alex -i                     # Explicit interactive mode

# Single prompt mode
./alex "Analyze the current directory structure"

# Session management
./alex -r session_id -i       # Resume session
./alex session list           # List sessions
./alex memory compress        # Compress session memory

# SWE-Bench evaluation
./alex run-batch --dataset.subset lite --workers 4 --output ./results

# Configuration
./alex config show            # Show configuration
```

## Architecture Overview

### Core Components

1. **ReAct Agent** (`internal/agent/`) - Think-Act-Observe cycle with streaming and memory
2. **MCP Protocol** (`internal/mcp/`) - Model Context Protocol with JSON-RPC 2.0
3. **Memory System** (`internal/memory/`, `internal/context/`) - Dual-layer with vector storage
4. **Tool System** (`internal/tools/`) - 13 built-in tools with MCP integration
5. **LLM Integration** (`internal/llm/`) - Multi-model support with caching
6. **Session Management** (`internal/session/`) - Persistent storage with compression
7. **SWE-Bench** (`evaluation/swe_bench/`) - Evaluation system with parallel processing
8. **Configuration** (`internal/config/`) - Multi-model config (default: OpenRouter + DeepSeek)

### Built-in Tools (13 total)
- **File Operations**: `file_read`, `file_update`, `file_replace`, `file_list` - Advanced file manipulation with path validation (4 tools)
- **Shell Execution**: `bash`, `code_execute` - Secure shell execution with sandbox controls and risk assessment (2 tools)
- **Search & Analysis**: `grep`, `ripgrep`, `find` - Intelligent search with context awareness and pattern matching (3 tools)
- **Task Management**: `todo_read`, `todo_update` - Session-persistent task tracking with markdown support (2 tools)
- **Web Integration**: `web_search` - Real-time web search with Tavily API integration (1 tool)
- **Reasoning**: `think` - Structured problem-solving and decision-making tool (1 tool)
- **MCP Protocol**: Dynamic external tool integration with JSON-RPC 2.0 and multi-transport support

### Security Features
- Risk assessment and path protection
- Command safety detection with sandbox execution
- Configurable restrictions and audit logging
- Multi-layered threat detection

### Advanced Features
- **Context Management**: Advanced memory compression with cache-friendly strategies and vector storage
- **Intelligent UI**: Clean diff display with syntax highlighting and real-time streaming
- **Task Management**: Session-based todo tracking with markdown support and persistence
- **Multi-Model LLM**: Intelligent model selection with automatic fallback and caching
- **Streaming Responses**: Real-time tool execution feedback with progress indicators
- **Enterprise Security**: Multi-layer threat detection, sandbox execution, and audit logging

## Performance
- **Lightning Fast**: Sub-30ms execution with intelligent caching
- **Concurrent Processing**: 10+ parallel tool execution with dependency analysis
- **Memory Efficient**: <100MB baseline memory with automatic cleanup and compression
- **Session Management**: File-based sessions with compression, backup, and restoration
- **Multi-Model Support**: Automatic model selection based on task complexity

## Code Principles

### Core Design Philosophy

**保持简洁清晰，如无需求勿增实体，尤其禁止过度配置**

- **Simplicity First**: Always choose the simplest solution that works
- **Clear Intent**: Code should be self-documenting through clear naming
- **Minimal Configuration**: Avoid configuration options unless absolutely necessary
- **Purposeful Entities**: Only create new types/interfaces when they serve a clear purpose

### Naming Guidelines
- **Functions**: `AnalyzeCode()`, `LoadPrompts()`, `ExecuteTool()`
- **Types**: `ReactAgent`, `PromptLoader`, `ToolExecutor`
- **Variables**: `taskResult`, `userMessage`, `promptTemplate`

### Architectural Principles
1. **Single Responsibility**: Each component has one clear purpose
2. **Minimal Dependencies**: Reduce coupling between components
3. **Clear Interfaces**: Define simple, focused interfaces
4. **Error Handling**: Fail fast with clear error messages
5. **No Over-Engineering**: Don't build for theoretical future needs

## Status
✅ **Production Ready v1.0** - Complete implementation with ReAct agent, full MCP protocol, dual-layer memory system, 13 built-in tools, SWE-Bench evaluation framework, session caching, modern terminal UI, enterprise security, and multi-platform NPM distribution

## Testing

```bash
# Test packages
go test ./internal/agent/ ./internal/tools/builtin/ ./internal/llm/ ./internal/memory/ ./internal/mcp/ ./internal/session/ ./evaluation/swe_bench/

# Quick tests
make test-functionality
make test-working

# Coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# SWE-Bench evaluation
./alex run-batch --dataset.subset lite --instance-limit 5 --workers 2
```