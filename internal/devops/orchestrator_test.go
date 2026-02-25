package devops

import (
	"context"
	"errors"
	"io"
	"testing"

	"alex/internal/devops/health"
	devlog "alex/internal/devops/log"
)

// mockService is a non-Buildable test service.
type mockService struct {
	name    string
	started bool
	stopped bool
	startFn func() error
	stopFn  func() error
}

func (m *mockService) Name() string { return m.name }
func (m *mockService) State() ServiceState {
	if m.started {
		return StateRunning
	}
	return StateStopped
}
func (m *mockService) Health(_ context.Context) health.Result {
	return health.Result{Healthy: m.started}
}
func (m *mockService) Start(_ context.Context) error {
	if m.startFn != nil {
		return m.startFn()
	}
	m.started = true
	return nil
}
func (m *mockService) Stop(_ context.Context) error {
	if m.stopFn != nil {
		return m.stopFn()
	}
	m.stopped = true
	m.started = false
	return nil
}

// mockBuildableService is a Buildable test service that records call order.
type mockBuildableService struct {
	mockService
	buildErr   error
	promoteErr error
	calls      []string // records "build", "stop", "start", "promote"
}

func (m *mockBuildableService) Build(_ context.Context) (string, error) {
	m.calls = append(m.calls, "build")
	if m.buildErr != nil {
		return "", m.buildErr
	}
	return "/tmp/staging", nil
}

func (m *mockBuildableService) Promote(_ string) error {
	m.calls = append(m.calls, "promote")
	return m.promoteErr
}

func (m *mockBuildableService) Start(ctx context.Context) error {
	m.calls = append(m.calls, "start")
	return m.mockService.Start(ctx)
}

func (m *mockBuildableService) Stop(ctx context.Context) error {
	m.calls = append(m.calls, "stop")
	return m.mockService.Stop(ctx)
}

func newTestOrchestrator() *Orchestrator {
	return &Orchestrator{
		section: devlog.NewSectionWriter(io.Discard, false),
	}
}

func TestRestartBuildableService(t *testing.T) {
	svc := &mockBuildableService{
		mockService: mockService{name: "backend"},
	}
	o := newTestOrchestrator()
	o.RegisterServices(svc)

	err := o.Restart(context.Background(), "backend")
	if err != nil {
		t.Fatalf("Restart() error: %v", err)
	}

	// Expected order: build → stop → promote → start
	want := []string{"build", "stop", "promote", "start"}
	if len(svc.calls) != len(want) {
		t.Fatalf("calls = %v, want %v", svc.calls, want)
	}
	for i, c := range svc.calls {
		if c != want[i] {
			t.Errorf("call[%d] = %q, want %q", i, c, want[i])
		}
	}
}

func TestRestartBuildFailPreservesOld(t *testing.T) {
	svc := &mockBuildableService{
		mockService: mockService{name: "backend", started: true},
		buildErr:    errors.New("compile error"),
	}
	o := newTestOrchestrator()
	o.RegisterServices(svc)

	err := o.Restart(context.Background(), "backend")
	if err == nil {
		t.Fatal("Restart() should return error on build failure")
	}

	// Stop should never have been called
	for _, c := range svc.calls {
		if c == "stop" {
			t.Error("Stop was called despite build failure — old process should be preserved")
		}
	}

	// The service should still be running
	if !svc.started {
		t.Error("service should still be running after build failure")
	}
}

func TestRestartNonBuildableService(t *testing.T) {
	svc := &mockService{name: "web", started: true}
	o := newTestOrchestrator()
	o.RegisterServices(svc)

	err := o.Restart(context.Background(), "web")
	if err != nil {
		t.Fatalf("Restart() error: %v", err)
	}

	if !svc.stopped {
		t.Error("non-buildable service should have been stopped")
	}
	if !svc.started {
		t.Error("non-buildable service should have been started")
	}
}

func TestRestartPromoteFailure(t *testing.T) {
	svc := &mockBuildableService{
		mockService: mockService{name: "backend"},
		promoteErr:  errors.New("rename failed"),
	}
	o := newTestOrchestrator()
	o.RegisterServices(svc)

	err := o.Restart(context.Background())
	if err == nil {
		t.Fatal("Restart() should return error on promote failure")
	}
	if !errors.Is(err, svc.promoteErr) {
		t.Errorf("error should wrap promote error, got: %v", err)
	}
}

func TestResolveTargetsAll(t *testing.T) {
	s1 := &mockService{name: "a"}
	s2 := &mockService{name: "b"}
	o := newTestOrchestrator()
	o.RegisterServices(s1, s2)

	targets := o.resolveTargets(nil)
	if len(targets) != 2 {
		t.Errorf("resolveTargets(nil) returned %d targets, want 2", len(targets))
	}
}

func TestResolveTargetsFiltered(t *testing.T) {
	s1 := &mockService{name: "a"}
	s2 := &mockService{name: "b"}
	s3 := &mockService{name: "c"}
	o := newTestOrchestrator()
	o.RegisterServices(s1, s2, s3)

	targets := o.resolveTargets([]string{"b"})
	if len(targets) != 1 {
		t.Fatalf("resolveTargets([b]) returned %d targets, want 1", len(targets))
	}
	if targets[0].Name() != "b" {
		t.Errorf("target name = %q, want %q", targets[0].Name(), "b")
	}
}
