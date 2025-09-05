package common

import (
	"time"
)

// StreamChunk represents a streaming response chunk
type StreamChunk struct {
	Content string `json:"content"`
	Delta   string `json:"delta,omitempty"`
	Done    bool   `json:"done,omitempty"`
}

// TodoItem represents a single todo task
type TodoItem struct {
	ID          string     `json:"id"`
	Content     string     `json:"content"`
	Status      string     `json:"status"`             // pending, in_progress, completed
	Priority    string     `json:"priority,omitempty"` // low, medium, high
	Order       int        `json:"order"`              // execution order (1, 2, 3...)
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

// ProjectSummary represents a project structure summary
type ProjectSummary struct {
	Name          string             `json:"name"`
	Description   string             `json:"description"`
	Type          string             `json:"type"` // go, node, python, etc.
	Languages     []string           `json:"languages"`
	Files         []FileInfo         `json:"files"`
	Directories   []DirectoryContext `json:"directories"`
	ConfigFiles   []string           `json:"configFiles"`
	Dependencies  []string           `json:"dependencies"`
	TestFiles     []string           `json:"testFiles"`
	DocumentFiles []string           `json:"documentFiles"`
	TotalFiles    int                `json:"totalFiles"`
	TotalLines    int                `json:"totalLines"`
	LastModified  time.Time          `json:"lastModified"`
	CreatedAt     time.Time          `json:"createdAt"`
}

// DirectoryContext represents directory context information
type DirectoryContext struct {
	Path         string     `json:"path"`
	FileCount    int        `json:"fileCount"`
	Purpose      string     `json:"purpose"`
	Languages    []string   `json:"languages"`
	Files        []FileInfo `json:"files"`
	LastModified time.Time  `json:"lastModified"`
}

// FileInfo represents file information
type FileInfo struct {
	Path         string    `json:"path"`
	Name         string    `json:"name"`
	Size         int64     `json:"size"`
	Language     string    `json:"language"`
	Type         string    `json:"type"` // source, test, config, doc
	Lines        int       `json:"lines"`
	LastModified time.Time `json:"lastModified"`
}

// ChatMessage represents a chat message
type ChatMessage struct {
	Role      string                 `json:"role"`
	Content   string                 `json:"content"`
	ToolCalls []ToolCall             `json:"tool_calls,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ToolCall represents a tool call
type ToolCall struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Function FunctionCall `json:"function"`
}

// FunctionCall represents a function call
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolResult represents a tool execution result
type ToolResult struct {
	Content  string                 `json:"content"`
	Success  bool                   `json:"success"`
	Error    string                 `json:"error,omitempty"`
	Data     map[string]interface{} `json:"data,omitempty"`
	Files    []string               `json:"files,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ToolDefinition represents a tool definition for LLM
type ToolDefinition struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

// FunctionDefinition represents a function definition
type FunctionDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}
