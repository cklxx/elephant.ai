# ALEX Formatting Guide

## Overview

ALEX supports rich text formatting in both command mode and interactive chat mode using **Markdown** with **syntax highlighting**.

---

## Supported Formatting

### 1. Code Blocks with Syntax Highlighting

**Markdown**:
````markdown
```python
def hello_world():
    print("Hello, World!")
```
````

**Rendered** (with colors):
```python
def hello_world():
    print("Hello, World!")
```

**Supported Languages**: 100+ languages via Chroma
- Python, Go, JavaScript, TypeScript, Rust, C, C++, Java
- Ruby, PHP, Swift, Kotlin, Scala
- Bash, Shell, SQL, HTML, CSS, JSON, YAML
- And many more...

---

### 2. Inline Code

**Markdown**:
```markdown
Use the `list_files` tool to list files.
```

**Rendered**:
Use the `list_files` tool to list files.

---

### 3. Headers

**Markdown**:
```markdown
# H1 Header
## H2 Header
### H3 Header
```

**Rendered** (with colors and styling):
# H1 Header
## H2 Header
### H3 Header

---

### 4. Lists

**Markdown**:
```markdown
- Item 1
- Item 2
  - Nested item
  - Another nested
- Item 3

1. First
2. Second
3. Third
```

**Rendered**:
- Item 1
- Item 2
  - Nested item
  - Another nested
- Item 3

1. First
2. Second
3. Third

---

### 5. Bold and Italic

**Markdown**:
```markdown
**bold text**
*italic text*
***bold and italic***
```

**Rendered**:
**bold text**
*italic text*
***bold and italic***

---

### 6. Tables

**Markdown**:
```markdown
| Column 1 | Column 2 | Column 3 |
|----------|----------|----------|
| Value 1  | Value 2  | Value 3  |
| Data A   | Data B   | Data C   |
```

**Rendered**:
| Column 1 | Column 2 | Column 3 |
|----------|----------|----------|
| Value 1  | Value 2  | Value 3  |
| Data A   | Data B   | Data C   |

---

### 7. Links

**Markdown**:
```markdown
[ALEX Repository](https://github.com/yourusername/alex)
```

