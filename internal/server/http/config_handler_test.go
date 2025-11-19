package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	runtimeconfig "alex/internal/config"
	configadmin "alex/internal/config/admin"
)

type memoryStore struct {
	overrides runtimeconfig.Overrides
}

func (s *memoryStore) LoadOverrides(context.Context) (runtimeconfig.Overrides, error) {
	return s.overrides, nil
}

func (s *memoryStore) SaveOverrides(_ context.Context, overrides runtimeconfig.Overrides) error {
	s.overrides = overrides
	return nil
}

func TestConfigHandlerHandleGetRuntimeConfig(t *testing.T) {
	t.Parallel()

	initial := runtimeconfig.Overrides{}
	manager := configadmin.NewManager(&memoryStore{overrides: initial}, initial)
	resolver := func(context.Context) (runtimeconfig.RuntimeConfig, runtimeconfig.Metadata, error) {
		cfg := runtimeconfig.RuntimeConfig{LLMProvider: "mock"}
		meta := runtimeconfig.Metadata{}
		return cfg, meta, nil
	}

	handler := NewConfigHandler(manager, resolver)

	req := httptest.NewRequest(http.MethodGet, "/api/internal/config/runtime", nil)
	rr := httptest.NewRecorder()

	handler.HandleGetRuntimeConfig(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var payload runtimeConfigResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.Effective.LLMProvider != "mock" {
		t.Fatalf("expected effective config from resolver, got %+v", payload.Effective)
	}
	if len(payload.Tasks) == 0 {
		t.Fatalf("expected readiness tasks to be included in payload")
	}
}

func TestConfigHandlerHandleUpdateRuntimeConfig(t *testing.T) {
	t.Parallel()

	mem := &memoryStore{}
	manager := configadmin.NewManager(mem, runtimeconfig.Overrides{})
	resolver := func(context.Context) (runtimeconfig.RuntimeConfig, runtimeconfig.Metadata, error) {
		cfg := runtimeconfig.RuntimeConfig{LLMModel: "gpt-4"}
		meta := runtimeconfig.Metadata{}
		return cfg, meta, nil
	}

	handler := NewConfigHandler(manager, resolver)

	body := runtimeConfigOverridesPayload{Overrides: runtimeconfig.Overrides{LLMProvider: ptrString("openrouter")}}
	data, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/api/internal/config/runtime", bytes.NewReader(data))
	rr := httptest.NewRecorder()

	handler.HandleUpdateRuntimeConfig(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	overrides, err := manager.CurrentOverrides(context.Background())
	if err != nil {
		t.Fatalf("CurrentOverrides returned error: %v", err)
	}
	if overrides.LLMProvider == nil || *overrides.LLMProvider != "openrouter" {
		t.Fatalf("expected override to be persisted, got %#v", overrides.LLMProvider)
	}

	var payload runtimeConfigResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(payload.Tasks) == 0 {
		t.Fatalf("expected readiness tasks to be returned after update")
	}
}

func TestConfigHandlerHandleRuntimeStreamSendsSnapshots(t *testing.T) {
	t.Parallel()

	mem := &memoryStore{}
	manager := configadmin.NewManager(mem, runtimeconfig.Overrides{})
	resolver := func(context.Context) (runtimeconfig.RuntimeConfig, runtimeconfig.Metadata, error) {
		cfg := runtimeconfig.RuntimeConfig{LLMProvider: "initial"}
		meta := runtimeconfig.Metadata{}
		return cfg, meta, nil
	}

	handler := NewConfigHandler(manager, resolver)

	req := httptest.NewRequest(http.MethodGet, "/api/internal/config/runtime/stream", nil)
	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()
	req = req.WithContext(ctx)
	writer := &sseRecorder{header: http.Header{}}

	done := make(chan struct{})
	go func() {
		handler.HandleRuntimeStream(writer, req)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	llm := "after"
	if err := manager.UpdateOverrides(context.Background(), runtimeconfig.Overrides{LLMProvider: &llm}); err != nil {
		t.Fatalf("UpdateOverrides returned error: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	cancel()

	<-done

	payloads := writer.payloads()
	if len(payloads) < 2 {
		t.Fatalf("expected at least two SSE payloads, got %d", len(payloads))
	}
	if !strings.Contains(payloads[len(payloads)-1], "after") {
		t.Fatalf("expected SSE stream to include updated overrides, got %q", payloads)
	}
}

type runtimeConfigOverridesPayload struct {
	Overrides runtimeconfig.Overrides `json:"overrides"`
}

type sseRecorder struct {
	header http.Header
	buf    bytes.Buffer
}

func (s *sseRecorder) Header() http.Header {
	return s.header
}

func (s *sseRecorder) Write(data []byte) (int, error) {
	return s.buf.Write(data)
}

func (s *sseRecorder) WriteHeader(statusCode int) {}

func (s *sseRecorder) Flush() {}

func (s *sseRecorder) payloads() []string {
	raw := s.buf.String()
	parts := strings.Split(strings.TrimSpace(raw), "\n\n")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		out = append(out, part)
	}
	return out
}

func ptrString(value string) *string {
	return &value
}
