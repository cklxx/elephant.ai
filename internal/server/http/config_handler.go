package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	runtimeconfig "alex/internal/config"
	configadmin "alex/internal/config/admin"
)

// RuntimeConfigResolver resolves the latest runtime configuration snapshot.
type RuntimeConfigResolver func(context.Context) (runtimeconfig.RuntimeConfig, runtimeconfig.Metadata, error)

// ConfigHandler serves internal runtime configuration APIs.
type ConfigHandler struct {
	manager  *configadmin.Manager
	resolver RuntimeConfigResolver
}

// NewConfigHandler constructs a handler when a manager is available.
func NewConfigHandler(manager *configadmin.Manager, resolver RuntimeConfigResolver) *ConfigHandler {
	if manager == nil || resolver == nil {
		return nil
	}
	return &ConfigHandler{manager: manager, resolver: resolver}
}

// runtimeConfigResponse represents payloads exchanged with the UI.
type runtimeConfigResponse struct {
	Effective runtimeconfig.RuntimeConfig          `json:"effective"`
	Overrides runtimeconfig.Overrides              `json:"overrides"`
	Sources   map[string]runtimeconfig.ValueSource `json:"sources,omitempty"`
	UpdatedAt time.Time                            `json:"updated_at"`
	Tasks     []configadmin.ReadinessTask          `json:"tasks"`
}

func (h *ConfigHandler) snapshot(ctx context.Context) (runtimeConfigResponse, error) {
	cfg, meta, err := h.resolver(ctx)
	if err != nil {
		return runtimeConfigResponse{}, err
	}
	overrides, err := h.manager.CurrentOverrides(ctx)
	if err != nil {
		return runtimeConfigResponse{}, err
	}
	return runtimeConfigResponse{
		Effective: cfg,
		Overrides: overrides,
		Sources:   meta.Sources(),
		UpdatedAt: meta.LoadedAt(),
		Tasks:     configadmin.DeriveReadinessTasks(cfg),
	}, nil
}

// HandleGetRuntimeConfig returns the current runtime configuration snapshot.
func (h *ConfigHandler) HandleGetRuntimeConfig(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		http.NotFound(w, r)
		return
	}
	payload, err := h.snapshot(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// HandleUpdateRuntimeConfig persists overrides provided by the UI.
func (h *ConfigHandler) HandleUpdateRuntimeConfig(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		http.NotFound(w, r)
		return
	}
	var body struct {
		Overrides runtimeconfig.Overrides `json:"overrides"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}
	if err := h.manager.UpdateOverrides(r.Context(), body.Overrides); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	payload, err := h.snapshot(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// HandleRuntimeStream streams updates via SSE.
func (h *ConfigHandler) HandleRuntimeStream(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		http.NotFound(w, r)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ctx := r.Context()
	sendSnapshot := func() error {
		payload, err := h.snapshot(ctx)
		if err != nil {
			return err
		}
		if err := writeSSEPayload(w, payload); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	if err := sendSnapshot(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	updates, unsubscribe := h.manager.Subscribe()
	defer unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return
		case <-updates:
			if err := sendSnapshot(); err != nil {
				return
			}
		}
	}
}

func writeSSEPayload(w http.ResponseWriter, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte("data: ")); err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	if _, err := w.Write([]byte("\n\n")); err != nil {
		return err
	}
	return nil
}
