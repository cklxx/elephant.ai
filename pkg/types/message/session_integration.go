package message

import (
	"encoding/json"
	"time"
)

// SessionIntegration provides integration utilities for session management
// This allows Session to use Message as subtype and element
type SessionIntegration struct {
	adapter *Adapter
}

// NewSessionIntegration creates a new session integration adapter
func NewSessionIntegration() *SessionIntegration {
	return &SessionIntegration{
		adapter: NewAdapter(),
	}
}

// SessionStorage represents the enhanced session storage that uses unified Message
type SessionStorage struct {
	ID        string                 `json:"id"`
	Messages  []*Message             `json:"messages"` // Use unified Message as element
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// NewSessionStorage creates a new session storage
func NewSessionStorage(id string) *SessionStorage {
	return &SessionStorage{
		ID:        id,
		Messages:  make([]*Message, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}
}

// AddMessage adds a unified message to the session
func (ss *SessionStorage) AddMessage(message *Message) {
	ss.Messages = append(ss.Messages, message)
	ss.UpdatedAt = time.Now()
}

// AddMessages adds multiple unified messages to the session
func (ss *SessionStorage) AddMessages(messages ...*Message) {
	ss.Messages = append(ss.Messages, messages...)
	ss.UpdatedAt = time.Now()
}

// GetMessages returns all messages in the session
func (ss *SessionStorage) GetMessages() []*Message {
	return ss.Messages
}

// GetMessagesByRole returns messages filtered by role
func (ss *SessionStorage) GetMessagesByRole(role string) []*Message {
	var filtered []*Message
	for _, msg := range ss.Messages {
		if msg.GetRole() == role {
			filtered = append(filtered, msg)
		}
	}
	return filtered
}

// GetLastMessage returns the last message in the session
func (ss *SessionStorage) GetLastMessage() *Message {
	if len(ss.Messages) == 0 {
		return nil
	}
	return ss.Messages[len(ss.Messages)-1]
}

// GetLastMessagesByRole returns the last N messages of a specific role
func (ss *SessionStorage) GetLastMessagesByRole(role string, count int) []*Message {
	var filtered []*Message

	// Iterate backwards to get the most recent messages first
	for i := len(ss.Messages) - 1; i >= 0 && len(filtered) < count; i-- {
		if ss.Messages[i].GetRole() == role {
			filtered = append([]*Message{ss.Messages[i]}, filtered...) // Prepend to maintain order
		}
	}

	return filtered
}

// GetMessagesAfter returns messages after a specific timestamp
func (ss *SessionStorage) GetMessagesAfter(timestamp time.Time) []*Message {
	var filtered []*Message
	for _, msg := range ss.Messages {
		if msg.GetTimestamp().After(timestamp) {
			filtered = append(filtered, msg)
		}
	}
	return filtered
}

// GetMessagesBefore returns messages before a specific timestamp
func (ss *SessionStorage) GetMessagesBefore(timestamp time.Time) []*Message {
	var filtered []*Message
	for _, msg := range ss.Messages {
		if msg.GetTimestamp().Before(timestamp) {
			filtered = append(filtered, msg)
		}
	}
	return filtered
}

// GetMessagesWithToolCalls returns messages that have tool calls
func (ss *SessionStorage) GetMessagesWithToolCalls() []*Message {
	var filtered []*Message
	for _, msg := range ss.Messages {
		if len(msg.GetToolCalls()) > 0 {
			filtered = append(filtered, msg)
		}
	}
	return filtered
}

// Count returns the number of messages in the session
func (ss *SessionStorage) Count() int {
	return len(ss.Messages)
}

// Clear removes all messages from the session
func (ss *SessionStorage) Clear() {
	ss.Messages = make([]*Message, 0)
	ss.UpdatedAt = time.Now()
}

// SetMetadata adds metadata to the session
func (ss *SessionStorage) SetMetadata(key string, value interface{}) {
	if ss.Metadata == nil {
		ss.Metadata = make(map[string]interface{})
	}
	ss.Metadata[key] = value
	ss.UpdatedAt = time.Now()
}

// GetMetadata retrieves metadata from the session
func (ss *SessionStorage) GetMetadata(key string) (interface{}, bool) {
	if ss.Metadata == nil {
		return nil, false
	}
	value, exists := ss.Metadata[key]
	return value, exists
}

// ToLLMFormat converts session messages to LLM format for API calls
func (ss *SessionStorage) ToLLMFormat() []LLMMessage {
	return ss.getIntegration().adapter.ConvertToLLMMessages(ss.Messages)
}

// ToSessionFormat converts messages to legacy session format for backward compatibility
func (ss *SessionStorage) ToSessionFormat() []SessionMessage {
	return ss.getIntegration().adapter.ConvertToSessionMessages(ss.Messages)
}

// FromLLMMessages populates session from LLM messages
func (ss *SessionStorage) FromLLMMessages(llmMessages []LLMMessage) {
	messages := ss.getIntegration().adapter.ConvertLLMMessages(llmMessages)
	ss.Messages = messages
	ss.UpdatedAt = time.Now()
}

// FromSessionMessages populates session from legacy session messages
func (ss *SessionStorage) FromSessionMessages(sessionMessages []SessionMessage) {
	messages := ss.getIntegration().adapter.ConvertSessionMessages(sessionMessages)
	ss.Messages = messages
	ss.UpdatedAt = time.Now()
}

// AddLLMMessage adds a message from LLM format
func (ss *SessionStorage) AddLLMMessage(llmMessage LLMMessage) {
	message := FromLLMMessage(llmMessage)
	ss.AddMessage(message)
}

// AddSessionMessage adds a message from legacy session format
func (ss *SessionStorage) AddSessionMessage(sessionMessage SessionMessage) {
	message := FromSessionMessage(sessionMessage)
	ss.AddMessage(message)
}

// getIntegration returns the session integration instance
func (ss *SessionStorage) getIntegration() *SessionIntegration {
	return NewSessionIntegration()
}

// SessionCompatibilityLayer provides backward compatibility for existing session code
type SessionCompatibilityLayer struct {
	storage *SessionStorage
}

// NewSessionCompatibilityLayer creates a compatibility layer for existing sessions
func NewSessionCompatibilityLayer(sessionID string) *SessionCompatibilityLayer {
	return &SessionCompatibilityLayer{
		storage: NewSessionStorage(sessionID),
	}
}

// GetStorage returns the underlying SessionStorage
func (scl *SessionCompatibilityLayer) GetStorage() *SessionStorage {
	return scl.storage
}

// Legacy methods for backward compatibility

// AddMessage adds a message using the legacy session message format
func (scl *SessionCompatibilityLayer) AddLegacyMessage(role, content string, toolCalls []SessionToolCall) {
	msg := NewMessage(role, content)

	// Convert tool calls
	for _, tc := range toolCalls {
		toolCall := FromSessionToolCall(tc)
		msg.AddToolCall(toolCall)
	}

	scl.storage.AddMessage(msg)
}

// GetLegacyMessages returns messages in legacy session format
func (scl *SessionCompatibilityLayer) GetLegacyMessages() []SessionMessage {
	return scl.storage.ToSessionFormat()
}

// GetLLMMessages returns messages in LLM format
func (scl *SessionCompatibilityLayer) GetLLMMessages() []LLMMessage {
	return scl.storage.ToLLMFormat()
}

// JSON marshaling support for SessionStorage
func (ss *SessionStorage) MarshalJSON() ([]byte, error) {
	type Alias SessionStorage
	return json.Marshal((*Alias)(ss))
}

func (ss *SessionStorage) UnmarshalJSON(data []byte) error {
	type Alias SessionStorage
	aux := (*Alias)(ss)
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	// Ensure slices and maps are initialized
	if ss.Messages == nil {
		ss.Messages = make([]*Message, 0)
	}
	if ss.Metadata == nil {
		ss.Metadata = make(map[string]interface{})
	}

	// Ensure timestamps are set
	if ss.CreatedAt.IsZero() {
		ss.CreatedAt = time.Now()
	}
	if ss.UpdatedAt.IsZero() {
		ss.UpdatedAt = time.Now()
	}

	return nil
}

// MessageSubtype demonstrates how Message can be used as a subtype in sessions
// This is an example of extending the base Message for session-specific needs
type SessionMessage_Enhanced struct {
	*Message                           // Message as subtype
	SessionID   string                 `json:"session_id"`
	Index       int                    `json:"index"`                  // Position in session
	ParentID    string                 `json:"parent_id,omitempty"`    // For threaded conversations
	ChildrenIDs []string               `json:"children_ids,omitempty"` // For threaded conversations
	SessionMeta map[string]interface{} `json:"session_meta,omitempty"` // Session-specific metadata
}

// NewSessionMessageEnhanced creates an enhanced session message
func NewSessionMessageEnhanced(sessionID string, index int, baseMessage *Message) *SessionMessage_Enhanced {
	return &SessionMessage_Enhanced{
		Message:     baseMessage,
		SessionID:   sessionID,
		Index:       index,
		ChildrenIDs: make([]string, 0),
		SessionMeta: make(map[string]interface{}),
	}
}

// This demonstrates that Message can be used as both:
// 1. Element in collections (SessionStorage.Messages []*Message)
// 2. Subtype in enhanced structures (SessionMessage_Enhanced embeds *Message)
