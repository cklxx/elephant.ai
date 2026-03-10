package bootstrap

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"alex/internal/app/di"
	"alex/internal/delivery/channels/lark"
	"alex/internal/runtime"
	"alex/internal/runtime/pool"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// newTestRuntime creates a minimal Runtime backed by a temp dir.
// It sets KAKU_BIN to a dummy script so panel.NewManager succeeds.
func newTestRuntime(t *testing.T) *runtime.Runtime {
	t.Helper()

	// Create a fake kaku binary that always succeeds.
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "kaku")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write fake kaku: %v", err)
	}
	t.Setenv("KAKU_BIN", binPath)

	storeDir := filepath.Join(t.TempDir(), "sessions")
	rt, err := runtime.New(storeDir, runtime.Config{})
	if err != nil {
		t.Fatalf("runtime.New: %v", err)
	}
	return rt
}

// bodyJSON decodes the response body as JSON into a map.
func bodyJSON(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		// might be an array; return nil to signal
		return nil
	}
	return m
}

// bodyString returns the response body as a trimmed string.
func bodyString(t *testing.T, resp *http.Response) string {
	t.Helper()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return strings.TrimSpace(string(data))
}

// ---------------------------------------------------------------------------
// RuntimeSessionHandler tests
// ---------------------------------------------------------------------------

func TestRuntimeSessionHandler_PostMissingParentPaneID(t *testing.T) {
	rt := newTestRuntime(t)
	h := NewRuntimeSessionHandler(rt, nil)

	body := `{"member":"claude_code","goal":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/runtime/sessions", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "parent_pane_id required") {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestRuntimeSessionHandler_PostInvalidJSON(t *testing.T) {
	rt := newTestRuntime(t)
	h := NewRuntimeSessionHandler(rt, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/runtime/sessions", strings.NewReader("{bad"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "invalid JSON body") {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestRuntimeSessionHandler_PostExplicitNullParentPaneID(t *testing.T) {
	rt := newTestRuntime(t)
	h := NewRuntimeSessionHandler(rt, nil)

	body := `{"member":"claude_code","goal":"test","parent_pane_id":null}`
	req := httptest.NewRequest(http.MethodPost, "/api/runtime/sessions", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400 for explicit null parent_pane_id", w.Code)
	}
}

func TestRuntimeSessionHandler_PostWithParentPaneID(t *testing.T) {
	rt := newTestRuntime(t)
	h := NewRuntimeSessionHandler(rt, nil)

	// Use -1 to skip pane creation (tracking-only mode).
	body := `{"member":"claude_code","goal":"wiring test","parent_pane_id":-1}`
	req := httptest.NewRequest(http.MethodPost, "/api/runtime/sessions", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("got %d, want 201; body: %s", w.Code, w.Body.String())
	}
}

func TestRuntimeSessionHandler_GetList(t *testing.T) {
	rt := newTestRuntime(t)
	h := NewRuntimeSessionHandler(rt, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/runtime/sessions", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", w.Code)
	}
	// Should be a JSON array (possibly empty).
	var arr []any
	if err := json.Unmarshal(w.Body.Bytes(), &arr); err != nil {
		t.Fatalf("list response is not valid JSON array: %v", err)
	}
}

func TestRuntimeSessionHandler_GetByID_NotFound(t *testing.T) {
	rt := newTestRuntime(t)
	h := NewRuntimeSessionHandler(rt, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/runtime/sessions/nonexistent", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404", w.Code)
	}
}

func TestRuntimeSessionHandler_MethodNotAllowed(t *testing.T) {
	rt := newTestRuntime(t)
	h := NewRuntimeSessionHandler(rt, nil)

	for _, method := range []string{http.MethodDelete, http.MethodPut, http.MethodPatch} {
		req := httptest.NewRequest(method, "/api/runtime/sessions", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: got %d, want 405", method, w.Code)
		}
	}
}

func TestRuntimeSessionHandler_DefaultMember(t *testing.T) {
	rt := newTestRuntime(t)
	h := NewRuntimeSessionHandler(rt, nil)

	// Omit member field; handler should default to claude_code.
	body := `{"goal":"default member test","parent_pane_id":-1}`
	req := httptest.NewRequest(http.MethodPost, "/api/runtime/sessions", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("got %d, want 201; body: %s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// RuntimePoolHandler tests
// ---------------------------------------------------------------------------

func TestRuntimePoolHandler_PostEmptyPaneIDs(t *testing.T) {
	rt := newTestRuntime(t)
	h := NewRuntimePoolHandler(rt, nil)

	body := `{"pane_ids":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/runtime/pool", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "pane_ids required") {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestRuntimePoolHandler_PostMissingPaneIDs(t *testing.T) {
	rt := newTestRuntime(t)
	h := NewRuntimePoolHandler(rt, nil)

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/runtime/pool", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400 for missing pane_ids", w.Code)
	}
}

