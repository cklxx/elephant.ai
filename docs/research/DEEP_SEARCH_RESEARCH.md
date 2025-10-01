# Deep Search Research Report

**Date:** 2025-10-01
**Purpose:** Comprehensive research on deep search implementations for ALEX code agent

---

## Executive Summary

Deep search represents a paradigm shift from single-query responses to autonomous, multi-step research capabilities. After analyzing industry implementations (Perplexity, OpenAI, Anthropic) and academic research, several key findings emerge:

**Key Findings:**

1. **Two-Phase Architecture is Standard**: All major implementations use a "Research → Synthesize" pattern where agents iteratively search and refine understanding before generating comprehensive reports.

2. **Reinforcement Learning Powers Autonomy**: OpenAI's Deep Research uses end-to-end RL on hard browsing/reasoning tasks, enabling dynamic strategy adaptation and backtracking.

3. **Token Budget Management is Critical**: Recent research (TALE method) shows 68.64% token cost reduction while maintaining accuracy through dynamic budget allocation based on task complexity.

4. **Multi-File Reasoning Requires Specialized Patterns**: Microsoft's Code Researcher explores 10 files per trajectory vs. 1.33 for traditional agents, demonstrating the need for systematic codebase exploration.

5. **Plan-Execute-Reflect is the Core Pattern**: Most successful implementations use iterative cycles of planning (task decomposition), execution (tool use), and reflection (self-evaluation) rather than linear workflows.

**Recommendations for ALEX:**

- Implement a two-stage deep search mode: iterative exploration phase + synthesis phase
- Extend current ReAct engine with reflection capabilities and dynamic query refinement
- Add token budget management for cost-efficient extended reasoning
- Build specialized codebase exploration tools for multi-file dependency analysis
- Integrate thinking/reasoning tools for complex problem decomposition

---

## 1. Industry Solutions Analysis

### 1.1 Perplexity Deep Research

**Overview:**
- Performs dozens of searches, reads hundreds of sources in 2-4 minutes
- References 150% more sources than competitors (50 vs 20)
- Free tier with limits; Pro unlimited

**Technical Approach:**

**Two-Stage Implementation:**
1. **Research with Reasoning**: Iteratively searches, reads documents, and reasons about next actions, refining research plan dynamically as it learns
2. **Report Writing**: Synthesizes all research into clear, comprehensive reports

**Key Characteristics:**
- Mimics human researcher's adaptive approach
- Dynamic strategy refinement based on new findings
- Evidence-based synthesis with citations

**Performance:**
- Completes most research tasks in <3 minutes
- High scores on industry benchmarks
- Extensive source coverage

### 1.2 OpenAI Deep Research

**Overview:**
- Powered by o3 model optimized for web browsing and data analysis
- Uses chain-of-thought reasoning for multi-step tasks
- Record 26.6% accuracy on Humanity's Last Exam (vs 9.1% for o1)

**Technical Approach:**

**Training Methodology:**
- End-to-end reinforcement learning on hard browsing and reasoning tasks
- Trained on real-world tasks requiring browser and Python tool use
- Same RL methods behind OpenAI o1

**Multi-Step Reasoning:**
- Decomposes high-level queries into subtasks
- Performs web searches iteratively
- Adapts in real-time by backtracking when encountering contradictory data
- Pivots approach based on findings mid-exploration

**API Implementation:**
```python
response = client.responses.create(
    model="o3-deep-research-2025-06-26",
    input=[
        {"role": "developer", "content": [{"type": "input_text", "text": system_message}]},
        {"role": "user", "content": [{"type": "input_text", "text": user_query}]}
    ],
    reasoning={"summary": "auto"},
    tools=[
        {"type": "web_search_preview"},
        {"type": "code_interpreter"}
    ]
)
```

**Response Structure:**
- `response.output[-1].content[0].text` - Final report
- `response.output[-1].content[0].annotations` - Inline citations
- `response.output` - Intermediate steps for inspection

