package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"

	"alex/internal/infra/mcp"
	"alex/internal/shared/logging"
)

type permissionServer struct {
	socketPath string
	taskID     string
	logger     logging.Logger
}

func runMCPPermissionServer(args []string) error {
	fs := flag.NewFlagSet("mcp-permission-server", flag.ContinueOnError)
	socketPath := fs.String("sock", "", "Unix socket path for permission relay")
	taskID := fs.String("task-id", "", "Background task id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*socketPath) == "" {
		return fmt.Errorf("--sock is required")
	}
	server := &permissionServer{
		socketPath: *socketPath,
		taskID:     strings.TrimSpace(*taskID),
		logger:     logging.NewComponentLogger("MCPPermissionServer"),
	}
	return server.serve(context.Background(), os.Stdin, os.Stdout)
}

func (s *permissionServer) serve(ctx context.Context, in *os.File, out *os.File) error {
	scanner := bufio.NewScanner(in)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 2*1024*1024)
	encoder := json.NewEncoder(out)

	for scanner.Scan() {
		line := scanner.Bytes()
		req, err := mcp.UnmarshalRequest(line)
		if err != nil {
			continue
		}
		if req.IsNotification() {
			continue
		}
		resp := s.handleRequest(ctx, req)
		if err := encoder.Encode(resp); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func (s *permissionServer) handleRequest(ctx context.Context, req *mcp.Request) *mcp.Response {
	switch req.Method {
	case "initialize":
		return mcp.NewResponse(req.ID, map[string]any{
			"protocolVersion": mcp.MCPProtocolVersion,
			"serverInfo": map[string]any{
				"name":    "elephant-permission",
				"version": appVersion(),
			},
			"capabilities": map[string]any{
				"tools": map[string]any{
					"listChanged": false,
				},
			},
		})
	case "tools/list":
		return mcp.NewResponse(req.ID, map[string]any{
			"tools": []map[string]any{
				{
					"name":        "approve",
					"description": "Request approval for an external agent tool invocation.",
					"inputSchema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"tool_name":  map[string]any{"type": "string"},
							"arguments":  map[string]any{"type": "object"},
							"file_paths": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
							"summary":    map[string]any{"type": "string"},
							"request_id": map[string]any{"type": "string"},
						},
						"required": []string{"tool_name"},
					},
				},
			},
		})
	case "tools/call":
		params := req.Params
		name, _ := params["name"].(string)
		if name != "approve" {
			return mcp.NewErrorResponse(req.ID, mcp.InvalidParams, "unsupported tool", nil)
		}
		args, _ := params["arguments"].(map[string]any)
		reqPayload := map[string]any{
			"task_id":   s.taskID,
			"tool_name": stringValue(args["tool_name"]),
			"summary":   stringValue(args["summary"]),
		}
		if reqID := stringValue(args["request_id"]); reqID != "" {
			reqPayload["request_id"] = reqID
		}
		if arguments, ok := args["arguments"].(map[string]any); ok {
			reqPayload["arguments"] = arguments
		}
		if filePaths := parseStringSlice(args["file_paths"]); len(filePaths) > 0 {
			reqPayload["file_paths"] = filePaths
		}

		respPayload, err := s.forward(ctx, reqPayload)
		if err != nil {
			return mcp.NewErrorResponse(req.ID, mcp.InternalError, err.Error(), nil)
		}
		content := fmt.Sprintf("approved=%v message=%s", respPayload.Approved, respPayload.Message)
		return mcp.NewResponse(req.ID, map[string]any{
			"content": []map[string]any{{"type": "text", "text": content}},
			"isError": false,
		})
	default:
		return mcp.NewErrorResponse(req.ID, mcp.MethodNotFound, fmt.Sprintf("unknown method: %s", req.Method), nil)
	}
}

type relayResponse struct {
	RequestID string `json:"request_id"`
	Approved  bool   `json:"approved"`
	OptionID  string `json:"option_id,omitempty"`
	Message   string `json:"message,omitempty"`
}

func (s *permissionServer) forward(ctx context.Context, payload map[string]any) (*relayResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "unix", s.socketPath)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')
	if _, err := conn.Write(data); err != nil {
		return nil, err
	}
	reader := bufio.NewScanner(conn)
	if !reader.Scan() {
		return nil, fmt.Errorf("no response from permission relay")
	}
	var resp relayResponse
	if err := json.Unmarshal(reader.Bytes(), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func parseStringSlice(raw any) []string {
	values, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, item := range values {
		if str, ok := item.(string); ok && strings.TrimSpace(str) != "" {
			out = append(out, strings.TrimSpace(str))
		}
	}
	return out
}

func stringValue(raw any) string {
	if str, ok := raw.(string); ok {
		return strings.TrimSpace(str)
	}
	return ""
}
