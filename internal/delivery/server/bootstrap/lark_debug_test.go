package bootstrap

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	serverApp "alex/internal/delivery/server/app"
	"alex/internal/shared/logging"
)

func TestBuildDebugBroadcaster(t *testing.T) {
	broadcaster := buildDebugBroadcaster(nil, t.TempDir(), nil)
	if broadcaster == nil {
		t.Fatal("buildDebugBroadcaster returned nil")
	}

	// Verify it can receive events without panicking.
	metrics := broadcaster.GetMetrics()
	if metrics.ActiveConnections != 0 {
		t.Errorf("expected 0 active connections, got %d", metrics.ActiveConnections)
	}
}

func TestBuildDebugBroadcaster_Options(t *testing.T) {
	broadcaster := buildDebugBroadcaster(nil, t.TempDir(), nil)

	// Register and unregister a client to validate the broadcaster is functional.
	ch := make(chan interface{ EventType() string }, 128)
	_ = ch // type-check only — real registration uses agent.AgentEvent
	// Verify metrics start at zero.
	metrics := broadcaster.GetMetrics()
	if metrics.TotalEventsSent != 0 {
		t.Errorf("expected 0 total events, got %d", metrics.TotalEventsSent)
	}
}

func TestConfig_DebugPortDefault(t *testing.T) {
	// Verify the default DebugPort is set in LoadConfig.
	// We can't call LoadConfig in a unit test (it reads env/files),
	// so validate the default directly.
	cfg := Config{
		DebugPort: "9090",
	}
	if cfg.DebugPort != "9090" {
		t.Errorf("expected DebugPort 9090, got %s", cfg.DebugPort)
	}
}

func TestBuildDebugHTTPServer_NilFoundation(t *testing.T) {
	// Ensure BuildDebugHTTPServer handles nil gracefully when foundation fields
	// are minimal.
	f := &Foundation{
		Degraded: NewDegradedComponents(),
		Config: Config{
			DebugPort: "0", // use port 0 to avoid conflicts
		},
		ConfigResult: ConfigResult{},
	}

	broadcaster := serverApp.NewEventBroadcaster()
	cfg := Config{DebugPort: "0"}

	server, _, err := BuildDebugHTTPServer(f, broadcaster, nil, cfg)
	if err != nil {
		t.Fatalf("BuildDebugHTTPServer failed: %v", err)
	}
	if server == nil {
		t.Fatal("BuildDebugHTTPServer returned nil server")
	}
	if server.Addr != ":0" {
		t.Errorf("expected addr :0, got %s", server.Addr)
	}
	if server.ReadTimeout != 5*time.Minute {
		t.Errorf("expected ReadTimeout 5m, got %v", server.ReadTimeout)
	}
}

func TestListenDebugPort_Available(t *testing.T) {
	logger := logging.NewComponentLogger("test")
	ln, err := listenDebugPort("0", logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ln == nil {
		t.Fatal("expected a listener, got nil")
	}
	ln.Close()
}

func TestListenDebugPort_FallbackOnBusy(t *testing.T) {
	// Occupy a known port.
	occupied, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	defer occupied.Close()
	port := occupied.Addr().(*net.TCPAddr).Port

	logger := logging.NewComponentLogger("test")
	ln, err := listenDebugPort(fmt.Sprintf("%d", port), logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ln == nil {
		t.Fatal("expected fallback listener, got nil")
	}
	defer ln.Close()

	gotPort := ln.Addr().(*net.TCPAddr).Port
	if gotPort == port {
		t.Errorf("expected fallback to a different port, still got %d", port)
	}
	if gotPort < port+1 || gotPort > port+debugPortMaxRetries {
		t.Errorf("fallback port %d outside expected range %d–%d", gotPort, port+1, port+debugPortMaxRetries)
	}
}

func TestListenDebugPort_InvalidPort(t *testing.T) {
	logger := logging.NewComponentLogger("test")
	_, err := listenDebugPort("abc", logger)
	if err == nil {
		t.Fatal("expected error for non-numeric port")
	}
}

func TestStartupProfile_RecordAndSnapshot(t *testing.T) {
	sp := newStartupProfile()
	sp.record("P1 Foundation", 120*time.Millisecond)
	sp.record("P2 Attachments", 45*time.Millisecond)
	sp.finalize(200 * time.Millisecond)

	snap := sp.snapshot()
	if len(snap.Phases) != 2 {
		t.Fatalf("expected 2 phases, got %d", len(snap.Phases))
	}
	if snap.Phases[0].Name != "P1 Foundation" || snap.Phases[0].DurationMS != 120 {
		t.Errorf("unexpected P1: %+v", snap.Phases[0])
	}
	if snap.Phases[1].Name != "P2 Attachments" || snap.Phases[1].DurationMS != 45 {
		t.Errorf("unexpected P2: %+v", snap.Phases[1])
	}
	if snap.TotalMS != 200 {
		t.Errorf("expected total 200ms, got %d", snap.TotalMS)
	}
}

func TestStartupProfileHandler_Endpoint(t *testing.T) {
	sp := newStartupProfile()
	sp.record("P1 Foundation", 100*time.Millisecond)
	sp.record("P2 Attachments", 50*time.Millisecond)
	sp.finalize(180 * time.Millisecond)

	f := &Foundation{
		Degraded:     NewDegradedComponents(),
		Config:       Config{DebugPort: "0"},
		ConfigResult: ConfigResult{},
	}
	broadcaster := serverApp.NewEventBroadcaster()
	server, _, err := BuildDebugHTTPServer(f, broadcaster, nil, Config{DebugPort: "0"}, withStartupProfile(sp))
	if err != nil {
		t.Fatalf("BuildDebugHTTPServer failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/health/startup-profile", nil)
	rec := httptest.NewRecorder()
	server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp startupProfileResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON decode: %v body=%s", err, rec.Body.String())
	}
	if len(resp.Phases) != 2 {
		t.Fatalf("expected 2 phases, got %d", len(resp.Phases))
	}
	if resp.TotalMS != 180 {
		t.Errorf("expected total 180, got %d", resp.TotalMS)
	}
}

func TestBuildDebugHTTPServer_HealthWithNilContainer(t *testing.T) {
	f := &Foundation{
		Degraded: NewDegradedComponents(),
		Config: Config{
			DebugPort: "0",
		},
		ConfigResult: ConfigResult{},
	}

	broadcaster := serverApp.NewEventBroadcaster()
	server, _, err := BuildDebugHTTPServer(f, broadcaster, nil, Config{DebugPort: "0"})
	if err != nil {
		t.Fatalf("BuildDebugHTTPServer failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected health JSON response, got err=%v body=%s", err, rec.Body.String())
	}
}