**Rendered**:
[ALEX Repository](https://github.com/yourusername/alex)

---

### 8. Blockquotes

**Markdown**:
```markdown
> This is a quote
> It can span multiple lines
```

**Rendered**:
> This is a quote
> It can span multiple lines

---

## Mode-Specific Rendering

### Command Mode (`./alex "cmd"`)

**Features**:
- Inline terminal output
- Markdown rendering for final answer
- Syntax highlighting via Glamour
- Simple tool indicators (⏺ icon + preview)

**Example**:
```bash
./alex "show me a python hello world"
```

**Output**:
```
Executing: show me a python hello world

⏺ 💭think
  → Analyzing task

✓ Task completed in 1 iterations
Tokens used: 245

Answer:
┃ def hello_world():
┃     print("Hello, World!")
```

---

### Interactive Chat Mode (`./alex`)

**Features**:
- Full-screen TUI with Bubbletea
- Scrollable message history (viewport)
- Real-time markdown rendering
- Color-coded messages:
  - User: Cyan border
  - Assistant: Blue border + markdown
  - Tool: Yellow (running), Green (success), Red (error)
  - System: Gray italic

**Example**:
```
┌─────────────────────────────────────────┐
│ ALEX Chat | Model: gpt-4 | Ready        │
├─────────────────────────────────────────┤
│                                          │
│  You: show me python code                │
│                                          │
│  ┃ def hello_world():                   │
│  ┃     print("Hello, World!")           │
│                                          │
├─────────────────────────────────────────┤
│ Type your next message...                │
└─────────────────────────────────────────┘
```

---

## Rendering Engine

### Glamour (Markdown → Terminal)

**Configuration**:
```go
renderer, _ := glamour.NewTermRenderer(
    glamour.WithAutoStyle(),           // Auto dark/light theme
    glamour.WithWordWrap(width - 8),   // Wrap to terminal width
)

rendered, _ := renderer.Render(markdownContent)
```

**Features**:
- Auto theme detection (dark/light based on terminal background)
- Word wrapping to prevent overflow
- Syntax highlighting via Chroma
- Full CommonMark support

---

## Custom Styling

### Tool Output Formatting

**Command Mode**:
```go
// Tool icons
📄 file_read
✍️ file_write
🔍 grep
💻 bash
🌐 web_search

// Status colors
⏺ Running  (Yellow)
✓ Success  (Green)
✗ Error    (Red)
```

**Chat Mode**:
```go
// Message borders
│ User message     (Cyan border)
│ Assistant msg    (Blue border)
│ Tool execution   (Colored border by status)
```

---

## Testing Formatting

### Quick Tests

**Test 1: Code Block**
```bash
./alex "show me a Go hello world function"
```

**Test 2: List**
```bash
./alex "list 5 programming languages with bullet points"
```

**Test 3: Table**
```bash
./alex "create a 3x3 markdown table"
```

**Test 4: Mixed Formatting**
```bash
./alex "explain linked lists with code examples"
```

### Interactive Chat Tests

1. Start chat: `./alex`
2. Test inputs:
   - "show me python code with comments"
   - "create a markdown table comparing Go and Rust"
   - "explain with bullet points and code examples"
   - "format a JSON object"

---

## Limitations

### Known Issues

1. **Terminal Width**:
   - Markdown wraps at terminal width - 8
   - Very long code lines may wrap awkwardly
   - Recommended: 80+ column terminal

2. **Color Support**:
   - Requires 256-color terminal
   - Works best with: iTerm2, Alacritty, Terminal.app, Windows Terminal
   - May have reduced colors in basic terminals

3. **Emoji Support**:
   - Tool icons (📄🔍💻) require Unicode font
   - Most modern terminals support this

4. **Table Rendering**:
   - Tables wrap at terminal width
   - Very wide tables may not render well
   - Keep tables narrow (<80 chars)

---

## Best Practices

### For Users

1. **Use Code Blocks**: Wrap code in triple backticks with language
   ````markdown
   ```python
   code here
   ```
   ````

2. **Request Formatting**: Be explicit
   - ✅ "show me code with syntax highlighting"
   - ✅ "format as markdown table"
   - ✅ "use bullet points"

3. **Terminal Size**: Use 80+ column width for best results

4. **Theme**: Glamour auto-detects dark/light theme
   - Dark terminal → dark theme
   - Light terminal → light theme

### For Developers

1. **Always Use Glamour**: For markdown rendering
   ```go
   rendered, _ := renderer.Render(content)
   ```

2. **Cache Renderer**: Don't create new renderer per message
   ```go
   if m.renderer == nil {
       m.renderer, _ = glamour.NewTermRenderer(...)
   }
   ```

3. **Handle Width**: Pass terminal width to renderer
   ```go
   glamour.WithWordWrap(terminalWidth - padding)
   ```

4. **Graceful Fallback**: If glamour fails, show plain text
   ```go
   rendered, err := renderer.Render(content)
   if err != nil {
       return content // Plain text fallback
   }
   ```

---

## Examples Gallery

### Example 1: Python Function
````markdown
```python
def fibonacci(n):
    """Calculate fibonacci number recursively"""
    if n <= 1:
        return n
    return fibonacci(n-1) + fibonacci(n-2)
```
````

### Example 2: Comparison Table
```markdown
| Feature | Go | Rust |
|---------|----|----|
| Speed | Fast | Fast |
| Memory Safety | GC | Ownership |
| Learning Curve | Easy | Hard |
```

### Example 3: Mixed Content
```markdown
# API Design Guide

## Best Practices

1. **Use RESTful conventions**
   - GET for retrieval
   - POST for creation
   - PUT for updates
   - DELETE for removal

2. **Example endpoint**:
   ```go
   // GET /api/users/:id
   func GetUser(c *gin.Context) {
       id := c.Param("id")
       user, err := db.GetUser(id)
       if err != nil {
           c.JSON(404, gin.H{"error": "Not found"})
           return
       }
       c.JSON(200, user)
   }
   ```

3. **Error handling**: Always return meaningful error messages
```

---

## Troubleshooting

### Issue: No Syntax Highlighting

**Cause**: Language not specified in code block

**Fix**:
````markdown
# Bad
```
code here
```

# Good
```python
code here
```
````

### Issue: Text Overflow

**Cause**: Terminal too narrow

**Fix**:
- Resize terminal to 80+ columns
- Use shorter code examples
- Request wrapped/formatted output

### Issue: Colors Not Showing

**Cause**: Terminal doesn't support 256 colors

**Fix**:
- Use modern terminal (iTerm2, Alacritty, etc.)
- Check `TERM` environment variable
- Set `TERM=xterm-256color`

### Issue: Markdown Not Rendering

**Cause**: Glamour renderer not initialized

**Fix**: Check logs, ensure Glamour is imported and initialized

---

## Summary

✅ **Supported**: Code blocks, syntax highlighting, headers, lists, tables, bold, italic, links, blockquotes

✅ **Modes**: Both command mode and interactive chat mode

✅ **Engine**: Glamour with Chroma for 100+ languages

✅ **Themes**: Auto dark/light detection

✅ **Performance**: Cached rendering, word wrapping

✅ **Fallback**: Plain text if rendering fails

---

**Questions?** Check the implementation in:
- `cmd/alex/stream_output.go` - Command mode rendering
- `cmd/alex/tui_chat/rendering.go` - Chat mode rendering
- `docs/implementation/CHAT_TUI_PHASE2_IMPLEMENTATION.md` - Full implementation details
