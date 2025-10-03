# ALEX - Alex-Code Documentation

ALEX (Agile Light Easy Xpert Code Agent) is a terminal-native AI programming assistant implemented in Go. It emphasizes lightweight, on-device reasoning with a Think-Act-Observe (ReAct) cycle, a pluggable tool ecosystem, and privacy-first operation suitable for on-premises deployments. This document describes the Alex-Code implementation, its architecture, how to develop and test it, and how to operate it in practice.

> This documentation reflects the Alex-Code project within this repository. It documents architecture, usage, development practices, and contribution guidance.

## Project Overview

- Purpose: A locally-operated AI coding assistant that runs on a developer's machine without relying on external inference services. It supports local LLM backends, multiple model providers, and an extensible MCP (Model Context Protocol) tool ecosystem.
- Core capabilities:
  - Think-Act-Observe cycle for structured reasoning and tool invocation
  - Multimodel LLM integration with caching, selection, and streaming outputs
  - MCP Protocol for external tool integration via JSON-RPC 2.0
  - Memory system with short-term (in-memory) and long-term (vector-based) storage with compression
  - SWE-Bench based evaluation framework for performance and quality benchmarking
  - Rich built-in tools for file IO, shell execution, search, web, memory, and more
- Target audience: developers and teams who require private, high-performance AI coding assistance on premises.

## Essential Development Commands

Build, test, and development helpers
- Build the Alex binary:

```
make build
```

- Full development workflow (format, vet, build, tests):

```
make dev
```

- Run all tests:

```
make test
```

- Format Go code:

```
make fmt
```

- Run go vet:

```
make vet
```

- Quick single binary run (interactive mode, auto-detect TTY):

```
./alex
```

- Interactive mode explicit flag:

```
./alex -i
```

- Single prompt mode (one-off command):

```
./alex "Analyze this directory"
```

- Resume a session:

```
./alex -r session_id -i
```

- List sessions:

```
./alex session list
```

- Show current configuration:

```
./alex config show
```

- SWE-Bench evaluation:

```
./alex run-batch --dataset.subset lite --workers 4 --output ./results
```

- Build for multiple platforms:

```
make build-all
```

- Install binary to GOPATH:

```
make install
```

### Quick-start examples

- Run a test suite in Go landmarks (where applicable):

```
go test ./...
```

- Run the interactive CLI with a sample prompt:

```
./alex "List all Go files and their purposes"
```

## Architecture Overview

Alex-Code is organized into modular components that communicate via clear interfaces. The key subsystems are:

- Core execution engine (ReAct Agent)
  - Location: internal/agent/
  - Implements Think-Act-Observe with streaming outputs and a built-in memory layer.
- Model Context Protocol (MCP)
  - Location: internal/mcp/
  - JSON-RPC 2.0 based transport and tool integration for model contexts.
- Memory and context management
  - Location: internal/memory/, internal/context/
  - Dual-layer architecture: short-term in-memory context and long-term vector storage with compression.
- Tools ecosystem
  - Location: internal/tools/
  - 13 built-in tools with a registry for discovery and dynamic MCP integration.
- LLM integration layer
  - Location: internal/llm/
  - Supports multi-model backends with caching and session-based model selection.
- Session management
  - Location: internal/session/
  - Persistent conversations and memory snapshots with compression.
- SWE-Bench evaluation framework
  - Location: evaluation/swe_bench/
  - Benchmarking and performance-quality measurement with parallel processing.
- Configuration management
  - Location: internal/config/
  - Supports layered configuration and environment-variable precedence.
- CLI and runtime entry points
  - Location: cmd/
  - Cobra-based command structure with TUI support files (modern_tui.go).

The architecture emphasizes:
- Separation of concerns and single responsibility per component
- Minimal, clear interfaces between subsystems
- Security-conscious design with strict input validation and sandboxing for tools
- Extensibility through a pluggable tool registry and MCP protocol

## Built-in Tools and Features

Alex-Code ships with a curated set of built-in tools wired into the MCP ecosystem. The default tool suite includes 13 tools across categories:

