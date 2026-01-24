package http

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	agent_eval "alex/evaluation/agent_eval"
	"alex/evaluation/swe_bench"
	agentapp "alex/internal/agent/app"
	agentports "alex/internal/agent/ports"
	"alex/internal/agent/types"
	runtimeconfig "alex/internal/config"
	"alex/internal/logging"
	"alex/internal/observability"
	"alex/internal/sandbox"
	"alex/internal/server/app"
	serverPorts "alex/internal/server/ports"
	"alex/internal/subscription"
	id "alex/internal/utils/id"
)

const (
	defaultMaxCreateTaskBodySize int64 = 20 << 20 // 20 MiB
	maxSessionListLimit                = 200
	maxSnapshotListLimit               = 200
	maxTaskListLimit                   = 200
	maxEvaluationListLimit             = 200
)

// APIHandler handles REST API endpoints
type APIHandler struct {
	coordinator           *app.ServerCoordinator
	healthChecker         *app.HealthCheckerImpl
	logger                logging.Logger
	internalMode          bool
	devMode               bool
	obs                   *observability.Observability
	evaluationSvc         *app.EvaluationService
	attachmentStore       *AttachmentStore
	sandboxClient         *sandbox.Client
	maxCreateTaskBodySize int64
	selectionResolver     *subscription.SelectionResolver
}

// APIHandlerOption configures API handler behavior.
type APIHandlerOption func(*APIHandler)

// WithAPIObservability wires observability components into the handler.
func WithAPIObservability(obs *observability.Observability) APIHandlerOption {
	return func(handler *APIHandler) {
		handler.obs = obs
	}
}

// WithEvaluationService wires evaluation service for web-triggered runs.
func WithEvaluationService(service *app.EvaluationService) APIHandlerOption {
	return func(handler *APIHandler) {
		handler.evaluationSvc = service
	}
}

// WithAttachmentStore wires an attachment store used to persist client-provided payloads
// and expose them as URL-backed attachments.
func WithAttachmentStore(store *AttachmentStore) APIHandlerOption {
	return func(handler *APIHandler) {
		handler.attachmentStore = store
	}
}

// WithSandboxClient wires a sandbox client for sandbox-related endpoints.
func WithSandboxClient(client *sandbox.Client) APIHandlerOption {
	return func(handler *APIHandler) {
		handler.sandboxClient = client
	}
}

// WithSelectionResolver wires a subscription selection resolver for per-request overrides.
func WithSelectionResolver(resolver *subscription.SelectionResolver) APIHandlerOption {
	return func(handler *APIHandler) {
		handler.selectionResolver = resolver
	}
}

// WithMaxCreateTaskBodySize overrides the maximum accepted body size for CreateTask requests.
func WithMaxCreateTaskBodySize(limit int64) APIHandlerOption {
	return func(handler *APIHandler) {
		if limit > 0 {
			handler.maxCreateTaskBodySize = limit
		}
	}
}

// WithDevMode enables development-only endpoints.
func WithDevMode(enabled bool) APIHandlerOption {
	return func(handler *APIHandler) {
		handler.devMode = enabled
	}
}

// NewAPIHandler creates a new API handler
func NewAPIHandler(coordinator *app.ServerCoordinator, healthChecker *app.HealthCheckerImpl, internalMode bool, opts ...APIHandlerOption) *APIHandler {
	handler := &APIHandler{
		coordinator:           coordinator,
		healthChecker:         healthChecker,
		logger:                logging.NewComponentLogger("APIHandler"),
		internalMode:          internalMode,
		maxCreateTaskBodySize: defaultMaxCreateTaskBodySize,
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(handler)
	}
	if handler.maxCreateTaskBodySize <= 0 {
		handler.maxCreateTaskBodySize = defaultMaxCreateTaskBodySize
	}
	if handler.selectionResolver == nil {
		handler.selectionResolver = subscription.NewSelectionResolver(func() runtimeconfig.CLICredentials {
			return runtimeconfig.LoadCLICredentials()
		})
	}
	return handler
}

// CreateTaskRequest matches TypeScript CreateTaskRequest interface
type CreateTaskRequest struct {
	Task         string                  `json:"task"`
	SessionID    string                  `json:"session_id,omitempty"`
	AgentPreset  string                  `json:"agent_preset,omitempty"` // Agent persona preset
	ToolPreset   string                  `json:"tool_preset,omitempty"`  // Tool access preset
	Attachments  []AttachmentPayload     `json:"attachments,omitempty"`
	LLMSelection *subscription.Selection `json:"llm_selection,omitempty"`
}

// CreateTaskResponse matches TypeScript CreateTaskResponse interface
type CreateTaskResponse struct {
	TaskID       string `json:"task_id"`
	SessionID    string `json:"session_id"`
	Status       string `json:"status"`
	ParentTaskID string `json:"parent_task_id,omitempty"`
}

type apiErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

// AttachmentPayload represents an attachment sent from the client.
type AttachmentPayload struct {
	Name                string `json:"name"`
	MediaType           string `json:"media_type"`
	Data                string `json:"data,omitempty"`
	URI                 string `json:"uri,omitempty"`
	Source              string `json:"source,omitempty"`
	Description         string `json:"description,omitempty"`
	Kind                string `json:"kind,omitempty"`
	Format              string `json:"format,omitempty"`
	RetentionTTLSeconds uint64 `json:"retention_ttl_seconds,omitempty"`
}

type ContextSnapshotItem struct {
	RequestID        string               `json:"request_id"`
	Iteration        int                  `json:"iteration"`
	Timestamp        string               `json:"timestamp"`
	TaskID           string               `json:"task_id,omitempty"`
	ParentTaskID     string               `json:"parent_task_id,omitempty"`
	Messages         []agentports.Message `json:"messages"`
	ExcludedMessages []agentports.Message `json:"excluded_messages,omitempty"`
}

type ContextSnapshotResponse struct {
	SessionID string                `json:"session_id"`
	Snapshots []ContextSnapshotItem `json:"snapshots"`
}

type ContextWindowPreviewResponse struct {
	SessionID     string                   `json:"session_id"`
	TokenEstimate int                      `json:"token_estimate"`
	TokenLimit    int                      `json:"token_limit"`
	PersonaKey    string                   `json:"persona_key,omitempty"`
	ToolMode      string                   `json:"tool_mode,omitempty"`
	ToolPreset    string                   `json:"tool_preset,omitempty"`
	Window        agentports.ContextWindow `json:"window"`
}

type SessionSnapshotItem struct {
	TurnID     int    `json:"turn_id"`
	LLMTurnSeq int    `json:"llm_turn_seq"`
	Summary    string `json:"summary"`
	CreatedAt  string `json:"created_at"`
}

