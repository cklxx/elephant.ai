# Chat TUI Design Document

## Executive Summary

This document outlines the design and implementation plan for enhancing ALEX's Terminal User Interface (TUI) into a modern, feature-rich chat interface. Based on comprehensive research of Go TUI libraries and real-world implementations, we recommend using **Bubbletea** with an incremental migration strategy.

## Research Findings

### Technology Selection: Bubbletea

**Selected Framework:** `github.com/charmbracelet/bubbletea`

**Key Advantages:**
- Best-in-class streaming support for LLM responses
- Event-driven architecture (Elm pattern) - natural fit for async operations
- Rich ecosystem: Glamour (markdown), Lipgloss (styling), Bubbles (components)
- Proven in production AI agents (OpenCode AI)
- Active development and strong community
- Flexible: supports inline and full-screen modes

**Supporting Libraries:**
- `github.com/charmbracelet/glamour` - Markdown rendering with syntax highlighting
- `github.com/charmbracelet/lipgloss` - Advanced styling and layout
- `github.com/charmbracelet/bubbles/viewport` - Scrollable message area
- `github.com/charmbracelet/bubbles/textarea` - Multiline input
- `github.com/mattn/go-sqlite3` - Session persistence (Phase 3)

### Alternative Libraries Considered

| Library | Pros | Cons | Verdict |
|---------|------|------|---------|
| **tview** | Rich widgets, rapid development | Strictly full-screen, less flexible | ❌ Not ideal for chat |
| **termui** | Great for dashboards | Not designed for chat, less active | ❌ Wrong use case |
| **tcell** | Full control, low-level | Requires building everything | ❌ Too much work |

## Architecture Design

### Component Hierarchy

```
TUIModel (Root)
├── HeaderComponent
│   ├── Session Info (ID, model, cost)
│   └── Status Indicators (iteration, thinking)
├── MessagesComponent (Viewport)
│   ├── UserMessageRenderer
│   ├── AssistantMessageRenderer
│   │   └── Markdown rendering (Glamour)
│   └── ToolCallRenderer
│       ├── Tool Icon & Name
│       ├── Arguments Preview
│       └── Result Preview
├── InputComponent (Textarea)
│   ├── Multiline Support
│   ├── External Editor (Ctrl+E)
│   └── Send on Enter
└── SidebarComponent (Phase 3)
    ├── Session List
    └── Tool History
```

### Data Flow

```
┌─────────────┐
│ Coordinator │ (Domain Layer)
└──────┬──────┘
       │ Events (IterationStart, Thinking, ToolCall, etc.)
       ▼
┌──────────────┐
│ Event Bridge │
└──────┬───────┘
       │ tea.Msg (converted)
       ▼
┌─────────────┐
│  TUI Model  │ (Update function)
└──────┬──────┘
       │ State changes
       ▼
┌─────────────┐
│  Components │ (View rendering)
└─────────────┘
```

### State Management (Elm Architecture)

```go
// Model - All application state
type TUIModel struct {
    // Components
    viewport viewport.Model
    textarea textarea.Model

    // State
    messages     []Message
    session      Session
    width, height int

    // Rendering cache
    messageCache map[string]cachedMessage
    renderer     *glamour.TermRenderer
}

// Update - All state transitions
func (m TUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case ToolCallStartMsg:
        m.messages = append(m.messages, newToolMessage(msg))
        return m, nil
    }
}

// View - Pure rendering
func (m TUIModel) View() string {
    return lipgloss.JoinVertical(
        lipgloss.Top,
        m.renderHeader(),
        m.viewport.View(),
        m.textarea.View(),
    )
}
```

## Implementation Plan

### Phase 1: Enhanced Current TUI (Week 1) ✅ Quick Wins

**Goals:**
- Add markdown rendering for AI responses
- Improve tool display with icons and formatting
- Better layout with Lipgloss
- Keyboard shortcuts help

**Changes:**

1. **Add Dependencies**
```bash
go get github.com/charmbracelet/glamour
go get github.com/charmbracelet/lipgloss
```

2. **Markdown Rendering**
```go
// In tui_streaming.go
type StreamingTUIModel struct {
    // ... existing fields
    renderer *glamour.TermRenderer
}

func (m *StreamingTUIModel) renderResponse(content string) string {
    if m.renderer == nil {
        m.renderer, _ = glamour.NewTermRenderer(
            glamour.WithAutoStyle(),
            glamour.WithWordWrap(m.width - 4),
        )
    }

    rendered, _ := m.renderer.Render(content)
    return lipgloss.NewStyle().
        BorderLeft(true).
        BorderForeground(lipgloss.Color("12")).
        PaddingLeft(1).
        Render(rendered)
}
```

