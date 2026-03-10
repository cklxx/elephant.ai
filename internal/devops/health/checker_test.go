package health

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// --- NewChecker ---

func TestNewChecker(t *testing.T) {
	c := NewChecker()
	if c == nil {
		t.Fatal("NewChecker returned nil")
	}
	if c.probes == nil {
		t.Fatal("probes map not initialized")
	}
	if c.client == nil {
		t.Fatal("http client not initialized")
	}
}

// --- Register ---

func TestRegister_DefaultTimeouts(t *testing.T) {
	c := NewChecker()
	c.Register("svc", Probe{Type: ProbeHTTP, Target: "http://localhost"})

	probe := c.probes["svc"]
	if probe.Timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want 5s", probe.Timeout)
	}
	if probe.Interval != time.Second {
		t.Errorf("Interval = %v, want 1s", probe.Interval)
	}
}

func TestRegister_CustomTimeouts(t *testing.T) {
	c := NewChecker()
	c.Register("svc", Probe{
		Type:     ProbeHTTP,
		Target:   "http://localhost",
		Timeout:  10 * time.Second,
		Interval: 2 * time.Second,
	})

	probe := c.probes["svc"]
	if probe.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want 10s", probe.Timeout)
	}
	if probe.Interval != 2*time.Second {
		t.Errorf("Interval = %v, want 2s", probe.Interval)
	}
}

func TestRegister_Overwrite(t *testing.T) {
	c := NewChecker()
	c.Register("svc", Probe{Type: ProbeHTTP, Target: "http://old"})
	c.Register("svc", Probe{Type: ProbeTCP, Target: "localhost:8080"})

	if c.probes["svc"].Type != ProbeTCP {
		t.Error("register should overwrite existing probe")
	}
}

// --- Check: unregistered ---

func TestCheck_Unregistered(t *testing.T) {
	c := NewChecker()
	result := c.Check(context.Background(), "nonexistent")
	if result.Healthy {
		t.Error("unregistered probe should not be healthy")
	}
	if result.Message == "" {
		t.Error("expected error message for unregistered probe")
	}
}

// --- Check: HTTP ---

func TestCheck_HTTP_Healthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewChecker()
	c.Register("web", Probe{Type: ProbeHTTP, Target: srv.URL})

	result := c.Check(context.Background(), "web")
	if !result.Healthy {
		t.Errorf("expected healthy, got: %s", result.Message)
	}
	if result.Latency == 0 {
		t.Error("latency should be > 0")
	}
}

func TestCheck_HTTP_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewChecker()
	c.Register("web", Probe{Type: ProbeHTTP, Target: srv.URL})

	result := c.Check(context.Background(), "web")
	if result.Healthy {
		t.Error("HTTP 500 should not be healthy")
	}
}

func TestCheck_HTTP_Redirect(t *testing.T) {
	// 3xx should be considered healthy (< 400).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Go client follows redirects, so we just return 200 in the end.
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewChecker()
	c.Register("web", Probe{Type: ProbeHTTP, Target: srv.URL})

	result := c.Check(context.Background(), "web")
	if !result.Healthy {
		t.Error("expected healthy for 2xx")
	}
}

func TestCheck_HTTP_ConnectionRefused(t *testing.T) {
	c := NewChecker()
	c.Register("web", Probe{Type: ProbeHTTP, Target: "http://127.0.0.1:1"})

	result := c.Check(context.Background(), "web")
	if result.Healthy {
		t.Error("connection refused should not be healthy")
	}
}

func TestCheck_HTTP_InvalidURL(t *testing.T) {
	c := NewChecker()
	c.Register("web", Probe{Type: ProbeHTTP, Target: "://bad"})

	result := c.Check(context.Background(), "web")
	if result.Healthy {
		t.Error("invalid URL should not be healthy")
	}
}

// --- Check: TCP ---

func TestCheck_TCP_Healthy(t *testing.T) {
	// Start a TCP listener.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	c := NewChecker()
	c.Register("tcp-svc", Probe{Type: ProbeTCP, Target: ln.Addr().String()})

	result := c.Check(context.Background(), "tcp-svc")
	if !result.Healthy {
		t.Errorf("TCP should be healthy: %s", result.Message)
	}
}

func TestCheck_TCP_Refused(t *testing.T) {
	c := NewChecker()
	c.Register("tcp-svc", Probe{Type: ProbeTCP, Target: "127.0.0.1:1", Timeout: 100 * time.Millisecond})

	result := c.Check(context.Background(), "tcp-svc")
	if result.Healthy {
		t.Error("TCP connection to closed port should not be healthy")
	}
}

// --- Check: Process ---

func TestCheck_Process_CurrentPID(t *testing.T) {
	pid := os.Getpid()
	c := NewChecker()
	c.Register("proc", Probe{Type: ProbeProcess, Target: strconv.Itoa(pid)})

	result := c.Check(context.Background(), "proc")
	if !result.Healthy {
		t.Errorf("current process should be healthy: %s", result.Message)
	}
}