**Key Features:**
- Autonomous task decomposition
- Web search integration
- Citation-rich reporting
- Structured output generation
- Background mode for long-running tasks

### 1.3 Anthropic Extended Thinking

**Overview:**
- Serial test-time compute: multiple sequential reasoning steps
- Hybrid reasoning model (Claude 3.7 Sonnet, Opus 4, Sonnet 4)
- User-controllable thinking budget

**Configuration:**
```json
{
    "thinking": {
        "type": "enabled",
        "budget_tokens": 16384
    }
}
```

**Key Parameters:**
- `budget_tokens`: Minimum 1,024; recommended 16k+ for complex tasks
- Compatible with Claude Opus and Sonnet 4.x models
- Supports streaming thinking responses

**Response Structure:**
```json
{
    "content": [
        {
            "type": "thinking",
            "thinking": "Step-by-step reasoning process...",
            "signature": "Encrypted verification token"
        },
        {
            "type": "text",
            "text": "Final response based on thinking"
        }
    ]
}
```

**Performance:**
- Math (MATH 500): 96.2% with extended thinking
- GPQA (Graduate-Level Q&A): 84.8% overall, 96.5% on physics
- Uses up to 128K tokens internally for hard problems

**Best Practices:**
- Start with larger budgets (16k+), adjust incrementally
- Use for complex tasks (math, coding, analysis)
- Monitor token usage to optimize costs
- Expect longer response times

**Limitations:**
- Not compatible with temperature/top_k modifications
- Cannot pre-fill responses
- Thinking blocks auto-removed from previous turns

---

## 2. Technical Approaches & Patterns

### 2.1 ReAct Pattern (Current ALEX Foundation)

**Core Pattern:**
```
Thought → Action → Observation → (repeat until solved)
```

**Components:**
- **Thought**: Free-form reasoning for task decomposition, information extraction, commonsense reasoning
- **Action**: Interface with external tools (file operations, searches, code execution)
- **Observation**: Results from tool usage that inform next reasoning step

**Advantages:**
- Reduces hallucination through external verification
- Improves interpretability and trustworthiness
- Creates natural feedback loop
- Overcomes error propagation issues

**Enhancements for Deep Search:**
- Add explicit reflection step after observation
- Implement query refinement based on findings
- Track explored paths to avoid redundant searches
- Enable backtracking when hitting dead ends

### 2.2 Plan-Execute-Reflect Pattern

**Three-Phase Workflow:**

**1. Planning Phase:**
- Central planner LLM breaks complex task into dynamic subtask list
- Task decomposition into actionable steps
- Dependency analysis for parallel execution opportunities

**2. Execution Phase:**
- Subtasks delegated to worker agents
- Predefined tools with permissions
- Parallel execution where dependencies allow

**3. Reflection Phase:**
- Self-feedback mechanism evaluating output quality
- Iterative refinement using self-correction loop
- Orchestrator synthesizes results
- Re-planning if goal not achieved

**Implementation Pattern:**
```
Plan → Execute → Reflect → (Re-plan if needed) → Synthesize
```

**Frameworks Supporting This:**
- LangChain/LangGraph
- Microsoft AutoGen
- CrewAI
- Semantic Kernel

### 2.3 Tree of Thoughts (ToT)

**Overview:**
- Generalizes chain-of-thought to explore multiple reasoning paths
- Branching structure enabling thorough problem-solving
- Combines reflection/evaluation with search algorithms (BFS/DFS)

**Components:**
- **Expander**: Generates one or more candidate solutions
- **Scorer**: Evaluates solutions along distinct dimensions
- **Search Algorithm**: Explores thought tree (BFS, DFS, or custom)

**Evolution to Graph of Thought:**
- Uses graph theory to structure reasoning
- Models thoughts as interconnected nodes and edges
- More sophisticated than linear sequences
- Allows flexible, non-linear reasoning paths