3. **Enhanced Tool Display**
```go
func renderToolCall(name string, result string, duration time.Duration) string {
    icon := getToolIcon(name)
    preview := createPreview(name, result)

    style := lipgloss.NewStyle().
        BorderLeft(true).
        BorderStyle(lipgloss.ThickBorder()).
        BorderForeground(lipgloss.Color("2")).
        PaddingLeft(1)

    return style.Render(fmt.Sprintf(
        "%s %s: %s (%s)",
        icon, name, preview, duration,
    ))
}

func getToolIcon(name string) string {
    icons := map[string]string{
        "file_read":   "📄",
        "file_write":  "✍️",
        "grep":        "🔍",
        "bash":        "💻",
        "web_search":  "🌐",
        "think":       "💭",
    }
    return icons[name]
}
```

4. **Help Footer**
```go
func (m StreamingTUIModel) helpView() string {
    return lipgloss.JoinHorizontal(lipgloss.Left,
        muted.Render("press "),
        primary.Bold(true).Render("enter"),
        muted.Render(" to send • "),
        primary.Bold(true).Render("ctrl+c"),
        muted.Render(" to quit • "),
        primary.Bold(true).Render("↑↓"),
        muted.Render(" to scroll"),
    )
}
```

**Deliverables:**
- Enhanced `cmd/alex/tui_streaming.go`
- Better visual hierarchy
- Code syntax highlighting in responses
- Tool execution visual feedback

**Effort:** 1-2 days

---

### Phase 2: Component-Based Architecture (Week 2-3) 🏗️ Foundation

**Goals:**
- Modular, maintainable component structure
- Message caching for performance
- Better separation of concerns
- Proper viewport + textarea integration

**New File Structure:**
```
cmd/alex/
├── tui_chat.go          # Main chat TUI (new)
├── tui_components/      # Component package (new)
│   ├── messages.go      # Messages viewport component
│   ├── editor.go        # Input editor component
│   ├── header.go        # Header/status component
│   └── types.go         # Shared types
└── tui_streaming.go     # Legacy TUI (kept for compatibility)
```

**1. Core Types**
```go
// cmd/alex/tui_components/types.go
package tui_components

import (
    "time"
    "github.com/charmbracelet/glamour"
)

type Message struct {
    ID        string
    Role      string // "user", "assistant", "tool"
    Content   string
    Timestamp time.Time

    // For tool messages
    ToolCall  *ToolCallInfo
}

type ToolCallInfo struct {
    ID        string
    Name      string
    Arguments map[string]interface{}
    Result    string
    Error     error
    Status    ToolStatus // Pending, Running, Success, Error
    Duration  time.Duration
}

type ToolStatus int

const (
    ToolPending ToolStatus = iota
    ToolRunning
    ToolSuccess
    ToolError
)

type cachedMessage struct {
    width   int
    content string
}
```

