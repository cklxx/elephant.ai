package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	appcontext "alex/internal/agent/app/context"
	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/agent/presets"
	"alex/internal/logging"
	"alex/internal/tools/builtin"

	"alex/internal/mcp"
)

type acpServer struct {
	container      *Container
	initialMessage string

	sessionsMu sync.Mutex
	sessions   map[string]*acpSession

	transportsMu sync.Mutex
	transports   map[string]rpcTransport

	cwdMu sync.Mutex

	logger logging.Logger
}

type acpSession struct {
	id       string
	cwd      string
	modeID   string
	clientID string

	cancelMu sync.Mutex
	cancel   context.CancelFunc

	promptMu sync.Mutex
	active   bool
}

type acpMode struct {
	ID          string
	Name        string
	Description string
}

var acpModes = []acpMode{
	{ID: string(presets.ToolPresetFull), Name: "Full Access", Description: "All tools available"},
	{ID: string(presets.ToolPresetReadOnly), Name: "Read-Only", Description: "No local writes or execution"},
	{ID: string(presets.ToolPresetSafe), Name: "Safe Mode", Description: "Excludes potentially dangerous tools"},
	{ID: string(presets.ToolPresetSandbox), Name: "Sandbox Mode", Description: "Disable local file/shell tools; sandbox_* tools are web-only"},
}

func newACPServer(container *Container, initialMessage string) *acpServer {
	return &acpServer{
		container:      container,
		initialMessage: strings.TrimSpace(initialMessage),
		sessions:       make(map[string]*acpSession),
		transports:     make(map[string]rpcTransport),
		logger:         logging.NewComponentLogger("ACPServer"),
	}
}

func (s *acpServer) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	if ctx == nil {
		ctx = context.Background()
	}
	rpc := newRPCConn(in, out)
	clientID := "stdio"
	s.registerTransport(clientID, rpc)
	defer s.removeTransport(clientID)

	for {
		payload, err := rpc.readMessage()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		payload = bytesTrimSpace(payload)
		if len(payload) == 0 {
			continue
		}

		req, resp, err := parseRPCPayload(payload)
		if err != nil {
			continue
		}

		if resp != nil {
			rpc.DeliverResponse(resp)
			continue
		}
		if req == nil {
			continue
		}

		if req.IsNotification() {
			go s.handleNotification(ctx, req, clientID)
			continue
		}
		go s.handleRequest(ctx, req, clientID)
	}
}

func (s *acpServer) handleRequest(ctx context.Context, req *mcp.Request, clientID string) {
	if req == nil {
		return
	}
	transport := s.getTransport(clientID)
	if transport == nil {
		return
	}
	resp := s.dispatch(ctx, req, clientID)
	if resp == nil {
		return
	}
	_ = transport.SendResponse(resp)
}

func (s *acpServer) handleNotification(_ context.Context, req *mcp.Request, _ string) {
	if req == nil {
		return
	}
	switch req.Method {
	case "session/cancel":
		_ = s.handleSessionCancel(req.Params)
	}
}

func (s *acpServer) dispatch(ctx context.Context, req *mcp.Request, clientID string) *mcp.Response {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "authenticate":
		return mcp.NewResponse(req.ID, map[string]any{})
	case "session/new":
		return s.handleSessionNew(ctx, req, clientID)
	case "session/load":
		return s.handleSessionLoad(ctx, req, clientID)
	case "session/prompt":
		return s.handleSessionPrompt(ctx, req)
	case "session/set_mode":
		return s.handleSessionSetMode(req)
	default:
		return mcp.NewErrorResponse(req.ID, mcp.MethodNotFound, fmt.Sprintf("unknown method: %s", req.Method), nil)
	}
}