func TestRuntimePoolHandler_PostInvalidJSON(t *testing.T) {
	rt := newTestRuntime(t)
	h := NewRuntimePoolHandler(rt, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/runtime/pool", strings.NewReader("not-json"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400", w.Code)
	}
}

func TestRuntimePoolHandler_PostNoPoolConfigured(t *testing.T) {
	rt := newTestRuntime(t)
	h := NewRuntimePoolHandler(rt, nil)

	body := `{"pane_ids":[1,2,3]}`
	req := httptest.NewRequest(http.MethodPost, "/api/runtime/pool", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	// Pool is nil → 503 Service Unavailable.
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d, want 503", w.Code)
	}
}

func TestRuntimePoolHandler_PostWithPool(t *testing.T) {
	rt := newTestRuntime(t)
	p := pool.New()
	rt.SetPool(p)
	h := NewRuntimePoolHandler(rt, nil)

	body := `{"pane_ids":[10,20]}`
	req := httptest.NewRequest(http.MethodPost, "/api/runtime/pool", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if resp["registered"] == nil {
		t.Fatal("response missing 'registered' key")
	}
}

func TestRuntimePoolHandler_GetStatusNoPool(t *testing.T) {
	rt := newTestRuntime(t)
	h := NewRuntimePoolHandler(rt, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/runtime/pool", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", w.Code)
	}
	// Should return empty array when pool is nil.
	var arr []any
	if err := json.Unmarshal(w.Body.Bytes(), &arr); err != nil {
		t.Fatalf("response is not valid JSON array: %v", err)
	}
	if len(arr) != 0 {
		t.Fatalf("expected empty array, got %d elements", len(arr))
	}
}

func TestRuntimePoolHandler_GetStatusWithPool(t *testing.T) {
	rt := newTestRuntime(t)
	p := pool.New()
	p.Register([]int{5, 6, 7})
	rt.SetPool(p)
	h := NewRuntimePoolHandler(rt, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/runtime/pool", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", w.Code)
	}
	var arr []any
	if err := json.Unmarshal(w.Body.Bytes(), &arr); err != nil {
		t.Fatalf("response is not valid JSON array: %v", err)
	}
	if len(arr) != 3 {
		t.Fatalf("expected 3 slots, got %d", len(arr))
	}
}

func TestRuntimePoolHandler_MethodNotAllowed(t *testing.T) {
	rt := newTestRuntime(t)
	h := NewRuntimePoolHandler(rt, nil)

	for _, method := range []string{http.MethodDelete, http.MethodPut, http.MethodPatch} {
		req := httptest.NewRequest(method, "/api/runtime/pool", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: got %d, want 405", method, w.Code)
		}
	}
}

// ---------------------------------------------------------------------------
// writeJSON helper tests
// ---------------------------------------------------------------------------

func TestWriteJSON_StatusAndContentType(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusCreated, map[string]string{"key": "val"})

	if w.Code != http.StatusCreated {
		t.Fatalf("got %d, want 201", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}

	var m map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if m["key"] != "val" {
		t.Fatalf("key = %q, want val", m["key"])
	}
}