func TestCheck_Process_InvalidPID(t *testing.T) {
	c := NewChecker()
	c.Register("proc", Probe{Type: ProbeProcess, Target: "not-a-number"})

	result := c.Check(context.Background(), "proc")
	if result.Healthy {
		t.Error("invalid PID should not be healthy")
	}
}

func TestCheck_Process_NonExistentPID(t *testing.T) {
	// PID 99999999 is very unlikely to exist.
	c := NewChecker()
	c.Register("proc", Probe{Type: ProbeProcess, Target: "99999999"})

	result := c.Check(context.Background(), "proc")
	if result.Healthy {
		t.Error("non-existent PID should not be healthy")
	}
}

func TestCheck_Process_PIDFile(t *testing.T) {
	pid := os.Getpid()
	pidFile := filepath.Join(t.TempDir(), "test.pid")
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("  %d\n", pid)), 0644); err != nil {
		t.Fatalf("write pid file: %v", err)
	}

	c := NewChecker()
	c.Register("proc", Probe{Type: ProbeProcess, Target: pidFile})

	result := c.Check(context.Background(), "proc")
	if !result.Healthy {
		t.Errorf("process from PID file should be healthy: %s", result.Message)
	}
}

func TestCheck_Process_PIDFileInvalidContent(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "bad.pid")
	if err := os.WriteFile(pidFile, []byte("garbage"), 0644); err != nil {
		t.Fatalf("write pid file: %v", err)
	}

	c := NewChecker()
	c.Register("proc", Probe{Type: ProbeProcess, Target: pidFile})

	result := c.Check(context.Background(), "proc")
	if result.Healthy {
		t.Error("garbage PID file should not be healthy")
	}
}

// --- Check: unknown probe type ---

func TestCheck_UnknownProbeType(t *testing.T) {
	c := NewChecker()
	c.probes["unknown"] = Probe{Type: ProbeType(999), Target: "test"}

	result := c.Check(context.Background(), "unknown")
	if result.Healthy {
		t.Error("unknown probe type should not be healthy")
	}
	if result.Message == "" {
		t.Error("expected message for unknown probe type")
	}
}

// --- WaitHealthy ---

func TestWaitHealthy_ImmediatelyHealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewChecker()
	c.Register("web", Probe{Type: ProbeHTTP, Target: srv.URL, Interval: 50 * time.Millisecond})

	err := c.WaitHealthy(context.Background(), "web", 5*time.Second)
	if err != nil {
		t.Errorf("WaitHealthy: %v", err)
	}
}

func TestWaitHealthy_Timeout(t *testing.T) {
	c := NewChecker()
	c.Register("dead", Probe{Type: ProbeTCP, Target: "127.0.0.1:1", Timeout: 50 * time.Millisecond, Interval: 50 * time.Millisecond})

	err := c.WaitHealthy(context.Background(), "dead", 200*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestWaitHealthy_Unregistered(t *testing.T) {
	c := NewChecker()
	err := c.WaitHealthy(context.Background(), "missing", time.Second)
	if err == nil {
		t.Fatal("expected error for unregistered probe")
	}
}

func TestWaitHealthy_ContextCancelled(t *testing.T) {
	c := NewChecker()
	c.Register("dead", Probe{Type: ProbeTCP, Target: "127.0.0.1:1", Timeout: 50 * time.Millisecond, Interval: 50 * time.Millisecond})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	err := c.WaitHealthy(ctx, "dead", 5*time.Second)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestWaitHealthy_BecomesHealthy(t *testing.T) {
	// Start with a failing server, then make it healthy.
	healthy := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if healthy {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer srv.Close()

	c := NewChecker()
	c.Register("web", Probe{Type: ProbeHTTP, Target: srv.URL, Interval: 50 * time.Millisecond})

	// Make healthy after a short delay.
	go func() {
		time.Sleep(150 * time.Millisecond)
		healthy = true
	}()

	err := c.WaitHealthy(context.Background(), "web", 2*time.Second)
	if err != nil {
		t.Errorf("WaitHealthy should succeed after server becomes healthy: %v", err)
	}
}

// --- Result fields ---

func TestCheck_LatencyPopulated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewChecker()
	c.Register("web", Probe{Type: ProbeHTTP, Target: srv.URL})

	result := c.Check(context.Background(), "web")
	if result.Latency <= 0 {
		t.Error("expected positive latency")
	}
}

// --- ProbeType constants ---

func TestProbeTypeConstants(t *testing.T) {
	if ProbeHTTP != 0 {
		t.Errorf("ProbeHTTP = %d, want 0", ProbeHTTP)
	}
	if ProbeTCP != 1 {
		t.Errorf("ProbeTCP = %d, want 1", ProbeTCP)
	}
	if ProbeProcess != 2 {
		t.Errorf("ProbeProcess = %d, want 2", ProbeProcess)
	}
}