func (s *acpServer) handleInitialize(req *mcp.Request) *mcp.Response {
	params := req.Params
	clientVersion, _ := intParam(params, "protocolVersion")
	_ = clientVersion

	resp := map[string]any{
		"protocolVersion": acpProtocolVersion,
		"agentInfo": map[string]any{
			"name":    "alex",
			"title":   "elephant.ai",
			"version": appVersion(),
		},
		"agentCapabilities": map[string]any{
			"loadSession": true,
			"promptCapabilities": map[string]any{
				"audio":           true,
				"image":           true,
				"embeddedContext": true,
			},
			"mcpCapabilities": map[string]any{
				"http": false,
				"sse":  false,
			},
			"sessionCapabilities": map[string]any{},
		},
		"authMethods": []any{},
	}

	return mcp.NewResponse(req.ID, resp)
}

func (s *acpServer) handleSessionNew(ctx context.Context, req *mcp.Request, clientID string) *mcp.Response {
	params := req.Params
	cwd := strings.TrimSpace(stringParam(params, "cwd"))
	if cwd == "" {
		return mcp.NewErrorResponse(req.ID, mcp.InvalidParams, "cwd is required", nil)
	}
	if !filepath.IsAbs(cwd) {
		return mcp.NewErrorResponse(req.ID, mcp.InvalidParams, "cwd must be an absolute path", nil)
	}

	mcpServers := sliceParam(params, "mcpServers")
	if mcpServers == nil {
		return mcp.NewErrorResponse(req.ID, mcp.InvalidParams, "mcpServers is required", nil)
	}

	session, err := s.container.SessionStore.Create(ctx)
	if err != nil {
		return mcp.NewErrorResponse(req.ID, mcp.InternalError, "failed to create session", err.Error())
	}

	if s.initialMessage != "" {
		session.Messages = append(session.Messages, ports.Message{
			Role:    "system",
			Content: s.initialMessage,
			Source:  ports.MessageSourceSystemPrompt,
		})
		if err := s.container.SessionStore.Save(ctx, session); err != nil {
			return mcp.NewErrorResponse(req.ID, mcp.InternalError, "failed to seed session message", err.Error())
		}
	}

	acpSession := &acpSession{
		id:       session.ID,
		cwd:      cwd,
		modeID:   string(presets.ToolPresetFull),
		clientID: clientID,
	}
	s.registerSession(acpSession)

	if err := s.configureMCPServers(ctx, acpSession, mcpServers); err != nil {
		return mcp.NewErrorResponse(req.ID, mcp.InternalError, "failed to configure MCP servers", err.Error())
	}

	resp := map[string]any{
		"sessionId": session.ID,
		"modes":     buildModeState(acpSession.modeID),
	}
	return mcp.NewResponse(req.ID, resp)
}

func (s *acpServer) handleSessionLoad(ctx context.Context, req *mcp.Request, clientID string) *mcp.Response {
	params := req.Params
	sessionID := strings.TrimSpace(stringParam(params, "sessionId"))
	if sessionID == "" {
		return mcp.NewErrorResponse(req.ID, mcp.InvalidParams, "sessionId is required", nil)
	}
	cwd := strings.TrimSpace(stringParam(params, "cwd"))
	if cwd == "" {
		return mcp.NewErrorResponse(req.ID, mcp.InvalidParams, "cwd is required", nil)
	}
	if !filepath.IsAbs(cwd) {
		return mcp.NewErrorResponse(req.ID, mcp.InvalidParams, "cwd must be an absolute path", nil)
	}

	mcpServers := sliceParam(params, "mcpServers")
	if mcpServers == nil {
		return mcp.NewErrorResponse(req.ID, mcp.InvalidParams, "mcpServers is required", nil)
	}

	if _, err := s.container.SessionStore.Get(ctx, sessionID); err != nil {
		return mcp.NewErrorResponse(req.ID, mcp.InvalidParams, "session not found", err.Error())
	}

	acpSession := s.ensureSession(sessionID, cwd)
	acpSession.clientID = clientID
	if err := s.configureMCPServers(ctx, acpSession, mcpServers); err != nil {
		return mcp.NewErrorResponse(req.ID, mcp.InternalError, "failed to configure MCP servers", err.Error())
	}

	resp := map[string]any{
		"modes": buildModeState(acpSession.modeID),
	}
	return mcp.NewResponse(req.ID, resp)
}

