# ALEX Code Refactoring Architecture Proposal

**Version**: 1.0
**Date**: 2025-09-30
**Status**: Proposal - Pending Review

---

## Executive Summary

This document proposes a complete architectural refactoring of ALEX (Agile Light Easy Xpert Code Agent) to address critical design issues identified in the current implementation. The proposed architecture adopts **Hexagonal Architecture (Ports and Adapters)** to achieve clean separation between foundational components (infrastructure) and business logic (domain).

### Key Problems Addressed

1. **God Objects**: ReactCore (536 lines), ReactAgent (13 fields), ExecuteTaskCore (397 lines)
2. **Tight Coupling**: Bidirectional dependencies between ReactCore ↔ ReactAgent
3. **No Dependency Injection**: All constructors create dependencies internally, making testing difficult
4. **Mixed Concerns**: ReAct loop logic mixed with LLM calls, tool execution, and message processing
5. **Unused Interfaces**: Well-defined interfaces in `interfaces.go` never used for polymorphism
6. **Hard-coded Dependencies**: Cannot test in isolation or swap implementations

### Proposed Solution

- **Hexagonal Architecture** with clear boundaries between domain, application, and infrastructure layers
- **Dependency Injection** via constructor injection with interface-based contracts
- **Interface-first design** with ports defined in domain layer, implemented in infrastructure
- **Pure business logic** in domain layer with zero infrastructure dependencies
- **Thin orchestration** in application layer as glue between domain and infrastructure

### Expected Benefits

- ✅ **Testability**: Domain layer fully testable with mocks, no real LLM calls needed
- ✅ **Maintainability**: Clear separation of concerns, easy to locate and modify code
- ✅ **Extensibility**: Add new providers/tools without touching domain logic
- ✅ **Flexibility**: Support multiple configurations and deployment environments
- ✅ **Performance**: Optimize infrastructure independently without affecting business logic

---

## Table of Contents