**2. Messages Component**
```go
// cmd/alex/tui_components/messages.go
package tui_components

import (
    "github.com/charmbracelet/bubbles/viewport"
    "github.com/charmbracelet/glamour"
    "github.com/charmbracelet/lipgloss"
)

type MessagesComponent struct {
    viewport  viewport.Model
    messages  []Message
    cache     map[string]cachedMessage
    renderer  *glamour.TermRenderer
    width     int
}

func NewMessagesComponent(width, height int) MessagesComponent {
    vp := viewport.New(width, height)
    vp.HighPerformanceRendering = true

    renderer, _ := glamour.NewTermRenderer(
        glamour.WithAutoStyle(),
        glamour.WithWordWrap(width - 4),
    )

    return MessagesComponent{
        viewport: vp,
        messages: []Message{},
        cache:    make(map[string]cachedMessage),
        renderer: renderer,
        width:    width,
    }
}

func (mc *MessagesComponent) AddMessage(msg Message) {
    mc.messages = append(mc.messages, msg)
    mc.updateViewport()
}

func (mc *MessagesComponent) UpdateMessage(id string, updateFn func(*Message)) {
    for i := range mc.messages {
        if mc.messages[i].ID == id {
            updateFn(&mc.messages[i])
            // Invalidate cache for this message
            delete(mc.cache, id)
            mc.updateViewport()
            break
        }
    }
}

func (mc *MessagesComponent) updateViewport() {
    var rendered []string

    for _, msg := range mc.messages {
        // Check cache
        if cached, ok := mc.cache[msg.ID]; ok && cached.width == mc.width {
            rendered = append(rendered, cached.content)
            continue
        }

        // Render message
        content := mc.renderMessage(msg)

        // Cache it
        mc.cache[msg.ID] = cachedMessage{
            width:   mc.width,
            content: content,
        }

        rendered = append(rendered, content)
    }

    // Update viewport
    content := lipgloss.NewStyle().
        Width(mc.width).
        Render(strings.Join(rendered, "\n\n"))

    mc.viewport.SetContent(content)
    mc.viewport.GotoBottom()
}

func (mc *MessagesComponent) renderMessage(msg Message) string {
    switch msg.Role {
    case "user":
        return mc.renderUserMessage(msg)
    case "assistant":
        return mc.renderAssistantMessage(msg)
    case "tool":
        return mc.renderToolMessage(msg)
    }
    return ""
}

func (mc *MessagesComponent) renderUserMessage(msg Message) string {
    style := lipgloss.NewStyle().
        BorderLeft(true).
        BorderForeground(lipgloss.Color("6")). // Cyan
        PaddingLeft(1).
        Foreground(lipgloss.Color("7"))

    return style.Render("You: " + msg.Content)
}

func (mc *MessagesComponent) renderAssistantMessage(msg Message) string {
    rendered, _ := mc.renderer.Render(msg.Content)

    style := lipgloss.NewStyle().
        BorderLeft(true).
        BorderForeground(lipgloss.Color("12")). // Bright Blue
        PaddingLeft(1)

    return style.Render(rendered)
}

func (mc *MessagesComponent) renderToolMessage(msg Message) string {
    if msg.ToolCall == nil {
        return ""
    }

    tc := msg.ToolCall
    icon := getToolIcon(tc.Name)

    var borderColor lipgloss.Color
    var statusText string

    switch tc.Status {
    case ToolRunning:
        borderColor = lipgloss.Color("11") // Yellow
        statusText = fmt.Sprintf("%s %s: %s...",
            icon, tc.Name, getToolAction(tc.Name))
    case ToolSuccess:
        borderColor = lipgloss.Color("2") // Green
        preview := createPreview(tc.Name, tc.Result)
        statusText = fmt.Sprintf("%s %s: %s (%s)",
            icon, tc.Name, preview, tc.Duration)
    case ToolError:
        borderColor = lipgloss.Color("9") // Red
        statusText = fmt.Sprintf("%s %s: %v",
            icon, tc.Name, tc.Error)
    }

    style := lipgloss.NewStyle().
        BorderLeft(true).
        BorderStyle(lipgloss.ThickBorder()).
        BorderForeground(borderColor).
        PaddingLeft(1)

    return style.Render(statusText)
}

func (mc *MessagesComponent) Update(msg tea.Msg) (MessagesComponent, tea.Cmd) {
    var cmd tea.Cmd
    mc.viewport, cmd = mc.viewport.Update(msg)
    return *mc, cmd
}

func (mc MessagesComponent) View() string {
    return mc.viewport.View()
}

func (mc *MessagesComponent) SetSize(width, height int) {
    if mc.width != width {
        mc.width = width
        mc.cache = make(map[string]cachedMessage) // Invalidate cache

        // Update renderer width
        mc.renderer, _ = glamour.NewTermRenderer(
            glamour.WithAutoStyle(),
            glamour.WithWordWrap(width - 4),
        )
    }

    mc.viewport.Width = width
    mc.viewport.Height = height
    mc.updateViewport()
}
```

**3. Editor Component**
```go
// cmd/alex/tui_components/editor.go
package tui_components

import (
    "github.com/charmbracelet/bubbles/textarea"
    tea "github.com/charmbracelet/bubbletea"
)

type EditorComponent struct {
    textarea textarea.Model
    focused  bool
}

func NewEditorComponent(width, height int) EditorComponent {
    ta := textarea.New()
    ta.Placeholder = "Type your message... (Enter to send, Shift+Enter for newline)"
    ta.Focus()
    ta.CharLimit = -1
    ta.ShowLineNumbers = false
    ta.SetWidth(width)
    ta.SetHeight(height)

    return EditorComponent{
        textarea: ta,
        focused:  true,
    }
}

func (ec *EditorComponent) Update(msg tea.Msg) (EditorComponent, tea.Cmd) {
    var cmd tea.Cmd
    ec.textarea, cmd = ec.textarea.Update(msg)
    return *ec, cmd
}

func (ec EditorComponent) View() string {
    return ec.textarea.View()
}

func (ec *EditorComponent) SetSize(width, height int) {
    ec.textarea.SetWidth(width)
    ec.textarea.SetHeight(height)
}

func (ec *EditorComponent) Value() string {
    return ec.textarea.Value()
}

func (ec *EditorComponent) Reset() {
    ec.textarea.Reset()
}

func (ec *EditorComponent) Focus() tea.Cmd {
    ec.focused = true
    return ec.textarea.Focus()
}

func (ec *EditorComponent) Blur() {
    ec.focused = false
    ec.textarea.Blur()
}
```

