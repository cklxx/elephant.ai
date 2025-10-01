# ALEX - Agile Light Easy Xpert Code Agent

[![CI](https://github.com/cklxx/Alex-Code/actions/workflows/ci.yml/badge.svg)](https://github.com/cklxx/Alex-Code/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/cklxx/Alex-Code)](https://goreportcard.com/report/github.com/cklxx/Alex-Code)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Terminal-native AI programming agent built with Go, featuring hexagonal architecture, ReAct agent pattern, dual-mode TUI, and comprehensive tooling ecosystem.

## ✨ Highlights

- **🎯 Dual Mode Interface**: Interactive chat TUI (`./alex`) + streaming command mode (`./alex "task"`)
- **🏗️ Clean Architecture**: Hexagonal design with pure domain logic separation
- **🤖 ReAct Agent**: Think-Act-Observe cycle with 15+ built-in tools
- **🎨 Rich TUI**: Markdown rendering with syntax highlighting via Glamour
- **🔧 Multi-Model Support**: OpenAI, DeepSeek, OpenRouter, Ollama
- **📊 SWE-Bench Ready**: Built-in evaluation framework

## 🚀 Quick Start

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

### Usage Modes

#### Interactive Chat TUI
```bash
./alex
# Enter interactive chat interface with:
# - Markdown rendering with syntax highlighting
# - Real-time tool execution display
# - Scrollable message history
# - Multi-turn conversations
```

#### Command Mode (Streaming Output)
```bash
./alex "List all Go files and their purposes"
# Streams output directly to terminal with:
# - Smart tool output formatting
# - Inline progress indicators
# - Completion summary
```

#### Session Management
```bash
./alex -r session_id -i    # Resume previous session
./alex session list        # List all sessions
```

#### Configuration
```bash
./alex config show         # Show current config
```

## 🎨 Interface Features

### Interactive Chat TUI
- **Bubbletea Framework**: Modern, performant terminal UI
- **Glamour Rendering**: Full markdown support with 100+ language syntax highlighting
- **Message Types**: User (cyan), Assistant (blue), Tool (colored by status), System (gray)
- **Smart Caching**: Rendered message cache for performance
- **Keyboard Controls**:
  - `Enter` - Send message
  - `Shift+Enter` - New line
  - `Ctrl+C` - Quit

### Command Mode (Streaming)
- **Inline Output**: No full-screen takeover
- **Smart Tool Display**: Different formatting per tool type:
  - Code execution: Full output always shown
  - File operations: Summary with line counts
  - Search tools: Match counts + preview
  - Git operations: Full results
- **Verbose Mode**: `ALEX_VERBOSE=1 ./alex "command"` for full tool output

## 🛠️ Built-in Tools

### File Operations
- `file_read`, `file_write`, `file_edit`, `file_replace`, `list_files`

### Shell & Execution
- `bash` - Shell command execution
- `code_execute` - Sandboxed code execution

### Search & Discovery
- `grep`, `ripgrep`, `find` - Pattern matching and file search
- `code_search` - Semantic code search

### Task Management
- `todo_read`, `todo_update` - Markdown task lists

### Web Integration
- `web_search` - Tavily API integration
- `web_fetch` - HTTP fetching with 15-min cache

### Git Operations
- `git_commit`, `git_pr`, `git_history` - Version control integration

### Reasoning
- `think` - Structured problem-solving
- `subagent` - Delegate complex tasks (with recursion prevention)

## ⚙️ Configuration

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
export OPENAI_API_KEY="your-key"        # Override config API key
export ALLOWED_TOOLS="file_read,bash"   # Restrict available tools
export USE_REACT_AGENT="true"           # Force ReAct mode
export ALEX_VERBOSE="1"                 # Show full tool output
```

## 🏗️ Architecture

### Hexagonal Architecture
```
alex/
├── cmd/alex/                  # CLI entry points
│   ├── main.go               # Mode detection (interactive vs command)
│   ├── stream_output.go      # Streaming command mode handler
│   └── tui_chat/             # Interactive chat TUI
│       ├── model.go          # Bubbletea model
│       ├── rendering.go      # Message rendering
│       ├── types.go          # Data structures
│       └── helpers.go        # Tool icons & previews
├── internal/
│   ├── agent/                # Hexagonal architecture layers
│   │   ├── domain/           # Pure business logic
│   │   │   ├── react_engine.go       # ReAct loop (SolveTask)
│   │   │   ├── tool_formatter.go     # Tool output formatting
│   │   │   └── events.go             # Event definitions
│   │   ├── app/              # Application services
│   │   │   └── coordinator.go        # Task coordination
│   │   └── ports/            # Interfaces (LLM, Tools, Session)
│   ├── llm/                  # LLM adapters
│   │   ├── factory.go        # Multi-model factory
│   │   └── openai_client.go  # OpenAI/OpenRouter/DeepSeek
│   ├── tools/                # Tool system
│   │   ├── builtin/          # 15+ built-in tools
│   │   └── registry.go       # Dynamic registration
│   ├── messaging/            # Message types
│   ├── parser/               # Tool call parsing (XML, JSON)
│   ├── context/              # Context management
│   └── session/              # Session persistence
└── evaluation/
    └── swe_bench/            # SWE-Bench integration
```

### Key Design Patterns
- **Domain Layer**: Pure business logic, no dependencies on infrastructure
- **Ports**: Interfaces for adapters (LLM, Tools, Session)
- **Adapters**: Infrastructure implementations (OpenAI client, file tools, etc.)
- **Event-Driven**: Domain events for TUI streaming (ToolCallStart, TaskComplete, etc.)

## 🔧 Development

### Main Workflow
```bash
make dev          # Format, vet, build (recommended)
make test         # Run all tests
make build        # Build alex binary
```

### Testing
```bash
# Unit tests
go test ./internal/agent/domain/ -v      # Domain layer
go test ./internal/tools/builtin/ -v     # Builtin tools

# Integration tests
go test ./internal/tools/builtin/ -v -run TestGit  # Git integration

# Coverage
make test-coverage
```

### NPM Publishing
```bash
make npm-copy-binaries    # Copy binaries to npm packages
make npm-publish          # Publish to npm registry
make npm-test-install     # Test local installation
```

### SWE-Bench Evaluation
```bash
make swe-bench-verified-test   # Test with 3 instances
make swe-bench-verified-small  # Run 50 instances
```

## 📚 Documentation

- **[CLAUDE.md](CLAUDE.md)** - Development guide for Claude Code
- **[ALEX.md](ALEX.md)** - Detailed project documentation
- **[FORMATTING_GUIDE.md](docs/FORMATTING_GUIDE.md)** - TUI formatting guide
- **[Architecture Docs](docs/architecture/)** - System design
- **[Implementation Docs](docs/implementation/)** - Feature implementation details
- **[SWE-Bench Guide](evaluation/swe_bench/README.md)** - Evaluation framework

## 🎯 Design Philosophy

**保持简洁清晰，如无需求勿增实体**

1. **Simplicity First** - Minimal necessary complexity
2. **Clear Naming** - Self-documenting code
3. **Minimal Configuration** - Avoid options unless essential
4. **Testing Required** - All new code needs tests
5. **Hexagonal Architecture** - Clean separation of concerns

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch
3. Run `make dev` before committing (format, vet, test)
4. Ensure all tests pass: `make test`
5. Submit a pull request

### Git Configuration
```bash
git config user.name "cklxx"
git config user.email "q1293822641@gmail.com"
```

## 📝 License

MIT License - See [LICENSE](LICENSE) file

## 🔗 Links

- **Repository**: https://github.com/cklxx/Alex-Code
- **Issues**: https://github.com/cklxx/Alex-Code/issues
- **NPM Package**: https://www.npmjs.com/package/alex-code
- **OpenRouter**: https://openrouter.ai/ (API keys)

---

Built with ❤️ using Go, Bubbletea, and Glamour
