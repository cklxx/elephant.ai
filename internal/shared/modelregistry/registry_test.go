package modelregistry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAPIResponse builds a models.dev-shaped JSON response for testing.
func fakeAPIResponse() map[string]providerPayload {
	return map[string]providerPayload{
		"anthropic": {
			Models: map[string]modelPayload{
				"claude-3-opus": {
					Limit:      limitPayload{Context: 200000},
					Cost:       costPayload{Input: 15.0, Output: 75.0},
					ToolCall:   true,
					Modalities: modalitiesPayload{Input: []string{"text", "image"}},
				},
				"claude-3-haiku": {
					Limit:      limitPayload{Context: 200000},
					Cost:       costPayload{Input: 0.25, Output: 1.25},
					ToolCall:   true,
					Modalities: modalitiesPayload{Input: []string{"text"}},
				},
			},
		},
		"openai": {
			Models: map[string]modelPayload{
				"gpt-4o": {
					Limit:      limitPayload{Context: 128000},
					Cost:       costPayload{Input: 5.0, Output: 15.0},
					ToolCall:   true,
					Modalities: modalitiesPayload{Input: []string{"text", "image"}},
				},
			},
		},
	}
}

func newTestServer(t *testing.T, payload any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
}

// loadRegistryFromFakeData populates a Registry via the real fetchFromAPI path.
func loadRegistryFromFakeData(t *testing.T) *Registry {
	t.Helper()
	srv := newTestServer(t, fakeAPIResponse())
	t.Cleanup(srv.Close)

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = "http"
			req.URL.Host = srv.Listener.Addr().String()
			return http.DefaultTransport.RoundTrip(req)
		}),
	}

	data, byProvider, err := fetchFromAPI(context.Background(), client)
	require.NoError(t, err)

	return &Registry{
		data:       data,
		byProvider: byProvider,
		fetchedAt:  time.Now(),
	}
}

// --- Lookup tests ---

func TestLookupExactBareKey(t *testing.T) {
	reg := loadRegistryFromFakeData(t)
	info, ok := reg.Lookup("claude-3-opus")
	require.True(t, ok)
	assert.Equal(t, "claude-3-opus", info.ID)
	assert.Equal(t, 200000, info.ContextWindow)
	assert.Equal(t, 15.0, info.InputPer1M)
	assert.Equal(t, 75.0, info.OutputPer1M)
	assert.True(t, info.SupportsTools)
	assert.True(t, info.SupportsVision)
}

func TestLookupCompoundKey(t *testing.T) {
	reg := loadRegistryFromFakeData(t)
	info, ok := reg.Lookup("anthropic/claude-3-opus")
	require.True(t, ok)
	assert.Equal(t, "claude-3-opus", info.ID)
	assert.Equal(t, "anthropic", info.Provider)
}

func TestLookupCompoundKeyFallsToBare(t *testing.T) {
	reg := loadRegistryFromFakeData(t)
	// Lookup with a provider prefix that doesn't have a compound key,
	// but the bare key exists.
	info, ok := reg.Lookup("unknown-provider/gpt-4o")
	require.True(t, ok)
	assert.Equal(t, "gpt-4o", info.ID)
}

func TestLookupMissReturnsZero(t *testing.T) {
	reg := loadRegistryFromFakeData(t)
	info, ok := reg.Lookup("nonexistent-model")
	assert.False(t, ok)
	assert.Equal(t, ModelInfo{}, info)
}

func TestLookupEmptyRegistry(t *testing.T) {
	reg := &Registry{}
	// Prevent triggerLoad from starting a real fetch.
	reg.fetchedAt = time.Now()
	reg.data = nil

	info, ok := reg.Lookup("anything")
	assert.False(t, ok)
	assert.Equal(t, ModelInfo{}, info)
}

func TestLookupWithEmptyData(t *testing.T) {
	reg := &Registry{
		data:      make(map[string]ModelInfo),
		fetchedAt: time.Now(),
	}
	info, ok := reg.Lookup("anything")
	assert.False(t, ok)
	assert.Equal(t, ModelInfo{}, info)
}

// --- ProviderModels tests ---

func TestProviderModelsReturnsModels(t *testing.T) {
	reg := loadRegistryFromFakeData(t)
	models := reg.ProviderModels("anthropic")
	assert.Len(t, models, 2)
	assert.Contains(t, models, "claude-3-opus")
	assert.Contains(t, models, "claude-3-haiku")
}

func TestProviderModelsOpenAI(t *testing.T) {
	reg := loadRegistryFromFakeData(t)
	models := reg.ProviderModels("openai")
	assert.Len(t, models, 1)
	assert.Contains(t, models, "gpt-4o")
}

func TestProviderModelsUnknownProvider(t *testing.T) {
	reg := loadRegistryFromFakeData(t)
	models := reg.ProviderModels("nonexistent")
	assert.Nil(t, models)
}

