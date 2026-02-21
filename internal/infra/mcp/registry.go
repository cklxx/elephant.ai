package mcp

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/shared/async"
	"alex/internal/shared/logging"
)

// Registry manages MCP servers and their tools
type Registry struct {
	configLoader     *ConfigLoader
	servers          map[string]*ServerInstance
	toolAdapters     map[string]*ToolAdapter
	mu               sync.RWMutex
	logger           logging.Logger
	ctx              context.Context
	cancel           context.CancelFunc
	playwrightConfig *PlaywrightBrowserConfig
}

// ServerInstance represents a running MCP server
type ServerInstance struct {
	Name         string
	Config       ServerConfig
	Process      *ProcessManager
	Client       *Client
	Status       ServerStatus
	LastError    error
	StartedAt    time.Time
	RestartCount int
}

// ServerStatus represents the server's current state
type ServerStatus string

const (
	StatusStarting ServerStatus = "starting"
	StatusRunning  ServerStatus = "running"
	StatusStopped  ServerStatus = "stopped"
	StatusError    ServerStatus = "error"
)

// RegistryOption customises registry construction.
type RegistryOption func(*Registry)

// WithConfigLoader allows callers to supply a preconfigured MCP config loader.
func WithConfigLoader(loader *ConfigLoader) RegistryOption {
	return func(r *Registry) {
		if loader != nil {
			r.configLoader = loader
		}
	}
}

// NewRegistry creates a new MCP registry
func NewRegistry(opts ...RegistryOption) *Registry {
	ctx, cancel := context.WithCancel(context.Background())
	registry := &Registry{
		configLoader: NewConfigLoader(),
		servers:      make(map[string]*ServerInstance),
		toolAdapters: make(map[string]*ToolAdapter),
		logger:       logging.NewComponentLogger("MCPRegistry"),
		ctx:          ctx,
		cancel:       cancel,
	}
	for _, opt := range opts {
		opt(registry)
	}
	return registry
}

// Initialize loads configuration and starts all MCP servers
func (r *Registry) Initialize() error {
	r.logger.Info("Initializing MCP registry")

	// Load configuration
	config, err := r.configLoader.Load()
	if err != nil {
		r.logger.Warn("Failed to load MCP config: %v", err)
		// Not a fatal error - just means no MCP servers configured
		return nil
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid MCP configuration: %w", err)
	}

	// Start Playwright MCP browser server if configured programmatically.
	if err := r.startPlaywrightIfConfigured(); err != nil {
		r.logger.Error("Failed to start Playwright MCP server: %v", err)
	}

	// Start all active servers from .mcp.json config files.
	activeServers := config.GetActiveServers()
	r.logger.Info("Starting %d MCP servers", len(activeServers))

	for name, serverConfig := range activeServers {
		if err := r.startServer(name, serverConfig); err != nil {
			r.logger.Error("Failed to start server '%s': %v", name, err)
			// Continue with other servers
		}
	}

	// Start health monitoring
	async.Go(r.logger, "mcp.monitorHealth", func() {
		r.monitorHealth()
	})

	return nil
}

// StartServerWithConfig starts a single MCP server using an explicit config.
// It returns the tool adapters loaded for the server (if any).
func (r *Registry) StartServerWithConfig(name string, config ServerConfig) ([]*ToolAdapter, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("server name is required")
	}

	r.mu.RLock()
	if instance, exists := r.servers[name]; exists {
		r.mu.RUnlock()
		if instance.Status == StatusRunning {
			return r.toolsForServer(name), nil
		}
	} else {
		r.mu.RUnlock()
	}

	if config.Disabled {
		return nil, fmt.Errorf("server %s is disabled", name)
	}

	if err := r.startServer(name, config); err != nil {
		return nil, err
	}

	return r.toolsForServer(name), nil
}

// Shutdown stops all MCP servers gracefully
func (r *Registry) Shutdown() error {
	r.logger.Info("Shutting down MCP registry")

	// Cancel context to stop monitoring
	r.cancel()

	r.mu.Lock()
	defer r.mu.Unlock()

	// Stop all servers
	var errors []error
	for name, instance := range r.servers {
		r.logger.Debug("Stopping server: %s", name)
		if err := instance.Client.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop server '%s': %w", name, err))
		}
	}

	// Clear state
	r.servers = make(map[string]*ServerInstance)
	r.toolAdapters = make(map[string]*ToolAdapter)

	if len(errors) > 0 {
		return fmt.Errorf("shutdown errors: %v", errors)
	}

	r.logger.Info("MCP registry shut down successfully")
	return nil
}

// startServer starts a single MCP server
func (r *Registry) startServer(name string, config ServerConfig) error {
	r.logger.Info("Starting MCP server: %s", name)

	// Create process manager
	processConfig := ProcessConfig{
		Command: config.Command,
		Args:    config.Args,
		Env:     config.Env,
	}
	process := NewProcessManager(processConfig)

	// Create client
	client := NewClient(name, process)

	// Create server instance
	instance := &ServerInstance{
		Name:      name,
		Config:    config,
		Process:   process,
		Client:    client,
		Status:    StatusStarting,
		StartedAt: time.Now(),
	}

	// Store instance
	r.mu.Lock()
	r.servers[name] = instance
	r.mu.Unlock()

	// Start client (will start process and initialize)
	if err := client.Start(r.ctx); err != nil {
		instance.Status = StatusError
		instance.LastError = err
		return fmt.Errorf("failed to start client: %w", err)
	}

	instance.Status = StatusRunning
	r.logger.Info("Server '%s' started successfully", name)

	// Load tools from server
	if err := r.loadServerTools(name, client); err != nil {
		r.logger.Error("Failed to load tools from server '%s': %v", name, err)
		// Not fatal - server is running, just no tools loaded
	}

	// Monitor for restarts
	async.Go(r.logger, "mcp.monitorRestart."+name, func() {
		r.monitorServerRestart(name, instance)
	})

	return nil
}