func (s *acpServer) handleSessionSetMode(req *mcp.Request) *mcp.Response {
	params := req.Params
	sessionID := strings.TrimSpace(stringParam(params, "sessionId"))
	if sessionID == "" {
		return mcp.NewErrorResponse(req.ID, mcp.InvalidParams, "sessionId is required", nil)
	}
	modeID := strings.TrimSpace(stringParam(params, "modeId"))
	if modeID == "" {
		return mcp.NewErrorResponse(req.ID, mcp.InvalidParams, "modeId is required", nil)
	}
	if !isValidMode(modeID) {
		return mcp.NewErrorResponse(req.ID, mcp.InvalidParams, "unsupported mode", nil)
	}

	session := s.getSession(sessionID)
	if session == nil {
		return mcp.NewErrorResponse(req.ID, mcp.InvalidParams, "session not found", nil)
	}

	session.modeID = modeID
	s.sendSessionUpdate(sessionID, map[string]any{
		"sessionUpdate": "current_mode_update",
		"currentModeId": modeID,
	})

	return mcp.NewResponse(req.ID, map[string]any{})
}

func (s *acpServer) handleSessionCancel(params map[string]any) error {
	sessionID := strings.TrimSpace(stringParam(params, "sessionId"))
	if sessionID == "" {
		return nil
	}
	session := s.getSession(sessionID)
	if session == nil {
		return nil
	}
	session.cancelMu.Lock()
	cancel := session.cancel
	session.cancelMu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

func (s *acpServer) handleSessionPrompt(ctx context.Context, req *mcp.Request) *mcp.Response {
	if ctx == nil {
		ctx = context.Background()
	}
	params := req.Params
	sessionID := strings.TrimSpace(stringParam(params, "sessionId"))
	if sessionID == "" {
		return mcp.NewErrorResponse(req.ID, mcp.InvalidParams, "sessionId is required", nil)
	}
	prompt := params["prompt"]

	session := s.getSession(sessionID)
	if session == nil {
		return mcp.NewErrorResponse(req.ID, mcp.InvalidParams, "session not found", nil)
	}

	parsed, err := parseACPPrompt(prompt)
	if err != nil {
		return mcp.NewErrorResponse(req.ID, mcp.InvalidParams, err.Error(), nil)
	}

	if !session.tryStartPrompt() {
		return mcp.NewErrorResponse(req.ID, mcp.InvalidRequest, "prompt already running", nil)
	}
	defer session.finishPrompt()

	promptCtx, cancel := context.WithCancel(ctx)
	session.setCancel(cancel)
	defer session.clearCancel()

	sendUserPromptUpdates(s, sessionID, parsed)

	listener := newACPEventListener(s, session)

	promptCtx = agent.WithOutputContext(promptCtx, &agent.OutputContext{Level: agent.LevelCore})
	promptCtx = builtin.WithToolSessionID(promptCtx, sessionID)
	promptCtx = builtin.WithApprover(promptCtx, newACPApprover(s, sessionID))
	promptCtx = builtin.WithAutoApprove(promptCtx, false)
	if session.cwd != "" {
		promptCtx = builtin.WithWorkingDir(promptCtx, session.cwd)
	}
	if len(parsed.Attachments) > 0 {
		promptCtx = appcontext.WithUserAttachments(promptCtx, parsed.Attachments)
	}
	if preset := toolPresetForMode(session.modeID); preset != "" {
		promptCtx = context.WithValue(promptCtx, appcontext.PresetContextKey{}, appcontext.PresetConfig{ToolPreset: preset})
	}

	var oldCwd string
	if session.cwd != "" {
		info, err := os.Stat(session.cwd)
		if err != nil || !info.IsDir() {
			session.cwd = ""
		}
	}
	switchedCwd := false
	if session.cwd != "" {
		s.cwdMu.Lock()
		current, err := os.Getwd()
		if err == nil {
			oldCwd = current
		}
		if err := os.Chdir(session.cwd); err != nil {
			s.cwdMu.Unlock()
			session.cwd = ""
		} else {
			switchedCwd = true
		}
	}

	result, execErr := s.container.AgentCoordinator.ExecuteTask(promptCtx, parsed.Text, sessionID, listener)

	if switchedCwd {
		if oldCwd != "" {
			_ = os.Chdir(oldCwd)
		}
		s.cwdMu.Unlock()
	}

	stopReason := mapStopReason(result, execErr)
	if execErr != nil && result == nil && stopReason == "" {
		return mcp.NewErrorResponse(req.ID, mcp.InternalError, "prompt execution failed", execErr.Error())
	}

	resp := map[string]any{
		"stopReason": stopReason,
	}
	return mcp.NewResponse(req.ID, resp)
}

func (s *acpServer) registerSession(session *acpSession) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	s.sessions[session.id] = session
}

