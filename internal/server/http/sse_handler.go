package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/server/app"
	"alex/internal/utils"
)

// SSEHandler handles Server-Sent Events connections
type SSEHandler struct {
	broadcaster *app.EventBroadcaster
	logger      *utils.Logger
}

// NewSSEHandler creates a new SSE handler
func NewSSEHandler(broadcaster *app.EventBroadcaster) *SSEHandler {
	return &SSEHandler{
		broadcaster: broadcaster,
		logger:      utils.NewComponentLogger("SSEHandler"),
	}
}

// HandleSSEStream handles SSE connection for real-time event streaming
func (h *SSEHandler) HandleSSEStream(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers (CORS headers are handled by middleware)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	// Get session ID from query parameter
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		http.Error(w, "session_id required", http.StatusBadRequest)
		return
	}

	h.logger.Info("SSE connection established for session: %s", sessionID)

	// Create event channel for this client
	clientChan := make(chan ports.AgentEvent, 100)

	// Register client with broadcaster
	h.broadcaster.RegisterClient(sessionID, clientChan)
	defer h.broadcaster.UnregisterClient(sessionID, clientChan)

	// Get flusher for streaming
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Send initial connection message
	if _, err := fmt.Fprintf(w, "event: connected\ndata: {\"session_id\":\"%s\"}\n\n", sessionID); err != nil {
		h.logger.Error("Failed to send connection message: %v", err)
		return
	}
	flusher.Flush()

	// Heartbeat ticker to keep connection alive
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Stream events until client disconnects or context is cancelled
	for {
		select {
		case event := <-clientChan:
			// Serialize event to JSON
			data, err := h.serializeEvent(event)
			if err != nil {
				h.logger.Error("Failed to serialize event: %v", err)
				continue
			}

			// Send SSE message
			// Format: event: <event_type>\ndata: <json>\n\n
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.EventType(), data); err != nil {
				h.logger.Error("Failed to send SSE message: %v", err)
				continue
			}
			flusher.Flush()

		case <-ticker.C:
			// Send heartbeat to keep connection alive
			if _, err := fmt.Fprintf(w, ": heartbeat\n\n"); err != nil {
				h.logger.Error("Failed to send heartbeat: %v", err)
				return
			}
			flusher.Flush()

		case <-r.Context().Done():
			// Client disconnected
			h.logger.Info("SSE connection closed for session: %s", sessionID)
			return
		}
	}
}

// serializeEvent converts domain event to JSON
func (h *SSEHandler) serializeEvent(event ports.AgentEvent) (string, error) {
	// Create a map with common fields
	data := map[string]interface{}{
		"event_type":  event.EventType(),
		"timestamp":   event.Timestamp().Format(time.RFC3339),
		"agent_level": event.GetAgentLevel(),
		"session_id":  event.GetSessionID(),
	}

	// Add event-specific fields based on type
	switch e := event.(type) {
	case *domain.TaskAnalysisEvent:
		data["action_name"] = e.ActionName
		data["goal"] = e.Goal

	case *domain.IterationStartEvent:
		data["iteration"] = e.Iteration
		data["total_iters"] = e.TotalIters

	case *domain.ThinkingEvent:
		data["iteration"] = e.Iteration
		data["message_count"] = e.MessageCount

	case *domain.ThinkCompleteEvent:
		data["iteration"] = e.Iteration
		data["content"] = e.Content
		data["tool_call_count"] = e.ToolCallCount

	case *domain.ToolCallStartEvent:
		data["iteration"] = e.Iteration
		data["call_id"] = e.CallID
		data["tool_name"] = e.ToolName
		data["arguments"] = e.Arguments

	case *domain.ToolCallCompleteEvent:
		data["call_id"] = e.CallID
		data["tool_name"] = e.ToolName
		data["result"] = e.Result
		if e.Error != nil {
			data["error"] = e.Error.Error()
		}
		data["duration"] = e.Duration.Milliseconds()

	case *domain.ToolCallStreamEvent:
		data["call_id"] = e.CallID
		data["chunk"] = e.Chunk
		data["is_complete"] = e.IsComplete

	case *domain.IterationCompleteEvent:
		data["iteration"] = e.Iteration
		data["tokens_used"] = e.TokensUsed
		data["tools_run"] = e.ToolsRun

	case *domain.TaskCompleteEvent:
		data["final_answer"] = e.FinalAnswer
		data["total_iterations"] = e.TotalIterations
		data["total_tokens"] = e.TotalTokens
		data["stop_reason"] = e.StopReason
		data["duration"] = e.Duration.Milliseconds()

	case *domain.ErrorEvent:
		data["iteration"] = e.Iteration
		data["phase"] = e.Phase
		if e.Error != nil {
			data["error"] = e.Error.Error()
		}
		data["recoverable"] = e.Recoverable
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}