// loadServerTools loads all tools from a server and creates adapters
func (r *Registry) loadServerTools(serverName string, client *Client) error {
	r.logger.Debug("Loading tools from server: %s", serverName)

	// List tools from server
	tools, err := client.ListTools(r.ctx)
	if err != nil {
		return fmt.Errorf("failed to list tools: %w", err)
	}

	r.logger.Info("Server '%s' provides %d tools", serverName, len(tools))

	// Create adapters for each tool
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, toolSchema := range tools {
		adapter := NewToolAdapter(serverName, client, toolSchema)
		toolName := adapter.Metadata().Name

		r.toolAdapters[toolName] = adapter
		r.logger.Debug("Registered tool: %s", toolName)
	}

	return nil
}

// RegisterWithToolRegistry registers all MCP tools with ALEX's tool registry
func (r *Registry) RegisterWithToolRegistry(toolRegistry tools.ToolRegistry) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	r.logger.Info("Registering %d MCP tools with ALEX tool registry", len(r.toolAdapters))

	var errors []error
	for name, adapter := range r.toolAdapters {
		if err := toolRegistry.Register(adapter); err != nil {
			errors = append(errors, fmt.Errorf("failed to register tool '%s': %w", name, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("registration errors: %v", errors)
	}

	r.logger.Info("All MCP tools registered successfully")
	return nil
}

// GetServer retrieves a server instance by name
func (r *Registry) GetServer(name string) (*ServerInstance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	instance, exists := r.servers[name]
	if !exists {
		return nil, fmt.Errorf("server not found: %s", name)
	}

	return instance, nil
}

// ListServers returns all server instances
func (r *Registry) ListServers() []*ServerInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()

	instances := make([]*ServerInstance, 0, len(r.servers))
	for _, instance := range r.servers {
		instances = append(instances, instance)
	}

	return instances
}

// ListTools returns all tool adapters
func (r *Registry) ListTools() []*ToolAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()

	adapters := make([]*ToolAdapter, 0, len(r.toolAdapters))
	for _, adapter := range r.toolAdapters {
		adapters = append(adapters, adapter)
	}

	return adapters
}

func (r *Registry) toolsForServer(serverName string) []*ToolAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()

	adapters := make([]*ToolAdapter, 0)
	for _, adapter := range r.toolAdapters {
		if adapter.serverName == serverName {
			adapters = append(adapters, adapter)
		}
	}
	return adapters
}

// RestartServer restarts a specific server
func (r *Registry) RestartServer(name string) error {
	r.logger.Info("Restarting server: %s", name)

	r.mu.Lock()
	instance, exists := r.servers[name]
	if !exists {
		r.mu.Unlock()
		return fmt.Errorf("server not found: %s", name)
	}
	r.mu.Unlock()

	// Stop client
	if err := instance.Client.Stop(); err != nil {
		r.logger.Warn("Error stopping client during restart: %v", err)
	}

	// Remove old tools
	r.removeServerTools(name)

	// Restart with exponential backoff
	if err := instance.Process.Restart(r.ctx, 3); err != nil {
		instance.Status = StatusError
		instance.LastError = err
		return fmt.Errorf("failed to restart server: %w", err)
	}

	instance.RestartCount++
	instance.Status = StatusRunning
	instance.LastError = nil

	// Reload tools
	if err := r.loadServerTools(name, instance.Client); err != nil {
		r.logger.Error("Failed to reload tools after restart: %v", err)
	}

	r.logger.Info("Server '%s' restarted successfully (restart count: %d)", name, instance.RestartCount)
	return nil
}

// removeServerTools removes all tools from a server
func (r *Registry) removeServerTools(serverName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Find and remove tools from this server
	for toolName, adapter := range r.toolAdapters {
		if adapter.serverName == serverName {
			delete(r.toolAdapters, toolName)
			r.logger.Debug("Removed tool: %s", toolName)
		}
	}
}

// monitorHealth periodically checks server health
func (r *Registry) monitorHealth() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.checkHealth()
		}
	}
}

// checkHealth checks health of all servers
func (r *Registry) checkHealth() {
	r.mu.RLock()
	servers := make([]*ServerInstance, 0, len(r.servers))
	for _, instance := range r.servers {
		servers = append(servers, instance)
	}
	r.mu.RUnlock()

	for _, instance := range servers {
		if !instance.Process.IsRunning() && instance.Status == StatusRunning {
			r.logger.Warn("Server '%s' appears to be down, updating status", instance.Name)
			instance.Status = StatusError
			instance.LastError = fmt.Errorf("process not running")
		}
	}
}

// monitorServerRestart monitors a server for unexpected restarts
func (r *Registry) monitorServerRestart(name string, instance *ServerInstance) {
	restartChan := instance.Process.RestartChannel()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-restartChan:
			r.logger.Warn("Server '%s' crashed, attempting restart", name)
			if err := r.RestartServer(name); err != nil {
				r.logger.Error("Failed to auto-restart server '%s': %v", name, err)
			}
		}
	}
}

// GetServerInfo returns information about a server
func (i *ServerInstance) GetServerInfo() *ServerInfo {
	if i.Client == nil {
		return nil
	}
	return i.Client.GetServerInfo()
}

// GetCapabilities returns the server's capabilities
func (i *ServerInstance) GetCapabilities() *ServerCapabilities {
	if i.Client == nil {
		return nil
	}
	return i.Client.GetCapabilities()
}

// Uptime returns how long the server has been running
func (i *ServerInstance) Uptime() time.Duration {
	return time.Since(i.StartedAt)
}
