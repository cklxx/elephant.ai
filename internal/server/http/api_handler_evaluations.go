package http

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	agent_eval "alex/evaluation/agent_eval"
	"alex/evaluation/swe_bench"
)

const (
	maxEvaluationListLimit = 200
	maxEvaluationBodySize  = 1 << 18
)

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

// HandleStartEvaluation launches a new evaluation job accessible from the web console.
func (h *APIHandler) HandleStartEvaluation(w http.ResponseWriter, r *http.Request) {
	if !h.requireMethod(w, r, http.MethodPost) {
		return
	}

	if h.evaluationSvc == nil {
		h.writeJSONError(w, http.StatusServiceUnavailable, "Evaluation service unavailable", fmt.Errorf("evaluation service not configured"))
		return
	}

	var req startEvaluationRequest
	if !h.decodeJSONBody(w, r, &req, maxEvaluationBodySize) {
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
	if !h.requireMethod(w, r, http.MethodGet) {
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
	if !h.requireMethod(w, r, http.MethodGet) {
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
	if !h.requireMethod(w, r, http.MethodDelete) {
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
	if !h.requireMethod(w, r, http.MethodGet) {
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
	if !h.requireMethod(w, r, http.MethodGet) {
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
	if !h.requireMethod(w, r, http.MethodGet) {
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
