package message

import (
	"encoding/json"
	"fmt"
	"time"
)

// ToolCallImpl is the unified tool call implementation
type ToolCallImpl struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
	Timestamp time.Time              `json:"timestamp"`
}

// NewToolCall creates a new unified tool call
func NewToolCall(id, name string, arguments map[string]interface{}) *ToolCallImpl {
	if arguments == nil {
		arguments = make(map[string]interface{})
	}

	return &ToolCallImpl{
		ID:        id,
		Name:      name,
		Arguments: arguments,
		Timestamp: time.Now(),
	}
}

// Implement ToolCall interface
func (tc *ToolCallImpl) GetID() string                        { return tc.ID }
func (tc *ToolCallImpl) GetName() string                      { return tc.Name }
func (tc *ToolCallImpl) GetArguments() map[string]interface{} { return tc.Arguments }
func (tc *ToolCallImpl) GetTimestamp() time.Time              { return tc.Timestamp }

func (tc *ToolCallImpl) GetArgumentsJSON() string {
	if len(tc.Arguments) == 0 {
		return "{}"
	}

	data, err := json.Marshal(tc.Arguments)
	if err != nil {
		return "{}"
	}

	return string(data)
}

// Setters
func (tc *ToolCallImpl) SetID(id string)                               { tc.ID = id }
func (tc *ToolCallImpl) SetName(name string)                           { tc.Name = name }
func (tc *ToolCallImpl) SetArguments(arguments map[string]interface{}) { tc.Arguments = arguments }
func (tc *ToolCallImpl) SetTimestamp(timestamp time.Time)              { tc.Timestamp = timestamp }

// SetArgumentsFromJSON parses JSON string and sets arguments
func (tc *ToolCallImpl) SetArgumentsFromJSON(jsonStr string) error {
	if jsonStr == "" {
		tc.Arguments = make(map[string]interface{})
		return nil
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &args); err != nil {
		return fmt.Errorf("failed to parse arguments JSON: %w", err)
	}

	tc.Arguments = args
	return nil
}

// AddArgument adds a single argument
func (tc *ToolCallImpl) AddArgument(key string, value interface{}) {
	if tc.Arguments == nil {
		tc.Arguments = make(map[string]interface{})
	}
	tc.Arguments[key] = value
}

// GetArgument gets a single argument
func (tc *ToolCallImpl) GetArgument(key string) (interface{}, bool) {
	if tc.Arguments == nil {
		return nil, false
	}
	value, exists := tc.Arguments[key]
	return value, exists
}

// ToLLMToolCall converts to LLM protocol tool call
func (tc *ToolCallImpl) ToLLMToolCall() LLMToolCall {
	return LLMToolCall{
		ID:   tc.ID,
		Type: "function",
		Function: LLMFunction{
			Name:      tc.Name,
			Arguments: tc.GetArgumentsJSON(),
		},
	}
}

// ToSessionToolCall converts to session storage tool call
func (tc *ToolCallImpl) ToSessionToolCall() SessionToolCall {
	return SessionToolCall{
		ID:   tc.ID,
		Name: tc.Name,
		Args: tc.Arguments,
	}
}

// FromLLMToolCall creates ToolCallImpl from LLM protocol tool call
func FromLLMToolCall(llmTC LLMToolCall) *ToolCallImpl {
	tc := &ToolCallImpl{
		ID:        llmTC.ID,
		Name:      llmTC.Function.Name,
		Arguments: make(map[string]interface{}),
		Timestamp: time.Now(),
	}

	// Parse arguments from JSON string
	if llmTC.Function.Arguments != "" {
		if err := tc.SetArgumentsFromJSON(llmTC.Function.Arguments); err != nil {
			// If parsing fails, store as raw string
			tc.Arguments = map[string]interface{}{
				"raw": llmTC.Function.Arguments,
			}
		}
	}

	return tc
}

