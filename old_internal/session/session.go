package session

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
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

	// Compression tracking
	SourceMessages []*Message `json:"source_messages,omitempty"`
	IsCompressed   bool       `json:"is_compressed,omitempty"`
}

// Manager handles session persistence and restoration
type Manager struct {
	sessionsDir      string
	sessions         map[string]*Session
	mutex            sync.RWMutex
	currentSessionID string

	// Memory management
	maxSessionsInMemory   int
	maxMessagesPerSession int
	lastCleanup           time.Time
	cleanupMutex          sync.Mutex

	// Access tracking for LRU cleanup
	sessionAccess map[string]time.Time
	accessMutex   sync.RWMutex

	// Async I/O for non-blocking persistence
	persistQueue    chan *Session
	persistCache    map[string]*Session
	persistMutex    sync.RWMutex
	lastPersist     map[string]time.Time
	persistInterval time.Duration
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

	m := &Manager{
		sessionsDir:           sessionsDir,
		sessions:              make(map[string]*Session),
		sessionAccess:         make(map[string]time.Time),
		maxSessionsInMemory:   20,  // Limit to 20 sessions in memory
		maxMessagesPerSession: 100, // Limit to 100 messages per session
		lastCleanup:           time.Now(),
		persistQueue:          make(chan *Session, 100),
		persistCache:          make(map[string]*Session),
		lastPersist:           make(map[string]time.Time),
		persistInterval:       30 * time.Second,
	}

	// Start background routines
	go m.backgroundCleanup()
	go m.backgroundPersistence()

	return m, nil
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

// GetCurrentSession returns the current active session
func (m *Manager) GetCurrentSession() (*Session, error) {
	m.mutex.RLock()
	sessionID := m.currentSessionID
	m.mutex.RUnlock()

	if sessionID == "" {
		return nil, fmt.Errorf("no active session")
	}

	m.mutex.RLock()
	session, exists := m.sessions[sessionID]
	m.mutex.RUnlock()

	if !exists {
		// Try to load from disk
		return m.loadSessionFromDisk(sessionID)
	}

	return session, nil
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

// RestoreSession loads an existing session from disk with memory management
func (m *Manager) RestoreSession(sessionID string) (*Session, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Check if already loaded in memory
	if session, exists := m.sessions[sessionID]; exists {
		m.updateAccess(sessionID)
		m.currentSessionID = sessionID
		return session, nil
	}

	// Check memory pressure and cleanup if needed
	if len(m.sessions) >= m.maxSessionsInMemory {
		m.cleanupOldestSessions()
	}

	// Load from disk asynchronously if possible
	session, err := m.loadSessionFromDisk(sessionID)
	if err != nil {
		return nil, err
	}

	// Apply message limits to prevent memory bloat
	if len(session.Messages) > m.maxMessagesPerSession {
		session.TrimMessages(m.maxMessagesPerSession)
	}

	m.sessions[sessionID] = session
	m.updateAccess(sessionID)
	m.currentSessionID = sessionID
	return session, nil
}

// SaveSession persists a session to disk (non-blocking with async option)
func (m *Manager) SaveSession(session *Session) error {
	session.mutex.Lock()
	session.Updated = time.Now()
	session.mutex.Unlock()

	// Try async save first for better performance
	if m.saveSessionAsync(session) {
		return nil
	}

	// Fallback to synchronous save
	return m.saveSessionSync(session)
}

// saveSessionAsync attempts non-blocking save
func (m *Manager) saveSessionAsync(session *Session) bool {
	// Check if we recently persisted this session
	m.persistMutex.RLock()
	lastPersistTime, exists := m.lastPersist[session.ID]
	m.persistMutex.RUnlock()

	if exists && time.Since(lastPersistTime) < m.persistInterval {
		// Queue for background persistence instead
		select {
		case m.persistQueue <- session:
			return true
		default:
			// Queue is full, fallback to sync
			return false
		}
	}

	// Need immediate persistence
	return false
}

// saveSessionSync performs synchronous save
func (m *Manager) saveSessionSync(session *Session) error {
	sessionFile := filepath.Join(m.sessionsDir, session.ID+".json")
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	if err := os.WriteFile(sessionFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	// Update persist tracking
	m.persistMutex.Lock()
	m.lastPersist[session.ID] = time.Now()
	m.persistMutex.Unlock()

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

// ListSessionObjects returns all sessions as Session objects
func (m *Manager) ListSessionObjects() ([]*Session, error) {
	sessionIDs, err := m.ListSessions()
	if err != nil {
		return nil, err
	}

	var sessions []*Session
	for _, sessionID := range sessionIDs {
		session, err := m.RestoreSession(sessionID)
		if err != nil {
			// Skip sessions that can't be loaded
			continue
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
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

// AddMessage adds a message to the session with automatic cleanup
func (s *Session) AddMessage(message *Message) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if message.Timestamp.IsZero() {
		message.Timestamp = time.Now()
	}

	s.Messages = append(s.Messages, message)
	s.Updated = time.Now()

	// Auto-trim if messages exceed reasonable limits
	const maxAutoMessages = 150
	if len(s.Messages) > maxAutoMessages {
		// Keep most recent messages
		start := len(s.Messages) - 100 // Trim to 100 messages
		s.Messages = s.Messages[start:]
		log.Printf("[INFO] Auto-trimmed session messages from %d to %d", len(s.Messages)+50, len(s.Messages))
	}
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

// Background cleanup methods for memory management

// backgroundCleanup runs periodic cleanup to manage memory usage
func (m *Manager) backgroundCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.cleanupMutex.Lock()
		// Check if cleanup is needed based on memory pressure
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		// Trigger cleanup if memory usage is high or time-based
		timeSinceLastCleanup := time.Since(m.lastCleanup)
		memoryPressure := memStats.HeapAlloc > 100*1024*1024 // 100MB

		if timeSinceLastCleanup > 10*time.Minute || memoryPressure {
			m.performMemoryCleanup()
			m.lastCleanup = time.Now()
		}
		m.cleanupMutex.Unlock()
	}
}

// performMemoryCleanup performs various cleanup operations
func (m *Manager) performMemoryCleanup() {
	// Clean up old sessions from memory
	if err := m.CleanupMemory(30 * time.Minute); err != nil {
		log.Printf("Warning: failed to cleanup memory: %v", err)
	}

	// Clean up expired sessions from disk
	if err := m.CleanupExpiredSessions(7 * 24 * time.Hour); err != nil { // 7 days
		log.Printf("Warning: failed to cleanup expired sessions: %v", err)
	}

	log.Printf("[INFO] Performed memory cleanup: %d sessions in memory", len(m.sessions))
}

// updateAccess records when a session was last accessed
func (m *Manager) updateAccess(sessionID string) {
	m.accessMutex.Lock()
	m.sessionAccess[sessionID] = time.Now()
	m.accessMutex.Unlock()
}

// cleanupOldestSessions removes the least recently used sessions from memory
func (m *Manager) cleanupOldestSessions() {
	if len(m.sessions) < m.maxSessionsInMemory {
		return
	}

	// Find oldest sessions to remove
	type sessionAccess struct {
		id       string
		lastUsed time.Time
	}

	var accesses []sessionAccess
	m.accessMutex.RLock()
	for sessionID := range m.sessions {
		lastUsed, exists := m.sessionAccess[sessionID]
		if !exists {
			lastUsed = time.Now().Add(-time.Hour) // Default to old if no access record
		}
		accesses = append(accesses, sessionAccess{id: sessionID, lastUsed: lastUsed})
	}
	m.accessMutex.RUnlock()

	// Sort by access time (oldest first)
	for i := 0; i < len(accesses)-1; i++ {
		for j := i + 1; j < len(accesses); j++ {
			if accesses[i].lastUsed.After(accesses[j].lastUsed) {
				accesses[i], accesses[j] = accesses[j], accesses[i]
			}
		}
	}

	// Remove oldest sessions until we're under the limit
	numToRemove := len(m.sessions) - m.maxSessionsInMemory + 1
	for i := 0; i < numToRemove && i < len(accesses); i++ {
		sessionID := accesses[i].id
		if sessionID != m.currentSessionID { // Don't remove current session
			delete(m.sessions, sessionID)
			m.accessMutex.Lock()
			delete(m.sessionAccess, sessionID)
			m.accessMutex.Unlock()
			log.Printf("[INFO] Removed session %s from memory due to pressure", sessionID)
		}
	}
}

// loadSessionFromDisk loads a session from disk with error handling
func (m *Manager) loadSessionFromDisk(sessionID string) (*Session, error) {
	sessionFile := filepath.Join(m.sessionsDir, sessionID+".json")
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to parse session data: %w", err)
	}

	return &session, nil
}

// ReplaceMessagesWithCompressed replaces a range of messages with a compressed version
func (s *Session) ReplaceMessagesWithCompressed(startIdx, endIdx int, compressedMsg *Message) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if startIdx < 0 || endIdx >= len(s.Messages) || startIdx > endIdx {
		return
	}

	// Store original messages as source in compressed message
	if compressedMsg.SourceMessages == nil {
		originalMessages := s.Messages[startIdx : endIdx+1]
		compressedMsg.SourceMessages = make([]*Message, len(originalMessages))
		copy(compressedMsg.SourceMessages, originalMessages)
		compressedMsg.IsCompressed = true

		// Update metadata
		if compressedMsg.Metadata == nil {
			compressedMsg.Metadata = make(map[string]interface{})
		}
		compressedMsg.Metadata["compression_time"] = time.Now().Format(time.RFC3339)
		compressedMsg.Metadata["source_count"] = len(originalMessages)
	}

	// Build new message list
	newMessages := make([]*Message, 0, len(s.Messages)-(endIdx-startIdx))

	// Add messages before the compressed range
	newMessages = append(newMessages, s.Messages[:startIdx]...)

	// Add the compressed message
	newMessages = append(newMessages, compressedMsg)

	// Add messages after the compressed range
	if endIdx+1 < len(s.Messages) {
		newMessages = append(newMessages, s.Messages[endIdx+1:]...)
	}

	s.Messages = newMessages
	s.Updated = time.Now()
}

// GetExpandedMessages returns all messages with compressed messages expanded
func (s *Session) GetExpandedMessages() []*Message {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var expanded []*Message
	for _, msg := range s.Messages {
		if msg.IsCompressed && len(msg.SourceMessages) > 0 {
			// Expand compressed message to source messages
			expanded = append(expanded, msg.SourceMessages...)
		} else {
			expanded = append(expanded, msg)
		}
	}
	return expanded
}

// GetCompressedMessageCount returns the number of compressed messages
func (s *Session) GetCompressedMessageCount() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	count := 0
	for _, msg := range s.Messages {
		if msg.IsCompressed {
			count++
		}
	}
	return count
}

// backgroundPersistence handles async session persistence
func (m *Manager) backgroundPersistence() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case session := <-m.persistQueue:
			if session != nil {
				if err := m.saveSessionSync(session); err != nil {
					log.Printf("[WARN] Background persist failed for session %s: %v", session.ID, err)
				}
			}

		case <-ticker.C:
			// Periodic flush of pending sessions
			m.flushPendingSessions()
		}
	}
}

// flushPendingSessions ensures all pending sessions are persisted
func (m *Manager) flushPendingSessions() {
	m.mutex.RLock()
	var pendingSessions []*Session

	// Find sessions that need persistence
	for sessionID, session := range m.sessions {
		m.persistMutex.RLock()
		lastPersist, exists := m.lastPersist[sessionID]
		m.persistMutex.RUnlock()

		session.mutex.RLock()
		lastUpdate := session.Updated
		session.mutex.RUnlock()

		// Persist if never persisted or if updated since last persist
		if !exists || lastUpdate.After(lastPersist) {
			pendingSessions = append(pendingSessions, session)
		}
	}
	m.mutex.RUnlock()

	// Persist pending sessions
	for _, session := range pendingSessions {
		if err := m.saveSessionSync(session); err != nil {
			log.Printf("[WARN] Flush persist failed for session %s: %v", session.ID, err)
		}
	}

	if len(pendingSessions) > 0 {
		log.Printf("[INFO] Flushed %d pending sessions to disk", len(pendingSessions))
	}
}