func TestWriteJSON_NilValue(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, nil)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", w.Code)
	}
}

// ---------------------------------------------------------------------------
// startLarkGateway tests — validation paths
// ---------------------------------------------------------------------------

func TestStartLarkGateway_DisabledConfig(t *testing.T) {
	cfg := Config{}
	// Lark not enabled.
	cleanup, err := startLarkGateway(context.Background(), cfg, nil, nil, nil)
	if err != nil {
		t.Fatalf("disabled config should not error: %v", err)
	}
	if cleanup != nil {
		t.Fatal("disabled config should return nil cleanup")
	}
}

func TestStartLarkGateway_NilContainer(t *testing.T) {
	cfg := Config{
		Channels: ChannelsConfig{Registry: NewChannelRegistry()},
	}
	cfg.Channels.SetLarkConfig(LarkGatewayConfig{Enabled: true})

	_, err := startLarkGateway(context.Background(), cfg, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil container")
	}
	if !strings.Contains(err.Error(), "requires server container") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStartLarkGateway_InvalidToolMode(t *testing.T) {
	cfg := Config{
		Channels: ChannelsConfig{Registry: NewChannelRegistry()},
	}
	cfg.Channels.SetLarkConfig(LarkGatewayConfig{
		Enabled:  true,
		ToolMode: "invalid_mode",
	})

	// Need a non-nil container to get past the container check.
	// We use a minimal placeholder — the function will fail on tool_mode validation.
	container := &dummyContainerForTest{}
	_, err := startLarkGateway(context.Background(), cfg, container.asDI(), nil, nil)
	if err == nil {
		t.Fatal("expected error for invalid tool_mode")
	}
	if !strings.Contains(err.Error(), "tool_mode must be cli or web") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// buildLarkPlanReviewStore tests
// ---------------------------------------------------------------------------

func TestBuildLarkPlanReviewStore_MemoryMode(t *testing.T) {
	cfg := lark.Config{PersistenceMode: "memory", PlanReviewPendingTTL: 60}
	store, err := buildLarkPlanReviewStore(context.Background(), cfg)
	if err != nil {
		t.Fatalf("memory mode failed: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestBuildLarkPlanReviewStore_FileMode(t *testing.T) {
	dir := t.TempDir()
	cfg := lark.Config{PersistenceMode: "file", PersistenceDir: dir, PlanReviewPendingTTL: 60}
	store, err := buildLarkPlanReviewStore(context.Background(), cfg)
	if err != nil {
		t.Fatalf("file mode failed: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestBuildLarkPlanReviewStore_UnsupportedMode(t *testing.T) {
	cfg := lark.Config{PersistenceMode: "redis"}
	_, err := buildLarkPlanReviewStore(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error for unsupported mode")
	}
	if !strings.Contains(err.Error(), "unsupported lark persistence mode") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// buildLarkChatSessionStore tests
// ---------------------------------------------------------------------------

func TestBuildLarkChatSessionStore_MemoryMode(t *testing.T) {
	cfg := lark.Config{PersistenceMode: "memory"}
	store, err := buildLarkChatSessionStore(context.Background(), cfg)
	if err != nil {
		t.Fatalf("memory mode failed: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestBuildLarkChatSessionStore_FileMode(t *testing.T) {
	dir := t.TempDir()
	cfg := lark.Config{PersistenceMode: "file", PersistenceDir: dir}
	store, err := buildLarkChatSessionStore(context.Background(), cfg)
	if err != nil {
		t.Fatalf("file mode failed: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestBuildLarkChatSessionStore_UnsupportedMode(t *testing.T) {
	cfg := lark.Config{PersistenceMode: "postgres"}
	_, err := buildLarkChatSessionStore(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error for unsupported mode")
	}
}

// ---------------------------------------------------------------------------
// buildLarkDeliveryOutboxStore tests
// ---------------------------------------------------------------------------

func TestBuildLarkDeliveryOutboxStore_MemoryMode(t *testing.T) {
	cfg := lark.Config{PersistenceMode: "memory"}
	store, err := buildLarkDeliveryOutboxStore(context.Background(), cfg)
	if err != nil {
		t.Fatalf("memory mode failed: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestBuildLarkDeliveryOutboxStore_FileMode(t *testing.T) {
	dir := t.TempDir()
	cfg := lark.Config{PersistenceMode: "file", PersistenceDir: dir}
	store, err := buildLarkDeliveryOutboxStore(context.Background(), cfg)
	if err != nil {
		t.Fatalf("file mode failed: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestBuildLarkDeliveryOutboxStore_UnsupportedMode(t *testing.T) {
	cfg := lark.Config{PersistenceMode: "mysql"}
	_, err := buildLarkDeliveryOutboxStore(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error for unsupported mode")
	}
}

// ---------------------------------------------------------------------------
// SchedulerStage / TimerManagerStage — disabled paths
// ---------------------------------------------------------------------------

func TestSchedulerStage_Disabled(t *testing.T) {
	f := &Foundation{
		Config: Config{},
		Logger: nil,
	}
	// Scheduler not enabled in config.
	stage := f.SchedulerStage(NewSubsystemManager(nil))
	if stage.Name != "scheduler" {
		t.Fatalf("stage name = %q, want scheduler", stage.Name)
	}
	if stage.Required {
		t.Fatal("scheduler stage should not be required")
	}
	// Init should be a no-op when disabled.
	if err := stage.Init(); err != nil {
		t.Fatalf("disabled scheduler should not error: %v", err)
	}
}

func TestTimerManagerStage_Disabled(t *testing.T) {
	f := &Foundation{
		Config: Config{},
		Logger: nil,
	}
	stage := f.TimerManagerStage(NewSubsystemManager(nil))
	if stage.Name != "timer-manager" {
		t.Fatalf("stage name = %q, want timer-manager", stage.Name)
	}
	if stage.Required {
		t.Fatal("timer-manager stage should not be required")
	}
	if err := stage.Init(); err != nil {
		t.Fatalf("disabled timer-manager should not error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ensureHeartbeatTimer tests
// ---------------------------------------------------------------------------

func TestEnsureHeartbeatTimer_NilManager(t *testing.T) {
	// Should be a safe no-op.
	if err := ensureHeartbeatTimer(nil, 30, nil); err != nil {
		t.Fatalf("nil manager should return nil: %v", err)
	}
}

// ---------------------------------------------------------------------------
// expandHome tests (supplement existing ones)
// ---------------------------------------------------------------------------

func TestExpandHome_TildeSlash(t *testing.T) {
	got := expandHome("~/foo/bar")
	if strings.HasPrefix(got, "~") {
		t.Fatalf("expandHome should expand ~/foo/bar, got %q", got)
	}
	if !strings.HasSuffix(got, "foo/bar") {
		t.Fatalf("expandHome should preserve suffix, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// instrumentedNotifier tests
// ---------------------------------------------------------------------------

func TestInstrumentedNotifier_NilMetrics(t *testing.T) {
	// With nil metrics, should return the base notifier unchanged.
	base := BuildNotifiers(Config{}, "test", nil)
	result := instrumentedNotifier(base, nil, "test_feature")
	if result == nil {
		t.Fatal("instrumentedNotifier should return non-nil even with nil metrics")
	}
}

// ---------------------------------------------------------------------------
// dummyContainerForTest — minimal DI container for validation tests
// ---------------------------------------------------------------------------

type dummyContainerForTest struct{}

func (d *dummyContainerForTest) asDI() *di.Container {
	// Return a minimal container. This will fail on BuildAlternateCoordinator
	// but that's fine — we're testing validation before that point.
	return &di.Container{}
}
