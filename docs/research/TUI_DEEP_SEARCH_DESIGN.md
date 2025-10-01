# TUI Deep Search Interface Design Research

**Date:** 2025-10-01
**Purpose:** Comprehensive research on TUI development for ALEX deep search feature
**Current Stack:** Bubble Tea, Lip Gloss, Go 1.23+

---

## Executive Summary

After comprehensive research on modern TUI frameworks and real-world implementations, **Bubble Tea remains the optimal choice** for implementing deep search visualization in ALEX. Key findings:

### Critical Decisions

1. **Framework Choice**: Bubble Tea ecosystem (bubbletea + bubbles + lipgloss)
   - 8,931+ Go projects using it (widespread adoption)
   - Elm Architecture provides clean state management
   - Strong component ecosystem and community

2. **Architecture Pattern**: State Machine + Model Composition
   - Root model as message router
   - Child models for each UI component (tree, output, status)
   - State machine for research phase transitions

3. **Core Components**:
   - **Progress Tree**: `tree-bubble` library for hierarchical task display
   - **Output Viewport**: `bubbles/viewport` for scrollable streaming content
   - **Split Layout**: Custom composition with dynamic sizing
   - **Status Bar**: Built-in spinner + progress + timer

4. **Performance Strategy**:
   - Commands (`tea.Cmd`) for ALL async operations
   - Incremental rendering to avoid flicker
   - Message throttling for high-frequency updates
   - Viewport for efficient large content handling

### Recommended Approach

**Multi-Panel State Machine TUI**:
```
┌─────────────────────────────────────────────────────────┐
│ Status: Phase 2/3: Exploring codebase ⠋ [========>   ]│
├─────────────────────┬───────────────────────────────────┤
│ Research Tree (30%) │ Streaming Output (70%)            │
│                     │                                   │
│ ✓ Query breakdown   │ > Analyzing auth flow...          │
│ ⠋ File exploration  │                                   │
│   ├─ auth.go ✓      │ Found 3 authentication methods:  │
│   ├─ jwt.go ⠋       │ 1. JWT token validation          │
│   └─ oauth.go       │ 2. OAuth 2.0 flow                │
│ ○ Synthesis         │ 3. API key auth                  │
│                     │                                   │
│ [Tab: Switch] [?]   │ [↑↓: Scroll] [Esc: Cancel]       │
└─────────────────────┴───────────────────────────────────┘
```

### Timeline Estimate

- **MVP** (Core functionality): 2-3 weeks
- **Enhanced** (Syntax highlighting, search): 1-2 weeks
- **Polish** (Keyboard shortcuts, help): 1 week

**Total: 4-6 weeks for production-ready implementation**

---

## 1. Framework & Library Analysis

### 1.1 Bubble Tea Ecosystem

**Core Framework**: github.com/charmbracelet/bubbletea (latest: v2, Sept 2025)

**Architecture**: Based on Elm Architecture
```
Model (State) → View (Render) → Update (Events) → Model (New State)
```

**Key Features**:
- **Framerate-based renderer**: Optimized for performance
- **Mouse support**: Click, drag, wheel
- **Focus reporting**: Track terminal focus
- **Streaming**: Handle real-time updates
- **Testing**: `teatest` package for E2E tests

**Adoption Metrics**:
- 8,931 Go projects import it
- Used in production: Glow, Soft Serve, VHS, Charm Cloud CLI
- Active development (last update: Sept 2025)

### 1.2 Component Libraries

**Official: github.com/charmbracelet/bubbles**

Available components:
- **Spinner**: 11 built-in spinner styles, customizable colors
- **Progress**: Customizable progress bar with gradient support
- **Viewport**: Scrollable content with mouse/keyboard
- **Text Input**: Single/multi-line input fields
- **Table**: Sortable tables with styling
- **List**: Interactive lists with filtering
- **Paginator**: Multi-page navigation
- **Timer**: Countdown/stopwatch

**Third-Party: tree-bubble**

```bash
go get github.com/savannahostrowski/tree-bubble
```

Features:
- Hierarchical tree view
- Expand/collapse nodes
- Keyboard navigation
- Integration with Bubble Tea model
- Example code in `/example` directory

**Styling: github.com/charmbracelet/lipgloss**

Advanced styling capabilities:
- Colors (256-color, true color, adaptive)
- Borders, padding, margins
- Text alignment, wrapping
- Width/height calculation helpers
- Responsive layouts

### 1.3 Alternative Frameworks (Considered & Rejected)

**tview** (github.com/rivo/tview)
- ✅ Rich component library (forms, grids, flex boxes)
- ✅ Mature and stable
- ❌ Event-driven (not functional)
- ❌ Less idiomatic Go
- ❌ Harder to test
- **Verdict**: Good for traditional forms, not ideal for streaming/reactive UIs

**termui** (github.com/gizak/termui)
- ✅ Dashboard-style widgets
- ✅ Charts and graphs
- ❌ Less active development
- ❌ No state management pattern
- **Verdict**: Better for monitoring dashboards, not interactive agents

**gocui** (github.com/jroimartin/gocui)
- ✅ Lightweight
- ❌ Low-level, manual event handling
- ❌ No component library
- **Verdict**: Too low-level for complex UIs

---

## 2. UI/UX Design Patterns

### 2.1 Industry Benchmarks

**Aider** (AI pair programming in terminal)
- REPL-style interface
- Shows code diffs directly in terminal
- Voice mode support
- Instant sync with IDE
- **Lesson**: Keep it simple, focus on output clarity

**Cursor CLI** (Agent in terminal)
- Two modes: Interactive TUI + non-interactive print
- File read/write + codebase search
- Approval prompts for shell commands
- **Lesson**: Mode separation for different use cases

