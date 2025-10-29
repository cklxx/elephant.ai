package app

import (
	"context"
	"sync"
	"time"

	"alex/internal/di"
	"alex/internal/server/ports"
	"alex/internal/tools"
)

// HealthCheckerImpl aggregates health probes for all components
type HealthCheckerImpl struct {
	probes []ports.HealthProbe
	mu     sync.RWMutex
}

// NewHealthChecker creates a new health checker
func NewHealthChecker() *HealthCheckerImpl {
	return &HealthCheckerImpl{
		probes: make([]ports.HealthProbe, 0),
	}
}

// RegisterProbe adds a health probe
func (h *HealthCheckerImpl) RegisterProbe(probe ports.HealthProbe) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.probes = append(h.probes, probe)
}

// CheckAll returns health status for all components
func (h *HealthCheckerImpl) CheckAll(ctx context.Context) []ports.ComponentHealth {
	h.mu.RLock()
	defer h.mu.RUnlock()

	results := make([]ports.ComponentHealth, 0, len(h.probes))
	for _, probe := range h.probes {
		results = append(results, probe.Check(ctx))
	}
	return results
}

type sandboxHealthChecker interface {
	HealthCheck(ctx context.Context) error
}

// SandboxProbe checks sandbox connectivity via the shared manager.
type SandboxProbe struct {
	manager sandboxHealthChecker
}

// NewSandboxProbe constructs a sandbox health probe using the shared manager.
func NewSandboxProbe(manager *tools.SandboxManager) *SandboxProbe {
	return &SandboxProbe{manager: manager}
}

// Check returns the health status of the sandbox endpoint.
func (p *SandboxProbe) Check(ctx context.Context) ports.ComponentHealth {
	if p == nil || p.manager == nil {
		return ports.ComponentHealth{
			Name:    "sandbox",
			Status:  ports.HealthStatusDisabled,
			Message: "Sandbox disabled by configuration",
		}
	}

	if real, ok := p.manager.(*tools.SandboxManager); ok && real == nil {
		return ports.ComponentHealth{
			Name:    "sandbox",
			Status:  ports.HealthStatusDisabled,
			Message: "Sandbox disabled by configuration",
		}
	}

	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := p.manager.HealthCheck(checkCtx); err != nil {
		formatted := tools.FormatSandboxError(err)
		return ports.ComponentHealth{
			Name:    "sandbox",
			Status:  ports.HealthStatusNotReady,
			Message: formatted.Error(),
		}
	}

	return ports.ComponentHealth{
		Name:    "sandbox",
		Status:  ports.HealthStatusReady,
		Message: "Sandbox reachable",
	}
}

// MCPProbe checks MCP registry health
type MCPProbe struct {
	container *di.Container
	enabled   bool
}

// NewMCPProbe creates a new MCP health probe
func NewMCPProbe(container *di.Container, enabled bool) *MCPProbe {
	return &MCPProbe{
		container: container,
		enabled:   enabled,
	}
}

// Check returns the health status of MCP
func (p *MCPProbe) Check(ctx context.Context) ports.ComponentHealth {
	if !p.enabled {
		return ports.ComponentHealth{
			Name:    "mcp",
			Status:  ports.HealthStatusDisabled,
			Message: "MCP disabled by configuration",
		}
	}

	status := p.container.MCPInitializationStatus()

	if status.Ready {
		servers := p.container.MCPRegistry.ListServers()
		tools := p.container.MCPRegistry.ListTools()

		return ports.ComponentHealth{
			Name:    "mcp",
			Status:  ports.HealthStatusReady,
			Message: "MCP initialized successfully",
			Details: map[string]interface{}{
				"servers":      len(servers),
				"tools":        len(tools),
				"attempts":     status.Attempts,
				"last_success": status.LastSuccess,
			},
		}
	}

	if status.Attempts > 0 {
		message := "MCP initialization in progress"
		if status.LastError != nil {
			message = status.LastError.Error()
		}

		return ports.ComponentHealth{
			Name:    "mcp",
			Status:  ports.HealthStatusNotReady,
			Message: message,
			Details: map[string]interface{}{
				"attempts":     status.Attempts,
				"last_attempt": status.LastAttempt,
			},
		}
	}

	return ports.ComponentHealth{
		Name:    "mcp",
		Status:  ports.HealthStatusNotReady,
		Message: "MCP not started",
	}
}

// LLMFactoryProbe checks LLM factory health
type LLMFactoryProbe struct {
	container *di.Container
}

// NewLLMFactoryProbe creates a new LLM factory health probe
func NewLLMFactoryProbe(container *di.Container) *LLMFactoryProbe {
	return &LLMFactoryProbe{
		container: container,
	}
}

// Check returns the health status of LLM factory
func (p *LLMFactoryProbe) Check(ctx context.Context) ports.ComponentHealth {
	// LLM factory is always available once container is built
	// We don't test actual API calls here to avoid external dependencies in health checks
	return ports.ComponentHealth{
		Name:    "llm_factory",
		Status:  ports.HealthStatusReady,
		Message: "LLM factory initialized",
		Details: map[string]interface{}{
			"note": "API connectivity not tested in health check",
		},
	}
}