**Implementation (LangGraph):**
- Practical agent patterns available
- End-to-end tutorials
- Integration with modern frameworks

### 2.4 Iterative Query Refinement

**Key Techniques:**

**1. Query Decomposition:**
- Break complex queries into simpler sub-queries
- Useful when answer spans multiple documents
- Sequential vs. parallel decomposition strategies

**2. RQ-RAG Architecture:**
- Explicit rewriting, decomposition, disambiguation
- Diverse search queries at each iteration
- Multiple expansion paths based on query type

**3. Domain-Specific Augmentation:**
- Augment queries with domain-specific terminology
- Add structured descriptors
- Iterative semi-automated refinement
- Performance improvement: 0.18 → 0.42 similarity score

**Approaches:**
- **Sub-Question Query Engine**: Dedicated engines per sub-question
- **Sequential Refinement**: Progressive enrichment with retrieved info
- **Adaptive Rewriting**: Failed query reformulation

### 2.5 Multi-Agent Architectures

**Orchestrator-Workers Pattern:**

**Roles:**
- **Planner**: Identifies dependencies and parallel opportunities
- **Workers**: Execute subtasks using appropriate tools
- **Solver/Synthesizer**: Combines outputs into cohesive result

**Specialized Agents:**
- **Research Agents**: Domain-specific searches (academic, technical, etc.)
- **Synthesis Agent**: Combines findings, identifies patterns/contradictions/gaps
- **Evaluation Agent**: Assesses information sufficiency

**Communication Patterns:**
- Shared context engine (org-wide for code agents)
- Message passing between agents
- Centralized orchestration vs. peer-to-peer

**Microsoft's MAgICoRe:**
- Categorizes problem difficulty (easy vs. hard)
- Coarse-grained aggregation for easy problems
- Fine-grained iterative refinement for hard problems
- Three agents: Solver, Reviewer, Refiner

---

## 3. Token Budget Management

### 3.1 Token Elasticity Phenomenon

**Discovery:**
- Reasoning process can be compressed with appropriate budgets
- Minimal budgets don't necessarily minimize token costs
- "When budget reduced beyond certain range, token cost increases"
- Optimal budget range exists for each task

### 3.2 TALE Method

**Approach:**
- Dynamically estimates token budgets based on task complexity
- Three estimation strategies: zero-shot, regression, fine-tuning
- Binary search to find minimal budget maintaining correctness
- Greedy search to minimize cost while preserving quality

**Performance:**
- **68.64% average token cost reduction**
- <5% accuracy decrease
- Tested on GSM8K, GSM8K-Zero, MathBench
- Works across GPT-4o, GPT-4o-mini, Yi-lightning

**Implementation:**
- Craft token-budget-aware prompts
- Estimate budget via zero-shot or regression
- Monitor token elasticity curve
- Find ideal budget range

### 3.3 Related Frameworks

**BudgetThinker:**
- Periodically inserts control tokens during inference
- Continuously informs model of remaining budget
- Precise control over thought process length

**SelfBudgeter:**
- Autonomously predicts required token budgets
- Self-imposed constraint adherence
- Optimizes accuracy-response length trade-off

**AnytimeReasoner:**
- Optimizes anytime reasoning performance
- Budget Relative Policy Optimization (BRPO)
- Robust and efficient under varying constraints

---

## 4. Code-Specific Deep Search

### 4.1 Microsoft Code Researcher

**Overview:**
- First deep research agent specifically for code
- Multi-step reasoning about semantics, patterns, commit history
- **Explores 10 files per trajectory vs. 1.33 for traditional agents**

**Capabilities:**
- Autonomous code repository exploration
- Understanding explicit implementations and implicit patterns
- Connecting related functionality across codebase
- Evidence-based explanations with code location references

**Key Insight:**
Deep codebase understanding requires systematic multi-file exploration, not single-file focus.

### 4.2 RefactorBench Insights

**Benchmark:**
- 100 large handcrafted multi-file refactoring tasks
- Popular open-source repositories
- Requires thorough dependency exploration