**Crush** (Charmbracelet's AI agent)
- Built with Bubble Tea
- Glamorous terminal interface
- Focus on aesthetics + usability
- **Lesson**: Leverage Bubble Tea's styling capabilities

**OpenCode** (Go-based CLI AI)
- TUI for interacting with AI models
- Supports multiple LLM providers
- Debugging and coding assistance
- **Lesson**: Multi-model support, debugging focus

### 2.2 Progress Visualization Patterns

**Three Approaches**:

1. **Spinner + Text** (Simplest)
```
⠋ Exploring codebase... (Step 2/5)
```
- ✅ Minimal space
- ✅ Simple implementation
- ❌ Limited information
- **Use case**: Inline progress in simple mode

2. **Progress Bar + Status**
```
Analyzing authentication flow [=========>    ] 65%
Current: Exploring jwt.go
```
- ✅ Clear progress indication
- ✅ Shows current action
- ❌ Linear only (not good for tree exploration)
- **Use case**: Sequential tasks

3. **Hierarchical Tree** (Recommended for Deep Search)
```
✓ Phase 1: Query decomposition
⠋ Phase 2: Multi-file exploration
  ├─ ✓ auth/jwt.go
  ├─ ⠋ auth/oauth.go (analyzing dependencies...)
  ├─ ○ auth/session.go (pending)
  └─ ○ middleware/auth.go (pending)
○ Phase 3: Synthesis
```
- ✅ Shows hierarchy and dependencies
- ✅ Visual progress at each level
- ✅ Expandable/collapsible
- ❌ More complex to implement
- **Use case**: Deep search with subtasks

### 2.3 Streaming Output Patterns

**Real-Time Text Streaming**:

```go
// Pattern 1: Append-only (ChatGPT style)
type Model struct {
    content string  // Accumulated text
}

func (m Model) Update(msg tea.Msg) {
    switch msg := msg.(type) {
    case StreamMsg:
        m.content += msg.Text  // Append new chunk
        // Auto-scroll viewport to bottom
    }
}

// Pattern 2: Viewport with auto-scroll
type Model struct {
    viewport viewport.Model
}

func (m Model) Update(msg tea.Msg) {
    switch msg := msg.(type) {
    case StreamMsg:
        m.viewport.SetContent(m.viewport.Content() + msg.Text)
        m.viewport.GotoBottom()  // Auto-scroll
    }
}
```

**Best Practices**:
- Use viewport for >20 lines of content
- Auto-scroll during streaming, allow manual scroll
- Visual indicator when scrolled away from bottom
- Throttle viewport updates (max 60 FPS)

### 2.4 Split-Pane Layout

**Vertical Split** (Recommended):
```
┌─────────────┬─────────────────────┐
│ Tree (30%)  │ Output (70%)        │
│             │                     │
│ Fixed       │ Scrollable viewport │
└─────────────┴─────────────────────┘
```

**Implementation**:
```go
func (m Model) View() string {
    // Calculate widths
    treeWidth := int(float64(m.width) * 0.3)
    outputWidth := m.width - treeWidth - 1  // -1 for border

    // Render components
    tree := m.treeModel.View()
    output := m.outputViewport.View()

    // Use lipgloss to join horizontally
    return lipgloss.JoinHorizontal(
        lipgloss.Top,
        lipgloss.NewStyle().Width(treeWidth).Render(tree),
        lipgloss.NewStyle().Width(outputWidth).Render(output),
    )
}
```

**Dynamic Resizing**:
```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height

        // Update child component sizes
        m.outputViewport.Width = int(float64(m.width) * 0.7)
        m.outputViewport.Height = m.height - 3  // -3 for status bar
    }
}
```

**Adjustable Split** (Future Enhancement):
```go
// Allow user to adjust split ratio with +/- keys
case tea.KeyMsg:
    switch msg.String() {
    case "+":
        m.splitRatio = min(0.5, m.splitRatio + 0.05)
    case "-":
        m.splitRatio = max(0.2, m.splitRatio - 0.05)
    }
```

### 2.5 Status Bar Design

**Information Hierarchy**:
```
[Phase] [Current Task] [Spinner] [Progress] [Time] [Actions]
```

**Example**:
```
Phase 2/3: Exploring ⠋ [=====>     ] 45% 0:32 [Tab:Switch][?:Help][Esc:Cancel]
```

**Implementation**:
```go
type StatusBar struct {
    phase         string
    currentTask   string
    spinner       spinner.Model
    progress      float64
    elapsed       time.Duration
    width         int
}

func (s StatusBar) View() string {
    // Spinner
    spinnerStr := s.spinner.View()

    // Progress bar (fixed width)
    progWidth := 12
    filled := int(s.progress * float64(progWidth))
    progressBar := "[" + strings.Repeat("=", filled) +
                  strings.Repeat(" ", progWidth-filled) + "]"

    // Time
    timeStr := fmt.Sprintf("%d:%02d",
        int(s.elapsed.Minutes()),
        int(s.elapsed.Seconds())%60)

    // Left side
    left := fmt.Sprintf("%s: %s %s %s %.0f%% %s",
        s.phase, s.currentTask, spinnerStr,
        progressBar, s.progress*100, timeStr)

    // Right side (actions)
    right := "[Tab:Switch][?:Help][Esc:Cancel]"

    // Pad to fill width
    padding := s.width - lipgloss.Width(left) - lipgloss.Width(right)
    if padding < 0 {
        padding = 0
    }

    return lipgloss.JoinHorizontal(
        lipgloss.Top,
        left,
        strings.Repeat(" ", padding),
        right,
    )
}
```

---

## 3. Architecture Recommendations

### 3.1 Bubble Tea Model Structure

**Hierarchical Model Composition**:

```go
// Root model (message router + compositor)
type RootModel struct {
    // Current mode
    mode Mode  // enum: TreeFocus, OutputFocus, Help

    // Child models
    statusBar    StatusBarModel
    tree         TreeModel
    output       OutputModel
    help         HelpModel

    // Shared state
    width        int
    height       int
    splitRatio   float64

    // Research state
    researchState ResearchState
}

// Tree model (left pane)
type TreeModel struct {
    tree         *tree.Model  // tree-bubble component
    nodes        []TreeNode
    focused      bool
}

// Output model (right pane)
type OutputModel struct {
    viewport     viewport.Model
    content      strings.Builder
    autoScroll   bool
    focused      bool
}

// Status bar model
type StatusBarModel struct {
    spinner      spinner.Model
    phase        ResearchPhase
    progress     float64
    elapsed      time.Duration
    startTime    time.Time
}

// Help model (overlay)
type HelpModel struct {
    visible      bool
    shortcuts    []Shortcut
}
```

### 3.2 Message Passing Patterns

**Message Types**:

```go
// System messages (from Bubble Tea)
type tea.WindowSizeMsg  // Terminal resized
type tea.KeyMsg         // Keyboard input
type tea.MouseMsg       // Mouse events

// Research messages (from ReAct engine)
type ResearchStartedMsg struct {
    Query string
    TotalPhases int
}

type PhaseChangedMsg struct {
    Phase ResearchPhase
    Description string
}

type SubtaskStartedMsg struct {
    ParentID string
    Task     string
}

type SubtaskCompletedMsg struct {
    TaskID   string
    Result   string
}

type OutputStreamMsg struct {
    Text string
    Source string  // "thought", "action", "observation"
}

type ResearchCompletedMsg struct {
    Report string
    Duration time.Duration
}

type ResearchErrorMsg struct {
    Error error
}

// UI messages (internal)
type TickMsg time.Time  // For spinner/timer
type FocusSwitchMsg struct{}
type ToggleHelpMsg struct{}
```

**Message Routing**:

```go
func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmds []tea.Cmd

    switch msg := msg.(type) {

    // Global messages (handle in root)
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        m.resizeChildren()

    case tea.KeyMsg:
        // Global shortcuts
        switch msg.String() {
        case "ctrl+c", "q":
            return m, tea.Quit
        case "?":
            m.help.visible = !m.help.visible
        case "tab":
            m.switchFocus()
        }

        // Route to focused component
        if m.mode == TreeFocus {
            newTree, cmd := m.tree.Update(msg)
            m.tree = newTree.(TreeModel)
            cmds = append(cmds, cmd)
        } else if m.mode == OutputFocus {
            newOutput, cmd := m.output.Update(msg)
            m.output = newOutput.(OutputModel)
            cmds = append(cmds, cmd)
        }

    // Research messages (broadcast to relevant children)
    case PhaseChangedMsg:
        m.statusBar.phase = msg.Phase
        m.tree.addPhaseNode(msg)
        m.output.appendPhaseHeader(msg)

    case OutputStreamMsg:
        newOutput, cmd := m.output.Update(msg)
        m.output = newOutput.(OutputModel)
        cmds = append(cmds, cmd)

    // Timer tick (for spinner/elapsed time)
    case TickMsg:
        newStatus, cmd := m.statusBar.Update(msg)
        m.statusBar = newStatus.(StatusBarModel)
        cmds = append(cmds, cmd, tickCmd())  // Schedule next tick
    }

    return m, tea.Batch(cmds...)
}
```

### 3.3 State Machine Integration

**Research Phases as States**:

```go
type ResearchPhase int

const (
    PhaseIdle ResearchPhase = iota
    PhaseDecomposition  // Breaking down query
    PhaseExploration    // Iterative search
    PhaseSynthesis      // Generating report
    PhaseComplete
    PhaseError
)

type ResearchState struct {
    currentPhase  ResearchPhase
    totalPhases   int
    phaseProgress map[ResearchPhase]float64
    activeTasks   map[string]TaskStatus
    completedTasks []string
    errors        []error
}

// State transitions
func (s *ResearchState) TransitionTo(phase ResearchPhase) error {
    // Validate transition
    if !s.isValidTransition(s.currentPhase, phase) {
        return fmt.Errorf("invalid transition: %v -> %v",
            s.currentPhase, phase)
    }

    s.currentPhase = phase
    return nil
}

func (s *ResearchState) isValidTransition(from, to ResearchPhase) bool {
    validTransitions := map[ResearchPhase][]ResearchPhase{
        PhaseIdle:          {PhaseDecomposition},
        PhaseDecomposition: {PhaseExploration, PhaseError},
        PhaseExploration:   {PhaseSynthesis, PhaseError},
        PhaseSynthesis:     {PhaseComplete, PhaseError},
        PhaseComplete:      {PhaseIdle},
        PhaseError:         {PhaseIdle},
    }

    allowed := validTransitions[from]
    for _, valid := range allowed {
        if valid == to {
            return true
        }
    }
    return false
}
```

**Integration with Bubble Tea**:

```go
func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {

    case PhaseChangedMsg:
        // Update state machine
        if err := m.researchState.TransitionTo(msg.Phase); err != nil {
            // Handle invalid transition
            return m, nil
        }

        // Update UI based on new phase
        m.statusBar.phase = msg.Phase
        m.statusBar.progress = 0.0  // Reset phase progress

        // Add phase node to tree
        m.tree.addPhaseNode(PhaseNode{
            Phase: msg.Phase,
            Description: msg.Description,
            Status: StatusInProgress,
        })

    case SubtaskCompletedMsg:
        // Update task status
        m.researchState.activeTasks[msg.TaskID] = TaskComplete
        m.researchState.completedTasks = append(
            m.researchState.completedTasks,
            msg.TaskID,
        )

        // Update phase progress
        m.updatePhaseProgress()

        // Mark tree node as complete
        m.tree.markNodeComplete(msg.TaskID)
    }
}
```

### 3.4 Integration with ALEX ReAct Engine

**Current Architecture**:
```
cmd/alex/cli.go (CLI entry)
    ↓
internal/agent/app/coordinator.go (Orchestrator)
    ↓
internal/agent/domain/react_engine.go (ReAct loop)
    ↓
internal/tools/* (Tool execution)
```

**New TUI Integration**:
```
cmd/alex/cli.go
    ↓
cmd/alex/tui_deep_search.go (New TUI mode)
    ↓
internal/agent/app/coordinator.go (Enhanced with callbacks)
    ↓
internal/agent/domain/react_engine.go (Emit events)
    ↓
internal/tools/* (Tool execution)
```

**Event Emission Pattern**:

```go
// In internal/agent/domain/react_engine.go

type ReactEventCallback func(event ReactEvent)

type ReactEngine struct {
    maxIterations int
    eventCallback ReactEventCallback  // NEW
}

type ReactEvent interface {
    EventType() string
}

type ThoughtEvent struct {
    Iteration int
    Thought   string
}

type ActionEvent struct {
    Iteration int
    Tool      string
    Input     map[string]interface{}
}

type ObservationEvent struct {
    Iteration int
    Output    string
}

// In SolveTask method:
func (e *ReactEngine) SolveTask(task string) (string, error) {
    for iteration := 0; iteration < e.maxIterations; iteration++ {
        // Get thought
        thought := e.getThought(task, history)
        if e.eventCallback != nil {
            e.eventCallback(ThoughtEvent{
                Iteration: iteration,
                Thought: thought,
            })
        }

        // Execute action
        result := e.executeAction(action)
        if e.eventCallback != nil {
            e.eventCallback(ObservationEvent{
                Iteration: iteration,
                Output: result,
            })
        }
    }
}
```

**TUI Bridge**:

```go
// In cmd/alex/tui_deep_search.go

type DeepSearchTUI struct {
    program      *tea.Program
    eventChan    chan ReactEvent
}

func (t *DeepSearchTUI) Run(query string) error {
    // Start TUI in goroutine
    go func() {
        t.program.Run()
    }()

    // Configure ReAct engine with callback
    engine := agent.NewReactEngine(10)
    engine.SetEventCallback(func(event ReactEvent) {
        // Send event to TUI via channel
        t.eventChan <- event
    })

    // Run research
    result, err := engine.SolveTask(query)

    // Send completion event
    t.eventChan <- CompletionEvent{Result: result, Error: err}

    return err
}

// In TUI model:
func (m RootModel) Init() tea.Cmd {
    return tea.Batch(
        tickCmd(),
        waitForReactEvents(m.eventChan),  // Listen for events
    )
}

func waitForReactEvents(ch chan ReactEvent) tea.Cmd {
    return func() tea.Msg {
        event := <-ch

        // Convert ReactEvent to Bubble Tea message
        switch e := event.(type) {
        case ThoughtEvent:
            return OutputStreamMsg{
                Text: "💭 " + e.Thought + "\n",
                Source: "thought",
            }
        case ActionEvent:
            return SubtaskStartedMsg{
                Task: fmt.Sprintf("%s(%v)", e.Tool, e.Input),
            }
        case ObservationEvent:
            return OutputStreamMsg{
                Text: "📋 " + e.Output + "\n",
                Source: "observation",
            }
        }
    }
}
```

---

## 4. Component Specifications

### 4.1 Progress Tree Component

**Purpose**: Show hierarchical research progress with expandable nodes

**Features**:
- Multi-level tree (phases → subtasks → files)
- Visual status indicators (✓ ⠋ ○ ✗)
- Expand/collapse nodes
- Keyboard navigation
- Auto-scroll to active node

**Implementation**:

```go
import tree "github.com/savannahostrowski/tree-bubble"

type TreeModel struct {
    tree     *tree.Model
    nodes    map[string]*TreeNode
    focused  bool
    width    int
    height   int
}

type TreeNode struct {
    ID          string
    Parent      string
    Title       string
    Status      NodeStatus
    Children    []string
    Expanded    bool
    Metadata    map[string]interface{}
}

type NodeStatus int

const (
    StatusPending NodeStatus = iota
    StatusInProgress
    StatusComplete
    StatusError
)

func NewTreeModel(width, height int) TreeModel {
    t := tree.New()

    return TreeModel{
        tree:   &t,
        nodes:  make(map[string]*TreeNode),
        width:  width,
        height: height,
    }
}

func (m *TreeModel) AddPhaseNode(phase ResearchPhase, description string) string {
    nodeID := fmt.Sprintf("phase_%d", phase)

    node := &TreeNode{
        ID:       nodeID,
        Parent:   "",  // Root level
        Title:    description,
        Status:   StatusInProgress,
        Children: []string{},
        Expanded: true,
    }

    m.nodes[nodeID] = node
    m.rebuildTree()

    return nodeID
}

func (m *TreeModel) AddSubtaskNode(parentID, task string) string {
    nodeID := uuid.New().String()

    node := &TreeNode{
        ID:       nodeID,
        Parent:   parentID,
        Title:    task,
        Status:   StatusInProgress,
        Children: []string{},
        Expanded: false,
    }

    m.nodes[nodeID] = node

    // Add to parent's children
    if parent, ok := m.nodes[parentID]; ok {
        parent.Children = append(parent.Children, nodeID)
    }

    m.rebuildTree()

    return nodeID
}

func (m *TreeModel) MarkNodeComplete(nodeID string) {
    if node, ok := m.nodes[nodeID]; ok {
        node.Status = StatusComplete
        m.rebuildTree()
    }
}

func (m *TreeModel) MarkNodeError(nodeID string, err error) {
    if node, ok := m.nodes[nodeID]; ok {
        node.Status = StatusError
        node.Metadata["error"] = err.Error()
        m.rebuildTree()
    }
}

func (m *TreeModel) rebuildTree() {
    // Convert nodes to tree-bubble format
    // This would rebuild the visual tree representation
}

func (m TreeModel) View() string {
    var b strings.Builder

    // Render tree with custom styling
    for _, node := range m.getRootNodes() {
        m.renderNode(&b, node, 0)
    }

    return lipgloss.NewStyle().
        Width(m.width).
        Height(m.height).
        Render(b.String())
}

func (m TreeModel) renderNode(b *strings.Builder, node *TreeNode, depth int) {
    indent := strings.Repeat("  ", depth)

    // Status icon
    icon := m.getStatusIcon(node.Status)

    // Expand/collapse indicator
    expandIcon := ""
    if len(node.Children) > 0 {
        if node.Expanded {
            expandIcon = "▼ "
        } else {
            expandIcon = "▶ "
        }
    }

    // Render line
    style := m.getNodeStyle(node.Status, m.focused)
    line := fmt.Sprintf("%s%s%s%s\n", indent, icon, expandIcon, node.Title)
    b.WriteString(style.Render(line))

    // Render children if expanded
    if node.Expanded {
        for _, childID := range node.Children {
            if child, ok := m.nodes[childID]; ok {
                m.renderNode(b, child, depth+1)
            }
        }
    }
}

func (m TreeModel) getStatusIcon(status NodeStatus) string {
    switch status {
    case StatusPending:
        return "○ "
    case StatusInProgress:
        return "⠋ "  // Will be animated with spinner
    case StatusComplete:
        return "✓ "
    case StatusError:
        return "✗ "
    default:
        return "  "
    }
}

func (m TreeModel) getNodeStyle(status NodeStatus, focused bool) lipgloss.Style {
    baseStyle := lipgloss.NewStyle()

    if focused {
        baseStyle = baseStyle.Bold(true)
    }

    switch status {
    case StatusComplete:
        return baseStyle.Foreground(lipgloss.Color("10"))  // Green
    case StatusError:
        return baseStyle.Foreground(lipgloss.Color("9"))   // Red
    case StatusInProgress:
        return baseStyle.Foreground(lipgloss.Color("12"))  // Blue
    default:
        return baseStyle.Foreground(lipgloss.Color("8"))   // Gray
    }
}
```

**Keyboard Navigation**:
```go
func (m TreeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "up", "k":
            m.selectPrevious()
        case "down", "j":
            m.selectNext()
        case "left", "h":
            m.collapseSelected()
        case "right", "l", "enter":
            m.expandSelected()
        }
    }
    return m, nil
}
```

### 4.2 Streaming Output Component

**Purpose**: Display real-time LLM output with auto-scroll

**Features**:
- Viewport for scrollable content
- Auto-scroll during streaming
- Manual scroll with visual indicator
- Syntax highlighting (future)
- Copy support

**Implementation**:

```go
import "github.com/charmbracelet/bubbles/viewport"

type OutputModel struct {
    viewport    viewport.Model
    content     strings.Builder
    autoScroll  bool
    focused     bool
    width       int
    height      int
}

func NewOutputModel(width, height int) OutputModel {
    vp := viewport.New(width, height)
    vp.MouseWheelEnabled = true
    vp.KeyMap = viewport.KeyMap{
        PageDown: key.NewBinding(
            key.WithKeys("pgdown", "space"),
        ),
        PageUp: key.NewBinding(
            key.WithKeys("pgup"),
        ),
        HalfPageDown: key.NewBinding(
            key.WithKeys("ctrl+d"),
        ),
        HalfPageUp: key.NewBinding(
            key.WithKeys("ctrl+u"),
        ),
        Down: key.NewBinding(
            key.WithKeys("down", "j"),
        ),
        Up: key.NewBinding(
            key.WithKeys("up", "k"),
        ),
    }

    return OutputModel{
        viewport:   vp,
        autoScroll: true,
        width:      width,
        height:     height,
    }
}

func (m *OutputModel) AppendText(text, source string) {
    // Add source prefix
    prefix := m.getSourcePrefix(source)

    // Append to content
    m.content.WriteString(prefix + text)

    // Update viewport
    m.viewport.SetContent(m.content.String())

    // Auto-scroll if enabled
    if m.autoScroll {
        m.viewport.GotoBottom()
    }
}

func (m OutputModel) getSourcePrefix(source string) string {
    switch source {
    case "thought":
        return lipgloss.NewStyle().
            Foreground(lipgloss.Color("12")).
            Render("💭 ")
    case "action":
        return lipgloss.NewStyle().
            Foreground(lipgloss.Color("14")).
            Render("🔧 ")
    case "observation":
        return lipgloss.NewStyle().
            Foreground(lipgloss.Color("10")).
            Render("📋 ")
    default:
        return ""
    }
}

func (m OutputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmd tea.Cmd

    switch msg := msg.(type) {

    case OutputStreamMsg:
        m.AppendText(msg.Text, msg.Source)
        return m, nil

    case tea.KeyMsg:
        // Disable auto-scroll on manual scroll
        if msg.String() == "up" || msg.String() == "down" ||
           msg.String() == "pgup" || msg.String() == "pgdown" {
            m.autoScroll = false
        }

        // Re-enable auto-scroll at bottom
        if msg.String() == "G" || msg.String() == "end" {
            m.autoScroll = true
            m.viewport.GotoBottom()
        }
    }

    // Update viewport
    m.viewport, cmd = m.viewport.Update(msg)

    return m, cmd
}

func (m OutputModel) View() string {
    view := m.viewport.View()

    // Add scroll indicator if not at bottom
    if !m.autoScroll && !m.viewport.AtBottom() {
        indicator := lipgloss.NewStyle().
            Foreground(lipgloss.Color("11")).
            Render("↓ More content below (G to jump to end)")

        view += "\n" + indicator
    }

    return view
}
```

### 4.3 Status Bar Component

**Purpose**: Show current phase, progress, and actions

**Implementation**:

```go
import "github.com/charmbracelet/bubbles/spinner"

type StatusBarModel struct {
    spinner     spinner.Model
    phase       ResearchPhase
    currentTask string
    progress    float64
    startTime   time.Time
    width       int
}

func NewStatusBarModel(width int) StatusBarModel {
    s := spinner.New()
    s.Spinner = spinner.Dot
    s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))

    return StatusBarModel{
        spinner:   s,
        startTime: time.Now(),
        width:     width,
    }
}

func (m StatusBarModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {

    case TickMsg:
        // Update spinner
        var cmd tea.Cmd
        m.spinner, cmd = m.spinner.Update(msg)
        return m, cmd

    case PhaseChangedMsg:
        m.phase = msg.Phase
        m.currentTask = msg.Description
        return m, nil

    case ProgressUpdateMsg:
        m.progress = msg.Progress
        return m, nil
    }

    return m, nil
}

func (m StatusBarModel) View() string {
    // Phase name
    phaseName := m.getPhaseString()

    // Spinner (only show if in progress)
    spinnerStr := ""
    if m.phase != PhaseComplete && m.phase != PhaseError {
        spinnerStr = m.spinner.View() + " "
    }

    // Progress bar
    progWidth := 12
    filled := int(m.progress * float64(progWidth))
    progressBar := "[" +
        strings.Repeat("=", filled) +
        ">" +
        strings.Repeat(" ", progWidth-filled-1) +
        "]"

    // Elapsed time
    elapsed := time.Since(m.startTime)
    timeStr := fmt.Sprintf("%d:%02d",
        int(elapsed.Minutes()),
        int(elapsed.Seconds())%60)

    // Left side
    left := fmt.Sprintf("%s %s%s %.0f%% %s",
        phaseName,
        spinnerStr,
        progressBar,
        m.progress*100,
        timeStr,
    )

    // Right side (keyboard shortcuts)
    right := "[Tab:Switch] [?:Help] [Esc:Cancel]"

    // Pad to fill width
    padding := m.width - lipgloss.Width(left) - lipgloss.Width(right)
    if padding < 0 {
        padding = 0
        right = ""  // Hide shortcuts if not enough space
    }

    statusStyle := lipgloss.NewStyle().
        Background(lipgloss.Color("236")).
        Foreground(lipgloss.Color("255")).
        Padding(0, 1)

    return statusStyle.Render(
        left + strings.Repeat(" ", padding) + right,
    )
}

func (m StatusBarModel) getPhaseString() string {
    switch m.phase {
    case PhaseDecomposition:
        return "Phase 1/3: Query Analysis"
    case PhaseExploration:
        return "Phase 2/3: Deep Exploration"
    case PhaseSynthesis:
        return "Phase 3/3: Synthesizing Report"
    case PhaseComplete:
        return "✓ Complete"
    case PhaseError:
        return "✗ Error"
    default:
        return "Initializing..."
    }
}
```

### 4.4 Help Overlay Component

**Purpose**: Show keyboard shortcuts and usage instructions

**Implementation**:

```go
type HelpModel struct {
    visible   bool
    shortcuts []ShortcutSection
    width     int
    height    int
}

type ShortcutSection struct {
    Title     string
    Shortcuts []Shortcut
}

type Shortcut struct {
    Keys        []string
    Description string
}

func NewHelpModel(width, height int) HelpModel {
    return HelpModel{
        visible: false,
        width:   width,
        height:  height,
        shortcuts: []ShortcutSection{
            {
                Title: "Navigation",
                Shortcuts: []Shortcut{
                    {[]string{"Tab"}, "Switch focus between panels"},
                    {[]string{"↑", "k"}, "Move up"},
                    {[]string{"↓", "j"}, "Move down"},
                    {[]string{"←", "h"}, "Collapse node / Scroll left"},
                    {[]string{"→", "l"}, "Expand node / Scroll right"},
                },
            },
            {
                Title: "Scrolling (Output Panel)",
                Shortcuts: []Shortcut{
                    {[]string{"Space", "PgDn"}, "Page down"},
                    {[]string{"PgUp"}, "Page up"},
                    {[]string{"Ctrl+d"}, "Half page down"},
                    {[]string{"Ctrl+u"}, "Half page up"},
                    {[]string{"G", "End"}, "Jump to bottom"},
                },
            },
            {
                Title: "Actions",
                Shortcuts: []Shortcut{
                    {[]string{"?"}, "Toggle this help"},
                    {[]string{"Esc"}, "Cancel research"},
                    {[]string{"Ctrl+c", "q"}, "Quit"},
                },
            },
        },
    }
}

func (m HelpModel) View() string {
    if !m.visible {
        return ""
    }

    var b strings.Builder

    // Title
    titleStyle := lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("12")).
        Padding(1, 0)

    b.WriteString(titleStyle.Render("Keyboard Shortcuts") + "\n\n")

    // Sections
    for _, section := range m.shortcuts {
        // Section title
        sectionStyle := lipgloss.NewStyle().
            Bold(true).
            Foreground(lipgloss.Color("14"))

        b.WriteString(sectionStyle.Render(section.Title) + "\n")

        // Shortcuts
        for _, shortcut := range section.Shortcuts {
            keysStyle := lipgloss.NewStyle().
                Foreground(lipgloss.Color("10"))

            keys := strings.Join(shortcut.Keys, ", ")
            line := fmt.Sprintf("  %-20s %s\n",
                keysStyle.Render(keys),
                shortcut.Description,
            )
            b.WriteString(line)
        }
        b.WriteString("\n")
    }

    // Create overlay
    content := b.String()

    overlayStyle := lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("12")).
        Padding(1, 2).
        Width(60).
        Background(lipgloss.Color("236"))

    overlay := overlayStyle.Render(content)

    // Center on screen
    return lipgloss.Place(
        m.width,
        m.height,
        lipgloss.Center,
        lipgloss.Center,
        overlay,
    )
}
```

---

## 5. Performance Considerations

### 5.1 Rendering Optimization

**Problem**: Frequent updates cause flicker and high CPU usage

**Solutions**:

1. **Throttle Updates** (Max 60 FPS)
```go
type ThrottledUpdater struct {
    lastUpdate time.Time
    minInterval time.Duration
    pending     bool
    pendingMsg  tea.Msg
}

func (t *ThrottledUpdater) ShouldUpdate(msg tea.Msg) bool {
    now := time.Now()

    if now.Sub(t.lastUpdate) < t.minInterval {
        // Too soon, store for later
        t.pending = true
        t.pendingMsg = msg
        return false
    }

    t.lastUpdate = now
    t.pending = false
    return true
}

// Usage in Update:
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case OutputStreamMsg:
        if !m.throttler.ShouldUpdate(msg) {
            return m, nil  // Skip this update
        }
        // ... process update
    }
}
```

2. **Batch Text Updates**
```go
type OutputBuffer struct {
    buffer    strings.Builder
    lastFlush time.Time
    flushInterval time.Duration
}

func (b *OutputBuffer) Append(text string) {
    b.buffer.WriteString(text)
}

func (b *OutputBuffer) ShouldFlush() bool {
    return time.Since(b.lastFlush) > b.flushInterval ||
           b.buffer.Len() > 1024  // Flush if buffer too large
}

func (b *OutputBuffer) Flush() string {
    text := b.buffer.String()
    b.buffer.Reset()
    b.lastFlush = time.Now()
    return text
}
```

3. **Incremental Rendering**
```go
// Only re-render changed components
func (m RootModel) View() string {
    // Cache unchanged parts
    if !m.statusBar.dirty {
        // Reuse cached status bar render
    } else {
        m.cachedStatusBar = m.statusBar.View()
        m.statusBar.dirty = false
    }

    // Only compute layout if size changed
    if m.layoutDirty {
        m.recomputeLayout()
        m.layoutDirty = false
    }

    return m.compositeView()
}
```

### 5.2 Background Task Patterns

**Use Commands, NOT Raw Goroutines**

❌ **Wrong**:
```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    go func() {
        // This breaks Bubble Tea's model!
        result := expensiveOperation()
        m.result = result  // RACE CONDITION!
    }()
    return m, nil
}
```

✅ **Correct**:
```go
// Define command
func doExpensiveOperation() tea.Msg {
    result := expensiveOperation()
    return OperationCompleteMsg{Result: result}
}

// Use in Update
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case StartOperationMsg:
        return m, doExpensiveOperation  // Return command

    case OperationCompleteMsg:
        m.result = msg.Result  // Safe update
        return m, nil
    }
}
```

**Long-Running Background Task Pattern**:
```go
// Create channel for communication
func subscribeToReactEngine(engineChan <-chan ReactEvent) tea.Cmd {
    return func() tea.Msg {
        // This blocks until event arrives
        event := <-engineChan

        // Convert to Bubble Tea message
        switch e := event.(type) {
        case ThoughtEvent:
            return OutputStreamMsg{
                Text: e.Thought,
                Source: "thought",
            }
        }
    }
}

// Re-subscribe after each event to create continuous loop
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case OutputStreamMsg:
        m.output.AppendText(msg.Text, msg.Source)

        // Re-subscribe for next event
        return m, subscribeToReactEngine(m.engineChan)
    }
}
```

### 5.3 Memory Management

**Problem**: Long research sessions accumulate large output

**Solutions**:

1. **Content Truncation**
```go
const MaxOutputLines = 10000

func (m *OutputModel) AppendText(text string) {
    m.content.WriteString(text)

    // Check line count
    lines := strings.Split(m.content.String(), "\n")
    if len(lines) > MaxOutputLines {
        // Keep last N lines
        keep := lines[len(lines)-MaxOutputLines:]
        m.content.Reset()
        m.content.WriteString(strings.Join(keep, "\n"))
    }

    m.viewport.SetContent(m.content.String())
}
```

2. **Tree Node Pruning**
```go
// Collapse completed phases to save memory
func (m *TreeModel) PruneCompleted() {
    for _, node := range m.nodes {
        if node.Status == StatusComplete &&
           time.Since(node.CompletedAt) > 5*time.Minute {
            // Collapse old completed nodes
            node.Expanded = false

            // Remove child nodes from memory (keep IDs only)
            node.Children = nil
        }
    }
}
```

3. **Viewport Optimization**
```go
// Only keep visible + buffer content in viewport
// (Bubble Tea viewport already does this)
vp := viewport.New(width, height)
vp.SetContent(largeText)  // Internally only renders visible portion
```

---

## 6. Implementation Examples

### 6.1 Minimal Working Example

**File: `cmd/alex/tui_minimal_example.go`**

```go
package main

import (
    "fmt"
    "os"
    "time"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/bubbles/spinner"
    "github.com/charmbracelet/lipgloss"
)

// Messages
type tickMsg time.Time
type researchMsg string

// Model
type model struct {
    spinner  spinner.Model
    phase    string
    output   []string
    quitting bool
}

func initialModel() model {
    s := spinner.New()
    s.Spinner = spinner.Dot
    s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))

    return model{
        spinner: s,
        phase:   "Starting...",
        output:  []string{},
    }
}

func (m model) Init() tea.Cmd {
    return tea.Batch(
        m.spinner.Tick,
        tick(),
        simulateResearch(),
    )
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {

    case tea.KeyMsg:
        switch msg.String() {
        case "ctrl+c", "q":
            m.quitting = true
            return m, tea.Quit
        }

    case tickMsg:
        return m, tick()

    case researchMsg:
        m.output = append(m.output, string(msg))
        return m, simulateResearch()  // Continue research

    case spinner.TickMsg:
        var cmd tea.Cmd
        m.spinner, cmd = m.spinner.Update(msg)
        return m, cmd
    }

    return m, nil
}

func (m model) View() string {
    if m.quitting {
        return "Goodbye!\n"
    }

    s := fmt.Sprintf("\n %s %s\n\n", m.spinner.View(), m.phase)

    for _, line := range m.output {
        s += "  " + line + "\n"
    }

    s += "\n  Press q to quit\n"

    return s
}

// Commands
func tick() tea.Cmd {
    return tea.Tick(time.Second, func(t time.Time) tea.Msg {
        return tickMsg(t)
    })
}

func simulateResearch() tea.Cmd {
    return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
        return researchMsg(fmt.Sprintf("Research update: %s", t.Format("15:04:05")))
    })
}

func main() {
    p := tea.NewProgram(initialModel())
    if _, err := p.Run(); err != nil {
        fmt.Printf("Error: %v", err)
        os.Exit(1)
    }
}
```

### 6.2 Split-Pane Prototype

```go
package main

import (
    "fmt"
    "strings"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/bubbles/viewport"
    "github.com/charmbracelet/lipgloss"
)

type model struct {
    width       int
    height      int
    splitRatio  float64
    leftPane    viewport.Model
    rightPane   viewport.Model
    focusLeft   bool
}

func initialModel() model {
    return model{
        splitRatio: 0.3,
        focusLeft:  true,
    }
}

func (m model) Init() tea.Cmd {
    return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmds []tea.Cmd

    switch msg := msg.(type) {

    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height

        // Calculate pane sizes
        leftWidth := int(float64(m.width) * m.splitRatio)
        rightWidth := m.width - leftWidth - 1  // -1 for border

        // Initialize viewports
        m.leftPane = viewport.New(leftWidth, m.height-2)
        m.rightPane = viewport.New(rightWidth, m.height-2)

        // Set content
        m.leftPane.SetContent(generateTreeContent())
        m.rightPane.SetContent(generateOutputContent())

    case tea.KeyMsg:
        switch msg.String() {
        case "ctrl+c", "q":
            return m, tea.Quit
        case "tab":
            m.focusLeft = !m.focusLeft
        }

        // Route to focused pane
        if m.focusLeft {
            var cmd tea.Cmd
            m.leftPane, cmd = m.leftPane.Update(msg)
            cmds = append(cmds, cmd)
        } else {
            var cmd tea.Cmd
            m.rightPane, cmd = m.rightPane.Update(msg)
            cmds = append(cmds, cmd)
        }
    }

    return m, tea.Batch(cmds...)
}

func (m model) View() string {
    // Styles
    focusedStyle := lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("12"))

    unfocusedStyle := lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("8"))

    // Apply styles
    leftStyle := unfocusedStyle
    rightStyle := unfocusedStyle
    if m.focusLeft {
        leftStyle = focusedStyle
    } else {
        rightStyle = focusedStyle
    }

    // Render panes
    left := leftStyle.Render(m.leftPane.View())
    right := rightStyle.Render(m.rightPane.View())

    // Join horizontally
    return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func generateTreeContent() string {
    return `✓ Phase 1: Query Analysis
⠋ Phase 2: Exploration
  ├─ ✓ auth.go
  ├─ ⠋ jwt.go
  └─ ○ oauth.go
○ Phase 3: Synthesis`
}

func generateOutputContent() string {
    lines := []string{
        "💭 Analyzing authentication flow...",
        "",
        "🔧 file_read(path='internal/auth/jwt.go')",
        "",
        "📋 Found JWT implementation:",
        "  - Token generation",
        "  - Token validation",
        "  - Refresh token logic",
        "",
        "💭 Need to check OAuth integration...",
    }
    return strings.Join(lines, "\n")
}

func main() {
    p := tea.NewProgram(
        initialModel(),
        tea.WithAltScreen(),
        tea.WithMouseCellMotion(),
    )

    if _, err := p.Run(); err != nil {
        fmt.Printf("Error: %v", err)
    }
}
```

### 6.3 Real Integration with ALEX

**File: `cmd/alex/tui_deep_search.go`**

```go
package main

import (
    "context"
    "fmt"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/your-org/alex/internal/agent/app"
    "github.com/your-org/alex/internal/agent/domain"
)

// Bridge between ReAct engine and TUI
type DeepSearchBridge struct {
    coordinator *app.Coordinator
    eventChan   chan domain.ReactEvent
    program     *tea.Program
}

func NewDeepSearchBridge(coordinator *app.Coordinator) *DeepSearchBridge {
    return &DeepSearchBridge{
        coordinator: coordinator,
        eventChan:   make(chan domain.ReactEvent, 100),
    }
}

func (b *DeepSearchBridge) Run(ctx context.Context, query string) error {
    // Create initial model
    initialModel := NewDeepSearchModel(b.eventChan)

    // Start TUI
    b.program = tea.NewProgram(
        initialModel,
        tea.WithAltScreen(),
        tea.WithMouseCellMotion(),
    )

    // Start research in background
    go b.runResearch(ctx, query)

    // Run TUI (blocks until quit)
    if _, err := b.program.Run(); err != nil {
        return err
    }

    return nil
}

func (b *DeepSearchBridge) runResearch(ctx context.Context, query string) {
    // Configure coordinator to emit events
    b.coordinator.SetEventCallback(func(event domain.ReactEvent) {
        select {
        case b.eventChan <- event:
        case <-ctx.Done():
            return
        }
    })

    // Run research
    result, err := b.coordinator.ExecuteTask(ctx, query)

    // Send completion event
    b.eventChan <- domain.CompletionEvent{
        Result: result,
        Error:  err,
    }
}

// CLI integration
func runDeepSearchTUI(query string) error {
    // Initialize coordinator
    coordinator, err := app.NewCoordinator(/* config */)
    if err != nil {
        return err
    }

    // Create bridge
    bridge := NewDeepSearchBridge(coordinator)

    // Run
    ctx := context.Background()
    return bridge.Run(ctx, query)
}
```

---

## 7. Testing Strategy

### 7.1 Unit Testing Components

**Use `teatest` package**:

```go
import (
    "testing"
    "github.com/charmbracelet/bubbletea"
    teatest "github.com/charmbracelet/bubbletea/testing"
)

func TestStatusBarUpdate(t *testing.T) {
    m := NewStatusBarModel(80)

    // Send phase change message
    newModel, _ := m.Update(PhaseChangedMsg{
        Phase:       PhaseExploration,
        Description: "Exploring codebase",
    })

    updated := newModel.(StatusBarModel)

    if updated.phase != PhaseExploration {
        t.Errorf("Expected phase %v, got %v", PhaseExploration, updated.phase)
    }
}

func TestTreeNodeAddition(t *testing.T) {
    tree := NewTreeModel(40, 20)

    // Add phase node
    phaseID := tree.AddPhaseNode(PhaseDecomposition, "Breaking down query")

    // Add subtask
    subtaskID := tree.AddSubtaskNode(phaseID, "Analyze keywords")

    // Verify structure
    if tree.nodes[subtaskID].Parent != phaseID {
        t.Error("Subtask parent not set correctly")
    }

    parent := tree.nodes[phaseID]
    if len(parent.Children) != 1 || parent.Children[0] != subtaskID {
        t.Error("Parent children not updated correctly")
    }
}
```

### 7.2 Integration Testing

**Test full TUI workflow**:

```go
func TestDeepSearchWorkflow(t *testing.T) {
    // Create test model with mock event channel
    eventChan := make(chan domain.ReactEvent, 10)
    m := NewDeepSearchModel(eventChan)

    // Simulate window size
    m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

    // Simulate research events
    eventChan <- domain.PhaseChangedEvent{
        Phase:       PhaseDecomposition,
        Description: "Analyzing query",
    }

    m, _ = m.Update(<-eventChan)

    // Verify state
    root := m.(RootModel)
    if root.researchState.currentPhase != PhaseDecomposition {
        t.Error("Phase not updated correctly")
    }

    // Verify view renders without error
    view := root.View()
    if view == "" {
        t.Error("View returned empty string")
    }
}
```

### 7.3 Manual Testing Script

**File: `scripts/test_tui.sh`**

```bash
#!/bin/bash

# Test different terminal sizes
echo "Testing 80x24 (small)"
resize -s 24 80
./alex deep-search "test query" --tui

echo "Testing 120x40 (medium)"
resize -s 40 120
./alex deep-search "test query" --tui

echo "Testing 200x60 (large)"
resize -s 60 200
./alex deep-search "test query" --tui

# Test different color modes
echo "Testing 256 color mode"
TERM=xterm-256color ./alex deep-search "test query" --tui

echo "Testing true color mode"
TERM=xterm-truecolor ./alex deep-search "test query" --tui
```

---

## 8. Implementation Roadmap

### Phase 1: MVP (2-3 weeks)

**Week 1: Core Components**
- [ ] Set up Bubble Tea project structure
- [ ] Implement basic RootModel with message routing
- [ ] Create StatusBarModel with spinner + progress
- [ ] Create TreeModel using tree-bubble library
- [ ] Create OutputModel with viewport
- [ ] Write unit tests for each component

**Week 2: Integration**
- [ ] Integrate tree-bubble component
- [ ] Implement split-pane layout
- [ ] Add keyboard navigation (Tab, arrow keys)
- [ ] Connect to ReAct engine via event channel
- [ ] Test with mock research events

**Week 3: Polish & Testing**
- [ ] Add help overlay
- [ ] Implement auto-scroll with manual override
- [ ] Add visual feedback for focus state
- [ ] Write integration tests
- [ ] Manual testing on different terminals
- [ ] Bug fixes

**Deliverables**:
- Working TUI showing real-time research progress
- Tree view of research phases/subtasks
- Streaming output viewport
- Basic keyboard navigation
- Help overlay

### Phase 2: Enhanced Features (1-2 weeks)

**Week 4: Visual Enhancements**
- [ ] Add syntax highlighting for code snippets (chroma library)
- [ ] Improve tree node styling (colors, icons)
- [ ] Add progress animations
- [ ] Implement status bar theming

**Week 5: Advanced Interactions**
- [ ] Add search in output (Ctrl+F)
- [ ] Implement copy support (Ctrl+C on selection)
- [ ] Add export to file (Ctrl+S)
- [ ] Adjustable split ratio (+/- keys)

**Deliverables**:
- Syntax highlighted output
- Search functionality
- Copy/export features
- Improved aesthetics

### Phase 3: Polish & Optimization (1 week)

**Week 6: Performance & UX**
- [ ] Implement rendering throttling
- [ ] Add memory management (content truncation)
- [ ] Optimize tree node rendering
- [ ] Add loading states for slow operations
- [ ] Comprehensive error handling
- [ ] User preferences (colors, layout)

**Deliverables**:
- Production-ready performance
- Graceful error handling
- User customization options

---

## 9. Open Questions & Design Decisions

### 9.1 Layout Decisions

**Q1: Fixed vs. Adjustable Split Ratio?**
- **Option A**: Fixed 30/70 split (simpler)
- **Option B**: User-adjustable with +/- keys (more flexible)
- **Recommendation**: Start with fixed, add adjustable in Phase 2

**Q2: Horizontal vs. Vertical Split?**
- **Option A**: Vertical (tree | output) - better for wide terminals
- **Option B**: Horizontal (tree / output) - better for tall terminals
- **Recommendation**: Vertical split, add horizontal as alternative view mode

**Q3: Status Bar Position?**
- **Option A**: Top (more visible)
- **Option B**: Bottom (traditional terminal style)
- **Recommendation**: Top for visibility, especially during long research

### 9.2 Interaction Patterns

**Q4: Tree Auto-Expand?**
- **Option A**: Auto-expand all nodes (shows everything)
- **Option B**: Collapse completed phases (cleaner)
- **Recommendation**: Auto-expand current phase, collapse old phases

**Q5: Output Auto-Scroll Behavior?**
- **Option A**: Always auto-scroll (can be disorienting)
- **Option B**: Auto-scroll unless manually scrolled up
- **Recommendation**: Option B with visual indicator

**Q6: Keyboard Navigation Style?**
- **Option A**: Vim-style (hjkl)
- **Option B**: Arrow keys only
- **Option C**: Both
- **Recommendation**: Both for accessibility

### 9.3 Visual Design

**Q7: Color Scheme?**
- **Option A**: Charm's default theme (blues/purples)
- **Option B**: Custom ALEX theme
- **Option C**: User-configurable
- **Recommendation**: Start with Charm default, add customization later

**Q8: Status Icons?**
- **Option A**: Unicode symbols (○ ⠋ ✓ ✗)
- **Option B**: ASCII fallback (- > + X)
- **Option C**: Detect terminal capability
- **Recommendation**: Option C (use unicode if supported)

### 9.4 Technical Decisions

**Q9: Event Channel Buffer Size?**
- **Option A**: Small buffer (10) - backpressure if TUI slow
- **Option B**: Large buffer (1000) - memory overhead
- **Recommendation**: 100 as middle ground, monitor in production

**Q10: Content Truncation Threshold?**
- **Option A**: 1,000 lines (conservative)
- **Option B**: 10,000 lines (generous)
- **Option C**: No limit (memory risk)
- **Recommendation**: 10,000 lines, add warning at 8,000

---

## 10. References & Resources

### Official Documentation

- **Bubble Tea**: https://github.com/charmbracelet/bubbletea
- **Bubbles Components**: https://github.com/charmbracelet/bubbles
- **Lip Gloss Styling**: https://github.com/charmbracelet/lipgloss
- **Bubble Tea Tutorials**: https://github.com/charmbracelet/bubbletea/tree/main/tutorials

### Third-Party Libraries

- **tree-bubble**: https://github.com/savannahostrowski/tree-bubble
- **chroma (syntax highlighting)**: https://github.com/alecthomas/chroma
- **go-term-markdown**: https://github.com/MichaelMure/go-term-markdown (already in ALEX)

### Best Practices & Patterns

- **Building Bubble Tea Programs**: https://leg100.github.io/en/posts/building-bubbletea-programs/
- **State Machine Pattern**: https://zackproser.com/blog/bubbletea-state-machine
- **Bubble Tea Commands**: https://charm.land/blog/commands-in-bubbletea/

### Example Projects

- **Glow** (Markdown renderer): https://github.com/charmbracelet/glow
- **Soft Serve** (Git server): https://github.com/charmbracelet/soft-serve
- **VHS** (Terminal recorder): https://github.com/charmbracelet/vhs
- **Lazygit** (Git TUI): https://github.com/jesseduffield/lazygit
- **K9s** (Kubernetes TUI): https://github.com/derailed/k9s

### AI Coding Agents (Inspiration)

- **Aider**: https://aider.chat/
- **Cursor CLI**: https://cursor.com/cli
- **Crush (Charmbracelet)**: https://typevar.dev/articles/charmbracelet/crush
- **OpenCode**: https://github.com/opencode-ai/opencode

### Articles & Tutorials

- "Rapidly building interactive CLIs with Bubbletea": https://www.inngest.com/blog/interactive-clis-with-bubbletea
- "Adventures in Go: Writing a TUI": https://sacules.github.io/post/adventures-go-tui-1/
- "Intro to Bubble Tea in Go": https://dev.to/andyhaskell/intro-to-bubble-tea-in-go-21lg

---

## 11. Conclusion

Bubble Tea provides an excellent foundation for building a production-quality deep search TUI for ALEX:

**Key Strengths**:
1. ✅ Elm Architecture ensures predictable state management
2. ✅ Rich ecosystem (bubbles, lipgloss, community libraries)
3. ✅ Active development and strong community
4. ✅ Battle-tested in production (Glow, Soft Serve, etc.)
5. ✅ Clean integration with Go (no CGo, pure Go)
6. ✅ Excellent testing support (teatest)

**Recommended Architecture**:
- **Root model**: Message router + screen compositor
- **Child models**: StatusBar, Tree, Output, Help
- **State machine**: Research phase management
- **Event bridge**: Channel-based communication with ReAct engine

**Timeline**:
- **MVP**: 2-3 weeks (core functionality)
- **Enhanced**: 1-2 weeks (syntax highlighting, search)
- **Polish**: 1 week (performance, UX refinements)
- **Total**: 4-6 weeks to production-ready

**Next Steps**:
1. ✅ Review this research document with team
2. ⬜ Build minimal prototype (1-2 days)
3. ⬜ Test with real users
4. ⬜ Iterate based on feedback
5. ⬜ Follow MVP roadmap

The proposed TUI design will provide ALEX users with:
- Real-time visibility into deep search progress
- Intuitive navigation of complex research trees
- Streaming output with full history
- Professional, terminal-native experience

This research provides a comprehensive foundation for implementation. The architecture, components, and patterns are well-defined and ready for development.
