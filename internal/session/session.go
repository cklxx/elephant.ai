package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"alex/internal/llm"
)

// Session represents a conversation session
type Session struct {
	ID       string     `json:"id"`
	Created  time.Time  `json:"created"`
	Updated  time.Time  `json:"updated"`
	Messages []*Message `json:"messages"`
	Context  string     `json:"context,omitempty"`

	// Session metadata
	WorkingDir string                 `json:"working_dir,omitempty"`
	Config     map[string]interface{} `json:"config,omitempty"`

	// Kimi API context caching
	KimiCacheID string `json:"kimi_cache_id,omitempty"`

	mutex sync.RWMutex
}

// Message represents a message in the session
type Message struct {
	Role       string                 `json:"role"`
	Content    string                 `json:"content"`
	Name       string                 `json:"name,omitempty"`
	ToolCalls  []llm.ToolCall         `json:"tool_calls,omitempty"`
	ToolCallId string                 `json:"tool_call_id,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
}

// Manager handles session persistence and restoration
type Manager struct {
	sessionsDir      string
	sessions         map[string]*Session
	mutex            sync.RWMutex
	currentSessionID string
}

// NewManager creates a new session manager
func NewManager() (*Manager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	sessionsDir := filepath.Join(homeDir, ".deep-coding-sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create sessions directory: %w", err)
	}

	return &Manager{
		sessionsDir: sessionsDir,
		sessions:    make(map[string]*Session),
	}, nil
}

// GetSessionsDir returns the sessions directory path
func (m *Manager) GetSessionsDir() string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.sessionsDir
}

func (m *Manager) GetSessionID() (string, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.currentSessionID, m.currentSessionID != ""
}

// StartSession creates a new session
func (m *Manager) StartSession(sessionID string) (*Session, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if sessionID == "" {
		sessionID = generateSessionID()
	}

	// Check if session already exists
	if _, exists := m.sessions[sessionID]; exists {
		return nil, fmt.Errorf("session %s already exists", sessionID)
	}

	// Get current working directory
	workingDir, _ := os.Getwd()

	session := &Session{
		ID:         sessionID,
		Created:    time.Now(),
		Updated:    time.Now(),
		Messages:   make([]*Message, 0),
		WorkingDir: workingDir,
		Config:     make(map[string]interface{}),
	}

	// Clean up any existing todo file for this session to ensure fresh start
	m.cleanupSessionTodoFile(sessionID)
	m.currentSessionID = sessionID
	m.sessions[sessionID] = session
	return session, nil
}

// RestoreSession loads an existing session from disk
func (m *Manager) RestoreSession(sessionID string) (*Session, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Check if already loaded in memory
	if session, exists := m.sessions[sessionID]; exists {
		m.currentSessionID = sessionID // Set current session ID for tools to access
		return session, nil
	}

	// Load from disk
	sessionFile := filepath.Join(m.sessionsDir, sessionID+".json")
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to parse session data: %w", err)
	}

	m.sessions[sessionID] = &session
	m.currentSessionID = sessionID // Set current session ID for tools to access
	return &session, nil
}

// SaveSession persists a session to disk
func (m *Manager) SaveSession(session *Session) error {
	session.mutex.Lock()
	session.Updated = time.Now()
	session.mutex.Unlock()

	sessionFile := filepath.Join(m.sessionsDir, session.ID+".json")
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	if err := os.WriteFile(sessionFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}

// ListSessions returns a list of available session IDs
func (m *Manager) ListSessions() ([]string, error) {
	entries, err := os.ReadDir(m.sessionsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var sessionIDs []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			sessionID := entry.Name()[:len(entry.Name())-5] // Remove .json extension
			sessionIDs = append(sessionIDs, sessionID)
		}
	}

	return sessionIDs, nil
}

// DeleteSession removes a session from memory and disk
func (m *Manager) DeleteSession(sessionID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Remove from memory
	delete(m.sessions, sessionID)

	// Remove session file from disk
	sessionFile := filepath.Join(m.sessionsDir, sessionID+".json")
	if err := os.Remove(sessionFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete session file: %w", err)
	}

	// Also clean up the session's todo file
	m.cleanupSessionTodoFile(sessionID)

	return nil
}

// CleanupExpiredSessions removes sessions older than the specified duration
func (m *Manager) CleanupExpiredSessions(maxAge time.Duration) error {
	sessionIDs, err := m.ListSessions()
	if err != nil {
		return err
	}

	cutoff := time.Now().Add(-maxAge)

	for _, sessionID := range sessionIDs {
		sessionFile := filepath.Join(m.sessionsDir, sessionID+".json")
		info, err := os.Stat(sessionFile)
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			if err := m.DeleteSession(sessionID); err != nil {
				fmt.Printf("Warning: failed to delete expired session %s: %v\n", sessionID, err)
			}
		}
	}

	return nil
}

// CleanupMemory removes sessions from memory that haven't been accessed recently
func (m *Manager) CleanupMemory(idleTimeout time.Duration) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	cutoff := time.Now().Add(-idleTimeout)
	var toRemove []string

	for sessionID, session := range m.sessions {
		session.mutex.RLock()
		lastAccessed := session.Updated
		session.mutex.RUnlock()

		if lastAccessed.Before(cutoff) {
			toRemove = append(toRemove, sessionID)
		}
	}

	for _, sessionID := range toRemove {
		delete(m.sessions, sessionID)
	}

	return nil
}

// TrimSessionMessages limits the number of messages in a session to prevent memory bloat
func (m *Manager) TrimSessionMessages(sessionID string, maxMessages int) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	session.mutex.Lock()
	defer session.mutex.Unlock()

	if len(session.Messages) > maxMessages {
		// Keep the most recent messages
		start := len(session.Messages) - maxMessages
		session.Messages = session.Messages[start:]
	}

	return nil
}

// GetMemoryStats returns memory usage statistics
func (m *Manager) GetMemoryStats() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	stats := map[string]interface{}{
		"sessions_in_memory": len(m.sessions),
		"total_messages":     0,
	}

	totalMessages := 0
	for _, session := range m.sessions {
		session.mutex.RLock()
		totalMessages += len(session.Messages)
		session.mutex.RUnlock()
	}

	stats["total_messages"] = totalMessages
	return stats
}

// Session methods

// AddMessage adds a message to the session
func (s *Session) AddMessage(message *Message) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if message.Timestamp.IsZero() {
		message.Timestamp = time.Now()
	}

	s.Messages = append(s.Messages, message)
	s.Updated = time.Now()
}

// TrimMessages trims the session messages to keep only the most recent maxMessages
func (s *Session) TrimMessages(maxMessages int) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if len(s.Messages) > maxMessages {
		// Keep the most recent messages
		start := len(s.Messages) - maxMessages
		s.Messages = s.Messages[start:]
	}
}

// GetMessages returns all messages in the session
func (s *Session) GetMessages() []*Message {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Return a copy to prevent external modification
	messages := make([]*Message, len(s.Messages))
	copy(messages, s.Messages)
	return messages
}

// GetContext returns the session context
func (s *Session) GetContext() string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	context := s.Context
	if context == "" && s.WorkingDir != "" {
		context = fmt.Sprintf("Working directory: %s", s.WorkingDir)
	}

	return context
}

// SetContext sets the session context
func (s *Session) SetContext(context string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.Context = context
	s.Updated = time.Now()
}

// GetConfig returns a config value
func (s *Session) GetConfig(key string) (interface{}, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	value, exists := s.Config[key]
	return value, exists
}

// SetConfig sets a config value
func (s *Session) SetConfig(key string, value interface{}) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.Config[key] = value
	s.Updated = time.Now()
}

// GetMessageCount returns the number of messages in the session
func (s *Session) GetMessageCount() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return len(s.Messages)
}

// ClearMessages removes all messages from the session
func (s *Session) ClearMessages() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.Messages = make([]*Message, 0)
	s.Updated = time.Now()
}

// SetKimiCacheID sets the Kimi cache ID for the session
func (s *Session) SetKimiCacheID(cacheID string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.KimiCacheID = cacheID
	s.Updated = time.Now()
}

// GetKimiCacheID gets the Kimi cache ID for the session
func (s *Session) GetKimiCacheID() string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.KimiCacheID
}

// ClearKimiCacheID clears the Kimi cache ID from the session
func (s *Session) ClearKimiCacheID() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.KimiCacheID = ""
	s.Updated = time.Now()
}

// cleanupSessionTodoFile removes any existing todo file for the session
func (m *Manager) cleanupSessionTodoFile(sessionID string) {
	todoFile := filepath.Join(m.sessionsDir, sessionID+"_todo.md")
	if err := os.Remove(todoFile); err != nil && !os.IsNotExist(err) {
		// Log but don't fail - this is a cleanup operation
		fmt.Printf("Warning: failed to cleanup todo file for session %s: %v\n", sessionID, err)
	}
}

// Helper function to generate a unique session ID
func generateSessionID() string {
	return fmt.Sprintf("session_%d", time.Now().UnixNano())
}
