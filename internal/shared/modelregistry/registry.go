package modelregistry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"alex/internal/shared/logging"
)

const (
	apiURL   = "https://models.dev/api.json"
	cacheTTL = 24 * time.Hour
)

// Registry caches model metadata fetched from models.dev.
// The first call to Lookup or ProviderModels triggers a non-blocking background
// fetch; callers receive (ModelInfo{}, false) until the fetch completes, and
// then fall through to their hardcoded fallbacks. This avoids any test or
// startup latency caused by the HTTP round-trip.
type Registry struct {
	mu         sync.RWMutex
	data       map[string]ModelInfo // keyed by "provider/modelID" and bare "modelID"
	byProvider map[string][]string  // provider -> model IDs
	fetchedAt  time.Time
	loading    bool         // a fetch goroutine is in flight
	client     *http.Client // nil = use built-in default
}

// Default is the package-level singleton registry.
var Default = &Registry{}

// Lookup looks up a model in the default registry.
func Lookup(modelID string) (ModelInfo, bool) {
	return Default.Lookup(modelID)
}

// ProviderModels returns all known model IDs for provider from the default registry.
func ProviderModels(provider string) []string {
	return Default.ProviderModels(provider)
}

// WaitUntilReady blocks until the default registry has data or timeout elapses.
// Returns true if data is available. Useful for warm-up in catalog loading or tests.
func WaitUntilReady(timeout time.Duration) bool {
	return Default.WaitUntilReady(timeout)
}

// WaitUntilReady blocks until the registry has data or timeout elapses.
func (r *Registry) WaitUntilReady(timeout time.Duration) bool {
	r.triggerLoad()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		r.mu.RLock()
		ready := len(r.data) > 0
		r.mu.RUnlock()
		if ready {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

// Lookup returns ModelInfo for the given model ID.
// It tries:
//  1. Exact key ("provider/modelID" or bare "modelID")
//  2. Bare model ID when caller passed "provider/model"
//
// Returns (ModelInfo{}, false) while the background fetch is in flight or on
// miss. Callers must have their own fallbacks.
func (r *Registry) Lookup(modelID string) (ModelInfo, bool) {
	r.triggerLoad()

	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.data == nil {
		return ModelInfo{}, false
	}

	if info, ok := r.data[modelID]; ok {
		return info, true
	}

	// Try bare ID when caller passed "provider/model".
	if idx := strings.Index(modelID, "/"); idx >= 0 {
		bare := modelID[idx+1:]
		if info, ok := r.data[bare]; ok {
			return info, true
		}
	}

	return ModelInfo{}, false
}

// ProviderModels returns all model IDs known for a provider.
func (r *Registry) ProviderModels(provider string) []string {
	r.triggerLoad()

	r.mu.RLock()
	defer r.mu.RUnlock()

	key := strings.ToLower(strings.TrimSpace(provider))
	ids := r.byProvider[key]
	if len(ids) == 0 {
		return nil
	}
	out := make([]string, len(ids))
	copy(out, ids)
	return out
}

// triggerLoad starts a background fetch if the cache is empty or stale and
// no fetch is already in flight. It returns immediately without blocking.
func (r *Registry) triggerLoad() {
	r.mu.RLock()
	fresh := !r.fetchedAt.IsZero() && time.Since(r.fetchedAt) < cacheTTL
	loading := r.loading
	r.mu.RUnlock()

	if fresh || loading {
		return
	}

	r.mu.Lock()
	// Double-check under write lock.
	if (!r.fetchedAt.IsZero() && time.Since(r.fetchedAt) < cacheTTL) || r.loading {
		r.mu.Unlock()
		return
	}
	r.loading = true
	r.mu.Unlock()

	go r.fetchAndStore()
}

// fetchAndStore performs the HTTP fetch, parses the response, and stores the
// result. It clears the loading flag on completion.
func (r *Registry) fetchAndStore() {
	defer func() {
		r.mu.Lock()
		r.loading = false
		r.fetchedAt = time.Now() // advance even on error to avoid hammering
		r.mu.Unlock()
	}()

	client := r.client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	data, byProvider, err := fetchFromAPI(ctx, client)
	if err != nil {
		logging.NewComponentLogger("modelregistry").Warn("models.dev fetch failed: %v", err)
		return
	}

	r.mu.Lock()
	r.data = data
	r.byProvider = byProvider
	r.mu.Unlock()
}

func fetchFromAPI(ctx context.Context, client *http.Client) (map[string]ModelInfo, map[string][]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("HTTP GET: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var raw map[string]providerPayload
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, nil, fmt.Errorf("decode JSON: %w", err)
	}

	data := make(map[string]ModelInfo, len(raw)*20)
	byProvider := make(map[string][]string, len(raw))

	// Sort provider IDs for deterministic bare-key assignment.
	// Canonical providers (e.g., "anthropic", "openai") sort before resellers
	// (e.g., "openrouter"), so their data wins for ambiguous bare model IDs.
	providerIDs := make([]string, 0, len(raw))
	for providerID := range raw {
		providerIDs = append(providerIDs, providerID)
	}
	sort.Strings(providerIDs)

	for _, providerID := range providerIDs {
		pPayload := raw[providerID]
		pid := strings.ToLower(strings.TrimSpace(providerID))
		for modelID, mData := range pPayload.Models {
			info := ModelInfo{
				ID:             modelID,
				Provider:       pid,
				ContextWindow:  mData.Limit.Context,
				InputPer1M:     mData.Cost.Input,
				OutputPer1M:    mData.Cost.Output,
				SupportsTools:  mData.ToolCall,
				SupportsVision: mData.supportsVision(),
			}
			// Compound key is unambiguous; always set it.
			data[pid+"/"+modelID] = info
			// Bare key: first provider (alphabetically) wins.
			// Prefer entries with valid pricing over those with zero pricing.
			if existing, exists := data[modelID]; !exists || (existing.InputPer1M == 0 && info.InputPer1M > 0) {
				data[modelID] = info
			}
			byProvider[pid] = append(byProvider[pid], modelID)
		}
	}

	return data, byProvider, nil
}

// ---------------------------------------------------------------------------
// JSON wire types for models.dev API
// ---------------------------------------------------------------------------

type providerPayload struct {
	Models map[string]modelPayload `json:"models"`
}

// modelPayload mirrors the models.dev API shape (as of 2026-03).
// Fields: cost.{input,output} (USD per 1M tokens), limit.context, tool_call,
// modalities.input (slice containing "image" when vision is supported).
type modelPayload struct {
	Limit      limitPayload      `json:"limit"`
	Cost       costPayload       `json:"cost"`
	ToolCall   bool              `json:"tool_call"`
	Modalities modalitiesPayload `json:"modalities"`
}

func (m modelPayload) supportsVision() bool {
	for _, mod := range m.Modalities.Input {
		if mod == "image" {
			return true
		}
	}
	return false
}

type limitPayload struct {
	Context int `json:"context"`
}

type costPayload struct {
	Input  float64 `json:"input"`
	Output float64 `json:"output"`
}

type modalitiesPayload struct {
	Input []string `json:"input"`
}