func (s *acpServer) ensureSession(id string, cwd string) *acpSession {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	if session, ok := s.sessions[id]; ok {
		session.cwd = cwd
		if session.modeID == "" {
			session.modeID = string(presets.ToolPresetFull)
		}
		return session
	}
	session := &acpSession{
		id:     id,
		cwd:    cwd,
		modeID: string(presets.ToolPresetFull),
	}
	s.sessions[id] = session
	return session
}

func (s *acpServer) getSession(id string) *acpSession {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	return s.sessions[id]
}

func (s *acpServer) registerTransport(clientID string, transport rpcTransport) {
	if clientID == "" || transport == nil {
		return
	}
	s.transportsMu.Lock()
	defer s.transportsMu.Unlock()
	s.transports[clientID] = transport
}

func (s *acpServer) removeTransport(clientID string) {
	if clientID == "" {
		return
	}
	s.transportsMu.Lock()
	defer s.transportsMu.Unlock()
	delete(s.transports, clientID)
}

func (s *acpServer) getTransport(clientID string) rpcTransport {
	if clientID == "" {
		return nil
	}
	s.transportsMu.Lock()
	defer s.transportsMu.Unlock()
	return s.transports[clientID]
}

func (s *acpServer) transportForSession(sessionID string) rpcTransport {
	if sessionID == "" {
		return nil
	}
	session := s.getSession(sessionID)
	if session == nil {
		return nil
	}
	return s.getTransport(session.clientID)
}

func (s *acpServer) sendSessionUpdate(sessionID string, update map[string]any) {
	if sessionID == "" || update == nil {
		return
	}
	transport := s.transportForSession(sessionID)
	if transport == nil {
		if s.logger != nil {
			s.logger.Warn("ACP update dropped: no transport for session %s", sessionID)
		}
		return
	}
	if err := transport.Notify("session/update", map[string]any{
		"sessionId": sessionID,
		"update":    update,
	}); err != nil && s.logger != nil {
		s.logger.Warn("ACP update notify failed: %v", err)
	}
}

func (session *acpSession) setCancel(cancel context.CancelFunc) {
	session.cancelMu.Lock()
	defer session.cancelMu.Unlock()
	session.cancel = cancel
}

func (session *acpSession) clearCancel() {
	session.cancelMu.Lock()
	defer session.cancelMu.Unlock()
	session.cancel = nil
}

func (session *acpSession) tryStartPrompt() bool {
	session.promptMu.Lock()
	defer session.promptMu.Unlock()
	if session.active {
		return false
	}
	session.active = true
	return true
}

func (session *acpSession) finishPrompt() {
	session.promptMu.Lock()
	defer session.promptMu.Unlock()
	session.active = false
}

