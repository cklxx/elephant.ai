# ALEX Documentation

**ALEX - Agile Light Easy Xpert Code Agent**
🚀 Terminal-native AI programming assistant built in Go with hexagonal architecture

## 📚 Documentation Structure

### 📖 Reference Documentation (`reference/`)
Core technical references and specifications:
- **ALEX.md** - Complete project overview, commands, architecture
- **AGENT_PRESETS.md** - Agent persona and tool preset system
- **PRESET_QUICK_REFERENCE.md** - Quick reference for preset usage
- **PRESET_SYSTEM_SUMMARY.md** - Preset system implementation details
- **MCP_GUIDE.md** - Model Context Protocol integration guide
- **FORMATTING_GUIDE.md** - Code formatting and style guide
- **COST_TRACKING.md** - LLM cost tracking system
- **OBSERVABILITY.md** - Logging and monitoring
- **webui-api.md** - Web UI API reference
- **tools_use_des.md** - Tool usage descriptions
- **compact_prompt.md** - Prompt optimization reference

### 📘 Guides (`guides/`)
Step-by-step tutorials and quickstart guides:
- **SSE_QUICK_START.md** - Quick start for SSE streaming
- **SSE_SERVER_GUIDE.md** - Complete SSE server guide
- **SSE_SERVER_IMPLEMENTATION.md** - SSE implementation details
- **ACCEPTANCE_TEST_PLAN.md** - Acceptance testing standards
- **QUICKSTART_SSE.md** - SSE quickstart tutorial

### 🏗️ Architecture (`architecture/`)
Detailed architecture documentation:
- **ALEX_ARCHITECTURE_ANALYSIS.md** - Core architecture analysis
- **ALEX_DETAILED_ARCHITECTURE.md** - Complete system blueprint
- **ALEX_HEXAGONAL_ARCH.md** - Hexagonal architecture guide
- **DEPENDENCY_INJECTION.md** - DI patterns and practices

### 📊 Analysis (`analysis/`)
Analysis reports and findings:
- **server_alignment_report.md** - Server architecture alignment analysis
- **research_ai_coding_guidance.md** - AI coding research
- **state-of-the-art-agent-architectures-analysis.md** - Agent architecture research

### 🎨 Design (`design/`)
Design specifications and patterns:
- **SSE_WEB_ARCHITECTURE.md** - SSE web architecture design
- **CHAT_TUI_DESIGN.md** - Chat TUI design specification
- **AGENT_TUI_COMMUNICATION.md** - Agent-TUI communication patterns
- **alex-performance-optimization-design.md** - Performance optimization design
- **OUTPUT_DESIGN.md** - Output formatting design
- **SMART_TOOL_DISPLAY.md** - Smart tool display patterns

### 📊 Diagrams (`diagrams/`)
Architecture visualizations (Mermaid format):
- **system_overview.md** - High-level system overview
- **data_flow.md** - Data flow architecture
- **react_cycle.md** - ReAct execution process
- **tool_ecosystem.md** - Tool system architecture
- **complete_architecture.md** - Complete architecture diagram

### 🔬 Research (`research/`)
Research notes and explorations:
- **ai_agents_comprehensive_research.md** - AI agents research
- **DEEP_SEARCH_RESEARCH.md** - Deep search capabilities
- **TUI_DEEP_SEARCH_DESIGN.md** - TUI deep search design

### 🚀 Operations (`operations/`)
Deployment and release documentation:
- **DEPLOYMENT.md** - Deployment guide
- **PUBLISHING.md** - NPM publishing process
- **RELEASE.md** - Release management

### 🌐 Web (`web/`)
Web UI specific documentation

## 🎯 Quick Start

### Essential Commands
```bash
make dev                     # Format, vet, build (main workflow)
make test                    # Run all tests
make build                   # Build ./alex binary
./alex                       # Run interactive mode
./alex "your task"           # Run single task
```

### Server Mode (SSE Streaming)
```bash
./alex server               # Start HTTP + SSE server
# See guides/SSE_QUICK_START.md for details
```

### Agent Presets
```bash
# Use specialized agent personas
./alex --agent-preset code-expert "Review this code"
./alex --agent-preset security-analyst "Audit this system"

# Control tool access
./alex --tool-preset read-only "Analyze codebase"

# See reference/AGENT_PRESETS.md for all presets
```

## 🏗️ Architecture Overview

### Core Components
- **🤖 ReactEngine** - Think-Act-Observe ReAct cycle
- **🧠 LLM Layer** - Multi-model support (OpenAI, DeepSeek, Ollama)
- **🔧 Tool System** - 15+ built-in tools + MCP external tools
- **💾 Session Management** - Persistent conversation history
- **📈 Event System** - Real-time streaming via SSE

### Hexagonal Architecture
```
Domain (Pure Logic) ← Ports (Interfaces) ← Adapters (Infrastructure)
```
See `architecture/ALEX_HEXAGONAL_ARCH.md` for details.

### Technology Stack
- **Language**: Go 1.21+
- **CLI**: Cobra + Viper
- **TUI**: Bubble Tea + Lipgloss
- **Web**: Next.js + TypeScript + Tailwind CSS
- **Protocol**: JSON-RPC 2.0 (MCP), SSE

## 🧪 Testing

```bash
go test ./...                              # Run all tests
go test ./internal/agent/domain/ -v       # Domain layer tests
go test ./internal/tools/builtin/ -v      # Tool tests

# Acceptance testing
bash tests/acceptance/api_test.sh          # API tests
bash tests/acceptance/sse_test.sh          # SSE tests
bash tests/acceptance/integration_test.sh  # Integration tests
```

## 📖 Documentation Philosophy

**Keep it simple and clear, add nothing without necessity (保持简洁清晰，如无需求勿增实体)**

- **Reference**: Technical specifications and API docs
- **Guides**: Step-by-step tutorials for specific tasks
- **Architecture**: Deep dives into system design
- **Analysis**: Research findings and alignment reports
- **Design**: Design patterns and specifications

## 🔗 External Resources

- **GitHub**: https://github.com/yourusername/Alex-Code
- **OpenRouter API**: Multi-model LLM provider
- **MCP Protocol**: https://modelcontextprotocol.io
- **SWE-Bench**: Code agent evaluation framework

## 📝 Contributing

See `reference/ALEX.md` for development guidelines and contribution standards.

---

**Last Updated**: 2025-10-03
**Version**: 1.0