**4. Main Chat TUI**
```go
// cmd/alex/tui_chat.go
package main

import (
    "fmt"
    "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "alex/cmd/alex/tui_components"
    "alex/internal/agent/app"
)

type ChatTUIModel struct {
    // Components
    messages tui_components.MessagesComponent
    editor   tui_components.EditorComponent

    // State
    session      app.Session
    width        int
    height       int

    // Integration
    coordinator  *app.Coordinator
    sessionID    string
    initialTask  string
    ready        bool
}

func NewChatTUI(coordinator *app.Coordinator, sessionID, task string) ChatTUIModel {
    return ChatTUIModel{
        coordinator: coordinator,
        sessionID:   sessionID,
        initialTask: task,
        ready:       false,
    }
}

func (m ChatTUIModel) Init() tea.Cmd {
    return tea.Batch(
        tea.EnterAltScreen,
        func() tea.Msg {
            return ReadyMsg{}
        },
    )
}

type ReadyMsg struct{}

func (m ChatTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.Type {
        case tea.KeyCtrlC:
            return m, tea.Quit
        case tea.KeyEnter:
            if !msg.Shift { // Shift+Enter adds newline, Enter sends
                return m.sendMessage()
            }
        }

    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height

        headerHeight := 3
        editorHeight := 5
        gapHeight := 2
        messagesHeight := msg.Height - headerHeight - editorHeight - gapHeight

        m.messages.SetSize(msg.Width, messagesHeight)
        m.editor.SetSize(msg.Width, editorHeight)

        if !m.ready {
            m.ready = true
            // Start initial task
            return m, m.executeTask(m.initialTask)
        }

    case ReadyMsg:
        // Ready to start

    // Agent events
    case app.IterationStartMsg:
        m.addSystemMessage(fmt.Sprintf("Iteration %d/%d", msg.Iteration, msg.TotalIters))

    case app.ThinkingMsg:
        m.addSystemMessage("💭 Thinking...")

    case app.ActionMsg:
        m.addAssistantMessage(msg.Action)

    case app.ToolCallStartMsg:
        m.addToolCall(msg.CallID, msg.ToolName, msg.Arguments)

    case app.ToolCallEndMsg:
        m.updateToolCall(msg.CallID, msg.Result, msg.Error)

    case app.ResponseMsg:
        m.addAssistantMessage(msg.Content)

    case app.CompleteMsg:
        m.addSystemMessage("✓ Task complete")

    case app.ErrorMsg:
        m.addErrorMessage(msg.Error.Error())
    }

    // Update components
    var cmds []tea.Cmd
    var cmd tea.Cmd

    m.messages, cmd = m.messages.Update(msg)
    cmds = append(cmds, cmd)

    m.editor, cmd = m.editor.Update(msg)
    cmds = append(cmds, cmd)

    return m, tea.Batch(cmds...)
}

func (m ChatTUIModel) View() string {
    if !m.ready {
        return "Initializing..."
    }

    header := m.renderHeader()
    messages := m.messages.View()
    editor := m.editor.View()
    help := m.renderHelp()

    return lipgloss.JoinVertical(
        lipgloss.Top,
        header,
        messages,
        "\n",
        editor,
        help,
    )
}

func (m ChatTUIModel) renderHeader() string {
    style := lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("12")).
        Background(lipgloss.Color("0")).
        Padding(0, 1)

    return style.Render(fmt.Sprintf(
        "ALEX Chat - Session: %s | Model: %s",
        m.sessionID[:8],
        "gpt-4", // TODO: get from config
    ))
}

func (m ChatTUIModel) renderHelp() string {
    muted := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
    key := lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)

    return lipgloss.JoinHorizontal(lipgloss.Left,
        muted.Render("press "),
        key.Render("enter"),
        muted.Render(" to send • "),
        key.Render("shift+enter"),
        muted.Render(" for newline • "),
        key.Render("ctrl+c"),
        muted.Render(" to quit"),
    )
}

func (m *ChatTUIModel) sendMessage() (tea.Model, tea.Cmd) {
    value := m.editor.Value()
    if value == "" {
        return m, nil
    }

    // Add user message
    m.addUserMessage(value)

    // Clear input
    m.editor.Reset()

    // Execute task
    return m, m.executeTask(value)
}

func (m *ChatTUIModel) executeTask(task string) tea.Cmd {
    return func() tea.Msg {
        // This will send events back through the tea.Program
        result, err := m.coordinator.ExecuteTask(
            context.Background(),
            task,
            m.sessionID,
        )

        if err != nil {
            return app.ErrorMsg{Error: err}
        }

        return app.CompleteMsg{Result: result}
    }
}

// Helper methods
func (m *ChatTUIModel) addUserMessage(content string) {
    msg := tui_components.Message{
        ID:        generateID(),
        Role:      "user",
        Content:   content,
        Timestamp: time.Now(),
    }
    m.messages.AddMessage(msg)
}

func (m *ChatTUIModel) addAssistantMessage(content string) {
    msg := tui_components.Message{
        ID:        generateID(),
        Role:      "assistant",
        Content:   content,
        Timestamp: time.Now(),
    }
    m.messages.AddMessage(msg)
}

func (m *ChatTUIModel) addSystemMessage(content string) {
    // Render as muted assistant message
    style := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
    m.addAssistantMessage(style.Render(content))
}

func (m *ChatTUIModel) addToolCall(id, name string, args map[string]interface{}) {
    msg := tui_components.Message{
        ID:        id,
        Role:      "tool",
        Timestamp: time.Now(),
        ToolCall: &tui_components.ToolCallInfo{
            ID:        id,
            Name:      name,
            Arguments: args,
            Status:    tui_components.ToolRunning,
        },
    }
    m.messages.AddMessage(msg)
}

func (m *ChatTUIModel) updateToolCall(id, result string, err error) {
    m.messages.UpdateMessage(id, func(msg *tui_components.Message) {
        if msg.ToolCall != nil {
            msg.ToolCall.Result = result
            msg.ToolCall.Error = err
            if err != nil {
                msg.ToolCall.Status = tui_components.ToolError
            } else {
                msg.ToolCall.Status = tui_components.ToolSuccess
            }
        }
    })
}
```