**Requirements:**
- Comprehensive multi-file reasoning
- Composition of multiple smaller changes
- Understanding cross-file dependencies
- Maintaining architectural consistency

### 4.3 Best Practices for Code Agents

**Research and Planning First:**
- Claude Code best practices emphasize research/planning before coding
- Jumping straight to solutions reduces quality
- Deep thinking upfront significantly improves performance

**Tools for Codebase Exploration:**
- File viewing and listing
- Content search (grep/ripgrep)
- Semantic search
- Symbol navigation
- Dependency graph analysis

**Multi-File Reasoning Patterns:**
- Build mental model of architecture
- Trace dependencies across files
- Understand implicit contracts and patterns
- Verify changes don't break distant dependencies

---

## 5. Agentic RAG & Information Synthesis

### 5.1 Agentic RAG Evolution

**Traditional RAG Limitations:**
- Single retrieval pass
- Poor multi-source integration
- No iterative refinement
- Static query formulation

**Agentic RAG Capabilities:**
- Iterative search and reasoning
- Dynamic retrieval refinement
- Multi-source synthesis
- Query adaptation based on findings

### 5.2 CoRAG (Microsoft)

**Key Innovation:**
- Chain-of-Retrieval Augmented Generation
- Iterative search and reasoning cycles
- Dynamic retrieval refinement before answer generation
- Effective multi-source information integration

**Workflow:**
```
Initial Query → Retrieval → Analysis → Query Refinement →
Retrieval → ... → Sufficient Info? → Generate Answer
```

### 5.3 Synthesis Patterns

**Evaluator-Optimizer Workflow:**
- Generate initial output
- Evaluate quality
- Refine based on feedback
- Iterate until quality threshold met

**Information Aggregation:**
- Identify patterns across sources
- Detect contradictions
- Find knowledge gaps
- Cross-reference facts
- Build coherent narrative

**Report Generation:**
- Structure findings logically
- Provide evidence/citations
- Highlight confidence levels
- Note limitations and uncertainties

---

## 6. Architecture Recommendations for ALEX

### 6.1 Layered Enhancement Approach

**Layer 1: Enhanced ReAct Engine (Current Foundation)**
```go
// Current: internal/agent/domain/react_engine.go
// Enhancements needed:

type ReactEngine struct {
    maxIterations     int
    reflectionEnabled bool          // NEW
    queryRefinement   bool          // NEW
    exploredPaths     []SearchPath  // NEW
    thinkingBudget    int           // NEW
}

// Add reflection after observation
func (e *ReactEngine) reflect(observation string) Reflection {
    // Evaluate quality of observation
    // Determine if query needs refinement
    // Check if backtracking needed
    // Assess progress toward goal
}
```

**Layer 2: Deep Search Mode**
```go
// New: internal/agent/domain/deep_search_engine.go

type DeepSearchEngine struct {
    reactEngine      *ReactEngine
    maxSearchDepth   int
    synthesizer      *Synthesizer
    budgetManager    *TokenBudgetManager
}

func (d *DeepSearchEngine) Research(query string) ResearchReport {
    // Phase 1: Iterative Exploration
    plan := d.decompose(query)
    findings := []Finding{}

    for _, subtask := range plan {
        budget := d.budgetManager.EstimateBudget(subtask)
        result := d.reactEngine.SolveTaskWithBudget(subtask, budget)
        findings = append(findings, result)

        // Reflect and adapt
        if d.shouldRefine(findings) {
            plan = d.refinePlan(plan, findings)
        }
    }

    // Phase 2: Synthesis
    report := d.synthesizer.GenerateReport(findings)
    return report
}
```

**Layer 3: Multi-File Code Explorer**
```go
// New: internal/tools/builtin/codebase_explorer.go

type CodebaseExplorer struct {
    visited       map[string]bool
    dependencies  DependencyGraph
    semanticIndex SemanticSearch
}

func (c *CodebaseExplorer) ExploreRelated(file string, depth int) []FileContext {
    // Build dependency graph
    // Follow imports/references
    // Extract relevant code sections
    // Track exploration breadth/depth
}
```