func TestProviderModelsReturnsCopy(t *testing.T) {
	reg := loadRegistryFromFakeData(t)
	models1 := reg.ProviderModels("anthropic")
	models2 := reg.ProviderModels("anthropic")
	// Mutating one slice should not affect the other.
	if len(models1) > 0 {
		models1[0] = "MUTATED"
		assert.NotEqual(t, models1[0], models2[0])
	}
}

// --- supportsVision tests ---

func TestSupportsVisionTrue(t *testing.T) {
	m := modelPayload{
		Modalities: modalitiesPayload{Input: []string{"text", "image"}},
	}
	assert.True(t, m.supportsVision())
}

func TestSupportsVisionFalse(t *testing.T) {
	m := modelPayload{
		Modalities: modalitiesPayload{Input: []string{"text"}},
	}
	assert.False(t, m.supportsVision())
}

func TestSupportsVisionEmptyModalities(t *testing.T) {
	m := modelPayload{}
	assert.False(t, m.supportsVision())
}

func TestSupportsVisionImageOnly(t *testing.T) {
	m := modelPayload{
		Modalities: modalitiesPayload{Input: []string{"image"}},
	}
	assert.True(t, m.supportsVision())
}

// --- ModelInfo field tests ---

func TestModelInfoFields(t *testing.T) {
	info := ModelInfo{
		ID:             "test-model",
		Provider:       "test-provider",
		ContextWindow:  128000,
		InputPer1M:     5.0,
		OutputPer1M:    15.0,
		SupportsTools:  true,
		SupportsVision: false,
	}
	assert.Equal(t, "test-model", info.ID)
	assert.Equal(t, "test-provider", info.Provider)
	assert.Equal(t, 128000, info.ContextWindow)
	assert.Equal(t, 5.0, info.InputPer1M)
	assert.Equal(t, 15.0, info.OutputPer1M)
	assert.True(t, info.SupportsTools)
	assert.False(t, info.SupportsVision)
}

func TestModelInfoZeroValue(t *testing.T) {
	var info ModelInfo
	assert.Empty(t, info.ID)
	assert.Empty(t, info.Provider)
	assert.Zero(t, info.ContextWindow)
	assert.Zero(t, info.InputPer1M)
	assert.Zero(t, info.OutputPer1M)
	assert.False(t, info.SupportsTools)
	assert.False(t, info.SupportsVision)
}

// --- WaitUntilReady tests ---

func TestWaitUntilReadyAlreadyLoaded(t *testing.T) {
	reg := loadRegistryFromFakeData(t)
	// Already has data — should return true immediately.
	ready := reg.WaitUntilReady(100 * time.Millisecond)
	assert.True(t, ready)
}

func TestWaitUntilReadyTimeout(t *testing.T) {
	reg := &Registry{
		fetchedAt: time.Now(), // prevent triggerLoad from fetching
		data:      nil,        // but no data
	}
	// Will timeout because data is nil and no fetch will run.
	ready := reg.WaitUntilReady(100 * time.Millisecond)
	assert.False(t, ready)
}

// --- Bare key priority tests ---

func TestBareKeyPrefersNonZeroPricing(t *testing.T) {
	// When two providers have the same model, the one with non-zero pricing wins.
	srv := newTestServer(t, map[string]providerPayload{
		"reseller": {
			Models: map[string]modelPayload{
				"shared-model": {
					Limit: limitPayload{Context: 100000},
					Cost:  costPayload{Input: 0, Output: 0}, // zero pricing
				},
			},
		},
		"canonical": {
			Models: map[string]modelPayload{
				"shared-model": {
					Limit: limitPayload{Context: 100000},
					Cost:  costPayload{Input: 10.0, Output: 30.0}, // real pricing
				},
			},
		},
	})
	defer srv.Close()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = "http"
			req.URL.Host = srv.Listener.Addr().String()
			return http.DefaultTransport.RoundTrip(req)
		}),
	}

	data, _, err := fetchFromAPI(context.Background(), client)
	require.NoError(t, err)

	info, ok := data["shared-model"]
	require.True(t, ok)
	// Should have the canonical provider's pricing.
	assert.Equal(t, 10.0, info.InputPer1M)
	assert.Equal(t, 30.0, info.OutputPer1M)
}

// --- triggerLoad tests ---

func TestTriggerLoadSkipsWhenFresh(t *testing.T) {
	reg := &Registry{
		fetchedAt: time.Now(),
		data:      map[string]ModelInfo{"m": {}},
	}
	// Should not start a goroutine — just return.
	reg.triggerLoad()
	reg.mu.RLock()
	assert.False(t, reg.loading)
	reg.mu.RUnlock()
}

func TestTriggerLoadSkipsWhenAlreadyLoading(t *testing.T) {
	reg := &Registry{loading: true}
	reg.triggerLoad()
	// Should still be loading (didn't start another).
	reg.mu.RLock()
	assert.True(t, reg.loading)
	reg.mu.RUnlock()
}