type SessionSnapshotsResponse struct {
	SessionID  string                `json:"session_id"`
	Items      []SessionSnapshotItem `json:"items"`
	NextCursor string                `json:"next_cursor,omitempty"`
}

type SessionPersonaRequest struct {
	UserPersona *agentports.UserPersonaProfile `json:"user_persona"`
}

type SessionPersonaResponse struct {
	SessionID   string                         `json:"session_id"`
	UserPersona *agentports.UserPersonaProfile `json:"user_persona,omitempty"`
}

type TurnSnapshotResponse struct {
	SessionID  string                      `json:"session_id"`
	TurnID     int                         `json:"turn_id"`
	LLMTurnSeq int                         `json:"llm_turn_seq"`
	Summary    string                      `json:"summary"`
	CreatedAt  string                      `json:"created_at"`
	Plans      []agentports.PlanNode       `json:"plans,omitempty"`
	Beliefs    []agentports.Belief         `json:"beliefs,omitempty"`
	WorldState map[string]any              `json:"world_state,omitempty"`
	Diff       map[string]any              `json:"diff,omitempty"`
	Messages   []agentports.Message        `json:"messages"`
	Feedback   []agentports.FeedbackSignal `json:"feedback,omitempty"`
}

type webVitalPayload struct {
	Name           string  `json:"name"`
	Value          float64 `json:"value"`
	Delta          float64 `json:"delta,omitempty"`
	ID             string  `json:"id,omitempty"`
	Label          string  `json:"label,omitempty"`
	Page           string  `json:"page,omitempty"`
	NavigationType string  `json:"navigation_type,omitempty"`
	Timestamp      int64   `json:"ts,omitempty"`
}

const maxWebVitalBodySize = 1 << 14
const maxEvaluationBodySize = 1 << 18

type startEvaluationRequest struct {
	DatasetPath   string `json:"dataset_path"`
	InstanceLimit int    `json:"instance_limit"`
	MaxWorkers    int    `json:"max_workers"`
	TimeoutSec    int64  `json:"timeout_seconds"`
	OutputDir     string `json:"output_dir"`
	ReportFormat  string `json:"report_format"`
	EnableMetrics *bool  `json:"enable_metrics,omitempty"`
	AgentID       string `json:"agent_id,omitempty"`
}

type evaluationJobResponse struct {
	ID            string                        `json:"id"`
	Status        string                        `json:"status"`
	Error         string                        `json:"error,omitempty"`
	AgentID       string                        `json:"agent_id,omitempty"`
	DatasetPath   string                        `json:"dataset_path,omitempty"`
	InstanceLimit int                           `json:"instance_limit,omitempty"`
	MaxWorkers    int                           `json:"max_workers,omitempty"`
	TimeoutSec    int64                         `json:"timeout_seconds,omitempty"`
	StartedAt     string                        `json:"started_at,omitempty"`
	CompletedAt   string                        `json:"completed_at,omitempty"`
	Summary       *agent_eval.AnalysisSummary   `json:"summary,omitempty"`
	Metrics       *agent_eval.EvaluationMetrics `json:"metrics,omitempty"`
	Agent         *agent_eval.AgentProfile      `json:"agent,omitempty"`
}

type evaluationDetailResponse struct {
	Evaluation evaluationJobResponse      `json:"evaluation"`
	Analysis   *agent_eval.AnalysisResult `json:"analysis,omitempty"`
	Results    []workerResultSummary      `json:"results,omitempty"`
	Agent      *agent_eval.AgentProfile   `json:"agent,omitempty"`
}

type agentListResponse struct {
	Agents []*agent_eval.AgentProfile `json:"agents"`
}

type agentHistoryResponse struct {
	Agent       *agent_eval.AgentProfile `json:"agent,omitempty"`
	Evaluations []evaluationJobResponse  `json:"evaluations"`
}

type workerResultSummary struct {
	TaskID          string  `json:"task_id"`
	InstanceID      string  `json:"instance_id"`
	Status          string  `json:"status"`
	DurationSeconds float64 `json:"duration_seconds,omitempty"`
	TokensUsed      int     `json:"tokens_used,omitempty"`
	Cost            float64 `json:"cost,omitempty"`
	AutoScore       float64 `json:"auto_score,omitempty"`
	Grade           string  `json:"grade,omitempty"`
	Error           string  `json:"error,omitempty"`
	FilesChanged    int     `json:"files_changed,omitempty"`
	ToolTraces      int     `json:"tool_traces,omitempty"`
	StartedAt       string  `json:"started_at,omitempty"`
	CompletedAt     string  `json:"completed_at,omitempty"`
}

// HandleCreateTask handles POST /api/tasks - creates and executes a new task
func (h *APIHandler) HandleCreateTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	// Limit request body size to avoid resource exhaustion attacks
	body := http.MaxBytesReader(w, r.Body, h.maxCreateTaskBodySize)
	defer func() {
		_ = body.Close()
	}()

	// Parse request body
	var req CreateTaskRequest

	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		var syntaxErr *json.SyntaxError
		var typeErr *json.UnmarshalTypeError
		var maxBytesErr *http.MaxBytesError
		switch {
		case errors.Is(err, io.EOF):
			h.writeJSONError(w, http.StatusBadRequest, "Request body is empty", err)
			return
		case errors.As(err, &syntaxErr):
			h.writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Invalid JSON at position %d", syntaxErr.Offset), err)
			return
		case errors.As(err, &typeErr):
			h.writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Invalid value for field '%s'", typeErr.Field), err)
			return
		case errors.As(err, &maxBytesErr):
			h.writeJSONError(w, http.StatusRequestEntityTooLarge, "Request body too large", err)
			return
		default:
			h.writeJSONError(w, http.StatusBadRequest, "Invalid request body", err)
			return
		}
	}

	// Ensure there are no extra JSON tokens
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		h.writeJSONError(w, http.StatusBadRequest, "Request body must contain a single JSON object", fmt.Errorf("unexpected extra JSON token"))
		return
	}

	if req.Task == "" {
		h.writeJSONError(w, http.StatusBadRequest, "Task is required", fmt.Errorf("task field empty"))
		return
	}

	sessionID, err := isValidOptionalSessionID(req.SessionID)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
		return
	}
	req.SessionID = sessionID

	attachments, err := h.parseAttachments(req.Attachments)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	h.logger.Info("Creating task: task='%s', sessionID='%s'", req.Task, req.SessionID)

	ctx := id.WithSessionID(r.Context(), req.SessionID)
	if user, ok := CurrentUser(r.Context()); ok {
		ctx = id.WithUserID(ctx, user.ID)
	}
	if len(attachments) > 0 {
		ctx = agentapp.WithUserAttachments(ctx, attachments)
	}
	if req.LLMSelection != nil && h.selectionResolver != nil {
		if resolved, ok := h.selectionResolver.Resolve(*req.LLMSelection); ok {
			ctx = agentapp.WithLLMSelection(ctx, resolved)
		}
	}

	// Execute task asynchronously - coordinator returns immediately after creating task record
	// Background goroutine will handle actual execution and update status
	task, err := h.coordinator.ExecuteTaskAsync(ctx, req.Task, req.SessionID, req.AgentPreset, req.ToolPreset)
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to create task", err)
		return
	}

	h.logger.Info("Task created successfully: taskID=%s, sessionID=%s", task.ID, task.SessionID)

	// Return task response matching TypeScript interface
	response := CreateTaskResponse{
		TaskID:       task.ID,
		SessionID:    task.SessionID,
		Status:       string(task.Status),
		ParentTaskID: task.ParentTaskID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response", err)
	}
}

