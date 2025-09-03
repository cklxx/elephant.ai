# ALEX - Agile Light Easy Xpert Code Agent

[![CI](https://github.com/cklxx/Alex-Code/actions/workflows/ci.yml/badge.svg)](https://github.com/cklxx/Alex-Code/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/cklxx/Alex-Code)](https://goreportcard.com/report/github.com/cklxx/Alex-Code)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Terminal-native AI programming agent built in Go with ReAct architecture, MCP protocol support, and SWE-Bench evaluation framework.

## Quick Start

### Installation

#### Option 1: Build from Source (Recommended)
```bash
# Requires Go 1.21+
git clone https://github.com/cklxx/Alex-Code.git
cd Alex-Code
make build

# Get API key from https://openrouter.ai/settings/keys
export OPENAI_API_KEY="your-openrouter-key"
./alex "Hello Alex!"
```

#### Option 2: NPM Installation
```bash
npm install -g alex-code
alex "Analyze this directory"
```

#### Option 3: Pre-built Binaries
Download from [Releases](https://github.com/cklxx/Alex-Code/releases)

### Basic Usage

```bash
# Interactive mode (auto-detects TTY)
./alex

# Single command
./alex "List all Go files and their purposes"

# Resume session
./alex -r session_id -i

# Show configuration
./alex config show
```

## Features

### Core Capabilities
- **ReAct Agent Architecture**: Think-Act-Observe cycle with streaming responses
- **13 Built-in Tools**: File operations, shell execution, search, web, task management
- **MCP Protocol**: Full Model Context Protocol implementation with JSON-RPC 2.0
- **Multi-Model Support**: OpenAI, DeepSeek, OpenRouter with automatic selection
- **Session Management**: Persistent conversations with context compression
- **SWE-Bench Integration**: Evaluation framework for benchmarking

### Built-in Tools
- **File Operations**: `file_read`, `file_update`, `file_replace`, `file_list`
- **Shell Execution**: `bash`, `code_execute` with security validation
- **Search**: `grep`, `ripgrep`, `find` with pattern matching
- **Task Management**: `todo_read`, `todo_update` with session persistence
- **Web**: `web_search` via Tavily API
- **Reasoning**: `think` for structured problem-solving

## Configuration

Alex creates `~/.alex-config.json` on first run:

```json
{
    "api_key": "sk-or-xxx",
    "base_url": "https://openrouter.ai/api/v1",
    "model": "deepseek/deepseek-chat-v3-0324:free",
    "max_tokens": 4000,
    "temperature": 0.7,
    "basic_model": {
        "model": "deepseek/deepseek-chat-v3-0324:free"
    },
    "reasoning_model": {
        "model": "deepseek/deepseek-r1:free"
    }
}
```

### Environment Variables
```bash
export OPENAI_API_KEY="your-key"        # Override config file
export ALLOWED_TOOLS="file_read,bash"   # Restrict tools
export USE_REACT_AGENT="true"          # Force ReAct mode
```

## Development

```bash
# Main workflow
make dev          # Format, vet, build, test
make test         # Run all tests
make build-all    # Build for all platforms

# Testing
make test-functionality    # Quick core test
make test-working         # Skip known issues
go test ./internal/agent/ # Test specific package

# SWE-Bench Evaluation
make swe-bench-verified-test   # Test with 3 instances
make swe-bench-verified-small  # Run 50 instances
make swe-bench-verified-full   # Run all 500 instances
```

## Architecture

```
alex/
├── cmd/                    # CLI entry points
├── internal/
│   ├── agent/             # ReAct agent implementation
│   ├── llm/              # Multi-model LLM factory
│   ├── tools/            # Tool system
│   │   └── builtin/      # Core tool implementations
│   ├── memory/           # Dual-layer memory system
│   ├── mcp/              # MCP protocol
│   ├── session/          # Session management
│   └── prompts/          # Markdown prompt templates
└── evaluation/
    └── swe_bench/        # SWE-Bench evaluation
```

## Documentation

- [CLAUDE.md](CLAUDE.md) - Development guide for Claude Code instances
- [Architecture Docs](docs/architecture/) - System design details
- [SWE-Bench Guide](evaluation/swe_bench/README.md) - Evaluation framework

## Contributing

1. Fork the repository
2. Create your feature branch
3. Run `make dev` before committing
4. Submit a pull request

## License

MIT License - See [LICENSE](LICENSE) file