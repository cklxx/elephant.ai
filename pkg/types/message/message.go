package message

import (
	"encoding/json"
	"time"
)

// Message is the unified message implementation
type Message struct {
	Role       string                 `json:"role"`
	Content    string                 `json:"content"`
	ToolCalls  []*ToolCallImpl        `json:"tool_calls,omitempty"`
	ToolCallID string                 `json:"tool_call_id,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`

	// Advanced fields for reasoning models
	Reasoning        string `json:"reasoning,omitempty"`
	ReasoningSummary string `json:"reasoning_summary,omitempty"`
	Think            string `json:"think,omitempty"`
}

// NewMessage creates a new unified message
func NewMessage(role, content string) *Message {
	return &Message{
		Role:      role,
		Content:   content,
		Metadata:  make(map[string]interface{}),
		Timestamp: time.Now(),
		ToolCalls: make([]*ToolCallImpl, 0),
	}
}

// NewUserMessage creates a user message
func NewUserMessage(content string) *Message {
	return NewMessage(string(MessageTypeUser), content)
}

// NewAssistantMessage creates an assistant message
func NewAssistantMessage(content string) *Message {
	return NewMessage(string(MessageTypeAssistant), content)
}

// NewSystemMessage creates a system message
func NewSystemMessage(content string) *Message {
	return NewMessage(string(MessageTypeSystem), content)
}

// NewToolMessage creates a tool response message
func NewToolMessage(content, toolCallID string) *Message {
	msg := NewMessage(string(MessageTypeTool), content)
	msg.ToolCallID = toolCallID
	return msg
}

// Implement BaseMessage interface
func (m *Message) GetRole() string                     { return m.Role }
func (m *Message) GetContent() string                  { return m.Content }
func (m *Message) GetToolCallID() string               { return m.ToolCallID }
func (m *Message) GetMetadata() map[string]interface{} { return m.Metadata }
func (m *Message) GetTimestamp() time.Time             { return m.Timestamp }
func (m *Message) GetReasoning() string                { return m.Reasoning }
func (m *Message) GetReasoningSummary() string         { return m.ReasoningSummary }
func (m *Message) GetThink() string                    { return m.Think }

func (m *Message) GetToolCalls() []ToolCall {
	toolCalls := make([]ToolCall, len(m.ToolCalls))
	for i, tc := range m.ToolCalls {
		toolCalls[i] = tc
	}
	return toolCalls
}

// Setters
func (m *Message) SetRole(role string)                         { m.Role = role }
func (m *Message) SetContent(content string)                   { m.Content = content }
func (m *Message) SetToolCallID(id string)                     { m.ToolCallID = id }
func (m *Message) SetMetadata(metadata map[string]interface{}) { m.Metadata = metadata }
func (m *Message) SetTimestamp(timestamp time.Time)            { m.Timestamp = timestamp }
func (m *Message) SetReasoning(reasoning string)               { m.Reasoning = reasoning }
func (m *Message) SetReasoningSummary(summary string)          { m.ReasoningSummary = summary }
func (m *Message) SetThink(think string)                       { m.Think = think }

// AddToolCall adds a tool call to the message
func (m *Message) AddToolCall(toolCall *ToolCallImpl) {
	m.ToolCalls = append(m.ToolCalls, toolCall)
}

// AddToolCallFromData creates and adds a tool call from data
func (m *Message) AddToolCallFromData(id, name string, args map[string]interface{}) {
	toolCall := NewToolCall(id, name, args)
	m.AddToolCall(toolCall)
}

// AddMetadata adds metadata key-value pair
func (m *Message) AddMetadata(key string, value interface{}) {
	if m.Metadata == nil {
		m.Metadata = make(map[string]interface{})
	}
	m.Metadata[key] = value
}

// ToLLMMessage converts to LLM protocol message
func (m *Message) ToLLMMessage() LLMMessage {
	llmMsg := LLMMessage{
		Role:             m.Role,
		Content:          m.Content,
		ToolCallID:       m.ToolCallID,
		Reasoning:        m.Reasoning,
		ReasoningSummary: m.ReasoningSummary,
		Think:            m.Think,
	}

	// Convert tool calls
	if len(m.ToolCalls) > 0 {
		llmMsg.ToolCalls = make([]LLMToolCall, len(m.ToolCalls))
		for i, tc := range m.ToolCalls {
			llmMsg.ToolCalls[i] = tc.ToLLMToolCall()
		}
	}

	return llmMsg
}

