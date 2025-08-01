package utils

import (
	"context"
	"time"

	"alex/internal/llm"
	agentsession "alex/internal/session"
)

// SessionHelper provides utility functions for session management
type SessionHelper struct {
	logger *ComponentLogger
}

// NewSessionHelper creates a new session helper
func NewSessionHelper(componentName string) *SessionHelper {
	return &SessionHelper{
		logger: Logger.GetLogger(componentName),
	}
}

// GetSessionWithFallback returns the provided session or falls back to the current session
func (sh *SessionHelper) GetSessionWithFallback(session *agentsession.Session, currentSession *agentsession.Session) *agentsession.Session {
	if session != nil {
		return session
	}

	if currentSession != nil {
		sh.logger.Debug("Using fallback current session")
		return currentSession
	}

	sh.logger.Debug("No session available")
	return nil
}

// ValidateSession checks if a session is valid and has the required properties
func (sh *SessionHelper) ValidateSession(session *agentsession.Session) bool {
	if session == nil {
		sh.logger.Debug("Session is nil")
		return false
	}

	if session.ID == "" {
		sh.logger.Debug("Session has empty ID")
		return false
	}

	return true
}

// AddMessageToSession adds an LLM message to a session with proper conversion
func (sh *SessionHelper) AddMessageToSession(llmMsg *llm.Message, session *agentsession.Session, fallbackSession *agentsession.Session) {
	targetSession := sh.GetSessionWithFallback(session, fallbackSession)
	if !sh.ValidateSession(targetSession) {
		sh.logger.Debug("Cannot add message to session: invalid session")
		return
	}

	// Convert LLM message to session message format
	sessionMsg := &agentsession.Message{
		Role:       llmMsg.Role,
		Content:    llmMsg.Content,
		ToolCallId: llmMsg.ToolCallId,
		Name:       llmMsg.Name,
		ToolCalls:  make([]llm.ToolCall, 0),
		Timestamp:  time.Now(),
		Metadata: map[string]interface{}{
			"source":    "llm_response",
			"timestamp": time.Now().Unix(),
		},
	}

	// Convert tool calls if present
	if len(llmMsg.ToolCalls) > 0 {
		for _, tc := range llmMsg.ToolCalls {
			sessionMsg.ToolCalls = append(sessionMsg.ToolCalls, llm.ToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: llm.Function{
					Name:        tc.Function.Name,
					Description: tc.Function.Description,
					Parameters:  tc.Function.Parameters,
					Arguments:   tc.Function.Arguments,
				},
			})
		}
		sessionMsg.Metadata["has_tool_calls"] = true
		sessionMsg.Metadata["tool_count"] = len(llmMsg.ToolCalls)
	}

	// Add to session
	targetSession.AddMessage(sessionMsg)
	sh.logger.Debug("Added message to session %s", targetSession.ID)
}

// AddToolResultToSession adds a tool result message to a session
func (sh *SessionHelper) AddToolResultToSession(toolResult string, toolName string, toolCallId string, session *agentsession.Session, fallbackSession *agentsession.Session) {
	targetSession := sh.GetSessionWithFallback(session, fallbackSession)
	if !sh.ValidateSession(targetSession) {
		sh.logger.Debug("Cannot add tool result to session: invalid session")
		return
	}

	sessionMsg := &agentsession.Message{
		Role:       "tool",
		Content:    toolResult,
		ToolCallId: toolCallId,
		Name:       toolName,
		Timestamp:  time.Now(),
		Metadata: map[string]interface{}{
			"source":    "tool_result",
			"timestamp": time.Now().Unix(),
		},
	}

	if toolCallId != "" {
		sessionMsg.Metadata["tool_call_id"] = toolCallId
	}
	if toolName != "" {
		sessionMsg.Metadata["tool_name"] = toolName
	}

	targetSession.AddMessage(sessionMsg)
	sh.logger.Debug("Added tool result to session %s", targetSession.ID)
}

// GetTodoFromSession reads TODO content from a session using a tool
func (sh *SessionHelper) GetTodoFromSession(ctx context.Context, session *agentsession.Session, fallbackSession *agentsession.Session, todoTool interface{}) string {
	targetSession := sh.GetSessionWithFallback(session, fallbackSession)
	if !sh.ValidateSession(targetSession) {
		sh.logger.Debug("Cannot read todos: invalid session")
		return ""
	}

	sessionID := targetSession.ID

	// This is a simplified interface - in actual implementation, you'd need to call the tool
	// Here we provide the structure for how it should work
	sh.logger.Debug("Reading todos for session %s", sessionID)

	// The actual tool call would happen here
	// For now, return empty string as placeholder
	return ""
}

// SessionManager provides high-level session operations
type SessionManager struct {
	helper *SessionHelper
}

// NewSessionManager creates a new session manager
func NewSessionManager(componentName string) *SessionManager {
	return &SessionManager{
		helper: NewSessionHelper(componentName),
	}
}

// ProcessLLMMessages processes a slice of LLM messages and adds them to the appropriate session
func (sm *SessionManager) ProcessLLMMessages(messages []llm.Message, session *agentsession.Session, fallbackSession *agentsession.Session) {
	for _, msg := range messages {
		// Skip system messages and messages that are already in the session
		if msg.Role == "system" {
			continue
		}

		switch msg.Role {
		case "assistant":
			sm.helper.AddMessageToSession(&msg, session, fallbackSession)
		case "tool":
			sm.helper.AddToolResultToSession(msg.Content, msg.Name, msg.ToolCallId, session, fallbackSession)
		}
	}
}

// Global session helper instances for common components
var (
	CoreSessionHelper     *SessionHelper
	ReactSessionHelper    *SessionHelper
	SubAgentSessionHelper *SessionHelper
)

func init() {
	CoreSessionHelper = NewSessionHelper("REACT-CORE")
	ReactSessionHelper = NewSessionHelper("REACT-AGENT")
	SubAgentSessionHelper = NewSessionHelper("SUB-AGENT")
}