**5. Event Bridge**
```go
// internal/agent/app/tui_bridge.go
package app

import (
    tea "github.com/charmbracelet/bubbletea"
)

// Event messages for TUI
type (
    IterationStartMsg struct {
        Iteration  int
        TotalIters int
    }

    ThinkingMsg struct{}

    ActionMsg struct {
        Action string
    }

    ToolCallStartMsg struct {
        CallID    string
        ToolName  string
        Arguments map[string]interface{}
    }

    ToolCallEndMsg struct {
        CallID string
        Result string
        Error  error
    }

    ResponseMsg struct {
        Content string
    }

    CompleteMsg struct {
        Result string
    }

    ErrorMsg struct {
        Error error
    }
)

type TUIEventBridge struct {
    program *tea.Program
}

func NewTUIEventBridge(program *tea.Program) *TUIEventBridge {
    return &TUIEventBridge{program: program}
}

func (b *TUIEventBridge) SendIterationStart(iter, total int) {
    b.program.Send(IterationStartMsg{
        Iteration:  iter,
        TotalIters: total,
    })
}

func (b *TUIEventBridge) SendThinking() {
    b.program.Send(ThinkingMsg{})
}

func (b *TUIEventBridge) SendAction(action string) {
    b.program.Send(ActionMsg{Action: action})
}

func (b *TUIEventBridge) SendToolCallStart(id, name string, args map[string]interface{}) {
    b.program.Send(ToolCallStartMsg{
        CallID:    id,
        ToolName:  name,
        Arguments: args,
    })
}

func (b *TUIEventBridge) SendToolCallEnd(id, result string, err error) {
    b.program.Send(ToolCallEndMsg{
        CallID: id,
        Result: result,
        Error:  err,
    })
}

func (b *TUIEventBridge) SendResponse(content string) {
    b.program.Send(ResponseMsg{Content: content})
}
```

**6. Integration in CLI**
```go
// cmd/alex/cli.go - Update RunChatMode
func RunChatMode(ctx context.Context, coordinator *app.Coordinator) error {
    sessionID := generateSessionID()

    // Get initial task from stdin
    fmt.Print("Enter your task: ")
    reader := bufio.NewReader(os.Stdin)
    task, _ := reader.ReadString('\n')
    task = strings.TrimSpace(task)

    // Create TUI model
    model := NewChatTUI(coordinator, sessionID, task)

    // Create program
    p := tea.NewProgram(
        model,
        tea.WithAltScreen(),
        tea.WithMouseCellMotion(),
    )

    // Set up event bridge
    bridge := app.NewTUIEventBridge(p)
    coordinator.SetEventBridge(bridge)

    // Run
    if _, err := p.Run(); err != nil {
        return fmt.Errorf("TUI error: %w", err)
    }

    return nil
}
```

**Deliverables:**
- New `cmd/alex/tui_chat.go` - Full Bubbletea implementation
- Component package `cmd/alex/tui_components/`
- Event bridge in coordinator
- CLI flag `--tui-mode=simple|modern`

