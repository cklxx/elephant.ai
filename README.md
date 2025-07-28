# ALEX - Agile Light Easy Xpert Code Agent

[![CI](https://github.com/cklxx/Alex-Code/actions/workflows/ci.yml/badge.svg)](https://github.com/cklxx/Alex-Code/actions/workflows/ci.yml)
[![Deploy to GitHub Pages](https://github.com/cklxx/Alex-Code/actions/workflows/deploy-pages.yml/badge.svg)](https://github.com/cklxx/Alex-Code/actions/workflows/deploy-pages.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/cklxx/Alex-Code)](https://goreportcard.com/report/github.com/cklxx/Alex-Code)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**ALEX** (Agile Light Easy Xpert Code Agent) is a lightweight AI code agent built in Go, focused on core coding capabilities. Designed with simplicity and efficiency in mind, it provides essential features like file operations, code analysis, and intelligent search, enabling developers to accomplish daily programming tasks effortlessly through natural language.

🌐 **[Visit our website](https://cklxx.github.io/Alex-Code/)** | 📚 **[Documentation](docs/)** | 🚀 **[Quick Start](#quick-start)**

## 🚀 Quick Download & Usage Guide

### One-Minute Setup

```bash
# 1. Clone and build Alex (requires Go 1.21+)
git clone https://github.com/cklxx/Alex-Code.git
cd Alex-Code
make build

# 2. Get your free API key from OpenRouter
# Visit: https://openrouter.ai/settings/keys

# 3. Start using Alex immediately
./alex                        # Interactive mode (will create config on first run)
./alex "List all .go files"   # Single command mode
```

### First Time Configuration

```bash
# Alex will create ~/.alex-config.json on first run
# Edit the file and replace "sk-or-xxx" with your actual OpenRouter API key

# Or set via environment variable (no file editing needed)
export OPENAI_API_KEY="your-openrouter-key-here"
./alex "Hello Alex!"

# Verify setup
./alex config show
```

### Common Usage Patterns

```bash
# Code analysis and assistance
./alex "Analyze this Go project structure"
./alex "Help me optimize this function"
./alex "Find potential bugs in the current directory"

# File operations
./alex "Create a new REST API endpoint"
./alex "Refactor the authentication middleware"
./alex "Add error handling to main.go"

# Interactive development session
./alex -i                     # Enter chat mode for extended conversations
```

### Quick Install (Recommended)

**Linux/macOS:**
```bash
curl -sSfL https://raw.githubusercontent.com/cklxx/Alex-Code/main/scripts/install.sh | sh
```

**Windows:**
```powershell
iwr -useb https://raw.githubusercontent.com/cklxx/Alex-Code/main/scripts/install.ps1 | iex
```

### Pre-built Binaries
Pre-compiled binaries for Linux, macOS, and Windows are available in the [Releases](https://github.com/cklxx/Alex-Code/releases) section.

## Quick Start

```bash
# Build Alex
make build                    # Builds ./alex binary

# Interactive conversational mode (ReAct agent by default)
./alex                        # Auto-detects TTY and enters interactive mode
./alex -i                     # Explicit interactive mode

# Single prompt mode (shows completion time)
./alex "Analyze the current directory structure"
# Output: ✅ Task completed in 1.2s

# With streaming responses (default behavior)
./alex "List all Go files"

# Session management
./alex -r session_id -i       # Resume specific session
./alex session list           # List all sessions
```

## Core Features

**🧠 Advanced ReAct Architecture**: Production-ready agent with Think-Act-Observe cycles, streaming responses, and intelligent context management  
**🧪 SWE-Bench Integration**: Complete evaluation framework compatible with SWE-Agent for standardized benchmarking  
**🔌 MCP Protocol Support**: Full Model Context Protocol implementation with stdio/SSE transports and tool integration  
**🧠 Intelligent Memory System**: Dual-layer memory with context compression, vector storage, and automatic summarization  
**🛠 Rich Tool Ecosystem**: 13 built-in tools including file ops, shell execution, smart search, web integration, task management, and reasoning tools  
**🌐 Multi-Model LLM System**: Advanced factory pattern supporting OpenAI, DeepSeek, OpenRouter with model-specific optimizations  
**🔒 Enterprise Security**: Comprehensive risk assessment, path protection, command validation, and sandbox execution  
**⚡ High Performance**: Native Go implementation with concurrent execution, memory optimization, and sub-30ms response times  
**📊 Advanced Session Management**: Persistent conversations with context preservation, memory compression, and todo tracking  
**🎯 Universal Accessibility**: Natural language interface optimized for developers at all experience levels

## Usage

### Interactive Mode - Your AI Coding Partner
```bash
./alex                        # Auto-detects terminal and enters interactive mode
./alex -i                     # Explicit interactive mode flag
```

### Configuration Management
```bash
./alex config show                   # Show current configuration
```

### Advanced Usage
```bash
# Configure model parameters
./alex --tokens 4000 --temperature 0.8 "Complex analysis task"

# Architecture selection (automatic fallback)
USE_REACT_AGENT=true ./alex -i       # Force ReAct agent
USE_LEGACY_AGENT=true ./alex -i      # Force legacy agent

# Development workflow
make dev                             # Format, vet, build, and test
make dev-safe                        # Safe development workflow
make test-functionality              # Quick functionality test
```

## Advanced Tool System & Architecture

### Built-in Tool Suite
**File Operations**: `file_read`, `file_update`, `file_replace`, `file_list` with intelligent path resolution  
**Shell Execution**: `bash`, `code_executor` with security validation and sandbox controls  
**Search & Analysis**: `grep`, `ripgrep`, `find` with advanced pattern matching and context awareness  
**Task Management**: `todo_read`, `todo_update` with session-aware persistence and markdown support  
**Web Integration**: `web_search` with Tavily API integration for real-time information retrieval  
**Reasoning Tools**: `think` for structured problem-solving and decision making

### 🔌 MCP (Model Context Protocol) Integration

Alex features full **MCP Protocol** support, enabling seamless integration with external tools and services:

**🌐 Protocol Implementation**
- **JSON-RPC 2.0**: Complete specification implementation with bidirectional communication
- **Multiple Transports**: STDIO and Server-Sent Events (SSE) support for flexible deployment
- **Tool Discovery**: Automatic tool registration and capability discovery from MCP servers

**🛠 Server Management**
- **Dynamic Spawning**: Automatic MCP server lifecycle management with configuration-driven setup
- **Health Monitoring**: Connection status tracking, automatic reconnection, and error recovery
- **Resource Management**: Efficient resource allocation and cleanup for MCP server processes

**🔧 Tool Integration**
- **Unified Tool Registry**: Seamless integration of MCP tools with built-in tool ecosystem
- **Security Validation**: Comprehensive parameter validation and security controls for external tools
- **Performance Optimization**: Intelligent caching and connection pooling for MCP operations

### 🧠 Advanced Memory & Context Management

**Dual-Layer Memory System**:
- **Short-term Memory**: In-memory conversation tracking with intelligent context window management
- **Long-term Memory**: Vector-based storage with ChromeM and SQLite backends for persistent knowledge
- **Context Compression**: Smart summarization and compression to maintain relevant context within token limits

**Performance Features**:
- **Concurrent Execution**: Intelligent parallel tool processing with dependency analysis
- **Memory Optimization**: Automatic cleanup, compression, and efficient resource management
- **Context Preservation**: Session-aware context management with backup and restoration capabilities

## Project Architecture

```
alex/
├── cmd/                    # CLI entry points and command handlers
│   ├── main.go            # Primary application entry point
│   ├── cobra_cli.go       # Cobra-based CLI implementation
│   ├── cobra_batch.go     # SWE-Bench batch processing
│   └── modern_tui.go      # Advanced terminal UI components
├── internal/               # Private application code
│   ├── agent/             # ReAct agent with advanced memory management
│   ├── llm/               # Multi-model LLM with session caching
│   ├── tools/             # Enhanced tool system with MCP integration
│   │   ├── builtin/       # 12+ core tool implementations
│   │   └── code_executor.go # Safe code execution framework
│   ├── memory/            # Dual-layer memory system
│   ├── context/           # Context management and compression
│   ├── mcp/               # Model Context Protocol implementation
│   │   ├── protocol/      # JSON-RPC 2.0 protocol layer
│   │   └── transport/     # STDIO and SSE transport mechanisms
│   ├── prompts/           # Centralized prompt templates (markdown-based)
│   ├── config/            # Advanced configuration management
│   └── session/           # Persistent session management
├── evaluation/            # SWE-Bench evaluation framework
│   └── swe_bench/         # Complete SWE-Agent compatible implementation
├── pkg/                   # Library code for external use
│   └── types/             # Comprehensive type definitions
├── docs/                  # Extensive documentation and guides
├── scripts/               # Development and automation scripts
└── examples/              # Usage examples and demonstrations
```

## Development

```bash
# Development workflow
make dev                   # Format, vet, build, and test functionality
make dev-safe              # Safe development workflow (excludes broken tests)
make dev-robust            # Ultra-robust workflow with dependency management

# Testing options
make test                  # Run all tests
make test-working          # Run only working tests
make test-functionality    # Quick test of core functionality

# Code quality
make fmt                   # Format Go code
make vet                   # Run go vet
make build                 # Build Alex binary

# Testing individual components
go test ./internal/agent/             # Test ReAct agent system
go test ./internal/tools/builtin/     # Test builtin tools
go test ./internal/session/           # Test session management

# Docker development
./scripts/docker.sh dev    # Start development environment
./scripts/docker.sh test   # Run tests in container
```

## 🌐 Website & Documentation

Alex includes a beautiful, modern website that showcases the project features and provides comprehensive documentation.

### Local Development
```bash
# Start local website server
cd docs/
./deploy.sh               # Choose option 1 for local server

# Or use Python directly
python -m http.server 8000
```

### Automated Deployment
The website automatically deploys to GitHub Pages via CI/CD:

- **🔄 Auto-deploy**: Pushes to `main` branch trigger deployment
- **⚡ Fast**: Typically deploys in 2-5 minutes  
- **🔍 Validated**: HTML validation and optimization included
- **📊 Stats**: Auto-generates project statistics

### Setup GitHub Pages
```bash
# One-time setup for GitHub Pages
./scripts/setup-github-pages.sh
```

This script will:
1. ✅ Verify all required files exist
2. 🔧 Configure repository URLs
3. 📤 Commit and push changes
4. 📋 Provide setup instructions

**Manual Setup Steps:**
1. Go to repository **Settings > Pages**
2. Set source to **"GitHub Actions"**
3. Enable **"Read and write permissions"** in **Settings > Actions**

🌐 **Live Website**: [https://cklxx.github.io/Alex-Code/](https://cklxx.github.io/Alex-Code/)

## Configuration

### Initial Setup

1. **Get OpenRouter API Key**: Visit [OpenRouter](https://openrouter.ai/settings/keys) to create a free account and get your API key
2. **First Run**: Alex will create default configuration on first use
3. **Set API Key**: Edit `~/.alex-config.json` and replace `"sk-or-xxx"` with your actual API key

Alex stores configuration in: `~/.alex-config.json`

### Configuration Management
```bash
./alex config show                   # Show current configuration
```

**Default Configuration:**
```json
{
    "api_key": "sk-or-xxx",
    "base_url": "https://openrouter.ai/api/v1", 
    "model": "deepseek/deepseek-chat-v3-0324:free",
    "max_tokens": 4000,
    "temperature": 0.7,
    "max_turns": 25,
    "basic_model": {
        "model": "deepseek/deepseek-chat-v3-0324:free",
        "max_tokens": 4000,
        "temperature": 0.7
    },
    "reasoning_model": {
        "model": "deepseek/deepseek-r1:free",
        "max_tokens": 8000,
        "temperature": 0.3
    }
}
```

### Multi-Model Configuration Explained

- **basic_model**: Used for general tasks and tool calling (lighter, faster)
- **reasoning_model**: Used for complex problem-solving and analysis (more capable)
- Alex automatically selects the appropriate model based on task complexity

### Environment Variables

Configuration precedence: **Environment Variables > Config File > Defaults**

```bash
export OPENAI_API_KEY="your-openrouter-key"  # Overrides config file api_key
export ALLOWED_TOOLS="file_read,bash,grep"   # Restrict available tools 
export USE_REACT_AGENT="true"                # Force ReAct agent mode
export USE_LEGACY_AGENT="true"               # Force legacy agent mode
```

### Common Configuration Tasks

```bash
# View current configuration
./alex config show

# Quick start with environment variable (no config file editing needed)
OPENAI_API_KEY="your-key" ./alex "Hello world"

# Test configuration
./alex "Test my setup"
```

## Why Alex Excels

**🚀 Advanced Architecture & Performance**
- **Dual Agent Design**: ReAct agent with automatic fallback to legacy mode for maximum reliability
- **Zero Dependencies**: Built on Go standard library for maximum stability and performance  
- **Concurrent Execution**: Intelligent parallel tool processing with dependency analysis
- **Memory Efficient**: Automatic session cleanup and smart resource management
- **Lightning Speed**: Sub-30ms response times with 40-100x performance improvement over predecessors

**🛠 Enterprise-Grade Features**
- **Security-First Design**: Multi-layered security with threat detection and risk assessment
- **Session Management**: Persistent conversations with context-aware todo management
- **Multi-Model Support**: Factory pattern supporting different LLM providers and model types
- **Tool Ecosystem**: Enhanced tool system with intelligent recommendations and metrics
- **Industry Standards**: Follows Go project layout, enterprise patterns, and modern AI frameworks

**🎯 Universal Accessibility**
- **Natural Language Interface**: No special syntax required, intuitive for all skill levels
- **Cross-Platform**: Seamless operation on macOS, Linux, and Windows
- **Lightweight Deployment**: Minimal resource usage, suitable for any development environment
- **Extensible Design**: Clean interfaces for custom tool development and integration

## Latest Updates (v1.0 - 2025)

**🚀 Major Architecture Advances:**
- **MCP Protocol Integration**: Full Model Context Protocol support with JSON-RPC 2.0, STDIO/SSE transports
- **Advanced Memory System**: Dual-layer memory with vector storage, context compression, and intelligent summarization
- **SWE-Bench Framework**: Complete evaluation system compatible with SWE-Agent for standardized benchmarking
- **Enhanced Context Management**: Smart context window management with preservation, compression, and restoration

**⚡ Performance & Reliability:**
- **Session Caching**: LLM response caching for improved performance and reduced API costs
- **Concurrent Tool Execution**: Intelligent parallel processing with dependency analysis
- **Memory Optimization**: Automatic cleanup, compression, and efficient resource management
- **Enterprise Security**: Enhanced validation, sandbox execution, and comprehensive risk assessment

**🛠 Developer Experience:**
- **Modern Terminal UI**: Advanced TUI with real-time streaming and interactive components
- **Comprehensive Tool Suite**: 12+ built-in tools with intelligent recommendations and validation
- **Docker Development**: Complete containerized development environment with multi-stage builds
- **Enhanced Build System**: Optimized Makefile with version injection and dependency management

## Documentation

- **[CLAUDE.md](CLAUDE.md)**: Comprehensive project instructions and architecture overview
- **[Architecture Documentation](docs/architecture/)**: Detailed system design and component documentation
- **[SWE-Bench Guide](evaluation/swe_bench/README.md)**: Complete guide to software engineering benchmarking
- **[Memory System Guide](docs/memory-system-guide.md)**: Advanced memory management and context handling
- **[MCP Integration Guide](docs/codeact/integration-guide.md)**: Model Context Protocol implementation details

## Contributing

We welcome contributions! Please see our development workflow:

1. **Setup**: `make dev-robust` for complete environment setup
2. **Testing**: `make test-functionality` for quick validation
3. **Quality**: `make fmt && make vet` before submitting
4. **Architecture**: Follow the patterns established in `internal/` packages

## License

MIT License