- File Operations: file_read, file_update, file_replace, file_list
- Shell Execution: bash, code_executor (sandboxed)
- Search & Analysis: grep, ripgrep, find
- Task Management: todo_read, todo_update
- Web Integration: web_search (via Tavily API integration in code)
- Thought/Reasoning: think (structured problem solving)
- MCP Protocol: dynamic external tool integration for extensibility

All tools are registered in the internal tool registry and are discoverable by MCP servers. Security and input validation are baked into tool adapters to prevent dangerous operations.

## Security Features

Alex-Code implements multi-layered security controls to protect hosts and data:
- Risk assessment and path protection: guard against path traversal and unsafe file operations.
- Command safety: sandboxed tool execution with strict parameter validation.
- Configurable restrictions: environment-based or config-based ACLs for tools and capabilities.
- Audit logging: traceability of tool invocations and memory actions for compliance.
- Threat detection: early checks for suspicious patterns and unsafe actions.
- Safe defaults: avoid leaking secrets or enabling unsafe system operations by default.

Security considerations are enforced at the tool boundary and memory boundary layers, with auditable logs for critical actions.

## Performance Characteristics

- Sub-30ms response times on typical developer hardware with optimized Go code paths.
- Parallel tool execution: the engine analyzes dependencies and executes independent tools concurrently.
- Memory efficiency: in-memory context, long-term vector storage, and compression strategies.
- SWE-Bench integration provides repeatable evaluation and quantitative metrics.

## Code Principles and Design Philosophy

The project adheres to a set of design philosophies intended to keep the codebase approachable and robust:
- Keep it simple and minimal: avoid unnecessary entities and over-configuration.
- Clear intent: meaningful naming and self-documenting code where possible.
- Minimal configuration: configure only what is necessary.
- Purposeful entities: introduce new types or interfaces only when they solve a clear problem.

## Naming Guidelines

- Functions: AnalyzeCode(), LoadPrompts(), ExecuteTool(), etc.
- Types: ReactAgent, PromptLoader, ToolExecutor.
- Variables: taskResult, userMessage, promptTemplate.

## Architectural Principles

1) Single Responsibility: each component has a single clear purpose.
2) Minimal Dependencies: reduce coupling between components.
3) Clear Interfaces: simple, focused interfaces.
4) Error Handling: fail fast with clear error messages.
5) No Over-Engineering: avoid speculative future-proofing.

## Current Status

- Production-ready with ReAct agent, MCP protocol, memory system, tools, SWE-Bench, caching, terminal UI, and security features. The repository also includes development tooling and CI scaffolding.

## Testing Instructions

Run tests for core components and tooling:

- Test packages:

```
go test ./internal/agent/ ./internal/tools/builtin/ ./internal/llm/ ./internal/memory/ ./internal/mcp/ ./internal/session/ ./evaluation/swe_bench/
```

- Quick tests (Makefile targets):

```
make test
```

- Coverage:

```
go test -coverprofile=coverage.out ./...
 go tool cover -html=coverage.out
```

- SWE-Bench evaluation:

```
./alex run-batch --dataset.subset lite --workers 2
```

## Development Notes

- The repository uses Go modules and a Makefile-based build system. Unit tests cover agent behavior, tool execution, and memory management. The codebase contains rigorous tests in internal and evaluation directories.
- For development, run make dev to ensure formatting, vetting, building, and tests run in a consistent environment.
- Docker-based development scripts exist for containerized testing and CI pipelines.

## Appendix

- Core repository layout:
  - cmd/        CLI entry points and command handlers
  - internal/   core application logic (agent, llm, tools, memory, mcp, prompts, config, session)
  - evaluation/ SWE-Bench
  - docs/       documentation and guides
  - scripts/    development automation
  - examples/   usage examples

- Important external references:
  - OpenRouter (model provider, API keys in configuration)
  - SWE-Bench documentation in evaluation/swe_bench/

If you need to adapt ALEX for your environment, start with make dev-robust and customize internal/config manager to fit your infrastructure.
