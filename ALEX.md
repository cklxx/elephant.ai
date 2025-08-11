# ALEX - Agile Light Easy Xpert Code Agent

ALEX is a terminal-native, offline-first AI programming agent implemented in Go. It emphasizes lightweight, fast retrospectives on codebases, reactive planning with a Think-Act-Observe (ReAct) cycle, and a pluggable tool system. ALEX focuses on developer workflows with local model support, privacy, and enterprise-friendly architecture. It ships with a robust memory system, an MCP (Model Context Protocol) integration layer, and a SWE-Bench evaluation framework to quantify performance and quality.

> This documentation reflects the Alex-Code project as implemented in the repository. The content below describes architecture, usage, and development practices for contributors and operators.

## Project Overview

- Purpose: Provide a local, privacy-first AI coding assistant that can operate entirely on the developer's machine without relying on external inference services. It supports local LLMs, multiple model backends, and an extensible tool ecosystem.
- Key capabilities:
  - Think-Act-Observe cycle to reason about tasks and execute tools
  - Multimodel LLM integration with caching and model selection
  - MCP (Model Context Protocol) for external tool integration
  - Memory system with short-term context and long-term vector storage with compression
  - SWE-Bench based evaluation framework for performance and quality benchmarking
  - A rich set of built-in tools for file operations, shell execution, search, web fetch, and more
- Target audience: developers and teams seeking a private, performance-oriented AI coding assistant that can be deployed on-premises or within controlled environments.

## Essential Development Commands

Building and testing

- Build the Alex binary
  - make build  # Builds ./alex binary

- Development workflow
  - make dev    # Format, vet, build, and test functionality
  - make test   # Run all tests
  - make fmt    # Format Go code
  - make vet    # Run go vet

Alex usage and operation

- Interactive mode (auto-detects TTY)
  - ./alex        # Auto-enters interactive mode
  - ./alex -i     # Explicit interactive mode

- Single prompt mode
  - ./alex "Analyze the current directory structure"

- Session management
  - ./alex -r session_id -i       # Resume a session
  - ./alex session list           # List sessions
  - ./alex memory compress        # Compress session memory

- SWE-Bench evaluation
  - ./alex run-batch --dataset.subset lite --workers 4 --output ./results

- Configuration
  - ./alex config show            # Show configuration

- Quick start (version-agnostic examples)
  - go test ./... to run unit tests where applicable

Note: The repository contains prebuilt binaries, scripts, and a development workflow that may differ between host OS. Use the Makefile targets for a consistent local workflow.

## Architecture Overview

Core Components

1) ReAct Agent (internal/agent/)
   - Implements Think-Act-Observe cycle with streaming outputs and an integrated memory layer.
2) MCP Protocol (internal/mcp/)
   - Model Context Protocol implementation with JSON-RPC 2.0, transport, and tool integration.
3) Memory System (internal/memory/, internal/context/)
   - Dual-layer memory: short-term (in-memory) and long-term (vector-based) with context compression.
4) Tool System (internal/tools/)
   - 13 built-in tools with integrated MCP support and a registry for dynamic discovery.
5) LLM Integration (internal/llm/)
   - Multi-model support with caching, session-based model selection, and streaming interfaces.
6) Session Management (internal/session/)
   - Persistent storage of conversations with compression and session-level tooling.
7) SWE-Bench (evaluation/swe_bench/)
   - Evaluation framework for benchmarking AI agents with parallel processing.
8) Configuration (internal/config/)
   - Multi-model configuration management with environment-variable precedence and defaults.

CLI and runtime entry points

- cmd/ contains the CLI, including main.go, Cobra-based command structure, and TUI components (modern_tui.go).
- The architecture is modular: internal packages provide core services; cmd handles user interaction and plugin orchestration.

## Built-in Tools and Features

ALEX ships with a rich set of built-in tools wired into the MCP ecosystem. The default tool suite includes 13 tools across various categories:

- File Operations: file_read, file_update, file_replace, file_list
- Shell Execution: bash, code_executor (sandboxed)
- Search & Analysis: grep, ripgrep, find
- Task Management: todo_read, todo_update
- Web Integration: web_search (Tavily API integration)
- Reasoning: think (structured problem solving)
- MCP Protocol: dynamic external tool integration for extensibility

All tools are integrated with a registry and can be discovered by MCP servers. The codebase includes tests for tool behaviors and security validations (e.g., input validation and sandbox rules).

## Security Features

ALEX applies multi-layered security controls to protect hosts and data:

- Risk assessment and path protection: guard against path traversal and unsafe file operations.
- Command safety: sandboxed tool execution with strict parameter validation.
- Configurable restrictions: environment-based or config-based ACLs for tools and capabilities.
- Audit logging: traceability of tool invocations and memory actions for compliance.
- Threat detection: early checks for suspicious patterns and unsafe actions.

## Performance Characteristics

- Sub-30ms response times on typical hardware with optimized Go code paths.
- Parallel tool execution: capable of running multiple tools concurrently (the system analyzes dependencies and optimizes parallelism).
- Memory efficiency: memory compression, session-based snapshots, and cache-friendly data structures.
- SWE-Bench integration provides repeatable, measurable evaluation.

## Code Principles and Design Philosophy

Core design philosophy (inspired by the project's own statements):

- Keep it simple and minimal: avoid unnecessary entities or over-configuration (保持简洁清晰，如无需求勿增实体，尤其禁止过度配置).
- Clear intent: code should be self-documenting through meaningful naming.
- Minimal configuration: configure only what is needed for practical use.
- Purposeful entities: introduce new types or interfaces only when they serve a clear purpose.

Naming guidelines
- Functions: AnalyzeCode(), LoadPrompts(), ExecuteTool(), etc.
- Types: ReactAgent, PromptLoader, ToolExecutor.
- Variables: taskResult, userMessage, promptTemplate.

Architectural Principles
1) Single Responsibility: each component has one clear purpose.
2) Minimal Dependencies: reduce coupling between components.
3) Clear Interfaces: simple, focused interfaces.
4) Error Handling: fail fast with clear error messages.
5) No Over-Engineering: avoid speculative future-proofing.

## Current Status

- Production-ready with ReAct agent, MCP protocol, memory system, tools, SWE-Bench, caching, terminal UI, and security features.

## Testing Instructions

Run the tests for core components and tooling:

- Test packages:
  - go test ./internal/agent/ ./internal/tools/builtin/ ./internal/llm/ ./internal/memory/ ./internal/mcp/ ./internal/session/ ./evaluation/swe_bench/

- Quick tests (make targets):
  - make test-functionality
  - make test-working

- Coverage:
  - go test -coverprofile=coverage.out ./...
  - go tool cover -html=coverage.out

- SWE-Bench evaluation:
  - ./alex run-batch --dataset.subset lite --workers 2

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