### 6.2 Integration Points with Current ReAct Engine

**Current ReAct Engine (internal/agent/domain/react_engine.go:33-181):**
```go
func (e *ReactEngine) SolveTask(task string) (string, error) {
    // Existing loop...
}
```

**Enhanced Version:**
```go
func (e *ReactEngine) SolveTaskWithReflection(
    task string,
    config ReflectionConfig,
) (string, []Reflection, error) {

    reflections := []Reflection{}
    queryVersion := 1

    for iteration := 0; iteration < e.maxIterations; iteration++ {
        // Existing thought/action/observation logic...

        // NEW: Reflection step
        reflection := e.reflect(observation, task)
        reflections = append(reflections, reflection)

        // NEW: Query refinement
        if reflection.ShouldRefineQuery {
            task = e.refineQuery(task, reflection.Insights)
            queryVersion++
        }

        // NEW: Backtracking
        if reflection.ShouldBacktrack {
            iteration -= reflection.BacktrackSteps
        }

        // Check if sufficient progress
        if reflection.GoalAchieved {
            break
        }
    }

    return finalResult, reflections, nil
}
```

### 6.3 Tool Requirements

**New Tools Needed:**

1. **Think/Reflect Tool** (already in builtin)
   - Enhanced with budget management
   - Structured reflection output
   - Progress tracking

2. **Query Decomposition Tool**
   ```go
   // internal/tools/builtin/query_decompose.go
   type QueryDecomposer struct{}

   func (q *QueryDecomposer) Decompose(query string) []Subtask {
       // LLM-based query decomposition
       // Identify dependencies
       // Estimate complexity per subtask
   }
   ```

3. **Synthesis Tool**
   ```go
   // internal/tools/builtin/synthesize.go
   type Synthesizer struct{}

   func (s *Synthesizer) Synthesize(findings []Finding) Report {
       // Identify patterns/contradictions
       // Build coherent narrative
       // Generate citations
       // Assess confidence
   }
   ```

4. **Codebase Graph Navigator**
   ```go
   // internal/tools/builtin/code_graph.go
   type CodeGraphNavigator struct{}

   func (c *CodeGraphNavigator) FindRelated(symbol string) []CodeRef {
       // Semantic code search
       // Dependency tracking
       // Symbol navigation
   }
   ```

5. **Multi-File Context Builder**
   ```go
   // internal/tools/builtin/multi_file_context.go
   type MultiFileContext struct{}

   func (m *MultiFileContext) BuildContext(files []string) ContextWindow {
       // Smart truncation
       // Relevance ranking
       // Dependency-aware ordering
   }
   ```

### 6.4 Token Budget Strategy

**Dynamic Budget Allocation:**
```go
type TokenBudgetManager struct {
    totalBudget     int
    usedTokens      int
    taskComplexity  map[string]int
}

func (t *TokenBudgetManager) EstimateBudget(task string) int {
    // Zero-shot complexity estimation
    complexity := t.estimateComplexity(task)

    // Allocate based on TALE method
    switch complexity {
    case Simple:
        return 1024   // Minimum
    case Medium:
        return 4096
    case Complex:
        return 16384
    case VeryComplex:
        return 32768
    }
}

func (t *TokenBudgetManager) AdjustBudget(task string, performance float64) int {
    // Track token elasticity
    // Binary search for optimal budget
    // Update complexity estimates
}
```

### 6.5 Workflow Orchestration

**Deep Search Mode Workflow:**

