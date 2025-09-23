# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**ALEX - Agile Light Easy Xpert Code Agent** is a production-ready terminal-native AI programming agent built in Go. It features ReAct agent architecture, MCP protocol implementation, intelligent memory management, and SWE-Bench evaluation framework.

## Essential Development Commands

```bash
# Build and Development
make build                    # Builds ./alex binary
make dev                      # Format, vet, build, test functionality (main workflow)
make dev-safe                 # Excludes broken tests
make dev-robust              # Includes dependency management

# Testing
make test                     # Run all tests
make test-working            # Run only working tests
make test-functionality      # Quick test of core functionality
go test ./internal/agent/    # Test specific package

# Comprehensive Testing Framework
go test ./internal/tools/builtin/ -v    # Test builtin tools (file_read, todo_read, todo_update)
go test ./internal/agent/ -v            # Test ReAct agent and tool registry
go test ./internal/context/ -v          # Test message compression and token estimation
go test ./internal/session/ -v          # Test session management and persistence

# Code Quality
make fmt                      # Format Go code
make vet                      # Run go vet
make clean                    # Clean build artifacts
```

### SWE-Bench Evaluation

```bash
# Test evaluation system
make swe-bench-verified-test       # Test with 3 real instances
cd evaluation/swe_bench && ./run_evaluation.sh real-test

# Run evaluations
make swe-bench-verified-small      # 50 instances
make swe-bench-verified-medium     # 150 instances
make swe-bench-verified-full       # 500 instances

# Batch processing
./alex run-batch --dataset.subset lite --instance-limit 5 --workers 2
```

## Architecture Overview

### Directory Structure

```
internal/
├── agent/           # ReAct agent with Think-Act-Observe cycle
├── llm/            # Multi-model LLM factory with caching
├── tools/          # Tool system with built-in tools
│   └── builtin/    # Core tool implementations
│   └── mcp/        # MCP protocol tools
├── context/        # Context management and compression
├── mcp/           # MCP protocol implementation
│   ├── protocol/  # JSON-RPC 2.0 layer
│   └── transport/ # STDIO and SSE transports
├── session/       # Persistent session management
├── config/        # Configuration management
└── prompts/       # Markdown-based prompt templates

evaluation/
└── swe_bench/     # SWE-Bench evaluation framework
```

### Built-in Tools

Located in `internal/tools/builtin/`, each tool has its own implementation file:
- **File Operations**: `file_read`, `file_update`, `file_replace`, `file_list` - Advanced file manipulation with path validation
- **Shell Execution**: `bash`, `code_execute` - Secure shell execution with sandbox controls and risk assessment
- **Search & Analysis**: `grep`, `ripgrep`, `find` - Intelligent search with context awareness and pattern matching
- **Task Management**: `todo_read`, `todo_update` - Session-persistent task tracking with markdown support
- **Web Integration**: `web_search`, `web_fetch` - Real-time web search and fetch capabilities
- **Reasoning**: `think` - Structured problem-solving and decision-making tool
- **Background Tasks**: Background command execution with monitoring
- **MCP Protocol**: Dynamic external tool integration with JSON-RPC 2.0 and multi-transport support

## Key Implementation Details

### ReAct Agent Architecture (`internal/agent/`)
- Core ReAct loop implementation in `core.go` and `react_agent.go`
- Task execution with Think-Act-Observe cycle
- Stream-based processing with callback support
- Tool call parsing and execution management
- Session-aware message handling

### LLM Integration (`internal/llm/`)
- Multi-model factory pattern supporting OpenAI, DeepSeek, OpenRouter, Ollama
- Session-aware caching for reduced API calls
- Automatic model selection based on task type (BasicModel, ReasoningModel)
- Streaming client support for real-time responses
- Default configuration uses OpenRouter with DeepSeek models

### Tool System (`internal/tools/`)
- Unified tool registry with dynamic registration
- Built-in tools with comprehensive validation
- MCP protocol integration for external tools
- Sub-agent tool for complex task delegation
- Path resolution and security validation

### Prompt System (`internal/prompts/`)
- Markdown-based templates: `initial.md`, `coder.md`, `enhanced_coder.md`, `improved_fallback.md`
- Dynamic prompt loading with context injection
- Specialized prompts for different agent modes
- Light prompt builder for efficient processing

### Session Management (`internal/session/`)
- File-based persistence in `~/.alex-sessions/`
- Automatic compression when context exceeds limits
- Todo tracking integrated with session state
- Resume capability with `-r session_id` flag
- Message queue for handling multiple requests

### MCP Protocol (`internal/tools/mcp/`)
- Full Model Context Protocol implementation
- JSON-RPC 2.0 protocol handling
- STDIO and SSE transport support
- Server spawning and lifecycle management
- Security controls and sandboxing

## Configuration

### Config File Location
`~/.alex-config.json` - Created automatically on first run

### Environment Variables
Precedence: Environment > Config File > Defaults

```bash
export OPENAI_API_KEY="your-openrouter-key"  # API key
export ALLOWED_TOOLS="file_read,bash,grep"   # Tool restrictions
export USE_REACT_AGENT="true"                # Force ReAct mode
```

### Default Multi-Model Configuration
- **basic_model**: DeepSeek Chat for general tasks and tool calling
- **reasoning_model**: DeepSeek R1 for complex problem-solving
- Base URL: `https://openrouter.ai/api/v1`
- Supports Ollama for local model execution

### MCP Configuration
- Server configurations in config file
- Security policies for command execution
- Logging and monitoring settings
- Auto-start and restart capabilities

