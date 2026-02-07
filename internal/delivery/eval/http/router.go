package http

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	agent_eval "alex/evaluation/agent_eval"
	"alex/evaluation/rl"
	serverApp "alex/internal/delivery/server/app"
	serverHTTP "alex/internal/delivery/server/http"
)

// EvalRouterDeps holds dependencies for the eval-server router.
type EvalRouterDeps struct {
	Evaluation  *serverApp.EvaluationService
	RLStorage   *rl.Storage
	RLExtractor *rl.Extractor
	QualityGate *rl.QualityGate
	RLConfig    rl.QualityConfig
}

// EvalRouterConfig holds configuration for the eval-server router.
type EvalRouterConfig struct {
	Environment    string
	AllowedOrigins []string
	RLOutputDir    string
}

// NewEvalRouter creates the HTTP router for the eval-server.
func NewEvalRouter(deps EvalRouterDeps, cfg EvalRouterConfig) http.Handler {
	mux := http.NewServeMux()

	handler := &evalHandler{
		evaluation:  deps.Evaluation,
		rlOutputDir: cfg.RLOutputDir,
	}

	// Health check
	mux.HandleFunc("GET /health", handler.handleHealth)

	// Evaluation endpoints
	mux.HandleFunc("GET /api/evaluations", handler.handleListEvaluations)
	mux.HandleFunc("POST /api/evaluations", handler.handleStartEvaluation)
	mux.HandleFunc("GET /api/evaluations/{evaluation_id}", handler.handleGetEvaluation)
	mux.HandleFunc("DELETE /api/evaluations/{evaluation_id}", handler.handleDeleteEvaluation)

	// Agent catalog
	mux.HandleFunc("GET /api/agents", handler.handleListAgents)
	mux.HandleFunc("GET /api/agents/{agent_id}", handler.handleGetAgent)
	mux.HandleFunc("GET /api/agents/{agent_id}/evaluations", handler.handleListAgentEvaluations)

	// RL data pipeline
	rlH := &rlHandler{
		storage:     deps.RLStorage,
		extractor:   deps.RLExtractor,
		qualityGate: deps.QualityGate,
		config:      deps.RLConfig,
	}
	mux.HandleFunc("GET /api/rl/stats", rlH.handleGetStats)
	mux.HandleFunc("GET /api/rl/trajectories", rlH.handleListTrajectories)
	mux.HandleFunc("GET /api/rl/trajectories/{trajectory_id}", rlH.handleGetTrajectory)
	mux.HandleFunc("GET /api/rl/config", rlH.handleGetConfig)
	mux.HandleFunc("PUT /api/rl/config", rlH.handleUpdateConfig)
	mux.HandleFunc("GET /api/rl/export", rlH.handleExport)

	// Middleware stack (lightweight — no auth, no streaming guards)
	var root http.Handler = mux
	root = loggingMiddleware(root)
	root = serverHTTP.CompressionMiddleware()(root)
	root = serverHTTP.CORSMiddleware(cfg.Environment, cfg.AllowedOrigins)(root)

	return root
}

type evalHandler struct {
	evaluation  *serverApp.EvaluationService
	rlOutputDir string
}

func (h *evalHandler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "eval-server",
	})
}

func (h *evalHandler) handleListEvaluations(w http.ResponseWriter, _ *http.Request) {
	if h.evaluation == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "Evaluation service unavailable")
		return
	}
	jobs := h.evaluation.ListJobs()
	writeJSON(w, http.StatusOK, map[string]any{"evaluations": jobs})
}

type startEvalRequest struct {
	DatasetPath   string `json:"dataset_path"`
	InstanceLimit int    `json:"instance_limit"`
	MaxWorkers    int    `json:"max_workers"`
	TimeoutSec    int64  `json:"timeout_seconds"`
	OutputDir     string `json:"output_dir"`
	ReportFormat  string `json:"report_format"`
	EnableMetrics *bool  `json:"enable_metrics,omitempty"`
	AgentID       string `json:"agent_id,omitempty"`
}

func (h *evalHandler) handleStartEvaluation(w http.ResponseWriter, r *http.Request) {
	if h.evaluation == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "Evaluation service unavailable")
		return
	}

	var req startEvalRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<18)).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request body")
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

	job, err := h.evaluation.Start(r.Context(), options)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"id":     job.ID,
		"status": string(job.Status),
	})
}

func (h *evalHandler) handleGetEvaluation(w http.ResponseWriter, r *http.Request) {
	if h.evaluation == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "Evaluation service unavailable")
		return
	}
	jobID := r.PathValue("evaluation_id")
	if jobID == "" {
		writeJSONError(w, http.StatusBadRequest, "evaluation_id is required")
		return
	}
	job, err := h.evaluation.GetJob(jobID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Evaluation not found")
		return
	}
	results, _ := h.evaluation.GetJobResults(jobID)
	writeJSON(w, http.StatusOK, map[string]any{"evaluation": job, "results": results})
}

func (h *evalHandler) handleDeleteEvaluation(w http.ResponseWriter, r *http.Request) {
	if h.evaluation == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "Evaluation service unavailable")
		return
	}
	jobID := r.PathValue("evaluation_id")
	if jobID == "" {
		writeJSONError(w, http.StatusBadRequest, "evaluation_id is required")
		return
	}
	if err := h.evaluation.DeleteEvaluation(jobID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to delete evaluation")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": jobID})
}

func (h *evalHandler) handleListAgents(w http.ResponseWriter, _ *http.Request) {
	if h.evaluation == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "Evaluation service unavailable")
		return
	}
	profiles, err := h.evaluation.ListAgentProfiles()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to list agents")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"agents": profiles})
}

func (h *evalHandler) handleGetAgent(w http.ResponseWriter, r *http.Request) {
	if h.evaluation == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "Evaluation service unavailable")
		return
	}
	agentID := r.PathValue("agent_id")
	if agentID == "" {
		writeJSONError(w, http.StatusBadRequest, "agent_id is required")
		return
	}
	profile, err := h.evaluation.GetAgentProfile(agentID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Agent not found")
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (h *evalHandler) handleListAgentEvaluations(w http.ResponseWriter, r *http.Request) {
	if h.evaluation == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "Evaluation service unavailable")
		return
	}
	agentID := r.PathValue("agent_id")
	if agentID == "" {
		writeJSONError(w, http.StatusBadRequest, "agent_id is required")
		return
	}
	evals, err := h.evaluation.ListAgentEvaluations(agentID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to list agent evaluations")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"evaluations": evals})
}

// writeJSON encodes v as JSON and writes it with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("[eval-server] json encode error: %v", err)
	}
}

// writeJSONError writes a JSON error response.
func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// loggingMiddleware logs request method, path, status, and duration.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		dur := time.Since(start)
		path := r.URL.Path
		if !strings.HasPrefix(path, "/health") {
			log.Printf("[eval-server] %s %s %d %s", r.Method, path, rw.status, dur.Round(time.Millisecond))
		}
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}
