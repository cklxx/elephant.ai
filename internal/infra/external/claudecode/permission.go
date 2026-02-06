package claudecode

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
	id "alex/internal/shared/utils/id"
)

type permissionRelay struct {
	ctx          context.Context
	taskID       string
	agentType    string
	allowedTools map[string]struct{}
	inputCh      chan<- agent.InputRequest
	pending      *sync.Map
	socketPath   string
	listener     net.Listener
	logger       Logger
}

type permissionRelayRequest struct {
	TaskID    string         `json:"task_id"`
	RequestID string         `json:"request_id"`
	ToolName  string         `json:"tool_name"`
	Arguments map[string]any `json:"arguments,omitempty"`
	FilePaths []string       `json:"file_paths,omitempty"`
	Summary   string         `json:"summary,omitempty"`
}

type permissionRelayResponse struct {
	RequestID string `json:"request_id"`
	Approved  bool   `json:"approved"`
	OptionID  string `json:"option_id,omitempty"`
	Message   string `json:"message,omitempty"`
}

type Logger interface {
	Debug(format string, args ...any)
	Warn(format string, args ...any)
}

func newPermissionRelay(ctx context.Context, taskID string, agentType string, allowedTools []string, inputCh chan<- agent.InputRequest, pending *sync.Map, logger Logger) (*permissionRelay, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	socketDir := filepath.Join(os.TempDir(), "elephant-perm")
	if err := os.MkdirAll(socketDir, 0o755); err != nil {
		return nil, fmt.Errorf("create permission socket dir: %w", err)
	}
	socketPath := filepath.Join(socketDir, fmt.Sprintf("%s.sock", id.NewKSUID()))
	allowed := make(map[string]struct{}, len(allowedTools))
	for _, tool := range allowedTools {
		if trimmed := strings.TrimSpace(tool); trimmed != "" {
			allowed[trimmed] = struct{}{}
		}
	}
	return &permissionRelay{
		ctx:          ctx,
		taskID:       taskID,
		agentType:    agentType,
		allowedTools: allowed,
		inputCh:      inputCh,
		pending:      pending,
		socketPath:   socketPath,
		logger:       logger,
	}, nil
}

func (r *permissionRelay) Start() (string, func(), error) {
	listener, err := net.Listen("unix", r.socketPath)
	if err != nil {
		return "", nil, fmt.Errorf("listen on permission socket: %w", err)
	}
	r.listener = listener

	go r.acceptLoop()

	cleanup := func() {
		if r.listener != nil {
			_ = r.listener.Close()
		}
		_ = os.Remove(r.socketPath)
	}
	return r.socketPath, cleanup, nil
}

func (r *permissionRelay) acceptLoop() {
	for {
		conn, err := r.listener.Accept()
		if err != nil {
			select {
			case <-r.ctx.Done():
				return
			default:
				r.logger.Warn("permission relay accept error: %v", err)
				return
			}
		}
		go r.handleConn(conn)
	}
}

func (r *permissionRelay) handleConn(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Bytes()
		var req permissionRelayRequest
		if err := json.Unmarshal(line, &req); err != nil {
			r.logger.Warn("permission relay decode error: %v", err)
			continue
		}
		resp := r.handleRequest(req)
		payload, err := json.Marshal(resp)
		if err != nil {
			r.logger.Warn("permission relay marshal error: %v", err)
			continue
		}
		payload = append(payload, '\n')
		if _, err := conn.Write(payload); err != nil {
			r.logger.Warn("permission relay write error: %v", err)
			return
		}
	}
}

func (r *permissionRelay) handleRequest(req permissionRelayRequest) permissionRelayResponse {
	if req.RequestID == "" {
		req.RequestID = id.NewRequestID()
	}
	if r.isAutoApproved(req.ToolName) {
		return permissionRelayResponse{RequestID: req.RequestID, Approved: true}
	}

	input := agent.InputRequest{
		TaskID:    req.TaskID,
		AgentType: r.agentType,
		RequestID: req.RequestID,
		Type:      agent.InputRequestPermission,
		Summary:   reqSummary(req),
		ToolCall: &agent.InputToolCall{
			Name:      req.ToolName,
			Arguments: req.Arguments,
			FilePaths: req.FilePaths,
		},
	}
	responseCh := make(chan agent.InputResponse, 1)
	key := requestKey(req.TaskID, req.RequestID)
	r.pending.Store(key, responseCh)
	defer r.pending.Delete(key)

	select {
	case r.inputCh <- input:
	case <-r.ctx.Done():
		return permissionRelayResponse{RequestID: req.RequestID, Approved: false, Message: "cancelled"}
	}

	select {
	case resp := <-responseCh:
		return permissionRelayResponse{
			RequestID: req.RequestID,
			Approved:  resp.Approved,
			OptionID:  resp.OptionID,
			Message:   resp.Text,
		}
	case <-time.After(30 * time.Minute):
		return permissionRelayResponse{RequestID: req.RequestID, Approved: false, Message: "timeout"}
	case <-r.ctx.Done():
		return permissionRelayResponse{RequestID: req.RequestID, Approved: false, Message: "cancelled"}
	}
}

func (r *permissionRelay) isAutoApproved(toolName string) bool {
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		return false
	}
	if _, ok := r.allowedTools[toolName]; ok {
		return true
	}
	// Support glob-style entries like "Bash(git *)"
	for allowed := range r.allowedTools {
		if strings.Contains(allowed, "*") {
			prefix := strings.TrimSuffix(allowed, "*")
			if strings.HasPrefix(toolName, prefix) {
				return true
			}
		}
	}
	return false
}

func reqSummary(req permissionRelayRequest) string {
	if req.Summary != "" {
		return req.Summary
	}
	if req.ToolName != "" {
		return fmt.Sprintf("Permission to execute: %s", req.ToolName)
	}
	return "Permission requested"
}

func requestKey(taskID, requestID string) string {
	return fmt.Sprintf("%s:%s", strings.TrimSpace(taskID), strings.TrimSpace(requestID))
}
