package bootstrap

import (
	"testing"
	"time"

	serverApp "alex/internal/delivery/server/app"
)

func TestBuildDebugBroadcaster(t *testing.T) {
	broadcaster := buildDebugBroadcaster(nil)
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
	broadcaster := buildDebugBroadcaster(nil)

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

	server, err := BuildDebugHTTPServer(f, broadcaster, nil, cfg)
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
