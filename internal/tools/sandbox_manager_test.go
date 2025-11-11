package tools

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"alex/internal/diagnostics"
)

func TestSandboxManagerInitializePublishesErrorProgress(t *testing.T) {
	diagnostics.ResetSandboxProgressForTests()
	t.Cleanup(diagnostics.ResetSandboxProgressForTests)

	mgr := NewSandboxManager("")
	ctx := context.Background()
	err := mgr.Initialize(ctx)
	if err == nil {
		t.Fatalf("expected initialization error for empty base URL")
	}

	latest, ok := diagnostics.LatestSandboxProgress()
	if !ok {
		t.Fatalf("expected sandbox progress to be recorded")
	}
	if latest.Status != diagnostics.SandboxProgressError {
		t.Fatalf("expected error status, got %q", latest.Status)
	}
	if latest.Stage != "configure_client" {
		t.Fatalf("expected configure_client stage, got %q", latest.Stage)
	}
}

func TestSandboxManagerInitializeWithoutDocker(t *testing.T) {
	diagnostics.ResetSandboxProgressForTests()
	t.Cleanup(diagnostics.ResetSandboxProgressForTests)

	server := newSandboxHealthServer(t)

	mgr := newSandboxManager(server.URL, nil)
	ctx := context.Background()

	if err := mgr.Initialize(ctx); err != nil {
		t.Fatalf("unexpected initialization error: %v", err)
	}

	if mgr.client == nil {
		t.Fatalf("expected sandbox client to be initialised")
	}

	latest, ok := diagnostics.LatestSandboxProgress()
	if !ok {
		t.Fatalf("expected sandbox progress to be recorded")
	}
	if latest.Status != diagnostics.SandboxProgressReady {
		t.Fatalf("expected ready status, got %q", latest.Status)
	}
}

func TestSandboxManagerInitializeWithDocker(t *testing.T) {
	diagnostics.ResetSandboxProgressForTests()
	t.Cleanup(diagnostics.ResetSandboxProgressForTests)

	server := newSandboxHealthServer(t)
	docker := &stubSandboxDockerController{result: SandboxDockerResult{Started: true, Image: "test-image"}}

	mgr := newSandboxManager(server.URL, docker)
	ctx := context.Background()

	if err := mgr.Initialize(ctx); err != nil {
		t.Fatalf("unexpected initialization error: %v", err)
	}

	if docker.calls != 1 {
		t.Fatalf("expected docker EnsureRunning to be called once, got %d", docker.calls)
	}
	if docker.baseURL != server.URL {
		t.Fatalf("expected docker to receive base URL %q, got %q", server.URL, docker.baseURL)
	}

	if err := mgr.Initialize(ctx); err != nil {
		t.Fatalf("unexpected initialization error on second call: %v", err)
	}

	if docker.calls != 1 {
		t.Fatalf("expected docker EnsureRunning to be called only once, got %d", docker.calls)
	}

	latest, ok := diagnostics.LatestSandboxProgress()
	if !ok {
		t.Fatalf("expected sandbox progress to be recorded")
	}
	if latest.Status != diagnostics.SandboxProgressReady {
		t.Fatalf("expected ready status, got %q", latest.Status)
	}
}

type stubSandboxDockerController struct {
	result  SandboxDockerResult
	err     error
	calls   int
	baseURL string
}

func (s *stubSandboxDockerController) EnsureRunning(ctx context.Context, baseURL string) (SandboxDockerResult, error) {
	s.calls++
	s.baseURL = baseURL
	return s.result, s.err
}

func newSandboxHealthServer(t *testing.T) *httptest.Server {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/shell/exec" {
			http.NotFound(w, r)
			return
		}

		defer func() {
			_ = r.Body.Close()
		}()
		var req struct {
			Command string `json:"command"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode shell exec request: %v", err)
		}
		if req.Command == "" {
			t.Fatalf("expected command to be provided")
		}

		output := "alex-sandbox-health"

		type consoleRecord struct {
			PS1     string  `json:"ps1"`
			Command string  `json:"command"`
			Output  *string `json:"output,omitempty"`
		}

		type shellExecResponse struct {
			Success bool `json:"success"`
			Data    struct {
				SessionID string          `json:"session_id"`
				Command   string          `json:"command"`
				Status    string          `json:"status"`
				Output    string          `json:"output"`
				Console   []consoleRecord `json:"console,omitempty"`
			} `json:"data"`
		}

		resp := shellExecResponse{Success: true}
		resp.Data.SessionID = "session-123"
		resp.Data.Command = req.Command
		resp.Data.Status = "completed"
		resp.Data.Output = output
		resp.Data.Console = []consoleRecord{{
			PS1:     "$",
			Command: req.Command,
			Output:  &output,
		}}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("failed to encode shell exec response: %v", err)
		}
	})

	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("skipping sandbox manager test: unable to create loopback listener: %v", err)
	}

	server := httptest.NewUnstartedServer(handler)
	server.Listener = ln
	server.Start()
	t.Cleanup(server.Close)
	return server
}

func TestFormatSandboxErrorBadGateway(t *testing.T) {
	err := errors.New(`Post "http://sandbox/v1/shell/exec": 502: <html><body><h1>502 Bad Gateway</h1></body></html>`)
	formatted := FormatSandboxError(err)
	if formatted.Error() != "sandbox gateway unavailable (502)" {
		t.Fatalf("expected 502 message, got %q", formatted.Error())
	}
}

func TestFormatSandboxErrorSanitizesHTML(t *testing.T) {
	err := errors.New(`<html><body><p>Failure &amp; stack</p></body></html>`)
	formatted := FormatSandboxError(err)
	expected := "sandbox error: Failure & stack"
	if formatted.Error() != expected {
		t.Fatalf("expected sanitized message %q, got %q", expected, formatted.Error())
	}
}