```
User Query
    ↓
[Mode Detection] → Simple query? → Use current ReAct engine
    ↓ Complex query
[Task Decomposition]
    - Query Decomposer tool
    - Estimate token budgets
    - Build execution plan
    ↓
[Iterative Exploration Phase]
    For each subtask:
        - ReAct loop with reflection
        - Query refinement based on findings
        - Multi-file exploration if code-related
        - Track explored paths
        - Backtrack if needed
    ↓
[Sufficiency Check]
    - Evaluate completeness
    - Identify gaps
    → If gaps: refine plan, continue exploration
    ↓
[Synthesis Phase]
    - Aggregate all findings
    - Identify patterns/contradictions
    - Generate structured report
    - Add citations/evidence
    ↓
[Return Report to User]
```

**CLI Integration:**
```bash
# New command
alex deep-search "analyze authentication flow in codebase"

# With options
alex deep-search --max-depth 10 --budget 50000 "research OAuth implementation patterns"
```

---

## 7. Implementation Roadmap

### Phase 1: MVP (Core Deep Search)

**Goal:** Basic two-phase deep search capability

**Features:**
1. ✅ Enhanced ReAct with reflection
   - Add reflection step after observation
   - Track explored paths
   - Basic progress assessment

2. ✅ Query decomposition
   - LLM-based task breakdown
   - Subtask dependency identification
   - Simple complexity estimation

3. ✅ Basic synthesis
   - Aggregate findings
   - Generate structured output
   - Simple citation tracking

4. ✅ Token budget management
   - Fixed budgets per complexity level
   - Usage tracking
   - Budget warnings

**Implementation Time:** 1-2 weeks

**Files to Modify:**
- `internal/agent/domain/react_engine.go` - Add reflection
- `internal/tools/builtin/think.go` - Enhance with structured output
- `internal/agent/app/coordinator.go` - Add deep search mode

**New Files:**
- `internal/agent/domain/deep_search_engine.go`
- `internal/agent/domain/reflection.go`
- `internal/tools/builtin/query_decompose.go`
- `internal/tools/builtin/synthesize.go`

### Phase 2: Advanced Features

**Goal:** Production-ready with optimizations

**Features:**
1. ✅ Dynamic token budget optimization
   - TALE method implementation
   - Token elasticity tracking
   - Adaptive budget allocation

2. ✅ Query refinement
   - Iterative query rewriting
   - Failed query reformulation
   - Domain-specific augmentation

3. ✅ Backtracking support
   - Save exploration checkpoints
   - Revert to previous states
   - Alternative path exploration

4. ✅ Multi-file code exploration
   - Dependency graph building
   - Semantic code search
   - Symbol navigation

**Implementation Time:** 2-3 weeks

**New Files:**
- `internal/agent/domain/token_budget_manager.go`
- `internal/agent/domain/query_refiner.go`
- `internal/tools/builtin/code_graph.go`
- `internal/tools/builtin/semantic_search.go`

### Phase 3: Advanced Multi-Agent (Future)

**Goal:** Multi-agent orchestration for complex research

**Features:**
1. ⬜ Specialized research agents
   - Code analysis agent
   - Web research agent
   - Documentation agent

2. ⬜ Agent orchestration
   - Parallel subtask execution
   - Agent communication
   - Result aggregation

3. ⬜ Advanced synthesis
   - Pattern detection
   - Contradiction resolution
   - Confidence scoring
   - Gap identification

4. ⬜ Learning from history
   - Track successful strategies
   - Optimize decomposition patterns
   - Improve budget estimates

**Implementation Time:** 3-4 weeks

**New Files:**
- `internal/agent/domain/multi_agent_orchestrator.go`
- `internal/agent/domain/specialized_agents.go`
- `internal/agent/domain/agent_communication.go`

---

## 8. Open Questions

### 8.1 Technical Questions

1. **LLM Selection for Reflection**
   - Use same model for reflection vs. specialized smaller model?
   - Trade-off: consistency vs. cost/speed

2. **Caching Strategy**
   - Cache intermediate results?
   - How to invalidate stale cache?
   - Storage mechanism?

3. **Failure Handling**
   - Max reflection iterations before giving up?
   - How to detect circular reasoning?
   - Graceful degradation strategy?