// FromSessionToolCall creates ToolCallImpl from session storage tool call
func FromSessionToolCall(sessionTC SessionToolCall) *ToolCallImpl {
	args := sessionTC.Args
	if args == nil {
		args = make(map[string]interface{})
	}

	return &ToolCallImpl{
		ID:        sessionTC.ID,
		Name:      sessionTC.Name,
		Arguments: args,
		Timestamp: time.Now(), // Session doesn't store timestamp, use current
	}
}

// ToolResultImpl is the unified tool result implementation
type ToolResultImpl struct {
	CallID    string                 `json:"call_id"`
	ToolName  string                 `json:"tool_name"`
	Success   bool                   `json:"success"`
	Content   string                 `json:"content"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Duration  time.Duration          `json:"duration"`
	Timestamp time.Time              `json:"timestamp"`
}

// NewToolResult creates a new tool result
func NewToolResult(callID, toolName string) *ToolResultImpl {
	return &ToolResultImpl{
		CallID:    callID,
		ToolName:  toolName,
		Success:   false,
		Data:      make(map[string]interface{}),
		Timestamp: time.Now(),
	}
}

// NewSuccessResult creates a successful tool result
func NewSuccessResult(callID, toolName, content string, duration time.Duration) *ToolResultImpl {
	result := NewToolResult(callID, toolName)
	result.Success = true
	result.Content = content
	result.Duration = duration
	return result
}

// NewErrorResult creates an error tool result
func NewErrorResult(callID, toolName, errorMsg string, duration time.Duration) *ToolResultImpl {
	result := NewToolResult(callID, toolName)
	result.Success = false
	result.Error = errorMsg
	result.Duration = duration
	return result
}

// Implement ToolResult interface
func (tr *ToolResultImpl) GetCallID() string               { return tr.CallID }
func (tr *ToolResultImpl) GetToolName() string             { return tr.ToolName }
func (tr *ToolResultImpl) GetSuccess() bool                { return tr.Success }
func (tr *ToolResultImpl) GetContent() string              { return tr.Content }
func (tr *ToolResultImpl) GetData() map[string]interface{} { return tr.Data }
func (tr *ToolResultImpl) GetError() string                { return tr.Error }
func (tr *ToolResultImpl) GetDuration() time.Duration      { return tr.Duration }

// Setters for ToolResult
func (tr *ToolResultImpl) SetSuccess(success bool)             { tr.Success = success }
func (tr *ToolResultImpl) SetContent(content string)           { tr.Content = content }
func (tr *ToolResultImpl) SetError(error string)               { tr.Error = error }
func (tr *ToolResultImpl) SetDuration(duration time.Duration)  { tr.Duration = duration }
func (tr *ToolResultImpl) SetData(data map[string]interface{}) { tr.Data = data }

// AddData adds a key-value pair to the result data
func (tr *ToolResultImpl) AddData(key string, value interface{}) {
	if tr.Data == nil {
		tr.Data = make(map[string]interface{})
	}
	tr.Data[key] = value
}

// JSON marshaling support for ToolCallImpl
func (tc *ToolCallImpl) MarshalJSON() ([]byte, error) {
	type Alias ToolCallImpl
	return json.Marshal((*Alias)(tc))
}

func (tc *ToolCallImpl) UnmarshalJSON(data []byte) error {
	type Alias ToolCallImpl
	aux := (*Alias)(tc)
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	// Ensure timestamp is set
	if tc.Timestamp.IsZero() {
		tc.Timestamp = time.Now()
	}

	// Ensure arguments is initialized
	if tc.Arguments == nil {
		tc.Arguments = make(map[string]interface{})
	}

	return nil
}

// JSON marshaling support for ToolResultImpl
func (tr *ToolResultImpl) MarshalJSON() ([]byte, error) {
	type Alias ToolResultImpl
	return json.Marshal((*Alias)(tr))
}

func (tr *ToolResultImpl) UnmarshalJSON(data []byte) error {
	type Alias ToolResultImpl
	aux := (*Alias)(tr)
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	// Ensure timestamp is set
	if tr.Timestamp.IsZero() {
		tr.Timestamp = time.Now()
	}

	// Ensure data is initialized
	if tr.Data == nil {
		tr.Data = make(map[string]interface{})
	}

	return nil
}
