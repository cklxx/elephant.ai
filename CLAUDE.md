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
├── tools/          # Tool system with 13 built-in tools
│   └── builtin/    # Core tool implementations
├── memory/         # Dual-layer memory system
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

### Built-in Tools (13 total)

Located in `internal/tools/builtin/`, each tool has its own implementation file:
- **File Operations**: `file_read`, `file_update`, `file_replace`, `file_list` - Advanced file manipulation with path validation (4 tools)
- **Shell Execution**: `bash`, `code_execute` - Secure shell execution with sandbox controls and risk assessment (2 tools)
- **Search & Analysis**: `grep`, `ripgrep`, `find` - Intelligent search with context awareness and pattern matching (3 tools)
- **Task Management**: `todo_read`, `todo_update` - Session-persistent task tracking with markdown support (2 tools)
- **Web Integration**: `web_search` - Real-time web search with Tavily API integration (1 tool)
- **Reasoning**: `think` - Structured problem-solving and decision-making tool (1 tool)
- **MCP Protocol**: Dynamic external tool integration with JSON-RPC 2.0 and multi-transport support

## Key Implementation Details

### LLM Integration (`internal/llm/`)
- Multi-model factory pattern supporting OpenAI, DeepSeek, OpenRouter
- Session-aware caching for reduced API calls
- Automatic model selection based on task type
- Default configuration uses OpenRouter with DeepSeek models

### Prompt System (`internal/prompts/`)
- Markdown-based templates: `initial.md`, `coder.md`, `enhanced_coder.md`
- Dynamic prompt loading with context injection
- Specialized prompts for different agent modes

### Session Management (`internal/session/`)
- File-based persistence in `~/.alex-sessions/`
- Automatic compression when context exceeds limits
- Todo tracking integrated with session state
- Resume capability with `-r session_id` flag

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