// ToSessionMessage converts to session storage message
func (m *Message) ToSessionMessage() SessionMessage {
	sessionMsg := SessionMessage{
		Role:      m.Role,
		Content:   m.Content,
		ToolID:    m.ToolCallID,
		Metadata:  make(map[string]interface{}),
		Timestamp: m.Timestamp,
	}

	// Copy metadata and add reasoning fields if present
	for k, v := range m.Metadata {
		sessionMsg.Metadata[k] = v
	}
	if m.Reasoning != "" {
		sessionMsg.Metadata["reasoning"] = m.Reasoning
	}
	if m.ReasoningSummary != "" {
		sessionMsg.Metadata["reasoning_summary"] = m.ReasoningSummary
	}
	if m.Think != "" {
		sessionMsg.Metadata["think"] = m.Think
	}

	// Convert tool calls
	if len(m.ToolCalls) > 0 {
		sessionMsg.ToolCalls = make([]SessionToolCall, len(m.ToolCalls))
		for i, tc := range m.ToolCalls {
			sessionMsg.ToolCalls[i] = tc.ToSessionToolCall()
		}
	}

	return sessionMsg
}

// FromLLMMessage creates Message from LLM protocol message
func FromLLMMessage(llmMsg LLMMessage) *Message {
	msg := &Message{
		Role:             llmMsg.Role,
		Content:          llmMsg.Content,
		ToolCallID:       llmMsg.ToolCallID,
		Reasoning:        llmMsg.Reasoning,
		ReasoningSummary: llmMsg.ReasoningSummary,
		Think:            llmMsg.Think,
		Metadata:         make(map[string]interface{}),
		Timestamp:        time.Now(),
		ToolCalls:        make([]*ToolCallImpl, 0),
	}

	// Convert tool calls
	for _, tc := range llmMsg.ToolCalls {
		toolCall := FromLLMToolCall(tc)
		msg.ToolCalls = append(msg.ToolCalls, toolCall)
	}

	return msg
}

// FromSessionMessage creates Message from session storage message
func FromSessionMessage(sessionMsg SessionMessage) *Message {
	msg := &Message{
		Role:       sessionMsg.Role,
		Content:    sessionMsg.Content,
		ToolCallID: sessionMsg.ToolID,
		Metadata:   make(map[string]interface{}),
		Timestamp:  sessionMsg.Timestamp,
		ToolCalls:  make([]*ToolCallImpl, 0),
	}

	// Copy metadata and extract reasoning fields
	for k, v := range sessionMsg.Metadata {
		switch k {
		case "reasoning":
			if s, ok := v.(string); ok {
				msg.Reasoning = s
			}
		case "reasoning_summary":
			if s, ok := v.(string); ok {
				msg.ReasoningSummary = s
			}
		case "think":
			if s, ok := v.(string); ok {
				msg.Think = s
			}
		case "tool_call_id":
			// Restore tool_call_id from metadata to ToolCallID field if ToolID is empty
			if s, ok := v.(string); ok && msg.ToolCallID == "" {
				msg.ToolCallID = s
			}
			// Still keep it in metadata for compatibility
			msg.Metadata[k] = v
		default:
			msg.Metadata[k] = v
		}
	}

	// Convert tool calls
	for _, tc := range sessionMsg.ToolCalls {
		toolCall := FromSessionToolCall(tc)
		msg.ToolCalls = append(msg.ToolCalls, toolCall)
	}

	return msg
}

// JSON marshaling support
func (m *Message) MarshalJSON() ([]byte, error) {
	type Alias Message
	return json.Marshal((*Alias)(m))
}

func (m *Message) UnmarshalJSON(data []byte) error {
	type Alias Message
	aux := (*Alias)(m)
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	// Ensure timestamp is set
	if m.Timestamp.IsZero() {
		m.Timestamp = time.Now()
	}

	// Ensure metadata is initialized
	if m.Metadata == nil {
		m.Metadata = make(map[string]interface{})
	}

	return nil
}
