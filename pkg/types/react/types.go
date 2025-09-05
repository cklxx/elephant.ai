package react

import (
	"time"
)

// ReactTaskContext represents the context for a ReAct task
type ReactTaskContext struct {
	TaskID           string                 `json:"taskId"`
	UserMessage      string                 `json:"userMessage"`
	StartTime        time.Time              `json:"startTime"`
	LastUpdate       time.Time              `json:"lastUpdate"`
	Status           string                 `json:"status"`
	History          []ReactExecutionStep   `json:"history"`
	TokensUsed       int                    `json:"tokensUsed"`
	PromptTokens     int                    `json:"promptTokens"`
	CompletionTokens int                    `json:"completionTokens"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// ReactExecutionStep represents a single step in ReAct execution
type ReactExecutionStep struct {
	Number      int                    `json:"number"`
	Timestamp   time.Time              `json:"timestamp"`
	Duration    time.Duration          `json:"duration"`
	Thought     string                 `json:"thought"`
	Action      string                 `json:"action"`
	Observation string                 `json:"observation"`
	ToolCall    []*ReactToolCall       `json:"toolCall,omitempty"`
	Result      []*ReactToolResult     `json:"result,omitempty"`
	TokensUsed  int                    `json:"tokensUsed"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ReactTaskResult represents the final result of a ReAct task
type ReactTaskResult struct {
	TaskID           string                 `json:"taskId"`
	Answer           string                 `json:"answer"`
	Success          bool                   `json:"success"`
	Confidence       float64                `json:"confidence"`
	Steps            []ReactExecutionStep   `json:"steps"`
	TokensUsed       int                    `json:"tokensUsed"`
	PromptTokens     int                    `json:"promptTokens"`
	CompletionTokens int                    `json:"completionTokens"`
	Duration         time.Duration          `json:"duration"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// ReactToolCall represents a tool call in ReAct execution
type ReactToolCall struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
	Timestamp time.Time              `json:"timestamp"`
}

// ReactToolResult represents the result of a tool call in ReAct execution
type ReactToolResult struct {
	CallID   string                 `json:"callId"`
	ToolName string                 `json:"toolName"`
	Success  bool                   `json:"success"`
	Content  string                 `json:"content"`
	Error    string                 `json:"error,omitempty"`
	Duration time.Duration          `json:"duration"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ReactConfig represents configuration for ReAct agent
type ReactConfig struct {
	MaxIterations       int      `json:"maxIterations"`
	ThinkingTimeout     int      `json:"thinkingTimeout"`
	ActionTimeout       int      `json:"actionTimeout"`
	ConfidenceThreshold float64  `json:"confidenceThreshold"`
	Temperature         float64  `json:"temperature"`
	MaxTokens           int      `json:"maxTokens"`
	StopSequences       []string `json:"stopSequences"`
	EnableStreaming     bool     `json:"enableStreaming"`
	EnableMemory        bool     `json:"enableMemory"`
	EnableContext       bool     `json:"enableContext"`
}

// NewReactConfig creates a new ReactConfig with default values
func NewReactConfig() *ReactConfig {
	return &ReactConfig{
		MaxIterations:       10,
		ThinkingTimeout:     30,
		ActionTimeout:       60,
		ConfidenceThreshold: 0.8,
		Temperature:         0.7,
		MaxTokens:           2000,
		StopSequences:       []string{"</thinking>", "<observation>"},
		EnableStreaming:     true,
		EnableMemory:        true,
		EnableContext:       true,
	}
}

// NewReactTaskContext creates a new ReactTaskContext
func NewReactTaskContext(taskID, userMessage string) *ReactTaskContext {
	return &ReactTaskContext{
		TaskID:      taskID,
		UserMessage: userMessage,
		StartTime:   time.Now(),
		LastUpdate:  time.Now(),
		Status:      "running",
		History:     []ReactExecutionStep{},
		Metadata:    make(map[string]interface{}),
	}
}
