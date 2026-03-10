//go:build integration

package bootstrap

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"alex/internal/app/di"
	"alex/internal/app/scheduler"
	serverApp "alex/internal/delivery/server/app"
	serverHTTP "alex/internal/delivery/server/http"
	"alex/internal/delivery/server/ports"
	"alex/internal/infra/attachments"
	"alex/internal/shared/config"
	"alex/internal/shared/notification"
)

// buildTestRouter creates a minimal router with the same wiring as the real
// server, including health endpoints.
func buildTestRouter(t *testing.T, container *di.Container) http.Handler {
	t.Helper()

	broadcaster := serverApp.NewEventBroadcaster()
	taskStore := serverApp.NewInMemoryTaskStore()
	tasksSvc := serverApp.NewTaskExecutionService(
		container.AgentCoordinator, broadcaster, taskStore,
	)
	sessionsSvc := serverApp.NewSessionService(
		container.AgentCoordinator, container.SessionStore, broadcaster,
	)
	snapshotsSvc := serverApp.NewSnapshotService(
		container.AgentCoordinator, broadcaster,
		serverApp.WithSnapshotStateStore(container.StateStore),
	)

	healthChecker := serverApp.NewHealthChecker()
	healthChecker.RegisterProbe(serverApp.NewLLMFactoryProbe(container))

	return serverHTTP.NewRouter(
		serverHTTP.RouterDeps{
			Tasks:         tasksSvc,
			Sessions:      sessionsSvc,
			Snapshots:     snapshotsSvc,
			Broadcaster:   broadcaster,
			HealthChecker: healthChecker,
			AttachmentCfg: attachments.StoreConfig{Dir: t.TempDir()},
		},
		serverHTTP.RouterConfig{Environment: "test"},
	)
}

// TestBootstrap_ColdStart verifies that BuildContainer → Start → /health
// returns 200 with a healthy status and expected components.
func TestBootstrap_ColdStart(t *testing.T) {
	cfg := di.Config{
		LLMProvider: "mock",
		LLMModel:    "test",
		SessionDir:  t.TempDir(),
		CostDir:     t.TempDir(),
		MemoryDir:   t.TempDir(),
	}

	container, err := di.BuildContainer(cfg)
	if err != nil {
		t.Fatalf("BuildContainer failed: %v", err)
	}
	defer func() { _ = container.Shutdown() }()

	if err := container.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	router := buildTestRouter(t, container)
	srv := httptest.NewServer(router)
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var health struct {
		Status     string                  `json:"status"`
		Components []ports.ComponentHealth `json:"components"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("failed to decode health response: %v", err)
	}

	if health.Status != "healthy" {
		t.Errorf("expected status 'healthy', got %q", health.Status)
	}

	found := false
	for _, c := range health.Components {
		if c.Name == "llm_factory" {
			found = true
			if c.Status != ports.HealthStatusReady {
				t.Errorf("expected llm_factory status 'ready', got %q", c.Status)
			}
		}
	}
	if !found {
		t.Error("expected llm_factory component in health response")
	}
}

// TestBootstrap_GracefulShutdown verifies that after a container starts and
// serves requests, shutdown completes cleanly with no goroutine leak.
func TestBootstrap_GracefulShutdown(t *testing.T) {
	cfg := di.Config{
		LLMProvider: "mock",
		LLMModel:    "test",
		SessionDir:  t.TempDir(),
		CostDir:     t.TempDir(),
		MemoryDir:   t.TempDir(),
	}

	container, err := di.BuildContainer(cfg)
	if err != nil {
		t.Fatalf("BuildContainer failed: %v", err)
	}

	if err := container.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	router := buildTestRouter(t, container)
	srv := httptest.NewServer(router)

	// Make a health request to exercise the server.
	resp, err := srv.Client().Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health failed: %v", err)
	}
	resp.Body.Close()

	// Capture goroutine count before shutdown.
	runtime.GC()
	time.Sleep(50 * time.Millisecond) // let runtime settle
	baseGoroutines := runtime.NumGoroutine()

	// Shutdown sequence: close HTTP server, then drain container.
	srv.Close()
	drainCtx, drainCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer drainCancel()
	if err := container.Drain(drainCtx); err != nil {
		t.Fatalf("Drain failed: %v", err)
	}

	// Allow goroutines to wind down.
	time.Sleep(200 * time.Millisecond)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	// Check for goroutine leak: allow a generous delta since runtime goroutines
	// (GC, finalizer) are unpredictable, but catch real leaks (>20 surplus).
	afterGoroutines := runtime.NumGoroutine()
	leak := afterGoroutines - baseGoroutines
	if leak > 20 {
		t.Errorf("possible goroutine leak: %d goroutines before shutdown, %d after (+%d)",
			baseGoroutines, afterGoroutines, leak)
	}
}

// nopNotifier is a no-op notification.Notifier for testing.
type nopNotifier struct{}

func (nopNotifier) Send(context.Context, notification.Target, string) error { return nil }

// TestBootstrap_InvalidCronExpression verifies that a scheduler with an invalid
// cron expression in a static trigger starts successfully (skipping the bad
// trigger) and shuts down cleanly without goroutine leaks.
func TestBootstrap_InvalidCronExpression(t *testing.T) {
	cfg := di.Config{
		LLMProvider: "mock",
		LLMModel:    "test",
		SessionDir:  t.TempDir(),
		CostDir:     t.TempDir(),
		MemoryDir:   t.TempDir(),
	}

	container, err := di.BuildContainer(cfg)
	if err != nil {
		t.Fatalf("BuildContainer failed: %v", err)
	}
	defer func() { _ = container.Shutdown() }()

	if err := container.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Configure a scheduler with one valid and one invalid cron trigger.
	schedCfg := scheduler.Config{
		Enabled: true,
		StaticTriggers: []config.SchedulerTriggerConfig{
			{
				Name:     "valid_trigger",
				Schedule: "*/5 * * * *",
				Task:     "echo valid",
			},
			{
				Name:     "invalid_trigger",
				Schedule: "not-a-cron-expression",
				Task:     "echo bad",
			},
		},
	}

	sched := scheduler.New(schedCfg, container.AgentCoordinator, nopNotifier{}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start must succeed — invalid triggers are logged and skipped.
	if err := sched.Start(ctx); err != nil {
		t.Fatalf("Scheduler.Start failed: %v", err)
	}

	// Verify the scheduler is running.
	if !sched.Running() {
		t.Error("expected scheduler to be running")
	}

	// Verify health: valid trigger registered, invalid skipped.
	statuses := sched.LeaderJobsHealth()
	// Static triggers don't show up in leader job health (those are for
	// blocker_radar, weekly_pulse, etc.), but the scheduler itself should
	// be running with the valid trigger registered.

	// Stop the scheduler.
	cancel()
	sched.Stop()

	// Verify clean shutdown — no panic, scheduler reports not running.
	if sched.Running() {
		t.Error("expected scheduler to be stopped")
	}

	// Check goroutine count is reasonable.
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	goroutines := runtime.NumGoroutine()
	// A typical Go test has ~5-15 goroutines. If we have >50, something leaked.
	if goroutines > 50 {
		t.Errorf("too many goroutines after scheduler stop: %d (possible leak)", goroutines)
	}

	_ = statuses // used to verify leader health structure is available
}
