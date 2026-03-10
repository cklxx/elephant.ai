package bridge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
)

type runtimeEventSink struct {
	roleLogPath  string
	eventLogPath string
	teamID       string
	roleID       string
	taskID       string
}

func newRuntimeEventSink(req agent.ExternalAgentRequest) *runtimeEventSink {
	var roleLogPath string
	var eventLogPath string
	var teamID string
	var roleID string
	if req.Config != nil {
		roleLogPath = strings.TrimSpace(req.Config["role_log_path"])
		eventLogPath = strings.TrimSpace(req.Config["team_event_log"])
		teamID = strings.TrimSpace(req.Config["team_id"])
		roleID = strings.TrimSpace(req.Config["role_id"])
	}
	return &runtimeEventSink{
		roleLogPath:  roleLogPath,
		eventLogPath: eventLogPath,
		teamID:       teamID,
		roleID:       roleID,
		taskID:       strings.TrimSpace(req.TaskID),
	}
}

func (s *runtimeEventSink) record(eventType string, fields map[string]any) {
	if s == nil {
		return
	}
	payload := map[string]any{
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"type":      eventType,
		"task_id":   s.taskID,
	}
	if s.teamID != "" {
		payload["team_id"] = s.teamID
	}
	if s.roleID != "" {
		payload["role_id"] = s.roleID
	}
	for k, v := range fields {
		payload[k] = v
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	if s.eventLogPath != "" {
		appendLogLine(s.eventLogPath, string(data))
	}
	if s.roleLogPath != "" {
		appendLogLine(s.roleLogPath, string(data))
	}
}

func (s *runtimeEventSink) recordFromSDK(ev SDKEvent) {
	switch ev.Type {
	case SDKEventTool:
		s.record("tool_call", map[string]any{
			"tool_name": ev.ToolName,
			"summary":   ev.Summary,
			"iter":      ev.Iter,
		})
	case SDKEventResult:
		s.record("result", map[string]any{
			"iters":    ev.Iters,
			"tokens":   ev.Tokens,
			"is_error": ev.IsError,
		})
	case SDKEventError:
		s.record("error", map[string]any{
			"message": ev.Message,
		})
	}
}

func appendLogLine(path string, line string) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(trimmedPath), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(trimmedPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(strings.TrimSpace(line) + "\n")
}
