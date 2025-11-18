package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"alex/internal/configcenter"
	"alex/internal/serverconfig"
	"alex/internal/utils"
)

// ConfigHandler exposes internal APIs for managing the shared server configuration.
type ConfigHandler struct {
	service *configcenter.Service
	logger  *utils.Logger
}

// NewConfigHandler builds a new handler backed by the provided service.
func NewConfigHandler(service *configcenter.Service) *ConfigHandler {
	if service == nil {
		return nil
	}
	return &ConfigHandler{service: service, logger: utils.NewComponentLogger("ConfigHandler")}
}

// HandleConfigRequest routes GET/PUT requests for the configuration snapshot.
func (h *ConfigHandler) HandleConfigRequest(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleGet(w, r)
	case http.MethodPut:
		h.handlePut(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

type configResponse struct {
	Config    serverconfig.Config `json:"config"`
	Version   int64               `json:"version"`
	UpdatedAt time.Time           `json:"updated_at"`
}

type configUpdateRequest struct {
	Config serverconfig.Config `json:"config"`
}

func (h *ConfigHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	snapshot, err := h.service.Get(r.Context())
	if err != nil {
		if errors.Is(err, configcenter.ErrNotFound) {
			http.Error(w, "configuration not found", http.StatusNotFound)
			return
		}
		h.logger.Error("config fetch failed: %v", err)
		http.Error(w, "failed to load configuration", http.StatusInternalServerError)
		return
	}

	respondJSON(w, configResponse{Config: snapshot.Config, Version: snapshot.Version, UpdatedAt: snapshot.UpdatedAt})
}

func (h *ConfigHandler) handlePut(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var req configUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	snapshot, err := h.service.Update(r.Context(), req.Config)
	if err != nil {
		h.logger.Error("config update failed: %v", err)
		http.Error(w, "failed to persist configuration", http.StatusInternalServerError)
		return
	}

	respondJSON(w, configResponse{Config: snapshot.Config, Version: snapshot.Version, UpdatedAt: snapshot.UpdatedAt})
}

func respondJSON(w http.ResponseWriter, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "failed to serialize response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}
