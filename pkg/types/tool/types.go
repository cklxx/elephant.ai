package tool

import (
	"fmt"
	"time"
)

// ToolCall represents a tool call
type ToolCall struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
	Timestamp time.Time              `json:"timestamp"`
}

// ToolResult represents a tool execution result
type ToolResult struct {
	CallID   string                 `json:"callId"`
	ToolName string                 `json:"toolName"`
	Success  bool                   `json:"success"`
	Content  string                 `json:"content"`
	Error    string                 `json:"error,omitempty"`
	Duration time.Duration          `json:"duration"`
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

// ToolExecution represents a tool execution event
type ToolExecution struct {
	ID        string                 `json:"id"`
	SessionID string                 `json:"sessionId"`
	ToolName  string                 `json:"toolName"`
	Arguments map[string]interface{} `json:"arguments"`
	Result    *ToolResult            `json:"result,omitempty"`
	StartTime time.Time              `json:"startTime"`
	EndTime   time.Time              `json:"endTime"`
	Duration  time.Duration          `json:"duration"`
	Status    string                 `json:"status"` // pending, running, completed, failed
	Error     string                 `json:"error,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ToolRegistry represents a registry of available tools
type ToolRegistry struct {
	Tools       map[string]ToolInfo `json:"tools"`
	Categories  []string            `json:"categories"`
	LastUpdated time.Time           `json:"lastUpdated"`
}

// ToolInfo represents information about a tool
type ToolInfo struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Category    string                 `json:"category"`
	Version     string                 `json:"version"`
	Parameters  map[string]interface{} `json:"parameters"`
	Examples    []ToolExample          `json:"examples,omitempty"`
	Enabled     bool                   `json:"enabled"`
	Timeout     time.Duration          `json:"timeout"`
	MaxRetries  int                    `json:"maxRetries"`
	Permissions []string               `json:"permissions,omitempty"`
}

// ToolExample represents an example usage of a tool
type ToolExample struct {
	Description string                 `json:"description"`
	Input       map[string]interface{} `json:"input"`
	Output      string                 `json:"output"`
}

// ToolValidationResult represents the result of tool validation
type ToolValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// ToolPermission represents a tool permission
type ToolPermission struct {
	Type        string `json:"type"`     // read, write, execute, network
	Resource    string `json:"resource"` // file path, URL pattern, etc.
	Description string `json:"description"`
}

// ToolConfig represents tool-specific configuration
type ToolConfig struct {
	Name        string                 `json:"name"`
	Enabled     bool                   `json:"enabled"`
	Timeout     time.Duration          `json:"timeout"`
	MaxRetries  int                    `json:"maxRetries"`
	Settings    map[string]interface{} `json:"settings"`
	Permissions []ToolPermission       `json:"permissions"`
}

// NewToolExecution creates a new ToolExecution
func NewToolExecution(sessionID, toolName string, arguments map[string]interface{}) *ToolExecution {
	return &ToolExecution{
		ID:        generateID(),
		SessionID: sessionID,
		ToolName:  toolName,
		Arguments: arguments,
		StartTime: time.Now(),
		Status:    "pending",
		Metadata:  make(map[string]interface{}),
	}
}

// generateID generates a unique ID for tool execution
func generateID() string {
	return fmt.Sprintf("tool_%d", time.Now().UnixNano())
}
