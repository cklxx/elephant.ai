package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"alex/internal/shared/logging"
)

const maxWebVitalBodySize = 1 << 14

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

// HandleWebVitals ingests frontend performance signals.
func (h *APIHandler) HandleWebVitals(w http.ResponseWriter, r *http.Request) {
	var payload webVitalPayload
	if !h.decodeJSONBody(w, r, &payload, maxWebVitalBodySize) {
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

// HandleDevLogTrace returns log excerpts correlated by log id.
func (h *APIHandler) HandleDevLogTrace(w http.ResponseWriter, r *http.Request) {
	if !h.devMode {
		http.NotFound(w, r)
		return
	}

	logID := strings.TrimSpace(r.URL.Query().Get("log_id"))
	if logID == "" {
		h.writeJSONError(w, http.StatusBadRequest, "log_id is required", nil)
		return
	}

	bundle := logging.FetchLogBundle(logID, logging.LogFetchOptions{
		MaxEntries: 400,
		MaxBytes:   1 << 20,
	})
	h.writeJSON(w, http.StatusOK, bundle)
}

// HandleDevLogIndex returns recent log_id summaries for quick analysis entrypoints.
func (h *APIHandler) HandleDevLogIndex(w http.ResponseWriter, r *http.Request) {
	if !h.devMode {
		http.NotFound(w, r)
		return
	}

	limit := 80
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			if err == nil {
				err = fmt.Errorf("limit must be > 0")
			}
			h.writeJSONError(w, http.StatusBadRequest, "limit must be a positive integer", err)
			return
		}
		limit = parsed
	}

	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			if err == nil {
				err = fmt.Errorf("offset must be >= 0")
			}
			h.writeJSONError(w, http.StatusBadRequest, "offset must be a non-negative integer", err)
			return
		}
		offset = parsed
	}

	// Fetch one extra entry to determine if there are more results.
	entries := logging.FetchRecentLogIndex(logging.LogIndexOptions{
		Limit:        limit + 1,
		Offset:       offset,
		MaxLineBytes: 1 << 20,
	})
	hasMore := len(entries) > limit
	if hasMore {
		entries = entries[:limit]
	}
	h.writeJSON(w, http.StatusOK, map[string]any{
		"entries":  entries,
		"has_more": hasMore,
	})
}

// HandleDevLogStructured returns structured, parsed log entries correlated by log id.
func (h *APIHandler) HandleDevLogStructured(w http.ResponseWriter, r *http.Request) {
	if !h.devMode {
		http.NotFound(w, r)
		return
	}

	logID := strings.TrimSpace(r.URL.Query().Get("log_id"))
	if logID == "" {
		h.writeJSONError(w, http.StatusBadRequest, "log_id is required", nil)
		return
	}

	search := strings.TrimSpace(r.URL.Query().Get("search"))

	bundle := logging.FetchStructuredLogBundle(logID, logging.LogFetchOptions{
		MaxEntries: 400,
		MaxBytes:   2 << 20,
		Search:     search,
	})
	h.writeJSON(w, http.StatusOK, bundle)
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
