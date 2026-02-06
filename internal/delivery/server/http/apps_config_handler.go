package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"alex/internal/shared/config"
)

// AppsConfigHandler serves internal apps configuration APIs.
type AppsConfigHandler struct {
	load func(...config.Option) (config.AppsConfig, string, error)
	save func(config.AppsConfig, ...config.Option) (string, error)
}

// NewAppsConfigHandler constructs a handler when loader/saver are available.
func NewAppsConfigHandler(
	load func(...config.Option) (config.AppsConfig, string, error),
	save func(config.AppsConfig, ...config.Option) (string, error),
) *AppsConfigHandler {
	if load == nil || save == nil {
		return nil
	}
	return &AppsConfigHandler{load: load, save: save}
}

type appsConfigResponse struct {
	Apps config.AppsConfig `json:"apps"`
	Path string            `json:"path,omitempty"`
}

func (h *AppsConfigHandler) snapshot() (appsConfigResponse, error) {
	apps, path, err := h.load()
	if err != nil {
		return appsConfigResponse{}, err
	}
	return appsConfigResponse{Apps: apps, Path: path}, nil
}

// HandleAppsConfig routes app config requests.
func (h *AppsConfigHandler) HandleAppsConfig(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.HandleGetAppsConfig(w, r)
	case http.MethodPut:
		h.HandleUpdateAppsConfig(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleGetAppsConfig returns the current apps configuration snapshot.
func (h *AppsConfigHandler) HandleGetAppsConfig(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		http.NotFound(w, r)
		return
	}
	payload, err := h.snapshot()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// HandleUpdateAppsConfig persists apps configuration provided by the UI.
func (h *AppsConfigHandler) HandleUpdateAppsConfig(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		http.NotFound(w, r)
		return
	}
	var body appsConfigResponse
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}
	apps := normalizeAppsConfig(body.Apps)
	if err := validateAppsConfig(apps); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if _, err := h.save(apps); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	payload, err := h.snapshot()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

func validateAppsConfig(apps config.AppsConfig) error {
	seen := make(map[string]struct{}, len(apps.Plugins))
	for i, plugin := range apps.Plugins {
		if plugin.ID == "" {
			return fmt.Errorf("plugins[%d].id is required", i)
		}
		if _, ok := seen[plugin.ID]; ok {
			return fmt.Errorf("duplicate plugin id: %s", plugin.ID)
		}
		seen[plugin.ID] = struct{}{}
	}
	return nil
}

func normalizeAppsConfig(apps config.AppsConfig) config.AppsConfig {
	for i, plugin := range apps.Plugins {
		plugin.ID = strings.ToLower(strings.TrimSpace(plugin.ID))
		plugin.Name = strings.TrimSpace(plugin.Name)
		plugin.Description = strings.TrimSpace(plugin.Description)
		plugin.IntegrationNote = strings.TrimSpace(plugin.IntegrationNote)
		plugin.Capabilities = trimStringList(plugin.Capabilities)
		plugin.Sources = trimStringList(plugin.Sources)
		apps.Plugins[i] = plugin
	}
	return apps
}

func trimStringList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	trimmed := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		trimmed = append(trimmed, value)
	}
	return trimmed
}