**Effort:** 3-5 days

---

### Phase 3: Full-Featured Chat TUI (Week 4) 🚀 Advanced

**Goals:**
- Session persistence with SQLite
- Conversation history sidebar
- Search functionality
- External editor support
- Improved UX with animations

**New Features:**

**1. Session Persistence**
```go
// internal/storage/session_store.go
package storage

import (
    "database/sql"
    _ "github.com/mattn/go-sqlite3"
)

type SessionStore struct {
    db *sql.DB
}

func NewSessionStore(dbPath string) (*SessionStore, error) {
    db, err := sql.Open("sqlite3", dbPath)
    if err != nil {
        return nil, err
    }

    // Create schema
    _, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS sessions (
            id TEXT PRIMARY KEY,
            title TEXT,
            model TEXT,
            created_at INTEGER,
            updated_at INTEGER
        );

        CREATE TABLE IF NOT EXISTS messages (
            id TEXT PRIMARY KEY,
            session_id TEXT,
            role TEXT,
            content TEXT,
            timestamp INTEGER,
            FOREIGN KEY (session_id) REFERENCES sessions(id)
        );

        CREATE INDEX IF NOT EXISTS idx_messages_session
        ON messages(session_id, timestamp);
    `)

    return &SessionStore{db: db}, err
}

func (s *SessionStore) SaveMessage(sessionID string, msg Message) error {
    _, err := s.db.Exec(`
        INSERT INTO messages (id, session_id, role, content, timestamp)
        VALUES (?, ?, ?, ?, ?)
    `, msg.ID, sessionID, msg.Role, msg.Content, msg.Timestamp.Unix())
    return err
}