4. **Context Window Management**
   - How to handle 10+ file exploration with limited context?
   - Smart truncation strategies?
   - Incremental context building?

### 8.2 Design Questions

1. **User Control**
   - How much control should users have over depth/budget?
   - Auto-detect deep search need vs. explicit mode?
   - Real-time progress updates?

2. **Output Format**
   - Structured report format?
   - Markdown with sections?
   - Interactive exploration UI?

3. **Integration with Current Tools**
   - Modify existing tools or create new variants?
   - Backward compatibility concerns?

4. **Testing Strategy**
   - How to test non-deterministic deep search?
   - Benchmark datasets?
   - Success metrics?

### 8.3 Research Gaps

1. **Optimal Reflection Frequency**
   - After every observation? Every N iterations?
   - Task-dependent?

2. **Subtask Granularity**
   - How fine-grained should decomposition be?
   - Dynamic adjustment based on complexity?

3. **Multi-File Exploration Limits**
   - 10 files like Code Researcher?
   - Configurable?
   - Depth vs. breadth trade-offs?

4. **Synthesis Quality Metrics**
   - How to evaluate synthesis quality?
   - User feedback integration?
   - Automated quality scoring?

---

## 9. References

### Industry Implementations
- Perplexity Deep Research: https://www.perplexity.ai/hub/blog/introducing-perplexity-deep-research
- OpenAI Deep Research: https://openai.com/index/introducing-deep-research/
- Claude Extended Thinking: https://docs.claude.com/en/docs/build-with-claude/extended-thinking
- OpenAI Deep Research API Cookbook: https://cookbook.openai.com/examples/deep_research_api/introduction_to_deep_research_api

### Academic Papers
- ReAct: Synergizing Reasoning and Acting (arXiv:2210.03629)
- Tree of Thoughts: Deliberate Problem Solving (arXiv:2305.10601)
- Token-Budget-Aware LLM Reasoning (arXiv:2412.18547)
- MAgICoRe: Multi-Agent Iterative Refinement (arXiv:2409.12147)
- Reasoning in Token Economies (arXiv:2406.06461)
- Agentic RAG Survey (arXiv:2501.09136)

### Tools & Frameworks
- LangChain/LangGraph: https://langchain-ai.github.io/langgraph/
- Microsoft Code Researcher: https://www.microsoft.com/en-us/research/publication/code-researcher-deep-research-agent-for-large-systems-code-and-commit-history/
- GitHub Deep Research for Code: https://github.com/codegen-sh/deep-research

### Best Practices
- Claude Code Best Practices: https://www.anthropic.com/engineering/claude-code-best-practices
- Azure AI Agent Orchestration Patterns: https://learn.microsoft.com/en-us/azure/architecture/ai-ml/guide/ai-agent-design-patterns
- Agentic Workflow Patterns: https://www.philschmid.de/agentic-pattern

---

## 10. Conclusion

Deep search represents a significant evolution in AI agent capabilities, moving from simple query-response to autonomous multi-step research. For ALEX, implementing deep search will require:

1. **Foundational Enhancements**: Add reflection and query refinement to existing ReAct engine
2. **Two-Phase Architecture**: Separate exploration and synthesis phases
3. **Token Budget Management**: Dynamic allocation using TALE method principles
4. **Code-Specific Tools**: Multi-file exploration and dependency analysis
5. **Iterative Development**: Start with MVP, gather feedback, enhance progressively

The research clearly shows that successful deep search implementations combine:
- **Autonomous reasoning** (plan-execute-reflect cycles)
- **Dynamic adaptation** (query refinement, backtracking)
- **Efficient resource use** (token budget management)
- **Comprehensive synthesis** (pattern detection, citation-rich reports)

ALEX's existing hexagonal architecture and ReAct foundation provide an excellent starting point. The recommended phased approach allows for incremental value delivery while building toward a comprehensive deep search capability that can rival commercial offerings.