// HandleWebVitals ingests frontend performance signals.
func (h *APIHandler) HandleWebVitals(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body := http.MaxBytesReader(w, r.Body, maxWebVitalBodySize)
	defer func() {
		_ = body.Close()
	}()
	var payload webVitalPayload
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}
	if payload.Name == "" {
		h.writeJSONError(w, http.StatusBadRequest, "name is required", fmt.Errorf("name missing"))
		return
	}
	page := canonicalPath(payload.Page)
	if h.obs != nil {
		h.obs.Metrics.RecordWebVital(r.Context(), payload.Name, payload.Label, page, payload.Value, payload.Delta)
	}
	w.WriteHeader(http.StatusAccepted)
}

// HandleSandboxBrowserInfo proxies sandbox browser info for the web console.
func (h *APIHandler) HandleSandboxBrowserInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.sandboxClient == nil {
		h.writeJSONError(w, http.StatusServiceUnavailable, "Sandbox not configured", nil)
		return
	}

	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	var response sandbox.Response[sandbox.BrowserInfo]
	if err := h.sandboxClient.DoJSON(r.Context(), http.MethodGet, "/v1/browser/info", nil, sessionID, &response); err != nil {
		h.writeJSONError(w, http.StatusBadGateway, "Sandbox request failed", err)
		return
	}
	if !response.Success {
		h.writeJSONError(w, http.StatusBadGateway, "Sandbox browser info failed", errors.New(response.Message))
		return
	}
	if response.Data == nil {
		h.writeJSONError(w, http.StatusBadGateway, "Sandbox browser info empty", nil)
		return
	}

	h.writeJSON(w, http.StatusOK, response.Data)
}

// HandleSandboxBrowserScreenshot proxies sandbox browser screenshots for the web console.
func (h *APIHandler) HandleSandboxBrowserScreenshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.sandboxClient == nil {
		h.writeJSONError(w, http.StatusServiceUnavailable, "Sandbox not configured", nil)
		return
	}

	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	payload, err := h.sandboxClient.GetBytes(r.Context(), "/v1/browser/screenshot", sessionID)
	if err != nil {
		h.writeJSONError(w, http.StatusBadGateway, "Sandbox screenshot failed", err)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "image/png")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

func (h *APIHandler) parseAttachments(payloads []AttachmentPayload) ([]agentports.Attachment, error) {
	if len(payloads) == 0 {
		return nil, nil
	}

	attachments := make([]agentports.Attachment, 0, len(payloads))
	for _, incoming := range payloads {
		name := strings.TrimSpace(incoming.Name)
		if name == "" {
			return nil, fmt.Errorf("attachment name is required")
		}

		mediaType := strings.TrimSpace(incoming.MediaType)
		if mediaType == "" {
			return nil, fmt.Errorf("attachment media_type is required")
		}
		isImage := strings.HasPrefix(strings.ToLower(mediaType), "image/")

		uri := strings.TrimSpace(incoming.URI)
		data := strings.TrimSpace(incoming.Data)
		if uri == "" && data == "" {
			return nil, fmt.Errorf("attachment '%s' must include data or uri", name)
		}

		var inlineBase64 string
		lowerURI := strings.ToLower(strings.TrimSpace(uri))
		if strings.HasPrefix(lowerURI, "data:") {
			data = uri
			uri = ""
		}

		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(data)), "data:") {
			_, decoded, ok := decodeDataURI(data)
			if !ok {
				return nil, fmt.Errorf("attachment '%s' includes invalid data uri", name)
			}
			if h.attachmentStore == nil {
				return nil, fmt.Errorf("attachment '%s' must include uri (base64 uploads are disabled)", name)
			}
			storedURI, err := h.attachmentStore.StoreBytes(name, mediaType, decoded)
			if err != nil {
				return nil, fmt.Errorf("store attachment '%s': %w", name, err)
			}
			uri = storedURI
			if isImage {
				inlineBase64 = base64.StdEncoding.EncodeToString(decoded)
			}
		} else if uri == "" && data != "" {
			decoded, err := base64.StdEncoding.DecodeString(data)
			if err != nil {
				return nil, fmt.Errorf("attachment '%s' includes invalid base64 payload", name)
			}
			if len(decoded) == 0 {
				return nil, fmt.Errorf("attachment '%s' payload is empty", name)
			}
			if h.attachmentStore == nil {
				return nil, fmt.Errorf("attachment '%s' must include uri (base64 uploads are disabled)", name)
			}
			storedURI, err := h.attachmentStore.StoreBytes(name, mediaType, decoded)
			if err != nil {
				return nil, fmt.Errorf("store attachment '%s': %w", name, err)
			}
			uri = storedURI
			if isImage {
				inlineBase64 = base64.StdEncoding.EncodeToString(decoded)
			}
		}

		if uri == "" || strings.HasPrefix(strings.ToLower(strings.TrimSpace(uri)), "data:") {
			return nil, fmt.Errorf("attachment '%s' must include uri (base64 uploads are disabled)", name)
		}

		source := strings.TrimSpace(incoming.Source)
		if source == "" {
			source = "user_upload"
		}
		attachment := agentports.Attachment{
			Name:                name,
			MediaType:           mediaType,
			Data:                inlineBase64,
			URI:                 uri,
			Description:         strings.TrimSpace(incoming.Description),
			Source:              source,
			Kind:                strings.TrimSpace(incoming.Kind),
			Format:              strings.TrimSpace(incoming.Format),
			RetentionTTLSeconds: incoming.RetentionTTLSeconds,
		}
		attachments = append(attachments, attachment)
	}

	return attachments, nil
}

// HandleGetSession handles GET /api/sessions/:id
func (h *APIHandler) HandleGetSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	// Extract session ID from URL path
	sessionID := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	sessionID = strings.TrimSpace(sessionID)
	if err := validateSessionID(sessionID); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	session, err := h.coordinator.GetSession(r.Context(), sessionID)
	if err != nil {
		h.writeJSONError(w, http.StatusNotFound, "Session not found", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(session); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response", err)
	}
}

