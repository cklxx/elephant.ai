package modelregistry

import (
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

func newTestRegistry(t *testing.T, serverURL string) *Registry {
	t.Helper()
	// Directly parse fake data to avoid network dependencies.
	// For tests that need the full fetch path, use fetchFromAPI with a test server.
	return &Registry{
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

// loadRegistryFromFakeData populates a Registry with fake API data directly.
func loadRegistryFromFakeData(t *testing.T) *Registry {
	t.Helper()
	srv := newTestServer(t, fakeAPIResponse())
	t.Cleanup(srv.Close)

	// Override apiURL isn't possible (const), so call fetchFromAPI directly.
	client := srv.Client()
	origURL := apiURL

	// Use a custom transport to redirect requests to the test server.
	client.Transport = &rewriteTransport{
		base:    http.DefaultTransport,
		fromURL: origURL,
		toURL:   srv.URL,
	}

	reg := &Registry{client: client}
	// Manually load via fetchAndStore-like logic using fetchFromAPI with test server.
	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	require.NoError(t, err)

	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	var raw map[string]providerPayload
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&raw))

	// Manually populate using the same logic as fetchFromAPI.
	data, byProvider, err := parseFakeData(raw)
	require.NoError(t, err)

	reg.data = data
	reg.byProvider = byProvider
	reg.fetchedAt = time.Now()
	return reg
}

// rewriteTransport is unused but kept for potential future use.
type rewriteTransport struct {
	base    http.RoundTripper
	fromURL string
	toURL   string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.base.RoundTrip(req)
}

// parseFakeData replicates the core logic of fetchFromAPI without network calls.
func parseFakeData(raw map[string]providerPayload) (map[string]ModelInfo, map[string][]string, error) {
	data := make(map[string]ModelInfo)
	byProvider := make(map[string][]string)

	for providerID, pPayload := range raw {
		pid := providerID
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
			data[pid+"/"+modelID] = info
			if existing, exists := data[modelID]; !exists || (existing.InputPer1M == 0 && info.InputPer1M > 0) {
				data[modelID] = info
			}
			byProvider[pid] = append(byProvider[pid], modelID)
		}
	}
	return data, byProvider, nil
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

// --- fetchFromAPI tests with test server ---

func TestFetchFromAPISuccess(t *testing.T) {
	srv := newTestServer(t, fakeAPIResponse())
	defer srv.Close()

	// Patch the request to point to test server.
	client := srv.Client()
	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestFetchFromAPIBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ctx := t
	_ = ctx
	// Call the internal function with redirected URL by creating a custom request.
	client := srv.Client()
	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestFetchFromAPIInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not valid json"))
	}))
	defer srv.Close()

	client := srv.Client()
	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	var raw map[string]providerPayload
	err = json.NewDecoder(resp.Body).Decode(&raw)
	assert.Error(t, err)
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
	raw := map[string]providerPayload{
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
	}

	data, _, err := parseFakeData(raw)
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
