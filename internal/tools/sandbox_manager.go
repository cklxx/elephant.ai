package tools

import (
        "bufio"
        "context"
        "fmt"
        "strings"
        "sync"
        "time"

        api "github.com/agent-infra/sandbox-sdk-go"
        "github.com/agent-infra/sandbox-sdk-go/browser"
        sandboxclient "github.com/agent-infra/sandbox-sdk-go/client"
        "github.com/agent-infra/sandbox-sdk-go/file"
        "github.com/agent-infra/sandbox-sdk-go/jupyter"
        "github.com/agent-infra/sandbox-sdk-go/option"
        "github.com/agent-infra/sandbox-sdk-go/shell"
)

// SandboxManager lazily initialises and shares sandbox SDK clients across tools.
type SandboxManager struct {
	baseURL string

	client        *sandboxclient.Client
	envSnapshot   map[string]string
	initOnce      sync.Once
	initErr       error
	environmentMu sync.Mutex
}

// NewSandboxManager constructs a manager for a given sandbox endpoint.
func NewSandboxManager(baseURL string) *SandboxManager {
	return &SandboxManager{baseURL: baseURL}
}

// Initialize ensures the underlying SDK clients are ready for use.
func (m *SandboxManager) Initialize(ctx context.Context) error {
	m.initOnce.Do(func() {
		if strings.TrimSpace(m.baseURL) == "" {
			m.initErr = fmt.Errorf("sandbox base URL is required")
			return
		}

		m.client = sandboxclient.NewClient(option.WithBaseURL(m.baseURL))
		m.initErr = m.healthCheck(ctx)
	})
	return m.initErr
}

func (m *SandboxManager) healthCheck(ctx context.Context) error {
	if m.client == nil {
		return fmt.Errorf("sandbox client not initialised")
	}

	req := &api.ShellExecRequest{Command: "echo 'alex-sandbox-health'"}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := m.client.Shell.ExecCommand(ctx, req)
	if err != nil {
		return err
	}
	data := resp.GetData()
	if data == nil {
		return fmt.Errorf("sandbox health check returned empty data")
	}
	if output := data.GetOutput(); output == nil || !strings.Contains(*output, "alex-sandbox-health") {
		return fmt.Errorf("unexpected sandbox health check output")
	}
	return nil
}

// File returns the shared sandbox file client. Initialize must be called first.
func (m *SandboxManager) File() *file.Client {
	if m.client == nil {
		return nil
	}
	return m.client.File
}

// Shell returns the shared sandbox shell client. Initialize must be called first.
func (m *SandboxManager) Shell() *shell.Client {
        if m.client == nil {
                return nil
        }
        return m.client.Shell
}

// Client exposes the underlying aggregate sandbox client for advanced scenarios.
func (m *SandboxManager) Client() *sandboxclient.Client {
        return m.client
}

// Browser returns the shared sandbox browser client. Initialize must be called first.
func (m *SandboxManager) Browser() *browser.Client {
        if m.client == nil {
                return nil
        }
        return m.client.Browser
}

// Jupyter returns the shared sandbox jupyter client. Initialize must be called first.
func (m *SandboxManager) Jupyter() *jupyter.Client {
	if m.client == nil {
		return nil
	}
	return m.client.Jupyter
}

// Environment retrieves a cached snapshot of sandbox environment variables.
func (m *SandboxManager) Environment(ctx context.Context) (map[string]string, error) {
	if err := m.Initialize(ctx); err != nil {
		return nil, err
	}

	m.environmentMu.Lock()
	defer m.environmentMu.Unlock()

	if m.envSnapshot != nil {
		// Return a shallow copy to protect internal cache.
		snapshot := make(map[string]string, len(m.envSnapshot))
		for k, v := range m.envSnapshot {
			snapshot[k] = v
		}
		return snapshot, nil
	}

	req := &api.ShellExecRequest{Command: "printenv"}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := m.client.Shell.ExecCommand(ctx, req)
	if err != nil {
		return nil, err
	}
	data := resp.GetData()
	if data == nil || data.GetOutput() == nil {
		return nil, fmt.Errorf("sandbox environment response missing output")
	}

	m.envSnapshot = parseEnv(*data.GetOutput())
	snapshot := make(map[string]string, len(m.envSnapshot))
	for k, v := range m.envSnapshot {
		snapshot[k] = v
	}
	return snapshot, nil
}

func parseEnv(stdout string) map[string]string {
	vars := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(stdout))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		vars[parts[0]] = parts[1]
	}
	return vars
}

// HealthCheck forces a fresh sandbox health probe.
func (m *SandboxManager) HealthCheck(ctx context.Context) error {
	if err := m.Initialize(ctx); err != nil {
		return err
	}
	return m.healthCheck(ctx)
}

// FormatSandboxError maps low-level sandbox SDK errors into user-friendly messages.
func FormatSandboxError(err error) error {
	if err == nil {
		return nil
	}

	message := err.Error()
	switch {
	case strings.Contains(message, "connection refused"):
		message = "sandbox unreachable - check SANDBOX_BASE_URL"
	case strings.Contains(message, "timeout"):
		message = "sandbox operation timed out"
	case strings.Contains(message, "not found"):
		message = "resource not found in sandbox"
	default:
		message = fmt.Sprintf("sandbox error: %s", message)
	}
	return fmt.Errorf("%s", message)
}