// HandleGetSessionPersona handles GET /api/sessions/:id/persona
func (h *APIHandler) HandleGetSessionPersona(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/sessions/"), "/persona")
	sessionID = strings.TrimSuffix(strings.TrimSpace(sessionID), "/")
	if err := validateSessionID(sessionID); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "Invalid session ID", err)
		return
	}

	session, err := h.coordinator.GetSession(r.Context(), sessionID)
	if err != nil {
		h.writeJSONError(w, http.StatusNotFound, "Session not found", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(SessionPersonaResponse{
		SessionID:   session.ID,
		UserPersona: session.UserPersona,
	}); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response", err)
	}
}

// HandleUpdateSessionPersona handles PUT /api/sessions/:id/persona
func (h *APIHandler) HandleUpdateSessionPersona(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/sessions/"), "/persona")
	sessionID = strings.TrimSuffix(strings.TrimSpace(sessionID), "/")
	if err := validateSessionID(sessionID); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "Invalid session ID", err)
		return
	}

	var req SessionPersonaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "Invalid request payload", err)
		return
	}
	if req.UserPersona == nil {
		h.writeJSONError(w, http.StatusBadRequest, "user_persona is required", fmt.Errorf("user_persona is required"))
		return
	}
	if req.UserPersona.UpdatedAt.IsZero() {
		req.UserPersona.UpdatedAt = time.Now()
	}

	session, err := h.coordinator.UpdateSessionPersona(r.Context(), sessionID, req.UserPersona)
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to update persona", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(SessionPersonaResponse{
		SessionID:   session.ID,
		UserPersona: session.UserPersona,
	}); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response", err)
	}
}

