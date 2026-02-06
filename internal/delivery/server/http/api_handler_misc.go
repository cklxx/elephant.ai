package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"alex/internal/infra/sandbox"
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

// HandleSandboxBrowserInfo proxies sandbox browser info for the web console.
func (h *APIHandler) HandleSandboxBrowserInfo(w http.ResponseWriter, r *http.Request) {
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
