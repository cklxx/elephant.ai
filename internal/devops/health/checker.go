package health

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Result captures the outcome of a health probe.
type Result struct {
	Healthy bool
	Message string
	Latency time.Duration
}

// ProbeType defines the kind of health check.
type ProbeType int

const (
	ProbeHTTP ProbeType = iota
	ProbeTCP
	ProbeProcess
)

// Probe configures a health check for a service.
type Probe struct {
	Type     ProbeType
	Target   string        // URL for HTTP, host:port for TCP, PID file for Process
	Timeout  time.Duration // per-check timeout
	Interval time.Duration // between checks when waiting
}

// Checker manages health probes for services.
type Checker struct {
	probes map[string]Probe
	client *http.Client
}

// NewChecker creates a new health checker.
func NewChecker() *Checker {
	return &Checker{
		probes: make(map[string]Probe),
		client: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				DisableKeepAlives: true,
			},
		},
	}
}

// Register adds a probe for a named service.
func (c *Checker) Register(name string, probe Probe) {
	if probe.Timeout == 0 {
		probe.Timeout = 5 * time.Second
	}
	if probe.Interval == 0 {
		probe.Interval = 1 * time.Second
	}
	c.probes[name] = probe
}

// Check performs a single health check for a named service.
func (c *Checker) Check(ctx context.Context, name string) Result {
	probe, ok := c.probes[name]
	if !ok {
		return Result{
			Healthy: false,
			Message: fmt.Sprintf("no probe registered for %s", name),
		}
	}

	start := time.Now()
	var healthy bool
	var message string

	switch probe.Type {
	case ProbeHTTP:
		healthy, message = c.checkHTTP(ctx, probe)
	case ProbeTCP:
		healthy, message = c.checkTCP(ctx, probe)
	case ProbeProcess:
		healthy, message = c.checkProcess(probe)
	default:
		message = fmt.Sprintf("unknown probe type %d", probe.Type)
	}

	return Result{
		Healthy: healthy,
		Message: message,
		Latency: time.Since(start),
	}
}

// WaitHealthy blocks until the named service is healthy or the timeout elapses.
func (c *Checker) WaitHealthy(ctx context.Context, name string, timeout time.Duration) error {
	probe, ok := c.probes[name]
	if !ok {
		return fmt.Errorf("no probe registered for %s", name)
	}

	deadline := time.Now().Add(timeout)
	interval := probe.Interval
	if interval == 0 {
		interval = 1 * time.Second
	}

	for time.Now().Before(deadline) {
		result := c.Check(ctx, name)
		if result.Healthy {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}

	return fmt.Errorf("%s did not become healthy within %s", name, timeout)
}

func (c *Checker) checkHTTP(ctx context.Context, probe Probe) (bool, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probe.Target, nil)
	if err != nil {
		return false, fmt.Sprintf("invalid URL: %v", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return false, fmt.Sprintf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return true, fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return false, fmt.Sprintf("HTTP %d", resp.StatusCode)
}

func (c *Checker) checkTCP(ctx context.Context, probe Probe) (bool, string) {
	var d net.Dialer
	d.Timeout = probe.Timeout
	conn, err := d.DialContext(ctx, "tcp", probe.Target)
	if err != nil {
		return false, fmt.Sprintf("TCP connect failed: %v", err)
	}
	conn.Close()
	return true, "TCP connected"
}

func (c *Checker) checkProcess(probe Probe) (bool, string) {
	pidStr := probe.Target
	if data, err := os.ReadFile(pidStr); err == nil {
		pidStr = strings.TrimSpace(string(data))
	}

	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return false, fmt.Sprintf("invalid PID: %s", pidStr)
	}

	if err := syscall.Kill(pid, 0); err != nil {
		return false, fmt.Sprintf("process %d not running", pid)
	}
	return true, fmt.Sprintf("process %d alive", pid)
}