1. [Current Architecture Analysis](#1-current-architecture-analysis)
2. [Industry Best Practices Research](#2-industry-best-practices-research)
3. [Proposed Hexagonal Architecture](#3-proposed-hexagonal-architecture)
4. [Foundational Components Layer](#4-foundational-components-layer)
5. [Business Logic Layer](#5-business-logic-layer)
6. [Directory Structure](#6-directory-structure)
7. [Dependency Flow](#7-dependency-flow)
8. [Migration Strategy](#8-migration-strategy)
9. [Testing Strategy](#9-testing-strategy)
10. [Comparison: Current vs Proposed](#10-comparison-current-vs-proposed)
11. [Next Steps](#11-next-steps)

---

## 1. Current Architecture Analysis

### 1.1 Major Issues Identified

#### God Objects

**ReactCore** (`internal/agent/core.go:16-536`):
- 536 lines of mixed responsibilities
- Manages task solving, message processing, tool execution, parallel coordination
- 10 fields with dependencies on agent, handlers, registry, logger, parallel execution
- Violates Single Responsibility Principle

**ReactAgent** (`internal/agent/react_agent.go:22-44`):
- 13 fields including LLM client, config manager, session manager, tool registry
- No clear boundaries between orchestration vs execution
- Massive struct that does everything

**ExecuteTaskCore** (`internal/agent/subagent.go:66-463`):
- 397 lines in a single method
- Mixes message queue handling, compression, LLM interaction, tool execution
- 5-6 levels of deep nesting

#### Tight Coupling

**ReactCore ↔ ReactAgent**:
- Bidirectional dependency creates circular coupling
- `ReactCore` needs `ReactAgent` to access LLM, session, config (line 17: `agent *ReactAgent`)
- `ReactAgent` needs `ReactCore` to execute tasks
- Creates maintenance nightmare

**Hard-coupled Handlers**:
- ReactCore directly depends on messageProcessor, llmHandler, toolHandler, promptHandler
- All created in NewReactCore (lines 41-63)
- Cannot swap implementations without modifying ReactCore

**Tool Execution Coupling**:
- `executeToolDirect` (core.go:224-330) directly accesses toolHandler.registry
- 70+ lines of nil checks and error handling
- No abstraction for tool execution interface

#### No Dependency Injection

**NewReactCore** (`core.go:28-83`):
```go
func NewReactCore(agent *ReactAgent, toolRegistry *ToolRegistry) *ReactCore {
    // Creates all dependencies internally
    messageProcessor := NewMessageProcessor(agent)
    llmHandler := NewLLMHandler(agent.llmClient, agent.sessionManager)
    toolHandler := NewToolHandler(toolRegistry)
    promptHandler := NewPromptHandler(agent.promptBuilder)
    // ...
    // Cannot test in isolation, cannot mock dependencies
}
```

**NewReactAgent** (`react_agent.go:91-142`):
- Creates LLM client, session manager, tool registry internally
- Sets global config provider
- No interface-based dependencies
- Cannot inject test doubles

#### Unused Interfaces

**interfaces.go** defines 5 interfaces but they're never used:
- `LLMClient`, `ToolExecutor`, `SessionManager`, `MessageProcessor`, `ReactEngine`
- All actual code depends on concrete types
- No benefit from interface abstraction

### 1.2 Specific Anti-Patterns

| Anti-Pattern | Location | Issue |
|-------------|----------|-------|
| **God Object** | ReactCore, ReactAgent, TaskExecutionContext | Too many responsibilities |
| **Feature Envy** | ReactCore accessing agent.* fields | Wrong object has the data |
| **Long Method** | ExecuteTaskCore (397 lines) | Cannot understand or test |
| **Deep Nesting** | ExecuteTaskCore, tool execution | Hard to follow logic |
| **Magic Numbers** | Token thresholds: 50000, 30000, 10000 | No named constants |
| **Duplicate Code** | Stream callbacks, tool error handling | Copy-paste programming |

### 1.3 Strengths to Preserve

- ✅ Comprehensive test coverage approach (react_agent_test.go, tool_registry_test.go)
- ✅ Clear domain concepts (ReactCore, ReactAgent, SubAgent well-named)
- ✅ Parallel execution with errgroup and proper concurrency control
- ✅ Tool system with smart caching (static/MCP/dynamic)
- ✅ Streaming support with StreamCallback pattern
- ✅ Error handling with graceful degradation

---

## 2. Industry Best Practices Research

### 2.1 Common Architecture Patterns

#### ReAct Pattern (Dominant in Industry)

The **ReAct (Reasoning and Acting)** pattern is used by IBM, Microsoft Azure, LangGraph, Anthropic Claude Code:

- **Think-Act-Observe cycle**: Agents alternate between reasoning, taking actions, and observing results
- **Interleaved execution**: Dynamic adjustment based on observations (not sequential planning)
- **Tool-calling foundation**: Leverage LLM native function calling capabilities

#### Multi-Agent Architectures

Four primary patterns identified:

1. **Network/Collaborative**: Agents communicate many-to-many (CrewAI)
2. **Supervisor/Orchestrator**: Central orchestrator delegates to sub-agents (AutoGen, LangGraph)
3. **Hierarchical**: Nested layers of orchestration for complex systems
4. **Swarm**: Dynamic handoffs based on expertise (OpenAI Swarm)

#### Architect/Editor Pattern (Aider)

Novel two-step separation:
- **Architect model**: Problem-solving and solution design (reasoning models)
- **Editor model**: Translates solution into code edits (code-focused models)
- **Benefit**: Specialization allows optimal model selection for each step

### 2.2 Hexagonal Architecture for Agents

Industry consensus strongly favors **hexagonal/ports-and-adapters architecture**:

**Core Principles**:
- **Domain Layer (Center)**: Agent entities, business rules, cognitive models - pure logic
- **Application Layer (Use Cases)**: Workflow orchestration, thin coordination
- **Infrastructure Layer (Adapters)**: LLM providers, databases, HTTP clients, file system

**Why Hexagonal for Agents**:
> "As agent systems become more complex, hexagonal architecture provides elegant separation of concerns, ensuring maintainability and evolution while keeping cognitive models independent of infrastructure details."

**Layer Responsibilities**:

| Layer | Responsibilities | Dependencies |
|-------|-----------------|--------------|
| **Domain** | Business logic, task decomposition, decision making | None (pure logic) |
| **Application** | Session lifecycle, workflow control, context triggers | Domain interfaces |
| **Infrastructure** | LLM clients, tool executors, storage, external APIs | Application interfaces |

### 2.3 Model Context Protocol (MCP)

Industry-standard separation between agent core and external context:

**Three Core Primitives**:
- **Tools**: Model-controlled actions (agent decides when to call)
- **Resources**: Application-controlled context (app provides data)
- **Prompts**: User-controlled interactions (user invokes explicitly)

**Protocol Layers**:
1. **Data Layer**: JSON-RPC 2.0, lifecycle management, core primitives
2. **Transport Layer**: STDIO (local) or SSE (remote) - abstracted from business logic

**Key Insight**: MCP separates **what** (primitives) from **how** (transport) and **when** (control).

### 2.4 Go Interface Best Practices

**Small, Focused Interfaces**:
```go
// Good: Single responsibility
type LLMClient interface {
    Complete(ctx context.Context, messages []Message) (*Response, error)
}

// Good: Separate concerns
type ToolExecutor interface {
    Execute(ctx context.Context, tool string, args map[string]any) (any, error)
}
```

**Consumer-Side Definition** (Dependency Inversion):
- Define interfaces where they're **used** (domain), not where they're implemented (infrastructure)
- Promotes loose coupling and easier testing

**Interface Embedding for Composition**:
```go
type StreamingClient interface {
    LLMClient
    StreamComplete(ctx context.Context, messages []Message) (<-chan Delta, error)
}
```

### 2.5 Key Insights for Refactoring

#### Top 10 Actionable Insights

1. **Adopt Hexagonal Architecture** - Separate domain logic from infrastructure
2. **Implement MCP-Style Primitives** - Separate Tools, Resources, Prompts by control model
3. **Extract Reusable Library** - Create `pkg/` module for core ReAct engine
4. **Use Constructor Injection** - Replace direct instantiation with DI everywhere
5. **Define Interfaces in Consumer Packages** - Move interface definitions to domain layer
6. **Separate Session Management** - Make it independent of agent logic
7. **Implement Layered Testing** - Unit (mocked), Integration (real infra), E2E (full workflows)
8. **Use Factory Pattern** - Explicit model selection based on task type
9. **Consider Graph-Based State Machines** - LangGraph's success shows benefits for complex workflows
10. **Implement Comprehensive Observability** - Trace every LLM call, tool execution, decision point

---

## 3. Proposed Hexagonal Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         DOMAIN LAYER                             │
│                    (Business Logic Core)                         │
│                                                                   │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  ReAct Engine (Pure Business Logic)                       │  │
│  │  - Think-Act-Observe cycle orchestration                  │  │
│  │  - Task decomposition logic                               │  │
│  │  - Decision making (which tool, when to stop)             │  │
│  │  - No dependencies on infrastructure                      │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                   │
│  Domain Interfaces (Ports):                                     │
│  ├─ LLMClient                                                   │
│  ├─ ToolExecutor                                                │
│  ├─ SessionStore                                                │
│  ├─ MessageManager                                              │
│  └─ ContextManager                                              │
└─────────────────────────────────────────────────────────────────┘
                              ▲
                              │ Uses (dependency inversion)
                              │
┌─────────────────────────────┴───────────────────────────────────┐
│                     APPLICATION LAYER                            │
│                  (Workflow Orchestration)                        │
│                                                                   │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  Agent Coordinator                                         │  │
│  │  - Session lifecycle management                            │  │
│  │  - Message queue coordination                              │  │
│  │  - Context compression triggers                            │  │
│  │  - Stream callback conversion                              │  │
│  │  - Thin glue between domain and infrastructure             │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                   │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  SubAgent Orchestrator                                     │  │
│  │  - Sub-task delegation                                     │  │
│  │  - Parallel execution coordination                         │  │
│  │  - Result aggregation                                      │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              ▲
                              │ Implements
                              │
┌─────────────────────────────┴───────────────────────────────────┐
│                   INFRASTRUCTURE LAYER                           │
│                      (Adapters)                                  │
│                                                                   │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐            │
│  │ LLM Clients │  │   Tools     │  │  Session    │            │
│  ├─────────────┤  ├─────────────┤  ├─────────────┤            │
│  │ OpenAI      │  │ Builtin     │  │ FileStore   │            │
│  │ DeepSeek    │  │ MCP         │  │ RedisStore  │            │
│  │ Ollama      │  │ SubAgent    │  │ MemoryStore │            │
│  └─────────────┘  └─────────────┘  └─────────────┘            │
│                                                                   │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐            │
│  │  Context    │  │  Messages   │  │ Function    │            │
│  │  Manager    │  │  Channel    │  │ Call Parser │            │
│  └─────────────┘  └─────────────┘  └─────────────┘            │
└─────────────────────────────────────────────────────────────────┘
```

### Key Principles

1. **Dependency arrows point inward** - Infrastructure depends on domain, never the reverse
2. **Interfaces defined in domain** - Infrastructure implements domain-defined contracts
3. **Pure business logic in center** - Domain layer has zero infrastructure imports
4. **Thin application layer** - Just coordination, no business logic
5. **Pluggable infrastructure** - Swap implementations without touching domain

---

## 4. Foundational Components Layer

See [FOUNDATIONAL_COMPONENTS.md](./FOUNDATIONAL_COMPONENTS.md) for detailed specifications of:

- **LLM Client** (`internal/llm/`) - Multi-provider LLM abstraction
- **Tool System** (`internal/tools/`) - Tool registry and execution
- **MCP Protocol** (`internal/mcp/`) - Model Context Protocol implementation
- **Context Manager** (`internal/context/`) - Token counting and compression
- **Session Store** (`internal/session/`) - Session persistence
- **Message Channel** (`internal/messaging/`) - Async message handling
- **Function Call Parser** (`internal/parser/`) - Tool call extraction

---

## 5. Business Logic Layer

See [BUSINESS_LOGIC.md](./BUSINESS_LOGIC.md) for detailed specifications of:

- **Domain Layer** (`internal/agent/domain/`) - Pure ReAct engine
- **Application Layer** (`internal/agent/app/`) - Agent coordinator and orchestration
- **SubAgent Orchestrator** - Task delegation and parallel execution
- **CLI I/O Display** (`cmd/agent/`) - Terminal interface

---

## 6. Directory Structure

```
/
├── cmd/
│   └── alex/
│       ├── main.go                    # Entry point
│       ├── container.go               # DI container
│       └── cli.go                     # CLI command handler
│
├── internal/
│   ├── agent/
│   │   ├── ports/                     # Domain interfaces (ports)
│   │   │   ├── llm.go                # LLMClient interface
│   │   │   ├── tools.go              # ToolExecutor, ToolRegistry
│   │   │   ├── session.go            # SessionStore interface
│   │   │   ├── context.go            # ContextManager interface
│   │   │   └── parser.go             # FunctionCallParser interface
│   │   │
│   │   ├── domain/                    # Pure business logic
│   │   │   ├── react_engine.go       # Core ReAct loop (no dependencies)
│   │   │   ├── task_state.go         # Task execution state
│   │   │   ├── types.go              # Domain types (Message, ToolCall, etc.)
│   │   │   └── react_engine_test.go  # Unit tests with mocks
│   │   │
│   │   └── app/                       # Application orchestration
│   │       ├── coordinator.go         # AgentCoordinator (thin glue)
│   │       ├── subagent.go           # SubAgentOrchestrator
│   │       ├── strategy.go           # Execution strategies
│   │       └── coordinator_test.go    # Integration tests
│   │
│   ├── llm/                           # Infrastructure: LLM clients
│   │   ├── factory.go                # LLM factory
│   │   ├── openai/
│   │   │   └── client.go             # OpenAI/OpenRouter client
│   │   ├── deepseek/
│   │   │   └── client.go             # DeepSeek client
│   │   └── ollama/
│   │       └── client.go             # Ollama client
│   │
│   ├── tools/                         # Infrastructure: Tools
│   │   ├── registry.go               # Tool registry implementation
│   │   ├── builtin/
│   │   │   ├── file_read.go
│   │   │   ├── bash.go
│   │   │   ├── grep.go
│   │   │   └── ...
│   │   ├── mcp/
│   │   │   ├── adapter.go            # MCP → ToolExecutor adapter
│   │   │   └── server.go
│   │   └── subagent/
│   │       └── tool.go               # SubAgent as tool
│   │
│   ├── mcp/                           # Infrastructure: MCP protocol
│   │   ├── manager.go                # Server lifecycle manager
│   │   ├── protocol/
│   │   │   ├── jsonrpc.go           # JSON-RPC 2.0
│   │   │   └── messages.go
│   │   └── transport/
│   │       ├── stdio.go
│   │       └── sse.go
│   │
│   ├── context/                       # Infrastructure: Context management
│   │   ├── manager.go                # Token counting & compression
│   │   ├── compressor.go            # Message compression strategies
│   │   └── estimator.go             # Token estimation
│   │
│   ├── session/                       # Infrastructure: Session persistence
│   │   ├── filestore/
│   │   │   └── store.go             # File-based storage
│   │   ├── memstore/
│   │   │   └── store.go             # In-memory (testing)
│   │   └── types.go                  # Session types
│   │
│   ├── messaging/                     # Infrastructure: Message handling
│   │   ├── queue.go                  # Message queue
│   │   ├── converter.go             # Format conversion
│   │   └── channel.go               # Async messaging
│   │
│   ├── parser/                        # Infrastructure: Function call parsing
│   │   ├── parser.go                # Multi-format parser
│   │   └── validator.go             # Tool call validation
│   │
│   ├── config/
│   │   └── config.go                # Configuration management
│   │
│   └── ui/                            # Infrastructure: CLI display
│       ├── input/
│       │   └── reader.go
│       └── output/
│           ├── writer.go            # Colored output
│           ├── stream.go            # Streaming display
│           └── progress.go          # Progress bars
│
├── pkg/                               # Public, reusable libraries
│   └── protocol/                     # Shared protocol definitions
│       └── types.go
│
├── evaluation/
│   └── swe_bench/                    # SWE-Bench evaluation
│
├── docs/
│   └── architecture/                 # Architecture documentation
│       ├── REFACTORING_PROPOSAL.md  # This document
│       ├── FOUNDATIONAL_COMPONENTS.md
│       ├── BUSINESS_LOGIC.md
│       └── MIGRATION_GUIDE.md
│
├── go.mod
└── go.sum
```

---

## 7. Dependency Flow

```
┌──────────────────────────────────────────────────────────────┐
│  cmd/alex/main.go                                             │
│  - Builds DI container                                        │
│  - Wires all dependencies                                     │
│  - Starts CLI                                                 │
└───────────────────┬──────────────────────────────────────────┘
                    │
                    ▼
┌──────────────────────────────────────────────────────────────┐
│  internal/agent/app/coordinator.go                            │
│  - Session lifecycle                                          │
│  - Context compression triggers                               │
│  - Delegates to domain                                        │
└───────────────────┬──────────────────────────────────────────┘
                    │
                    ▼
┌──────────────────────────────────────────────────────────────┐
│  internal/agent/domain/react_engine.go                        │
│  - Pure ReAct loop                                            │
│  - Uses injected services (LLM, Tools, Parser)                │
│  - No infrastructure knowledge                                │
└───────────────────┬──────────────────────────────────────────┘
                    │
                    ▼
┌──────────────────────────────────────────────────────────────┐
│  internal/agent/ports/*.go (Interfaces)                       │
│  - Defined in domain layer                                    │
│  - Implemented in infrastructure layer                        │
│  - Dependency Inversion Principle                             │
└───────────────────┬──────────────────────────────────────────┘
                    │
                    ▼
┌──────────────────────────────────────────────────────────────┐
│  Infrastructure Implementations                               │
│  - internal/llm/                                              │
│  - internal/tools/                                            │
│  - internal/session/                                          │
│  - internal/context/                                          │
│  - internal/parser/                                           │
│  - internal/messaging/                                        │
└──────────────────────────────────────────────────────────────┘
```

**Key Principle**: Domain defines interfaces, infrastructure implements them. Dependency arrows point **inward** (from infrastructure toward domain).

---

## 8. Migration Strategy

### Timeline: 12 Weeks

### Phase 1: Foundation (Week 1-2)

**Goals**: Establish interface contracts and DI infrastructure

**Tasks**:
1. Create `internal/agent/ports/` directory
2. Define all domain interfaces (`LLMClient`, `ToolExecutor`, `SessionStore`, etc.)
3. Extract pure domain types to `internal/agent/domain/types.go`
4. Create DI container in `cmd/alex/container.go`
5. Write comprehensive unit tests for domain layer (with mocks)

**Deliverables**:
- ✅ All interfaces defined with clear contracts
- ✅ DI container with dependency wiring
- ✅ Mock implementations for testing
- ✅ Unit test infrastructure ready

**Risk Mitigation**:
- Keep existing code running in parallel
- No breaking changes to public API
- Gradual migration allows rollback at any point

---

### Phase 2: Infrastructure Refactor (Week 3-4)

**Goals**: Make infrastructure components implement new interfaces

**Tasks**:
1. Refactor LLM clients to implement `ports.LLMClient`
   - Update OpenAI client
   - Update DeepSeek client
   - Update Ollama client
2. Refactor tools to implement `ports.ToolExecutor`
   - Update builtin tools
   - Update MCP adapter
   - Update SubAgent tool
3. Extract session storage to `internal/session/`
   - Create filestore implementation
   - Create memstore for testing
4. Extract context management to `internal/context/`
   - Token counting
   - Message compression strategies

**Deliverables**:
- ✅ All LLM clients implement common interface
- ✅ All tools implement ToolExecutor interface
- ✅ Session store implementations ready
- ✅ Context manager extracted and tested

**Validation**:
- Integration tests pass with new implementations
- Backward compatibility maintained
- Performance benchmarks show no regression

---

### Phase 3: Domain Extraction (Week 5-6)

**Goals**: Create pure domain layer with zero infrastructure dependencies

**Tasks**:
1. Extract pure ReAct loop to `internal/agent/domain/react_engine.go`
   - Move Think-Act-Observe cycle logic
   - Remove all infrastructure dependencies
   - Use only injected interfaces
2. Create `TaskState` struct for execution state
3. Create `Services` struct to bundle injected dependencies
4. Write comprehensive unit tests with mocked dependencies
5. Validate ReAct logic works with mocks

**Deliverables**:
- ✅ ReactEngine with zero infrastructure imports
- ✅ 100% unit test coverage with mocks
- ✅ Clear separation of concerns
- ✅ Domain logic independently testable

**Success Criteria**:
- {% raw %}`go list -f '{{.Imports}}' internal/agent/domain`{% endraw %} shows no infrastructure imports
- All tests pass without real LLM calls
- Test execution time < 1 second

---

### Phase 4: Application Layer (Week 7-8)

**Goals**: Create thin orchestration layer

**Tasks**:
1. Create `AgentCoordinator` in `internal/agent/app/coordinator.go`
   - Session lifecycle management
   - Context compression triggers
   - Stream callback conversion
   - Delegates to ReactEngine
2. Move session lifecycle out of domain
3. Move message queue coordination to application layer
4. Create `SubAgentOrchestrator` for parallel execution
5. Implement execution strategies (serial, parallel, auto)

**Deliverables**:
- ✅ AgentCoordinator as thin glue layer
- ✅ Session management in application layer
- ✅ SubAgent orchestration extracted
- ✅ Integration tests pass

**Validation**:
- Application layer has < 200 lines per file
- No business logic in application layer
- All coordination logic clearly separated

---

### Phase 5: Testing & Validation (Week 9-10)

**Goals**: Comprehensive test coverage at all layers

**Tasks**:
1. **Unit Tests** (Domain Layer):
   - ReactEngine with mocked services
   - Edge cases (max iterations, early stop, errors)
   - Concurrent execution safety
2. **Integration Tests** (Application Layer):
   - Real infrastructure components (except LLM - mocked)
   - Session persistence and recovery
   - Context compression scenarios
3. **E2E Tests**:
   - Full agent workflows with mock LLM
   - SWE-Bench regression testing
   - Performance benchmarking
4. **Test Coverage Analysis**:
   - Aim for >80% coverage in domain layer
   - >70% coverage in application layer
   - Critical paths 100% covered

**Deliverables**:
- ✅ Comprehensive test suite at all layers
- ✅ SWE-Bench regression tests pass
- ✅ Performance benchmarks show no degradation
- ✅ Test coverage reports

**Success Criteria**:
- All tests pass: `make test`
- No flaky tests in CI
- Test execution time acceptable (<5 minutes)

---

### Phase 6: Cleanup & Documentation (Week 11-12)

**Goals**: Remove old code, update documentation, polish

**Tasks**:
1. Remove old code:
   - Delete unused files
   - Remove deprecated functions
   - Clean up imports
2. Update CLAUDE.md:
   - New architecture overview
   - Updated testing instructions
   - New development workflow
3. Create architecture documentation:
   - FOUNDATIONAL_COMPONENTS.md
   - BUSINESS_LOGIC.md
   - MIGRATION_GUIDE.md
4. Performance optimization:
   - Profile critical paths
   - Optimize hot spots
   - Benchmark comparisons
5. Final validation:
   - Full SWE-Bench evaluation
   - Production readiness checklist
   - Security audit

**Deliverables**:
- ✅ Old code removed
- ✅ Documentation updated
- ✅ Performance optimized
- ✅ Production-ready codebase

---

### Migration Checklist

- [ ] Phase 1: Foundation (Week 1-2)
  - [ ] Interfaces defined in `internal/agent/ports/`
  - [ ] DI container created
  - [ ] Mock implementations ready
  - [ ] Unit test infrastructure
- [ ] Phase 2: Infrastructure Refactor (Week 3-4)
  - [ ] LLM clients refactored
  - [ ] Tools refactored
  - [ ] Session store extracted
  - [ ] Context manager extracted
- [ ] Phase 3: Domain Extraction (Week 5-6)
  - [ ] ReactEngine extracted
  - [ ] Zero infrastructure dependencies
  - [ ] 100% unit test coverage
- [ ] Phase 4: Application Layer (Week 7-8)
  - [ ] AgentCoordinator created
  - [ ] Session lifecycle moved
  - [ ] SubAgent orchestration extracted
- [ ] Phase 5: Testing & Validation (Week 9-10)
  - [ ] Unit tests comprehensive
  - [ ] Integration tests pass
  - [ ] E2E tests pass
  - [ ] SWE-Bench regression passed
- [ ] Phase 6: Cleanup & Documentation (Week 11-12)
  - [ ] Old code removed
  - [ ] Documentation updated
  - [ ] Performance optimized
  - [ ] Production-ready

---

## 9. Testing Strategy

### 9.1 Testing Pyramid

```
              ┌──────────────┐
              │  E2E Tests   │  10% - Full workflows with mock LLM
              │   (~20)      │
              ├──────────────┤
         ┌────┴──────────────┴────┐
         │  Integration Tests      │  30% - Real infrastructure
         │      (~60)              │
         ├─────────────────────────┤
    ┌────┴─────────────────────────┴────┐
    │       Unit Tests                   │  60% - Mocked dependencies
    │         (~200)                     │
    └────────────────────────────────────┘
```

### 9.2 Unit Tests (Domain Layer)

**Goal**: Test pure business logic with mocked dependencies

**Example**:

```go
// internal/agent/domain/react_engine_test.go
func TestReactEngine_SolveTask(t *testing.T) {
    // Arrange: Create mocks
    mockLLM := &mocks.MockLLMClient{}
    mockTools := &mocks.MockToolRegistry{}
    mockParser := &mocks.MockParser{}
    mockContext := &mocks.MockContextManager{}

    services := domain.Services{
        LLM:          mockLLM,
        ToolExecutor: mockTools,
        Parser:       mockParser,
        Context:      mockContext,
    }

    engine := domain.NewReactEngine(10)

    // Act: Setup expectations
    mockLLM.On("Complete", mock.Anything, mock.Anything).Return(&CompletionResponse{
        Content: "Let me read the file",
        ToolCalls: []ToolCall{
            {Name: "file_read", Arguments: map[string]any{"path": "test.go"}},
        },
    }, nil)

    mockTools.On("Execute", mock.Anything, mock.Anything).Return(&ToolResult{
        Content: "file contents",
    }, nil)

    // Act: Execute
    result, err := engine.SolveTask(context.Background(), "read test.go", &domain.TaskState{}, services)

    // Assert
    assert.NoError(t, err)
    assert.NotNil(t, result)
    mockLLM.AssertExpectations(t)
    mockTools.AssertExpectations(t)
}

func TestReactEngine_MaxIterations(t *testing.T) {
    // Test that engine stops after max iterations
}

func TestReactEngine_EarlyStop(t *testing.T) {
    // Test that engine stops when final answer reached
}

func TestReactEngine_ToolExecutionError(t *testing.T) {
    // Test error handling during tool execution
}
```

**Coverage Goals**:
- All ReAct loop paths
- Error conditions
- Edge cases (empty tools, max iterations, context limits)
- Concurrent execution safety

---

### 9.3 Integration Tests (Application Layer)

**Goal**: Test coordination with real infrastructure (except LLM)

**Example**:

```go
// internal/agent/app/coordinator_test.go
func TestCoordinator_ExecuteTask_WithRealComponents(t *testing.T) {
    // Arrange: Use real components except LLM
    mockLLM := &mocks.MockLLMClient{}

    llmFactory := &testFactory{client: mockLLM}
    toolRegistry := tools.NewRegistry()      // Real registry
    sessionStore := memstore.New()           // In-memory store
    contextMgr := context.NewManager()       // Real manager
    parser := parser.New()                   // Real parser

    engine := domain.NewReactEngine(10)

    coordinator := app.NewAgentCoordinator(
        llmFactory,
        toolRegistry,
        sessionStore,
        contextMgr,
        parser,
        engine,
        testConfig(),
    )

    // Act: Execute task
    result, err := coordinator.ExecuteTask(context.Background(), "test task", "", nil)

    // Assert
    assert.NoError(t, err)
    assert.NotNil(t, result)
}

func TestCoordinator_SessionPersistence(t *testing.T) {
    // Test session save and resume
}

func TestCoordinator_ContextCompression(t *testing.T) {
    // Test compression triggers correctly
}
```

**Coverage Goals**:
- Session lifecycle (create, save, resume)
- Context compression scenarios
- Message queue handling
- Stream callback conversion
- Error recovery

---

### 9.4 E2E Tests

**Goal**: Test full workflows with mock LLM responses

**Example**:

```go
// e2e_test.go
func TestE2E_FileReadTask(t *testing.T) {
    // Setup: Create full system with mock LLM
    mockLLM := &recordingLLM{
        responses: []string{
            `I'll read the file. <tool_call>{"name": "file_read", "args": {"path": "test.go"}}</tool_call>`,
            `The file contains: package main...`,
        },
    }

    container := buildTestContainer(mockLLM)

    // Execute: Run full task
    result := container.CLI.Execute([]string{"alex", "read test.go"})

    // Assert: Verify full workflow
    assert.Equal(t, 0, result.ExitCode)
    assert.Contains(t, result.Output, "package main")
}

func TestE2E_MultiStepTask(t *testing.T) {
    // Test task requiring multiple tool calls
}

func TestE2E_ParallelSubAgents(t *testing.T) {
    // Test parallel sub-task execution
}
```

**Coverage Goals**:
- Common user workflows
- Multi-step reasoning tasks
- Parallel execution
- Error scenarios
- SWE-Bench representative instances

---

### 9.5 Test Infrastructure

**Mock Implementations**:

```go
// internal/agent/ports/mocks/llm.go
type MockLLMClient struct {
    mock.Mock
}

func (m *MockLLMClient) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
    args := m.Called(ctx, req)
    return args.Get(0).(*CompletionResponse), args.Error(1)
}

// internal/agent/ports/mocks/tools.go
type MockToolRegistry struct {
    mock.Mock
}

// ... similar for all interfaces
```

**Test Helpers**:

```go
// internal/agent/domain/testutil/fixtures.go
func NewTestServices() domain.Services {
    return domain.Services{
        LLM:          &mocks.MockLLMClient{},
        ToolExecutor: &mocks.MockToolRegistry{},
        Parser:       &mocks.MockParser{},
        Context:      &mocks.MockContextManager{},
    }
}

func NewTestTaskState() *domain.TaskState {
    return &domain.TaskState{
        Messages:   []Message{},
        TokenCount: 0,
        Iterations: 0,
    }
}
```

---

### 9.6 Test Execution

**Commands**:

```bash
# Unit tests (fast, mocked)
go test ./internal/agent/domain/ -v

# Integration tests (real infrastructure)
go test ./internal/agent/app/ -v

# E2E tests (full workflows)
go test ./e2e/ -v

# All tests
make test

# Coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

**CI Pipeline**:

```yaml
# .github/workflows/test.yml
- name: Unit Tests
  run: go test -race ./internal/agent/domain/

- name: Integration Tests
  run: go test -race ./internal/agent/app/

- name: E2E Tests
  run: go test -race ./e2e/

- name: SWE-Bench Regression
  run: ./evaluation/swe_bench/run_evaluation.sh real-test
```

---

## 10. Comparison: Current vs Proposed

| Aspect | Current Architecture | Proposed Architecture |
|--------|---------------------|----------------------|
| **Architecture Pattern** | Mixed, no clear pattern | Hexagonal Architecture |
| **Separation of Concerns** | Mixed business + infrastructure | Clean layered separation |
| **Business Logic Location** | Scattered across files | Centralized in domain layer |
| **Testing** | Hard to test (real dependencies) | Easy to test (mocked ports) |
| **Test Coverage** | Partial, difficult to achieve | Comprehensive, easy to achieve |
| **Coupling** | Tight bidirectional coupling | Loose coupling via interfaces |
| **Dependencies** | Hard-coded in constructors | Injected via DI container |
| **Interfaces** | Defined but unused | Defined in domain, used everywhere |
| **Domain Purity** | Imports infrastructure packages | Zero infrastructure imports |
| **File Size** | 536-line God Objects | Small, focused files (<200 lines) |
| **Extensibility** | Requires modifying core | Add new adapters, no core changes |
| **Reusability** | Agent tied to ALEX | ReAct engine is reusable library |
| **Code Organization** | Unclear responsibilities | Clear layer boundaries |
| **Error Handling** | Scattered, duplicated | Centralized patterns |
| **Performance Optimization** | Coupled to business logic | Optimize infrastructure independently |
| **LLM Provider Changes** | Modify agent code | Swap factory implementation |
| **Tool Addition** | Modify registry and agent | Register in DI container |
| **Storage Backend Changes** | Rewrite session code | Swap SessionStore implementation |
| **Development Speed** | Slow (need to understand everything) | Fast (work on one layer) |
| **Onboarding** | Difficult (complex dependencies) | Easy (clear architecture) |
| **Debugging** | Hard (tangled dependencies) | Easy (clear boundaries) |
| **Refactoring Safety** | Risky (ripple effects) | Safe (interface contracts) |

---

## 11. Next Steps

### Immediate Actions

1. **Review This Proposal**
   - Gather feedback from team
   - Validate architectural decisions
   - Adjust based on concerns

2. **Approve Migration Strategy**
   - Confirm 12-week timeline
   - Allocate resources
   - Set up project tracking

3. **Create Detailed Task Breakdown**
   - Break each phase into specific tasks
   - Estimate effort for each task
   - Assign owners

4. **Set Up Infrastructure**
   - Create architecture documentation repository
   - Set up test infrastructure
   - Configure CI/CD for new structure

### Decision Points

**Option A: Full Refactoring (Recommended)**
- Follow entire 12-week migration plan
- Complete architectural transformation
- Maximum long-term benefits

**Option B: Incremental Refactoring**
- Start with Phase 1-2 only
- Validate approach with partial migration
- Decide whether to continue based on results

**Option C: Proof of Concept First**
- Refactor one component (e.g., LLM client) to validate pattern
- If successful, proceed with full migration
- Lower risk, longer timeline

### Success Metrics

- **Code Quality**: Reduced coupling, increased cohesion
- **Test Coverage**: >80% domain layer, >70% application layer
- **Performance**: No regression from current implementation
- **Development Speed**: Faster feature development post-refactoring
- **Bug Rate**: Reduced bugs due to better testing
- **SWE-Bench**: Maintain or improve current performance

### Risk Mitigation

- **Parallel Development**: Keep existing code working during migration
- **Incremental Rollout**: Gradual migration allows rollback at any point
- **Comprehensive Testing**: Catch issues early with robust test suite
- **Documentation**: Clear migration guide for team
- **Regular Check-ins**: Weekly reviews to adjust course if needed

---

## Appendix

### A. References

**Industry Best Practices**:
- [Applying Hexagonal Architecture in AI Agent Development](https://medium.com/@martia_es/applying-hexagonal-architecture-in-ai-agent-development-44199f6136d3)
- [Clean Architecture in Go Microservices](https://threedots.tech/post/introducing-clean-architecture/)
- [Model Context Protocol Architecture](https://modelcontextprotocol.io/docs/learn/architecture)
- [Building Effective Agents - Anthropic](https://www.anthropic.com/engineering/building-effective-agents)

**Go-Specific Patterns**:
- [Go Interfaces Design Patterns & Best Practices](https://blog.marcnuri.com/go-interfaces-design-patterns-and-best-practices)
- [Dependency Injection in Go](https://medium.com/avenue-tech/dependency-injection-in-go-35293ef7b6)
- [golang-standards/project-layout](https://github.com/golang-standards/project-layout)

**Agent Frameworks**:
- [LangGraph Concepts](https://langchain-ai.github.io/langgraph/concepts/agentic_concepts/)
- [CrewAI Documentation](https://docs.crewai.com/)
- [Comparing AI Agent Frameworks](https://langfuse.com/blog/2025-03-19-ai-agent-comparison)

### B. Glossary

- **Hexagonal Architecture**: Architecture pattern that separates core business logic from external concerns through ports (interfaces) and adapters (implementations)
- **Domain Layer**: Pure business logic with no infrastructure dependencies
- **Application Layer**: Thin orchestration layer coordinating between domain and infrastructure
- **Infrastructure Layer**: Implementations of external concerns (databases, APIs, file system)
- **Ports**: Interfaces defined in domain layer
- **Adapters**: Infrastructure implementations of ports
- **Dependency Inversion**: High-level modules should not depend on low-level modules; both should depend on abstractions
- **ReAct Pattern**: Reasoning and Acting - interleaved think-act-observe cycle
- **MCP**: Model Context Protocol - standard for connecting AI agents to external context

### C. Contact

For questions or feedback on this proposal:
- **Architecture Review**: Schedule meeting to discuss design decisions
- **Implementation Details**: See detailed component specs in linked documents
- **Migration Support**: Refer to MIGRATION_GUIDE.md (to be created in Phase 1)

---

**Document Version**: 1.0
**Last Updated**: 2025-09-30
**Status**: Proposal - Pending Review