// SessionResponse matches TypeScript Session interface
type SessionResponse struct {
	ID        string `json:"id"`
	Title     string `json:"title,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	TaskCount int    `json:"task_count"`
	LastTask  string `json:"last_task,omitempty"`
}

// SessionListResponse matches TypeScript SessionListResponse interface
type SessionListResponse struct {
	Sessions []SessionResponse `json:"sessions"`
	Total    int               `json:"total"`
}

type CreateSessionResponse struct {
	SessionID string `json:"session_id"`
}

type ShareSessionResponse struct {
	SessionID  string                   `json:"session_id"`
	ShareToken string                   `json:"share_token"`
	Events     []map[string]interface{} `json:"events,omitempty"`
	Title      string                   `json:"title,omitempty"`
	CreatedAt  string                   `json:"created_at,omitempty"`
	UpdatedAt  string                   `json:"updated_at,omitempty"`
}

// HandleCreateSession handles POST /api/sessions
func (h *APIHandler) HandleCreateSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	session, err := h.coordinator.CreateSession(r.Context())
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to create session", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(CreateSessionResponse{SessionID: session.ID}); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response", err)
	}
}

// HandleCreateSessionShare handles POST /api/sessions/:id/share
func (h *APIHandler) HandleCreateSessionShare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	sessionID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/sessions/"), "/share")
	sessionID = strings.TrimSuffix(strings.TrimSpace(sessionID), "/")
	if err := validateSessionID(sessionID); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	shareToken, err := h.coordinator.EnsureSessionShareToken(r.Context(), sessionID, false)
	if err != nil {
		if strings.Contains(err.Error(), "session not found") {
			h.writeJSONError(w, http.StatusNotFound, "Session not found", err)
			return
		}
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to create share token", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(ShareSessionResponse{
		SessionID:  sessionID,
		ShareToken: shareToken,
	}); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response", err)
	}
}

// HandleListSessions handles GET /api/sessions
func (h *APIHandler) HandleListSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			h.writeJSONError(w, http.StatusBadRequest, "limit must be a positive integer", err)
			return
		}
		limit = parsed
	}
	if limit > maxSessionListLimit {
		limit = maxSessionListLimit
	}

	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			h.writeJSONError(w, http.StatusBadRequest, "offset must be a non-negative integer", err)
			return
		}
		offset = parsed
	}

	sessionIDs, err := h.coordinator.ListSessions(r.Context(), limit, offset)
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to list sessions", err)
		return
	}

	// Convert session IDs to full session objects
	sessions := make([]SessionResponse, 0, len(sessionIDs))
	for _, id := range sessionIDs {
		session, err := h.coordinator.GetSession(r.Context(), id)
		if err != nil {
			continue // Skip sessions that can't be loaded
		}

		// Get tasks for this session to populate task_count and last_task
		tasks, _ := h.coordinator.ListSessionTasks(r.Context(), id)
		taskCount := len(tasks)
		lastTask := ""
		if taskCount > 0 {
			// Tasks are sorted newest first
			lastTask = tasks[0].Description
		}

		sessions = append(sessions, SessionResponse{
			ID:        session.ID,
			Title:     strings.TrimSpace(session.Metadata["title"]),
			CreatedAt: session.CreatedAt.Format(time.RFC3339),
			UpdatedAt: session.UpdatedAt.Format(time.RFC3339),
			TaskCount: taskCount,
			LastTask:  lastTask,
		})
	}

	response := SessionListResponse{
		Sessions: sessions,
		Total:    len(sessions),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response", err)
	}
}

// HandleDeleteSession handles DELETE /api/sessions/:id
func (h *APIHandler) HandleDeleteSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	// Extract session ID from URL path
	sessionID := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	sessionID = strings.TrimSpace(sessionID)
	if err := validateSessionID(sessionID); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	if err := h.coordinator.DeleteSession(r.Context(), sessionID); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to delete session", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleListSnapshots handles GET /api/sessions/:id/snapshots
func (h *APIHandler) HandleListSnapshots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	trimmed := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	trimmed = strings.TrimSuffix(trimmed, "/snapshots")
	sessionID := strings.TrimSuffix(strings.TrimSpace(trimmed), "/")
	if err := validateSessionID(sessionID); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		} else {
			h.writeJSONError(w, http.StatusBadRequest, "limit must be a positive integer", err)
			return
		}
	}
	if limit > maxSnapshotListLimit {
		limit = maxSnapshotListLimit
	}
	cursor := strings.TrimSpace(r.URL.Query().Get("cursor"))
	items, nextCursor, err := h.coordinator.ListSnapshots(r.Context(), sessionID, cursor, limit)
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to list snapshots", err)
		return
	}
	responseItems := make([]SessionSnapshotItem, 0, len(items))
	for _, meta := range items {
		responseItems = append(responseItems, SessionSnapshotItem{
			TurnID:     meta.TurnID,
			LLMTurnSeq: meta.LLMTurnSeq,
			Summary:    meta.Summary,
			CreatedAt:  meta.CreatedAt.Format(time.RFC3339),
		})
	}
	resp := SessionSnapshotsResponse{SessionID: sessionID, Items: responseItems}
	if nextCursor != "" {
		resp.NextCursor = nextCursor
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response", err)
	}
}

// HandleGetTurnSnapshot handles GET /api/sessions/:id/turns/:turnID
func (h *APIHandler) HandleGetTurnSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	parts := strings.Split(trimmed, "/turns/")
	if len(parts) != 2 {
		h.writeJSONError(w, http.StatusBadRequest, "Invalid snapshot path", fmt.Errorf("invalid path %s", r.URL.Path))
		return
	}
	sessionID := strings.TrimSuffix(strings.TrimSpace(parts[0]), "/")
	if err := validateSessionID(sessionID); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
		return
	}
	turnStr := parts[1]
	if strings.Contains(turnStr, "/") {
		h.writeJSONError(w, http.StatusBadRequest, "Invalid turn path", fmt.Errorf("invalid path segment %s", turnStr))
		return
	}
	turnID, err := strconv.Atoi(strings.TrimSpace(turnStr))
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "turn_id must be numeric", err)
		return
	}
	snapshot, err := h.coordinator.GetSnapshot(r.Context(), sessionID, turnID)
	if err != nil {
		h.writeJSONError(w, http.StatusNotFound, "Snapshot not found", err)
		return
	}
	resp := TurnSnapshotResponse{
		SessionID:  snapshot.SessionID,
		TurnID:     snapshot.TurnID,
		LLMTurnSeq: snapshot.LLMTurnSeq,
		Summary:    snapshot.Summary,
		CreatedAt:  snapshot.CreatedAt.Format(time.RFC3339),
		Plans:      snapshot.Plans,
		Beliefs:    snapshot.Beliefs,
		WorldState: snapshot.World,
		Diff:       snapshot.Diff,
		Messages:   snapshot.Messages,
		Feedback:   snapshot.Feedback,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response", err)
	}
}

// HandleReplaySession handles POST /api/sessions/:id/replay
func (h *APIHandler) HandleReplaySession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}
	sessionID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/sessions/"), "/replay")
	sessionID = strings.TrimSuffix(strings.TrimSpace(sessionID), "/")
	if err := validateSessionID(sessionID); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
		return
	}
	if err := h.coordinator.ReplaySession(r.Context(), sessionID); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to schedule replay", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":     "scheduled",
		"session_id": sessionID,
	})
}

// TaskStatusResponse matches TypeScript TaskStatusResponse interface
type TaskStatusResponse = types.AgentTask

// HandleGetTask handles GET /api/tasks/:id
func (h *APIHandler) HandleGetTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	// Extract task ID from URL path
	taskID := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	if taskID == "" || strings.Contains(taskID, "/") {
		h.writeJSONError(w, http.StatusBadRequest, "Task ID required", fmt.Errorf("invalid task id '%s'", taskID))
		return
	}

	task, err := h.coordinator.GetTask(r.Context(), taskID)
	if err != nil {
		h.writeJSONError(w, http.StatusNotFound, "Task not found", err)
		return
	}

	// Convert to TaskStatusResponse
	response := TaskStatusResponse{
		TaskID:       task.ID,
		SessionID:    task.SessionID,
		ParentTaskID: task.ParentTaskID,
		Status:       string(task.Status),
		CreatedAt:    task.CreatedAt.Format(time.RFC3339),
		Error:        task.Error,
	}

	if task.CompletedAt != nil {
		completedStr := task.CompletedAt.Format(time.RFC3339)
		response.CompletedAt = &completedStr
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response", err)
	}
}

// HandleListTasks handles GET /api/tasks
func (h *APIHandler) HandleListTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	if sessionID != "" {
		if err := validateSessionID(sessionID); err != nil {
			h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
			return
		}
	}

	// Parse pagination parameters
	limit := 10
	offset := 0

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if limit > maxTaskListLimit {
		limit = maxTaskListLimit
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	var tasks []*serverPorts.Task
	var total int

	if sessionID != "" {
		sessionTasks, err := h.coordinator.ListSessionTasks(r.Context(), sessionID)
		if err != nil {
			h.writeJSONError(w, http.StatusInternalServerError, "Failed to list tasks", err)
			return
		}

		total = len(sessionTasks)
		if offset >= total {
			tasks = []*serverPorts.Task{}
		} else {
			end := offset + limit
			if end > total {
				end = total
			}
			tasks = sessionTasks[offset:end]
		}
	} else {
		allTasks, totalCount, err := h.coordinator.ListTasks(r.Context(), limit, offset)
		if err != nil {
			h.writeJSONError(w, http.StatusInternalServerError, "Failed to list tasks", err)
			return
		}
		tasks = allTasks
		total = totalCount
	}

	// Convert to TaskStatusResponse array
	taskResponses := make([]TaskStatusResponse, len(tasks))
	for i, task := range tasks {
		taskResponses[i] = TaskStatusResponse{
			TaskID:       task.ID,
			SessionID:    task.SessionID,
			ParentTaskID: task.ParentTaskID,
			Status:       string(task.Status),
			CreatedAt:    task.CreatedAt.Format(time.RFC3339),
			Error:        task.Error,
		}
		if task.CompletedAt != nil {
			completedStr := task.CompletedAt.Format(time.RFC3339)
			taskResponses[i].CompletedAt = &completedStr
		}
	}

	response := map[string]interface{}{
		"tasks":  taskResponses,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response", err)
	}
}

// HandleCancelTask handles POST /api/tasks/:id/cancel
func (h *APIHandler) HandleCancelTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	// Extract task ID from URL path
	path := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	taskID := strings.TrimSuffix(path, "/cancel")
	if taskID == "" || taskID == path {
		h.writeJSONError(w, http.StatusBadRequest, "Task ID required", fmt.Errorf("invalid task id '%s'", taskID))
		return
	}

	if err := h.coordinator.CancelTask(r.Context(), taskID); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "Failed to cancel task", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "cancelled",
		"task_id": taskID,
	}); err != nil {
		h.logger.Error("Failed to encode cancel response: %v", err)
	}
}

// HandleForkSession handles POST /api/sessions/:id/fork
func (h *APIHandler) HandleForkSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	// Extract session ID from URL path
	path := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	sessionID := strings.TrimSuffix(path, "/fork")
	if sessionID == "" || sessionID == path {
		h.writeJSONError(w, http.StatusBadRequest, "Session ID required", fmt.Errorf("invalid session id '%s'", sessionID))
		return
	}

	sessionID = strings.TrimSpace(sessionID)
	if err := validateSessionID(sessionID); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	newSession, err := h.coordinator.ForkSession(r.Context(), sessionID)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "Failed to fork session", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(newSession); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response", err)
	}
}

// HandleInternalSessionRequest routes internal session endpoints.
func (h *APIHandler) HandleInternalSessionRequest(w http.ResponseWriter, r *http.Request) {
	if !h.internalMode {
		http.NotFound(w, r)
		return
	}

	if strings.HasSuffix(r.URL.Path, "/context") {
		h.HandleGetContextSnapshots(w, r)
		return
	}

	http.NotFound(w, r)
}

// HandleDevSessionRequest routes development-only session endpoints.
func (h *APIHandler) HandleDevSessionRequest(w http.ResponseWriter, r *http.Request) {
	if !h.devMode {
		http.NotFound(w, r)
		return
	}

	if strings.HasSuffix(r.URL.Path, "/context-window") {
		h.HandleGetContextWindowPreview(w, r)
		return
	}

	http.NotFound(w, r)
}

// HandleGetContextWindowPreview handles GET /api/dev/sessions/:id/context-window.
func (h *APIHandler) HandleGetContextWindowPreview(w http.ResponseWriter, r *http.Request) {
	if !h.devMode {
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/dev/sessions/")
	path = strings.TrimSuffix(path, "/context-window")
	sessionID := strings.Trim(path, "/")
	if sessionID == "" {
		h.writeJSONError(w, http.StatusBadRequest, "Session ID required", fmt.Errorf("invalid session id"))
		return
	}

	if err := validateSessionID(sessionID); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	preview, err := h.coordinator.PreviewContextWindow(r.Context(), sessionID)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "Failed to build context window", err)
		return
	}

	sentAttachments := newStringLRU(sseSentAttachmentCacheSize)
	sanitizedWindow := preview.Window
	sanitizedWindow.SessionID = sessionID
	sanitizedWindow.Messages = sanitizeMessagesForDelivery(preview.Window.Messages, sentAttachments)

	response := ContextWindowPreviewResponse{
		SessionID:     sessionID,
		TokenEstimate: preview.TokenEstimate,
		TokenLimit:    preview.TokenLimit,
		PersonaKey:    preview.PersonaKey,
		ToolMode:      preview.ToolMode,
		ToolPreset:    preview.ToolPreset,
		Window:        sanitizedWindow,
	}

	h.writeJSON(w, http.StatusOK, response)
}

// HandleGetContextSnapshots handles GET /api/internal/sessions/:id/context.
func (h *APIHandler) HandleGetContextSnapshots(w http.ResponseWriter, r *http.Request) {
	if !h.internalMode {
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/internal/sessions/")
	path = strings.TrimSuffix(path, "/context")
	sessionID := strings.Trim(path, "/")
	if sessionID == "" {
		h.writeJSONError(w, http.StatusBadRequest, "Session ID required", fmt.Errorf("invalid session id"))
		return
	}

	if err := validateSessionID(sessionID); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	snapshots := h.coordinator.GetContextSnapshots(sessionID)
	response := ContextSnapshotResponse{
		SessionID: sessionID,
		Snapshots: make([]ContextSnapshotItem, len(snapshots)),
	}

	sentAttachments := newStringLRU(sseSentAttachmentCacheSize)

	for i, snapshot := range snapshots {
		item := ContextSnapshotItem{
			RequestID:    snapshot.RequestID,
			Iteration:    snapshot.Iteration,
			Timestamp:    snapshot.Timestamp.Format(time.RFC3339Nano),
			TaskID:       snapshot.TaskID,
			ParentTaskID: snapshot.ParentTaskID,
		}
		item.Messages = sanitizeMessagesForDelivery(snapshot.Messages, sentAttachments)
		if len(snapshot.Excluded) > 0 {
			item.ExcludedMessages = sanitizeMessagesForDelivery(snapshot.Excluded, sentAttachments)
		}
		response.Snapshots[i] = item
	}

	h.writeJSON(w, http.StatusOK, response)
}

func sanitizeMessagesForDelivery(messages []agentports.Message, sentAttachments *stringLRU) []agentports.Message {
	if len(messages) == 0 {
		return nil
	}

	sanitized := make([]agentports.Message, 0, len(messages))
	for _, msg := range messages {
		cloned := msg
		if sanitizedAttachments := sanitizeAttachmentsForStream(msg.Attachments, sentAttachments, nil, nil, false); len(sanitizedAttachments) > 0 {
			cloned.Attachments = sanitizedAttachments
		} else {
			cloned.Attachments = nil
		}

		sanitized = append(sanitized, cloned)
	}

	return sanitized
}

// HandleStartEvaluation launches a new evaluation job accessible from the web console.
func (h *APIHandler) HandleStartEvaluation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	if h.evaluationSvc == nil {
		h.writeJSONError(w, http.StatusServiceUnavailable, "Evaluation service unavailable", fmt.Errorf("evaluation service not configured"))
		return
	}

	body := http.MaxBytesReader(w, r.Body, maxEvaluationBodySize)
	defer func() {
		_ = body.Close()
	}()

	var req startEvaluationRequest
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		var syntaxErr *json.SyntaxError
		var typeErr *json.UnmarshalTypeError
		var maxBytesErr *http.MaxBytesError
		switch {
		case errors.Is(err, io.EOF):
			h.writeJSONError(w, http.StatusBadRequest, "Request body is empty", err)
			return
		case errors.As(err, &syntaxErr):
			h.writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Invalid JSON at position %d", syntaxErr.Offset), err)
			return
		case errors.As(err, &typeErr):
			h.writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Invalid value for field '%s'", typeErr.Field), err)
			return
		case errors.As(err, &maxBytesErr):
			h.writeJSONError(w, http.StatusRequestEntityTooLarge, "Request body too large", err)
			return
		default:
			h.writeJSONError(w, http.StatusBadRequest, "Invalid request body", err)
			return
		}
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		h.writeJSONError(w, http.StatusBadRequest, "Request body must contain a single JSON object", fmt.Errorf("unexpected extra JSON token"))
		return
	}

	enableMetrics := true
	if req.EnableMetrics != nil {
		enableMetrics = *req.EnableMetrics
	}

	options := &agent_eval.EvaluationOptions{
		DatasetPath:    req.DatasetPath,
		InstanceLimit:  req.InstanceLimit,
		MaxWorkers:     req.MaxWorkers,
		TimeoutPerTask: time.Duration(req.TimeoutSec) * time.Second,
		OutputDir:      req.OutputDir,
		AgentID:        req.AgentID,
		EnableMetrics:  enableMetrics,
		ReportFormat:   req.ReportFormat,
	}

	job, err := h.evaluationSvc.Start(r.Context(), options)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	h.writeJSON(w, http.StatusAccepted, h.buildEvaluationResponse(job, nil))
}

// HandleListEvaluations enumerates known evaluation jobs and their summaries.
func (h *APIHandler) HandleListEvaluations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	if h.evaluationSvc == nil {
		h.writeJSONError(w, http.StatusServiceUnavailable, "Evaluation service unavailable", fmt.Errorf("evaluation service not configured"))
		return
	}

	query, hasFilters, err := parseEvaluationQuery(r)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "Invalid query parameters", err)
		return
	}

	if hasFilters {
		evaluations, err := h.evaluationSvc.QueryEvaluations(query)
		if err != nil {
			h.writeJSONError(w, http.StatusInternalServerError, "Failed to list evaluations", err)
			return
		}

		responses := make([]evaluationJobResponse, 0, len(evaluations))
		for _, eval := range evaluations {
			responses = append(responses, h.buildEvaluationResponseFromResults(eval))
		}

		h.writeJSON(w, http.StatusOK, map[string]any{"evaluations": responses})
		return
	}

	jobs := h.evaluationSvc.ListJobs()
	if query.Limit > 0 && len(jobs) > query.Limit {
		jobs = jobs[:query.Limit]
	}

	responses := make([]evaluationJobResponse, len(jobs))
	for i, job := range jobs {
		responses[i] = h.buildEvaluationResponse(job, job.Results)
	}

	h.writeJSON(w, http.StatusOK, map[string]any{"evaluations": responses})
}

// HandleGetEvaluation returns detailed metrics and instance summaries for a job.
func (h *APIHandler) HandleGetEvaluation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	if h.evaluationSvc == nil {
		h.writeJSONError(w, http.StatusServiceUnavailable, "Evaluation service unavailable", fmt.Errorf("evaluation service not configured"))
		return
	}

	jobID := strings.TrimPrefix(r.URL.Path, "/api/evaluations/")
	if jobID == "" || strings.Contains(jobID, "/") {
		h.writeJSONError(w, http.StatusBadRequest, "Invalid evaluation id", fmt.Errorf("invalid evaluation id '%s'", jobID))
		return
	}

	job, err := h.evaluationSvc.GetJob(jobID)
	if err != nil {
		h.writeJSONError(w, http.StatusNotFound, "Evaluation not found", err)
		return
	}

	results, err := h.evaluationSvc.GetJobResults(jobID)
	if err != nil {
		h.logger.Warn("Evaluation results not ready for %s: %v", jobID, err)
	}

	response := evaluationDetailResponse{
		Evaluation: h.buildEvaluationResponse(job, results),
	}
	if results != nil {
		response.Analysis = results.Analysis
		response.Agent = results.Agent
		response.Results = summarizeWorkerResults(results.Results, results.AutoScores)
	}

	h.writeJSON(w, http.StatusOK, response)
}

// HandleDeleteEvaluation removes a persisted evaluation snapshot by job id.
func (h *APIHandler) HandleDeleteEvaluation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	if h.evaluationSvc == nil {
		h.writeJSONError(w, http.StatusServiceUnavailable, "Evaluation service unavailable", fmt.Errorf("evaluation service not configured"))
		return
	}

	jobID := strings.TrimPrefix(r.URL.Path, "/api/evaluations/")
	if jobID == "" || strings.Contains(jobID, "/") {
		h.writeJSONError(w, http.StatusBadRequest, "Invalid evaluation id", fmt.Errorf("invalid evaluation id '%s'", jobID))
		return
	}

	if err := h.evaluationSvc.DeleteEvaluation(jobID); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, os.ErrNotExist) {
			status = http.StatusNotFound
		}
		h.writeJSONError(w, status, "Failed to delete evaluation", err)
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{"deleted": jobID})
}

// HandleListAgents returns all stored agent profiles.
func (h *APIHandler) HandleListAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	if h.evaluationSvc == nil {
		h.writeJSONError(w, http.StatusServiceUnavailable, "Evaluation service unavailable", fmt.Errorf("evaluation service not configured"))
		return
	}

	profiles, err := h.evaluationSvc.ListAgentProfiles()
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to list agents", err)
		return
	}

	h.writeJSON(w, http.StatusOK, agentListResponse{Agents: profiles})
}

// HandleGetAgent returns a single agent profile if present.
func (h *APIHandler) HandleGetAgent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	if h.evaluationSvc == nil {
		h.writeJSONError(w, http.StatusServiceUnavailable, "Evaluation service unavailable", fmt.Errorf("evaluation service not configured"))
		return
	}

	agentID := strings.TrimPrefix(r.URL.Path, "/api/agents/")
	if agentID == "" || strings.Contains(agentID, "/") {
		h.writeJSONError(w, http.StatusBadRequest, "Invalid agent id", fmt.Errorf("invalid agent id '%s'", agentID))
		return
	}

	profile, err := h.evaluationSvc.GetAgentProfile(agentID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, os.ErrNotExist) {
			status = http.StatusNotFound
		}
		h.writeJSONError(w, status, "Agent not found", err)
		return
	}

	h.writeJSON(w, http.StatusOK, profile)
}

// HandleListAgentEvaluations returns evaluation snapshots associated with a given agent.
func (h *APIHandler) HandleListAgentEvaluations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	if h.evaluationSvc == nil {
		h.writeJSONError(w, http.StatusServiceUnavailable, "Evaluation service unavailable", fmt.Errorf("evaluation service not configured"))
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/agents/")
	agentID := strings.TrimSuffix(path, "/evaluations")
	agentID = strings.TrimSuffix(agentID, "/")
	if agentID == "" || strings.Contains(agentID, "/") {
		h.writeJSONError(w, http.StatusBadRequest, "Invalid agent id", fmt.Errorf("invalid agent id '%s'", agentID))
		return
	}

	query, _, err := parseEvaluationQuery(r)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "Invalid query parameters", err)
		return
	}
	query.AgentID = agentID

	evaluations, err := h.evaluationSvc.QueryEvaluations(query)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, os.ErrNotExist) {
			status = http.StatusNotFound
		}
		h.writeJSONError(w, status, "Failed to list agent evaluations", err)
		return
	}

	responses := make([]evaluationJobResponse, 0, len(evaluations))
	for _, eval := range evaluations {
		responses = append(responses, h.buildEvaluationResponseFromResults(eval))
	}

	profile, err := h.evaluationSvc.GetAgentProfile(agentID)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to load agent profile", err)
		return
	}

	h.writeJSON(w, http.StatusOK, agentHistoryResponse{Agent: profile, Evaluations: responses})
}

func parseEvaluationQuery(r *http.Request) (agent_eval.EvaluationQuery, bool, error) {
	values := r.URL.Query()
	query := agent_eval.EvaluationQuery{}
	hasFilters := false

	if agentID := values.Get("agent_id"); agentID != "" {
		query.AgentID = agentID
		hasFilters = true
	}

	if limitStr := values.Get("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 0 {
			return query, false, fmt.Errorf("invalid limit")
		}
		if limit > maxEvaluationListLimit {
			limit = maxEvaluationListLimit
		}
		query.Limit = limit
		hasFilters = true
	}

	if afterStr := values.Get("after"); afterStr != "" {
		after, err := time.Parse(time.RFC3339, afterStr)
		if err != nil {
			return query, false, fmt.Errorf("invalid after timestamp: %w", err)
		}
		query.After = after
		hasFilters = true
	}

	if beforeStr := values.Get("before"); beforeStr != "" {
		before, err := time.Parse(time.RFC3339, beforeStr)
		if err != nil {
			return query, false, fmt.Errorf("invalid before timestamp: %w", err)
		}
		query.Before = before
		hasFilters = true
	}

	if minScoreStr := values.Get("min_score"); minScoreStr != "" {
		minScore, err := strconv.ParseFloat(minScoreStr, 64)
		if err != nil || minScore < 0 {
			return query, false, fmt.Errorf("invalid min_score")
		}
		query.MinScore = minScore
		hasFilters = true
	}

	if dataset := values.Get("dataset"); dataset != "" {
		query.DatasetPath = dataset
		hasFilters = true
	}

	if datasetType := values.Get("dataset_type"); datasetType != "" {
		query.DatasetType = datasetType
		hasFilters = true
	}

	if rawTags := values.Get("tags"); rawTags != "" {
		parsed := parseTags(rawTags)
		if len(parsed) > 0 {
			query.Tags = parsed
			hasFilters = true
		}
	}

	return query, hasFilters, nil
}

func parseTags(raw string) []string {
	parts := strings.Split(raw, ",")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		tags = append(tags, trimmed)
	}
	return tags
}

func (h *APIHandler) buildEvaluationResponse(job *agent_eval.EvaluationJob, results *agent_eval.EvaluationResults) evaluationJobResponse {
	if job == nil {
		return evaluationJobResponse{}
	}

	resp := evaluationJobResponse{
		ID:     job.ID,
		Status: string(job.Status),
	}
	if job.Error != nil {
		resp.Error = job.Error.Error()
	}

	if job.Config != nil {
		resp.AgentID = job.Config.AgentID
		resp.DatasetPath = job.Config.DatasetPath
		resp.InstanceLimit = job.Config.InstanceLimit
		resp.MaxWorkers = job.Config.MaxWorkers
		if job.Config.TimeoutPerTask > 0 {
			resp.TimeoutSec = int64(job.Config.TimeoutPerTask.Seconds())
		}
	} else if results != nil && results.Config != nil {
		resp.DatasetPath = results.Config.DatasetPath
		resp.InstanceLimit = results.Config.InstanceLimit
		resp.MaxWorkers = results.Config.MaxWorkers
		if results.Config.TimeoutPerTask > 0 {
			resp.TimeoutSec = int64(results.Config.TimeoutPerTask.Seconds())
		}
	}

	if !job.StartTime.IsZero() {
		resp.StartedAt = job.StartTime.Format(time.RFC3339)
	}
	if !job.EndTime.IsZero() {
		resp.CompletedAt = job.EndTime.Format(time.RFC3339)
	}

	if results == nil {
		results = job.Results
	}

	if results != nil {
		if results.Analysis != nil {
			resp.Summary = &results.Analysis.Summary
		}
		if resp.AgentID == "" {
			resp.AgentID = results.AgentID
		}
		resp.Metrics = results.Metrics
		resp.Agent = results.Agent
	}

	return resp
}

func (h *APIHandler) buildEvaluationResponseFromResults(results *agent_eval.EvaluationResults) evaluationJobResponse {
	if results == nil {
		return evaluationJobResponse{}
	}

	resp := evaluationJobResponse{
		ID:      results.JobID,
		Status:  string(agent_eval.JobStatusCompleted),
		AgentID: results.AgentID,
	}

	if results.Config != nil {
		resp.DatasetPath = results.Config.DatasetPath
		resp.InstanceLimit = results.Config.InstanceLimit
		resp.MaxWorkers = results.Config.MaxWorkers
		if results.Config.TimeoutPerTask > 0 {
			resp.TimeoutSec = int64(results.Config.TimeoutPerTask.Seconds())
		}
	}

	if results.Analysis != nil {
		resp.Summary = &results.Analysis.Summary
	}
	resp.Metrics = results.Metrics
	resp.Agent = results.Agent

	return resp
}

func summarizeWorkerResults(results []swe_bench.WorkerResult, scores []agent_eval.AutoScore) []workerResultSummary {
	if len(results) == 0 {
		return nil
	}

	scoreByTask := make(map[string]agent_eval.AutoScore)
	for _, score := range scores {
		scoreByTask[score.TaskID] = score
	}

	summaries := make([]workerResultSummary, 0, len(results))
	for _, result := range results {
		score := scoreByTask[result.TaskID]
		summary := workerResultSummary{
			TaskID:       result.TaskID,
			InstanceID:   result.InstanceID,
			Status:       string(result.Status),
			TokensUsed:   result.TokensUsed,
			Cost:         result.Cost,
			AutoScore:    score.Score,
			Grade:        score.Grade,
			Error:        result.Error,
			FilesChanged: len(result.FilesChanged),
			ToolTraces:   len(result.Trace),
		}

		if result.Duration > 0 {
			summary.DurationSeconds = result.Duration.Seconds()
		}
		if !result.StartTime.IsZero() {
			summary.StartedAt = result.StartTime.Format(time.RFC3339)
		}
		if !result.EndTime.IsZero() {
			summary.CompletedAt = result.EndTime.Format(time.RFC3339)
		}

		summaries = append(summaries, summary)
	}

	return summaries
}

// HandleHealthCheck handles GET /health
func (h *APIHandler) HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	// Check all component health
	components := h.healthChecker.CheckAll(r.Context())

	// Determine overall status
	overallStatus := "healthy"
	allReady := true
	for _, comp := range components {
		// Only care about components that should be ready (not disabled)
		if comp.Status != "disabled" && comp.Status != "ready" {
			allReady = false
		}
		if comp.Status == "error" {
			overallStatus = "unhealthy"
			break
		}
	}

	if !allReady && overallStatus != "unhealthy" {
		overallStatus = "degraded"
	}

	response := map[string]interface{}{
		"status":     overallStatus,
		"components": components,
	}

	// Set HTTP status based on health
	httpStatus := http.StatusOK
	if overallStatus == "unhealthy" {
		httpStatus = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode health check response: %v", err)
	}
}

func (h *APIHandler) writeJSONError(w http.ResponseWriter, status int, message string, err error) {
	if err != nil {
		h.logger.Error("HTTP %d - %s: %v", status, message, err)
	} else {
		h.logger.Warn("HTTP %d - %s", status, message)
	}

	resp := apiErrorResponse{Error: message}
	if err != nil {
		resp.Details = err.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if encodeErr := json.NewEncoder(w).Encode(resp); encodeErr != nil {
		h.logger.Error("Failed to encode error response: %v", encodeErr)
	}
}

func (h *APIHandler) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		h.logger.Error("Failed to encode JSON response: %v", err)
	}
}