func (s *SessionStore) LoadSession(sessionID string) ([]Message, error) {
    rows, err := s.db.Query(`
        SELECT id, role, content, timestamp
        FROM messages
        WHERE session_id = ?
        ORDER BY timestamp ASC
    `, sessionID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var messages []Message
    for rows.Next() {
        var msg Message
        var timestamp int64
        rows.Scan(&msg.ID, &msg.Role, &msg.Content, &timestamp)
        msg.Timestamp = time.Unix(timestamp, 0)
        messages = append(messages, msg)
    }

    return messages, nil
}

func (s *SessionStore) ListSessions() ([]Session, error) {
    rows, err := s.db.Query(`
        SELECT id, title, model, created_at, updated_at
        FROM sessions
        ORDER BY updated_at DESC
        LIMIT 50
    `)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var sessions []Session
    for rows.Next() {
        var sess Session
        var created, updated int64
        rows.Scan(&sess.ID, &sess.Title, &sess.Model, &created, &updated)
        sess.CreatedAt = time.Unix(created, 0)
        sess.UpdatedAt = time.Unix(updated, 0)
        sessions = append(sessions, sess)
    }

    return sessions, nil
}
```

**2. Sidebar Component**
```go
// cmd/alex/tui_components/sidebar.go
package tui_components

type SidebarComponent struct {
    sessions      []Session
    selectedIdx   int
    visible       bool
    width         int
    height        int
}

func (sc SidebarComponent) View() string {
    if !sc.visible {
        return ""
    }

    style := lipgloss.NewStyle().
        BorderRight(true).
        BorderForeground(lipgloss.Color("8")).
        Width(sc.width).
        Height(sc.height)

    var items []string
    items = append(items,
        lipgloss.NewStyle().
            Bold(true).
            Foreground(lipgloss.Color("12")).
            Render("Recent Sessions"),
        "",
    )

    for i, sess := range sc.sessions {
        itemStyle := lipgloss.NewStyle()
        if i == sc.selectedIdx {
            itemStyle = itemStyle.
                Background(lipgloss.Color("8")).
                Foreground(lipgloss.Color("15"))
        }

        items = append(items, itemStyle.Render(fmt.Sprintf(
            "  %s\n  %s",
            sess.Title,
            sess.UpdatedAt.Format("Jan 2, 15:04"),
        )))
    }

    return style.Render(strings.Join(items, "\n"))
}
```

**3. Search Mode**
```go
// Add to ChatTUIModel
type ChatTUIModel struct {
    // ... existing fields
    searchMode    bool
    searchQuery   string
    searchResults []int
    currentResult int
}

func (m ChatTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // ... existing cases

    case tea.KeyMsg:
        if m.searchMode {
            switch msg.String() {
            case "esc":
                m.searchMode = false
                m.searchQuery = ""
                m.searchResults = nil
            case "enter":
                m.performSearch()
            case "n":
                m.nextSearchResult()
            case "N":
                m.prevSearchResult()
            default:
                m.searchQuery += msg.String()
            }
        } else {
            switch msg.String() {
            case "/":
                m.searchMode = true
                m.searchQuery = ""
            }
        }
}

func (m *ChatTUIModel) performSearch() {
    m.searchResults = []int{}
    query := strings.ToLower(m.searchQuery)

    for i, msg := range m.messages.messages {
        if strings.Contains(strings.ToLower(msg.Content), query) {
            m.searchResults = append(m.searchResults, i)
        }
    }

    if len(m.searchResults) > 0 {
        m.currentResult = 0
        m.scrollToMessage(m.searchResults[0])
    }
}
```

**4. External Editor**
```go
// Add to EditorComponent
func (ec *EditorComponent) OpenExternalEditor() tea.Cmd {
    editor := os.Getenv("EDITOR")
    if editor == "" {
        editor = "vim"
    }

    tmpfile, _ := os.CreateTemp("", "alex_message_*.md")
    tmpfile.WriteString(ec.textarea.Value())
    tmpfile.Close()

    cmd := exec.Command(editor, tmpfile.Name())
    return tea.ExecProcess(cmd, func(err error) tea.Msg {
        content, _ := os.ReadFile(tmpfile.Name())
        os.Remove(tmpfile.Name())
        return ExternalEditorMsg{Content: string(content)}
    })
}

type ExternalEditorMsg struct {
    Content string
}

// In main Update:
case tea.KeyMsg:
    if msg.String() == "ctrl+e" {
        return m, m.editor.OpenExternalEditor()
    }

case ExternalEditorMsg:
    m.editor.textarea.SetValue(msg.Content)
```

**Deliverables:**
- SQLite session persistence
- Sidebar with session history
- Search functionality (/)
- External editor (Ctrl+E)
- Session switching

**Effort:** 5-7 days

---

## Migration Strategy

### Incremental Rollout

**Week 1: Phase 1**
- Add Glamour to existing `tui_streaming.go`
- Users get immediate visual improvements
- No breaking changes

**Week 2-3: Phase 2**
- Build `tui_chat.go` alongside existing TUI
- Add CLI flag: `--tui-mode=simple|modern`
- Default: `simple` (existing)
- Users can opt-in to `modern` (new)

**Week 4: Phase 3**
- Add advanced features to `modern` mode
- Gather user feedback
- Iterate on UX

**Week 5: Transition**
- Make `modern` the default
- Keep `simple` as fallback
- Deprecation notice for `simple`

**Week 6: Cleanup**
- Remove `tui_streaming.go` if `modern` is stable
- Full migration complete

### CLI Flags

```bash
# Use legacy simple TUI
alex --tui-mode=simple

# Use new Bubbletea TUI (default after Week 5)
alex --tui-mode=modern
alex  # Same as above

# Disable TUI entirely (output only)
alex --no-tui
```

## Performance Optimizations

### Message Caching
- Cache rendered messages by ID + width
- Invalidate cache on width change
- Reduces re-rendering by ~90%

### Viewport Optimization
- High-performance rendering mode
- Only renders visible area
- Smooth scrolling for 1000+ messages

### Lazy Loading
- Load last 50 messages on start
- Load more on scroll-up
- Prevents memory bloat

### Throttled Updates
- Stream updates every 100ms (not every token)
- Batch viewport updates
- Smoother UX, less flicker

## Testing Strategy

### Unit Tests

```go
// cmd/alex/tui_components/messages_test.go
func TestMessagesComponent_AddMessage(t *testing.T) {
    mc := NewMessagesComponent(80, 24)

    msg := Message{
        ID:      "test-1",
        Role:    "user",
        Content: "Hello",
    }

    mc.AddMessage(msg)

    if len(mc.messages) != 1 {
        t.Errorf("Expected 1 message, got %d", len(mc.messages))
    }
}

func TestMessagesComponent_Cache(t *testing.T) {
    mc := NewMessagesComponent(80, 24)

    msg := Message{ID: "test-1", Role: "user", Content: "Test"}
    mc.AddMessage(msg)

    // Should be cached
    if _, ok := mc.cache["test-1"]; !ok {
        t.Error("Message not cached")
    }

    // Update message
    mc.UpdateMessage("test-1", func(m *Message) {
        m.Content = "Updated"
    })

    // Cache should be invalidated
    if _, ok := mc.cache["test-1"]; ok {
        t.Error("Cache not invalidated on update")
    }
}
```

### Integration Tests

```go
// cmd/alex/tui_chat_test.go
func TestChatTUI_SendMessage(t *testing.T) {
    coordinator := setupTestCoordinator()
    model := NewChatTUI(coordinator, "test-session", "test task")

    // Simulate window size
    model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

    // Simulate typing
    model.editor.textarea.SetValue("Hello AI")

    // Simulate Enter
    model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})

    // Should have user message
    if len(model.messages.messages) == 0 {
        t.Error("Message not added")
    }

    if model.messages.messages[0].Role != "user" {
        t.Error("Wrong message role")
    }
}
```

### Manual Testing Checklist

- [ ] Message rendering (user, assistant, tool)
- [ ] Markdown code blocks with syntax highlighting
- [ ] Scrolling (auto-scroll, manual scroll)
- [ ] Tool execution display (pending, success, error)
- [ ] Input handling (Enter, Shift+Enter)
- [ ] Window resize
- [ ] Long messages (wrapping)
- [ ] Streaming LLM responses
- [ ] Error handling
- [ ] Session persistence (Phase 3)
- [ ] Search functionality (Phase 3)
- [ ] External editor (Phase 3)

## Dependencies

### Phase 1
```go
require (
    github.com/charmbracelet/glamour v0.6.0
    github.com/charmbracelet/lipgloss v0.9.1
)
```

### Phase 2
```go
require (
    github.com/charmbracelet/bubbletea v0.25.0
    github.com/charmbracelet/bubbles v0.18.0
    github.com/charmbracelet/glamour v0.6.0
    github.com/charmbracelet/lipgloss v0.9.1
)
```

### Phase 3
```go
require (
    github.com/charmbracelet/bubbletea v0.25.0
    github.com/charmbracelet/bubbles v0.18.0
    github.com/charmbracelet/glamour v0.6.0
    github.com/charmbracelet/lipgloss v0.9.1
    github.com/mattn/go-sqlite3 v1.14.18
)
```

## Risk Mitigation

### Risk 1: Breaking Existing Users
**Mitigation:** Keep legacy TUI, use feature flags, gradual rollout

### Risk 2: Performance Issues
**Mitigation:** Caching, throttling, viewport optimization, benchmarking

### Risk 3: Complexity Creep
**Mitigation:** Strict adherence to Elm architecture, component isolation, code reviews

### Risk 4: Terminal Compatibility
**Mitigation:** Test on multiple terminals (iTerm2, Alacritty, Terminal.app, Windows Terminal), graceful degradation

## Success Metrics

### User Experience
- Improved readability with markdown rendering
- Faster navigation with keyboard shortcuts
- Better context with session history

### Developer Experience
- Modular, testable components
- Clear separation of concerns
- Easy to add new features

### Performance
- <100ms message rendering
- Smooth scrolling with 1000+ messages
- <50ms input latency

## Future Enhancements (Post Phase 3)

### Advanced Features
- **Multi-session tabs** - Switch between conversations with Tab
- **Tool result inspection** - Expand/collapse tool outputs
- **Message editing** - Edit and resend previous messages
- **Export conversations** - Markdown, JSON, HTML export
- **Themes** - Customizable color schemes
- **Notifications** - Desktop notifications for long-running tasks

### GUI Wrapper (Optional)
- **Wails integration** - Web UI with Go backend
- **Electron alternative** - Native look and feel
- **Keep TUI** - TUI remains primary interface

## References

### Documentation
- [Bubbletea Tutorial](https://github.com/charmbracelet/bubbletea/tree/master/tutorials)
- [Glamour Examples](https://github.com/charmbracelet/glamour/tree/master/styles)
- [Lipgloss Docs](https://github.com/charmbracelet/lipgloss)

### Example Projects
- [OpenCode AI](https://github.com/opencode-ai/opencode) - AI agent with Bubbletea
- [Glow](https://github.com/charmbracelet/glow) - Markdown viewer
- [Soft Serve](https://github.com/charmbracelet/soft-serve) - Git TUI

### Community
- [Charm Slack](https://charm.sh/slack)
- [Bubbletea Discussions](https://github.com/charmbracelet/bubbletea/discussions)

---

## Conclusion

This design provides a clear, incremental path to a modern chat TUI for ALEX. By using Bubbletea and following established patterns from successful projects, we can deliver:

1. **Immediate value** with Phase 1 (markdown rendering)
2. **Solid foundation** with Phase 2 (component architecture)
3. **Advanced features** with Phase 3 (persistence, search)

The migration strategy minimizes risk while maximizing user value. Each phase is independently valuable and can be shipped incrementally.

**Next Steps:**
1. Review and approve this design
2. Add dependencies for Phase 1
3. Begin implementation with enhanced `tui_streaming.go`
4. Gather feedback and iterate

---

*Document Version: 1.0*
*Last Updated: 2025-10-01*
*Author: Claude (with cklxx)*