## SWE-Bench Evaluation System

### Evaluation Framework (`evaluation/swe_bench/`)
- Complete SWE-Bench integration with agent, batch processing, and monitoring
- Support for lite (300), full (2294), and verified (500) datasets
- Real instance testing with `real_instances.json`
- Batch processing with configurable workers and timeouts
- Result tracking and analysis
- Performance monitoring and metrics

### Running Evaluations
```bash
# Quick test with real instances
cd evaluation/swe_bench && ./run_evaluation.sh real-test

# Different dataset sizes
./run_evaluation.sh quick-test    # 5 instances
./run_evaluation.sh small-batch   # 50 instances
./run_evaluation.sh medium-batch  # 150 instances
./run_evaluation.sh full          # 500 instances
```

## Code Principles

**Core Philosophy**: 保持简洁清晰，如无需求勿增实体，尤其禁止过度配置

- **Simplicity First**: Choose the simplest solution that works
- **Clear Naming**: Self-documenting code through descriptive names
- **Minimal Configuration**: Avoid options unless essential
- **No Over-Engineering**: Build for current needs, not theoretical futures

### Naming Conventions
- Functions: `AnalyzeCode()`, `LoadPrompts()`, `ExecuteTool()`
- Types: `ReactAgent`, `PromptLoader`, `ToolExecutor`
- Variables: `taskResult`, `userMessage`, `promptTemplate`

### Key Interfaces
- `ReactCoreInterface` - Core ReAct functionality
- `ToolExecutor` - Tool execution abstraction
- `StreamCallback` - Streaming response handling
- `llm.Client` - LLM client interface

## Testing Strategy and Quality Assurance

### Current Test Coverage (2025-01)
```
✅ Agent System (internal/agent/)
├── ReactAgent: Comprehensive unit tests with dependency injection
├── ToolRegistry: Dynamic registration and caching tests
├── ParallelSubAgent: Concurrency and workflow tests
└── Interfaces: Mock implementations and contract validation

✅ Builtin Tools (internal/tools/builtin/)
├── file_read: 11 tests covering file operations, Go AST analysis, edge cases
├── todo_read: 10 tests covering session management and file operations
└── todo_update: 13 tests covering validation, JSON handling, formatting

✅ LLM Integration (internal/llm/)
├── Factory pattern and model selection tests
├── Caching and instance management tests
└── Interface compliance and error handling tests

⚠️ Pending High-Priority Test Coverage:
├── Session Management (internal/session/) - Message compression, persistence
├── Context Management (internal/context/) - Token limits, compression triggers
├── MCP Protocol (internal/mcp/) - JSON-RPC, transport layers
└── End-to-End Testing - Complete agent workflows with compression
```

### Testing Principles and Standards
- **Comprehensive Coverage**: Unit tests, integration tests, edge cases
- **Dependency Injection**: Proper mocking and isolation for reliable tests
- **Real-World Scenarios**: File permissions, network errors, concurrent access
- **Robust Assertions**: Flexible checks that adapt to implementation changes
- **Session Testing**: Todo persistence, compression, and session recovery

### Test Execution Strategy
```bash
# Development Testing Workflow
make test-working        # Quick validation during development
go test ./internal/agent/ -v    # Agent system comprehensive tests
go test ./internal/tools/builtin/ -v    # Builtin tools validation

# Quality Assurance Testing
make test               # Full test suite execution
go test -race ./...     # Concurrency safety validation
go test -bench=.        # Performance benchmarking
```

## Performance and Monitoring

### Performance Framework (`internal/performance/`)
- Benchmark suite with A/B testing capabilities
- Integration testing scenarios
- Monitoring and verification tools
- Automated performance checks in CI/CD

### Commands
```bash
make perf-benchmark    # Run performance benchmarks
make perf-test        # Run performance test scenarios
make perf-baseline    # Create performance baseline
make perf-monitor     # Start performance monitoring
```

## Development Workflow and Best Practices

### Code Quality Requirements
- **All new code MUST include comprehensive tests**
- **ReAct agent modifications require dependency injection testing**
- **Builtin tools require unit tests with edge case coverage**
- **Session management changes require persistence testing**
- **Run `make dev` before commits to ensure code quality**

### Testing Requirements for New Features
```bash
# Required test coverage for new code:
1. Unit tests with >80% coverage
2. Integration tests for component interactions
3. Edge case testing (errors, edge conditions, concurrency)
4. Dependency injection and mocking for external dependencies
5. Performance benchmarks for critical paths
```

### Git Workflow Standards
```bash
# Standard development cycle:
make dev                    # Format, vet, build, test
go test ./internal/agent/ -v    # Validate agent changes
go test ./internal/tools/builtin/ -v    # Validate tool changes
git add -A && git commit    # Commit with descriptive message
git push                    # Push to remote repository
```

# important-instruction-reminders
Do what has been asked; nothing more, nothing less.
NEVER create files unless they're absolutely necessary for achieving your goal.
ALWAYS prefer editing an existing file to creating a new one.
NEVER proactively create documentation files (*.md) or README files. Only create documentation files if explicitly requested by the User.

**TESTING REQUIREMENTS (2025-01)**:
- All new agent functionality MUST include comprehensive test coverage
- Use proper dependency injection patterns for testable code
- Include edge case testing for error scenarios and concurrent access
- Test session persistence and message compression scenarios end-to-end
- Run full test suite before commits: `make test` or `go test ./...`
- Builtin tools require validation, execution, and error handling tests