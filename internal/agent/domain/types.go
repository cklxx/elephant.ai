package domain

// Message represents a conversation message (domain type)
type Message struct {
	Role        string
	Content     string
	ToolCalls   []ToolCall
	ToolResults []ToolResult
	Metadata    map[string]any
}

// ToolCall represents a tool invocation request
type ToolCall struct {
	ID        string
	Name      string
	Arguments map[string]any
}

// ToolResult is the result of tool execution
type ToolResult struct {
	CallID   string
	Content  string
	Error    error
	Metadata map[string]any
}

// TaskState tracks execution state during ReAct loop
type TaskState struct {
	// System prompt to prepend to conversation
	SystemPrompt string

	// Messages in current conversation
	Messages []Message

	// Current iteration count
	Iterations int

	// Total token count
	TokenCount int

	// Tool results accumulated
	ToolResults []ToolResult

	// Whether task is complete
	Complete bool

	// Final answer (if complete)
	FinalAnswer string
}

// TaskResult is the final result of task execution
type TaskResult struct {
	Answer     string
	Messages   []Message
	Iterations int
	TokensUsed int
	StopReason string // "max_iterations", "final_answer", "error"
}
