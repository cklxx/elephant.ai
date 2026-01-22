package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	runtimeconfig "alex/internal/config"
	configadmin "alex/internal/config/admin"
	"alex/internal/httpclient"
	"alex/internal/logging"
	"alex/internal/subscription"
)

// RuntimeConfigResolver resolves the latest runtime configuration snapshot.
type RuntimeConfigResolver func(context.Context) (runtimeconfig.RuntimeConfig, runtimeconfig.Metadata, error)

// ConfigHandler serves internal runtime configuration APIs.
type ConfigHandler struct {
	manager        *configadmin.Manager
	resolver       RuntimeConfigResolver
	catalogService SubscriptionCatalogService
}

// NewConfigHandler constructs a handler when a manager is available.
func NewConfigHandler(manager *configadmin.Manager, resolver RuntimeConfigResolver) *ConfigHandler {
	if manager == nil || resolver == nil {
		return nil
	}
	return &ConfigHandler{
		manager:        manager,
		resolver:       resolver,
		catalogService: nil,
	}
}

// runtimeConfigResponse represents payloads exchanged with the UI.
type runtimeConfigResponse struct {
	Effective runtimeconfig.RuntimeConfig          `json:"effective"`
	Overrides runtimeconfig.Overrides              `json:"overrides"`
	Sources   map[string]runtimeconfig.ValueSource `json:"sources,omitempty"`
	UpdatedAt time.Time                            `json:"updated_at"`
	Tasks     []configadmin.ReadinessTask          `json:"tasks"`
}

// SubscriptionCatalogService resolves the current model catalog from CLI subscriptions.
type SubscriptionCatalogService interface {
	Catalog(context.Context) subscription.Catalog
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

// HandleGetRuntimeModels returns CLI-discovered model catalogs.
func (h *ConfigHandler) HandleGetRuntimeModels(w http.ResponseWriter, r *http.Request) {
	h.HandleGetSubscriptionCatalog(w, r)
}

// HandleGetSubscriptionCatalog returns CLI-discovered model catalogs.
func (h *ConfigHandler) HandleGetSubscriptionCatalog(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		http.NotFound(w, r)
		return
	}
	service := h.catalogService
	if service == nil {
		service = defaultCatalogService(h.resolver)
	}
	writeJSON(w, http.StatusOK, service.Catalog(r.Context()))
}

func defaultCatalogService(resolver RuntimeConfigResolver) SubscriptionCatalogService {
	logger := logging.NewComponentLogger("SubscriptionCatalog")
	client := httpclient.New(20*time.Second, logger)
	return subscription.NewCatalogService(func() runtimeconfig.CLICredentials {
		return runtimeconfig.LoadCLICredentials()
	}, client, 15*time.Second, subscription.WithOllamaTargetResolver(func(ctx context.Context) (subscription.OllamaTarget, bool) {
		if resolver == nil {
			return subscription.OllamaTarget{}, false
		}
		cfg, meta, err := resolver(ctx)
		if err == nil {
			provider := strings.ToLower(strings.TrimSpace(cfg.LLMProvider))
			if provider == "ollama" {
				baseURL := strings.TrimSpace(cfg.BaseURL)
				source := string(meta.Source("base_url"))
				if baseURL == "" {
					baseURL = "http://localhost:11434"
					if source == "" {
						source = string(runtimeconfig.SourceDefault)
					}
				}
				return subscription.OllamaTarget{BaseURL: baseURL, Source: source}, true
			}
		}

		if baseURL, source := resolveOllamaEnvTarget(); baseURL != "" {
			return subscription.OllamaTarget{BaseURL: baseURL, Source: source}, true
		}
		return subscription.OllamaTarget{Source: string(runtimeconfig.SourceDefault)}, true
	}))
}

func resolveOllamaEnvTarget() (string, string) {
	lookup := runtimeconfig.DefaultEnvLookup
	if base, ok := lookup("OLLAMA_BASE_URL"); ok {
		base = strings.TrimSpace(base)
		if base != "" {
			return base, string(runtimeconfig.SourceEnv)
		}
	}
	if host, ok := lookup("OLLAMA_HOST"); ok {
		host = strings.TrimSpace(host)
		if host == "" {
			return "", ""
		}
		if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
			return host, string(runtimeconfig.SourceEnv)
		}
		return "http://" + host, string(runtimeconfig.SourceEnv)
	}
	return "", ""
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
