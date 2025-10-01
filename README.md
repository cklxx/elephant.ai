# ALEX

Terminal-native AI programming agent

[![CI](https://github.com/cklxx/Alex-Code/actions/workflows/ci.yml/badge.svg)](https://github.com/cklxx/Alex-Code/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/cklxx/Alex-Code)](https://goreportcard.com/report/github.com/cklxx/Alex-Code)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## Philosophy

**保持简洁清晰，如无需求勿增实体**

Built with hexagonal architecture, clean separation of concerns, and focus on essential complexity only.

## Features

- Interactive chat + streaming command mode
- 15+ built-in tools (file ops, shell, search, git, web)
- Multi-model support (OpenAI, DeepSeek, OpenRouter, Ollama)
- Session persistence and resumption
- Markdown rendering with syntax highlighting

## Installation

**NPM**
```bash
npm install -g alex-code
```

**From Source**
```bash
git clone https://github.com/cklxx/Alex-Code.git
cd Alex-Code && make build
```

**Releases**
[github.com/cklxx/Alex-Code/releases](https://github.com/cklxx/Alex-Code/releases)

## Usage

**Interactive Mode**
```bash
alex
```

**Command Mode**
```bash
alex "analyze this codebase"
```

**Session Management**
```bash
alex -r session_id -i    # resume
alex session list        # list all
```

## Configuration

Alex creates `~/.alex-config.json` on first run:

```json
{
  "api_key": "sk-or-xxx",
  "base_url": "https://openrouter.ai/api/v1",
  "model": "deepseek/deepseek-chat-v3-0324:free"
}
```

Override with environment:
```bash
export OPENAI_API_KEY="your-key"
export ALEX_VERBOSE="1"
```

## Tools

File: `read` `write` `edit` `replace` `list`
Shell: `bash` `code_execute`
Search: `grep` `ripgrep` `find` `code_search`
Task: `todo_read` `todo_update`
Web: `web_search` `web_fetch`
Git: `commit` `pr` `history`
Reasoning: `think` `subagent`

## Architecture

```
Domain (Pure Logic)
  ↓ depends on
Ports (Interfaces)
  ↑ implemented by
Adapters (Infrastructure)
```

**Structure**
```
internal/
├── agent/
│   ├── domain/     # react_engine, tool_formatter
│   ├── app/        # coordinator
│   └── ports/      # interfaces
├── llm/            # openai, deepseek, ollama
├── tools/
│   ├── builtin/    # 15+ tools
│   └── registry.go
└── session/        # persistence
```

## Development

**Workflow**
```bash
make dev     # format, vet, build
make test    # all tests
```

**Testing**
```bash
go test ./internal/agent/domain/ -v
go test ./internal/tools/builtin/ -v
```

**Release**
```bash
node scripts/update-version.js 0.x.x
make release-npm
```

## Documentation

- [CLAUDE.md](CLAUDE.md) - Claude Code guide
- [docs/architecture/](docs/architecture/) - System design
- [evaluation/swe_bench/](evaluation/swe_bench/) - SWE-Bench

## License

MIT

---

Built with Go · [github.com/cklxx/Alex-Code](https://github.com/cklxx/Alex-Code)
