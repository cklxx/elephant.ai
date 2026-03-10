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

// ---------------------------------------------------------------------------
// fetchFromAPI — uses httptest to test the actual function
// ---------------------------------------------------------------------------

func TestFetchFromAPI_Success(t *testing.T) {
	srv := newTestServer(t, fakeAPIResponse())
	defer srv.Close()

	// Use a transport that rewrites the const apiURL to the test server.
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = "http"
			req.URL.Host = srv.Listener.Addr().String()
			return http.DefaultTransport.RoundTrip(req)
		}),
	}

	data, byProvider, err := fetchFromAPI(context.Background(), client)
	require.NoError(t, err)

	// Verify compound keys exist.
	info, ok := data["anthropic/claude-3-opus"]
	require.True(t, ok)
	assert.Equal(t, "claude-3-opus", info.ID)
	assert.Equal(t, "anthropic", info.Provider)
	assert.Equal(t, 200000, info.ContextWindow)
	assert.Equal(t, 15.0, info.InputPer1M)
	assert.True(t, info.SupportsVision)

	// Verify bare key.
	_, ok = data["gpt-4o"]
	assert.True(t, ok)

	// Verify byProvider.
	assert.Contains(t, byProvider, "anthropic")
	assert.Contains(t, byProvider, "openai")
	assert.Len(t, byProvider["anthropic"], 2)
}

func TestFetchFromAPI_BadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = "http"
			req.URL.Host = srv.Listener.Addr().String()
			return http.DefaultTransport.RoundTrip(req)
		}),
	}

	_, _, err := fetchFromAPI(context.Background(), client)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status 503")
}

func TestFetchFromAPI_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"broken`))
	}))
	defer srv.Close()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = "http"
			req.URL.Host = srv.Listener.Addr().String()
			return http.DefaultTransport.RoundTrip(req)
		}),
	}

	_, _, err := fetchFromAPI(context.Background(), client)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode JSON")
}

func TestFetchFromAPI_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second) // intentionally slow
	}))
	defer srv.Close()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = "http"
			req.URL.Host = srv.Listener.Addr().String()
			return http.DefaultTransport.RoundTrip(req)
		}),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, _, err := fetchFromAPI(ctx, client)
	require.Error(t, err)
}

func TestFetchFromAPI_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]providerPayload{})
	}))
	defer srv.Close()

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = "http"
			req.URL.Host = srv.Listener.Addr().String()
			return http.DefaultTransport.RoundTrip(req)
		}),
	}

	data, byProvider, err := fetchFromAPI(context.Background(), client)
	require.NoError(t, err)
	assert.Empty(t, data)
	assert.Empty(t, byProvider)
}

// ---------------------------------------------------------------------------
// fetchAndStore
// ---------------------------------------------------------------------------

func TestFetchAndStore_PopulatesRegistryViaTestServer(t *testing.T) {
	srv := newTestServer(t, fakeAPIResponse())
	defer srv.Close()

	reg := &Registry{
		client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = srv.Listener.Addr().String()
				return http.DefaultTransport.RoundTrip(req)
			}),
		},
	}

	reg.loading = true
	reg.fetchAndStore()

	reg.mu.RLock()
	defer reg.mu.RUnlock()
	assert.False(t, reg.loading, "loading flag should be cleared")
	assert.False(t, reg.fetchedAt.IsZero(), "fetchedAt should be set")
	assert.NotNil(t, reg.data)
	assert.Contains(t, reg.data, "anthropic/claude-3-opus")
}

func TestFetchAndStore_ErrorStillAdvancesFetchedAt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	reg := &Registry{
		client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = srv.Listener.Addr().String()
				return http.DefaultTransport.RoundTrip(req)
			}),
		},
	}

	reg.loading = true
	reg.fetchAndStore()

	reg.mu.RLock()
	defer reg.mu.RUnlock()
	assert.False(t, reg.loading)
	assert.False(t, reg.fetchedAt.IsZero(), "fetchedAt should advance even on error")
	assert.Nil(t, reg.data, "data should remain nil on error")
}

// ---------------------------------------------------------------------------
// triggerLoad — goroutine spawning
// ---------------------------------------------------------------------------

func TestTriggerLoad_SpawnsGoroutineWhenStale(t *testing.T) {
	srv := newTestServer(t, fakeAPIResponse())
	defer srv.Close()

	reg := &Registry{
		client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				req.URL.Scheme = "http"
				req.URL.Host = srv.Listener.Addr().String()
				return http.DefaultTransport.RoundTrip(req)
			}),
		},
	}

	reg.triggerLoad()

	// Wait for the goroutine to complete.
	ready := reg.WaitUntilReady(5 * time.Second)
	assert.True(t, ready)

	info, ok := reg.Lookup("claude-3-opus")
	assert.True(t, ok)
	assert.Equal(t, "claude-3-opus", info.ID)
}

func TestTriggerLoad_DoubleCheckPreventsSecondFetch(t *testing.T) {
	// Simulate a race: fetchedAt becomes fresh between RLock and Lock.
	reg := &Registry{
		fetchedAt: time.Now(),
		data:      map[string]ModelInfo{"m": {}},
	}
	// Should not spawn a goroutine.
	reg.triggerLoad()

	reg.mu.RLock()
	assert.False(t, reg.loading)
	reg.mu.RUnlock()
}

// ---------------------------------------------------------------------------
// Bare key priority with sorted providers
// ---------------------------------------------------------------------------

func TestFetchFromAPI_BareKeyPrioritySortedProviders(t *testing.T) {
	srv := newTestServer(t, map[string]providerPayload{
		"zebra": {
			Models: map[string]modelPayload{
				"shared-model": {
					Limit: limitPayload{Context: 100000},
					Cost:  costPayload{Input: 0, Output: 0},
				},
			},
		},
		"alpha": {
			Models: map[string]modelPayload{
				"shared-model": {
					Limit: limitPayload{Context: 100000},
					Cost:  costPayload{Input: 10.0, Output: 30.0},
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

	// "alpha" sorts before "zebra", and has non-zero pricing.
	info := data["shared-model"]
	assert.Equal(t, "alpha", info.Provider)
	assert.Equal(t, 10.0, info.InputPer1M)
}

// ---------------------------------------------------------------------------
// Helper: roundTripFunc
// ---------------------------------------------------------------------------

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
