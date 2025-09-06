package agent

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"alex/internal/config"
	"alex/internal/session"
)

// Agent represents the main agent with simplified architecture
type Agent struct {
	// Core components - injected dependencies
	llmClient      LLMClient
	toolExecutor   ToolExecutor
	sessionManager SessionManager
	reactEngine    ReactEngine
	
	// Configuration
	config *config.Config
	
	// State management
	currentSession *session.Session
	mu             sync.RWMutex
	
	// Message queue for handling multiple requests
	messageQueue *MessageQueue
}

// AgentConfig holds configuration for creating an Agent
type AgentConfig struct {
	LLMClient      LLMClient
	ToolExecutor   ToolExecutor
	SessionManager SessionManager
	ReactEngine    ReactEngine
	Config         *config.Config
}

// NewAgent creates a new Agent instance with dependency injection
func NewAgent(cfg AgentConfig) (*Agent, error) {
	if cfg.LLMClient == nil {
		return nil, fmt.Errorf("LLMClient is required")
	}
	if cfg.ToolExecutor == nil {
		return nil, fmt.Errorf("ToolExecutor is required")
	}
	if cfg.SessionManager == nil {
		return nil, fmt.Errorf("SessionManager is required")
	}
	if cfg.ReactEngine == nil {
		return nil, fmt.Errorf("ReactEngine is required")
	}

	return &Agent{
		llmClient:      cfg.LLMClient,
		toolExecutor:   cfg.ToolExecutor,
		sessionManager: cfg.SessionManager,
		reactEngine:    cfg.ReactEngine,
		config:         cfg.Config,
		messageQueue:   NewMessageQueue(),
	}, nil
}

// ProcessMessage processes a user message with streaming support
func (a *Agent) ProcessMessage(ctx context.Context, userMessage string, callback StreamCallback) error {
	a.mu.RLock()
	currentSession := a.currentSession
	a.mu.RUnlock()

	// If no active session, create one automatically
	if currentSession == nil {
		sessionID := fmt.Sprintf("auto_%d", time.Now().UnixNano())
		newSession, err := a.sessionManager.StartSession(sessionID)
		if err != nil {
			return fmt.Errorf("failed to create session automatically: %w", err)
		}
		
		a.mu.Lock()
		a.currentSession = newSession
		a.mu.Unlock()
		
		log.Printf("Auto-created session: %s", sessionID)
	}

	// Process the message using the ReAct engine
	result, err := a.reactEngine.ProcessTask(ctx, userMessage, callback)
	if err != nil {
		return fmt.Errorf("message processing failed: %w", err)
	}

	// Send completion signal
	if callback != nil {
		callback(StreamChunk{
			Type:             "complete",
			Content:          "Task completed",
			Complete:         true,
			TotalTokensUsed:  result.TokensUsed,
			PromptTokens:     result.PromptTokens,
			CompletionTokens: result.CompletionTokens,
		})
	}

	// Process any queued messages
	return a.processQueuedMessages(ctx, callback)
}

// StartSession starts a new session
func (a *Agent) StartSession(sessionID string) (*session.Session, error) {
	sess, err := a.sessionManager.StartSession(sessionID)
	if err != nil {
		return nil, err
	}

	a.mu.Lock()
	a.currentSession = sess
	a.mu.Unlock()

	return sess, nil
}

// RestoreSession restores an existing session
func (a *Agent) RestoreSession(sessionID string) (*session.Session, error) {
	sess, err := a.sessionManager.RestoreSession(sessionID)
	if err != nil {
		return nil, err
	}

	a.mu.Lock()
	a.currentSession = sess
	a.mu.Unlock()

	return sess, nil
}

// GetAvailableTools returns a list of available tools
func (a *Agent) GetAvailableTools(ctx context.Context) []string {
	return a.toolExecutor.ListTools(ctx)
}

// GetSessionID returns the current session ID
func (a *Agent) GetSessionID() (string, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.currentSession == nil {
		return "", fmt.Errorf("no active session")
	}
	return a.currentSession.ID, nil
}

// GetSessionHistory returns the current session's message history
func (a *Agent) GetSessionHistory() []*session.Message {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.currentSession == nil {
		return nil
	}
	return a.currentSession.Messages
}

// AddMessage adds a message to the processing queue
func (a *Agent) AddMessage(ctx context.Context, message string, config *config.Config, callback StreamCallback) {
	item := MessageQueueItem{
		Message:   message,
		Timestamp: time.Now(),
		Callback:  callback,
		Context:   ctx,
		Config:    config,
		Metadata: map[string]any{
			"queued_at": time.Now().Unix(),
		},
	}
	a.messageQueue.Enqueue(item)
}

// GetQueueSize returns the current message queue size
func (a *Agent) GetQueueSize() int {
	return a.messageQueue.Size()
}

// ClearMessageQueue clears all pending messages
func (a *Agent) ClearMessageQueue() {
	a.messageQueue.Clear()
}

// processQueuedMessages processes any messages in the queue
func (a *Agent) processQueuedMessages(ctx context.Context, callback StreamCallback) error {
	if !a.messageQueue.HasPendingMessages() {
		return nil
	}

	if pendingItem, hasItem := a.messageQueue.Dequeue(); hasItem {
		if callback != nil {
			callback(StreamChunk{
				Type:     "next_message_start",
				Content:  fmt.Sprintf("📬 Starting next message: %s", pendingItem.Message),
				Metadata: map[string]any{"phase": "queue_processing"},
			})
		}

		// Recursively process the next message
		return a.ProcessMessage(pendingItem.Context, pendingItem.Message, pendingItem.Callback)
	}

	return nil
}