func buildModeState(current string) map[string]any {
	available := make([]map[string]any, 0, len(acpModes))
	for _, mode := range acpModes {
		entry := map[string]any{
			"id":   mode.ID,
			"name": mode.Name,
		}
		if mode.Description != "" {
			entry["description"] = mode.Description
		}
		available = append(available, entry)
	}
	if current == "" {
		current = string(presets.ToolPresetFull)
	}
	return map[string]any{
		"currentModeId":  current,
		"availableModes": available,
	}
}

func isValidMode(modeID string) bool {
	for _, mode := range acpModes {
		if mode.ID == modeID {
			return true
		}
	}
	return false
}

func toolPresetForMode(modeID string) string {
	if modeID == "" {
		return ""
	}
	if presets.IsValidToolPreset(modeID) {
		return modeID
	}
	return ""
}

func mapStopReason(result *agent.TaskResult, execErr error) string {
	if errors.Is(execErr, context.Canceled) {
		return "cancelled"
	}
	if result == nil {
		return ""
	}
	reason := strings.ToLower(strings.TrimSpace(result.StopReason))
	switch reason {
	case "cancelled", "canceled":
		return "cancelled"
	case "max_tokens":
		return "max_tokens"
	case "max_iterations", "max_turn_requests":
		return "max_turn_requests"
	case "refusal":
		return "refusal"
	default:
		return "end_turn"
	}
}

func (s *acpServer) configureMCPServers(_ context.Context, _ *acpSession, servers []any) error {
	if s.container.MCPRegistry == nil || s.container.AgentCoordinator == nil {
		return nil
	}
	for _, raw := range servers {
		spec, err := parseMCPServerSpec(raw)
		if err != nil {
			return err
		}
		if spec == nil {
			continue
		}
		adapters, err := s.container.MCPRegistry.StartServerWithConfig(spec.Name, mcp.ServerConfig{
			Command: spec.Command,
			Args:    spec.Args,
			Env:     spec.Env,
		})
		if err != nil {
			return err
		}
		if len(adapters) == 0 {
			continue
		}
		registry := s.container.AgentCoordinator.GetToolRegistry()
		for _, adapter := range adapters {
			if err := registry.Register(adapter); err != nil {
				if !strings.Contains(err.Error(), "tool already exists") {
					return err
				}
			}
		}
	}
	return nil
}

type acpMCPServerSpec struct {
	Name    string
	Command string
	Args    []string
	Env     map[string]string
}

func parseMCPServerSpec(raw any) (*acpMCPServerSpec, error) {
	item, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid mcpServers entry")
	}
	name := strings.TrimSpace(stringParam(item, "name"))
	if name == "" {
		return nil, fmt.Errorf("mcp server name is required")
	}

	serverType := strings.ToLower(strings.TrimSpace(stringParam(item, "type")))
	command := strings.TrimSpace(stringParam(item, "command"))
	if serverType == "" && command != "" {
		serverType = "stdio"
	}

	switch serverType {
	case "stdio", "":
		if command == "" {
			return nil, fmt.Errorf("mcp server command is required")
		}
		args := stringSlice(item["args"])
		env := map[string]string{}
		for _, rawEnv := range stringSliceEnv(item["env"]) {
			if rawEnv.name == "" {
				continue
			}
			env[rawEnv.name] = rawEnv.value
		}
		return &acpMCPServerSpec{Name: name, Command: command, Args: args, Env: env}, nil
	case "http", "sse":
		return nil, fmt.Errorf("mcp server type %s not supported", serverType)
	default:
		return nil, fmt.Errorf("unknown mcp server type %s", serverType)
	}
}

type acpEnvVar struct {
	name  string
	value string
}

func stringSliceEnv(value any) []acpEnvVar {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]acpEnvVar, 0, len(raw))
	for _, item := range raw {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(stringParam(entry, "name"))
		val := strings.TrimSpace(stringParam(entry, "value"))
		out = append(out, acpEnvVar{name: name, value: val})
	}
	return out
}
