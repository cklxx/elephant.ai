package mcp

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"alex/internal/tools/builtin"
	"alex/internal/tools/mcp/protocol"
	"alex/internal/tools/mcp/transport"
)

// ServerHealth represents health status of a server
type ServerHealth struct {
	LastChecked time.Time
	IsHealthy   bool
	ErrorCount  int
	LastError   string
}

// Manager manages MCP clients and servers
type Manager struct {
	config        *MCPConfig
	serverManager *ServerManager
	clients       map[string]*Client
	transports    map[string]Transport
	toolRegistry  *MCPToolRegistry
	healthStatus  map[string]*ServerHealth
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewManager creates a new MCP manager
func NewManager(config *MCPConfig) *Manager {
	return &Manager{
		config:        config,
		serverManager: NewServerManager(),
		clients:       make(map[string]*Client),
		transports:    make(map[string]Transport),
		toolRegistry:  NewMCPToolRegistry(),
		healthStatus:  make(map[string]*ServerHealth),
	}
}

// Start starts the MCP manager
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled {
		return nil
	}

	m.ctx, m.cancel = context.WithCancel(ctx)

	// Start auto-start servers
	enabledServers := m.config.GetEnabledServers()
	for _, serverConfig := range enabledServers {
		if serverConfig.AutoStart {
			if err := m.startServer(serverConfig); err != nil {
				return fmt.Errorf("failed to start server %s: %w", serverConfig.ID, err)
			}
		}
	}

	// Start auto-refresh if enabled
	if m.config.AutoRefresh {
		go m.autoRefreshLoop()
	}

	return nil
}

// Stop stops the MCP manager
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cancel != nil {
		m.cancel()
	}

	// Stop all clients
	for _, client := range m.clients {
		_ = client.Disconnect()
	}

	// Stop all servers
	for id, transportInterface := range m.transports {
		if serverConfig, exists := m.config.Servers[id]; exists {
			if stdioTransport, ok := transportInterface.(*transport.StdioTransport); ok {
				_ = m.serverManager.StopServer(context.Background(), serverConfig, stdioTransport)
			}
		}
	}

	m.clients = make(map[string]*Client)
	m.transports = make(map[string]Transport)

	return nil
}

// startServer starts an MCP server
func (m *Manager) startServer(serverConfig *ServerConfig) error {
	// Perform health check for HTTP servers
	if !m.checkServerHealth(serverConfig) {
		fmt.Printf("[WARN] Skipping unhealthy server %s\n", serverConfig.ID)
		return fmt.Errorf("server %s failed health check", serverConfig.ID)
	}
	// Convert config types
	mcpServerConfig := &ServerConfig{
		ID:          serverConfig.ID,
		Name:        serverConfig.Name,
		Type:        SpawnerType(serverConfig.Type),
		Command:     serverConfig.Command,
		Args:        serverConfig.Args,
		Env:         serverConfig.Env,
		WorkDir:     serverConfig.WorkDir,
		AutoStart:   serverConfig.AutoStart,
		AutoRestart: serverConfig.AutoRestart,
		Timeout:     serverConfig.Timeout,
		Enabled:     serverConfig.Enabled,
	}

	// Handle HTTP servers specially - check BEFORE spawning
	var clientTransport Transport

	if mcpServerConfig.Type == SpawnerTypeHTTP {
		// Create SSE transport directly for HTTP servers
		sseConfig := &transport.SSETransportConfig{
			Endpoint: mcpServerConfig.Command,
			Headers:  make(map[string]string),
			Timeout:  mcpServerConfig.Timeout,
		}

		// Add any custom headers from env
		for k, v := range mcpServerConfig.Env {
			if strings.HasPrefix(k, "HEADER_") {
				headerName := strings.TrimPrefix(k, "HEADER_")
				sseConfig.Headers[headerName] = v
			}
		}

		clientTransport = transport.NewSSETransport(sseConfig)
		fmt.Printf("[INFO] Manager: Created SSE transport for %s\n", mcpServerConfig.Command)
	} else {
		// Use spawner for other server types
		stdioTransport, err := m.serverManager.SpawnServer(m.ctx, mcpServerConfig)
		if err != nil {
			return fmt.Errorf("failed to spawn server: %w", err)
		}
		clientTransport = stdioTransport
	}

	// Create client
	client := NewClient(&ClientConfig{
		ClientInfo: protocol.ClientInfo{
			Name:    "Alex",
			Version: "1.0.0",
		},
		Capabilities: protocol.ClientCapabilities{
			Roots: &protocol.RootsCapability{
				ListChanged: true,
			},
			Sampling: &protocol.SamplingCapability{},
		},
		Transport: clientTransport,
		Timeout:   serverConfig.Timeout,
	})

	// Connect client using manager's long-term context
	// Don't use timeout context here as it would cancel all future operations
	if err := client.Connect(m.ctx, &ClientConfig{
		ClientInfo: protocol.ClientInfo{
			Name:    "Alex",
			Version: "1.0.0",
		},
		Capabilities: protocol.ClientCapabilities{
			Roots: &protocol.RootsCapability{
				ListChanged: true,
			},
			Sampling: &protocol.SamplingCapability{},
		},
		Transport: clientTransport,
		Timeout:   serverConfig.Timeout,
	}); err != nil {
		_ = clientTransport.Disconnect()
		fmt.Printf("[WARN] Connection timeout or failed for server %s: %v\n", serverConfig.ID, err)

		// Update health status
		if health, exists := m.healthStatus[serverConfig.ID]; exists {
			health.ErrorCount++
			health.LastError = err.Error()
			health.IsHealthy = false
		}

		return fmt.Errorf("failed to connect client: %w", err)
	}

	// Store client and transport
	m.clients[serverConfig.ID] = client
	m.transports[serverConfig.ID] = clientTransport

	// Register tools
	if err := m.toolRegistry.RegisterClient(serverConfig.ID, client); err != nil {
		return fmt.Errorf("failed to register tools: %w", err)
	}

	return nil
}

// StopServer stops a specific MCP server
func (m *Manager) StopServer(serverID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, exists := m.clients[serverID]
	if !exists {
		return fmt.Errorf("server %s not found", serverID)
	}

	// Disconnect client
	_ = client.Disconnect()

	// Stop server
	if transportInterface, exists := m.transports[serverID]; exists {
		if serverConfig, exists := m.config.Servers[serverID]; exists {
			mcpServerConfig := &ServerConfig{
				ID:          serverConfig.ID,
				Name:        serverConfig.Name,
				Type:        SpawnerType(serverConfig.Type),
				Command:     serverConfig.Command,
				Args:        serverConfig.Args,
				Env:         serverConfig.Env,
				WorkDir:     serverConfig.WorkDir,
				AutoStart:   serverConfig.AutoStart,
				AutoRestart: serverConfig.AutoRestart,
				Timeout:     serverConfig.Timeout,
				Enabled:     serverConfig.Enabled,
			}
			if stdioTransport, ok := transportInterface.(*transport.StdioTransport); ok {
				_ = m.serverManager.StopServer(context.Background(), mcpServerConfig, stdioTransport)
			}
		}
	}

	// Unregister tools
	m.toolRegistry.UnregisterClient(serverID)

	// Clean up
	delete(m.clients, serverID)
	delete(m.transports, serverID)

	return nil
}

// StartServer starts a specific MCP server
func (m *Manager) StartServer(serverID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	serverConfig, exists := m.config.Servers[serverID]
	if !exists {
		return fmt.Errorf("server config %s not found", serverID)
	}

	if !serverConfig.Enabled {
		return fmt.Errorf("server %s is disabled", serverID)
	}

	// Check if already running
	if _, exists := m.clients[serverID]; exists {
		return fmt.Errorf("server %s is already running", serverID)
	}

	return m.startServer(serverConfig)
}

// RestartServer restarts a specific MCP server
func (m *Manager) RestartServer(serverID string) error {
	if err := m.StopServer(serverID); err != nil {
		return fmt.Errorf("failed to stop server: %w", err)
	}

	return m.StartServer(serverID)
}

// GetClient returns an MCP client by server ID
func (m *Manager) GetClient(serverID string) (*Client, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, exists := m.clients[serverID]
	return client, exists
}

// ListActiveServers returns the IDs of all active servers
func (m *Manager) ListActiveServers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.clients))
	for id := range m.clients {
		ids = append(ids, id)
	}
	return ids
}

// GetServerStatus returns the status of a server
func (m *Manager) GetServerStatus(serverID string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, exists := m.clients[serverID]
	if !exists {
		return false, nil
	}

	return client.IsConnected(), nil
}

// IntegrateWithBuiltinTools integrates MCP tools with Alex's builtin tool list
func (m *Manager) IntegrateWithBuiltinTools(existingTools []builtin.Tool) []builtin.Tool {
	return m.toolRegistry.IntegrateWithBuiltinTools(existingTools)
}

// GetToolRegistry returns the MCP tool registry
func (m *Manager) GetToolRegistry() *MCPToolRegistry {
	return m.toolRegistry
}

// autoRefreshLoop periodically refreshes tools from all servers
func (m *Manager) autoRefreshLoop() {
	ticker := time.NewTicker(m.config.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			if err := m.refreshTools(); err != nil {
				// Log error but continue
				// TODO: Add proper logging
				continue
			}
		}
	}
}

// refreshTools refreshes tools from all active servers
func (m *Manager) refreshTools() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.toolRegistry.RefreshTools()
}

// AddServerConfig adds a new server configuration
func (m *Manager) AddServerConfig(serverConfig *ServerConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := ValidateServerConfig(serverConfig); err != nil {
		return fmt.Errorf("invalid server config: %w", err)
	}

	if m.config.Servers == nil {
		m.config.Servers = make(map[string]*ServerConfig)
	}

	m.config.Servers[serverConfig.ID] = serverConfig

	// Auto-start if enabled and manager is running
	if serverConfig.AutoStart && serverConfig.Enabled && m.ctx != nil {
		return m.startServer(serverConfig)
	}

	return nil
}

// RemoveServerConfig removes a server configuration
func (m *Manager) RemoveServerConfig(serverID string) error {
	// Stop server if running
	if _, exists := m.clients[serverID]; exists {
		if err := m.StopServer(serverID); err != nil {
			return fmt.Errorf("failed to stop server before removal: %w", err)
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.config.Servers != nil {
		delete(m.config.Servers, serverID)
	}

	return nil
}

// UpdateConfig updates the MCP configuration
func (m *Manager) UpdateConfig(newConfig *MCPConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := newConfig.ValidateConfig(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Stop if disabling
	if !newConfig.Enabled && m.config.Enabled {
		for id := range m.clients {
			_ = m.StopServer(id)
		}
	}

	m.config = newConfig

	// Start if enabling
	if newConfig.Enabled && m.ctx != nil {
		enabledServers := newConfig.GetEnabledServers()
		for _, serverConfig := range enabledServers {
			if serverConfig.AutoStart {
				if _, exists := m.clients[serverConfig.ID]; !exists {
					_ = m.startServer(serverConfig)
				}
			}
		}
	}

	return nil
}

// GetConfig returns the current MCP configuration
func (m *Manager) GetConfig() *MCPConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// IsEnabled returns whether MCP is enabled
func (m *Manager) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.Enabled
}

// checkServerHealth performs a health check for HTTP servers
func (m *Manager) checkServerHealth(serverConfig *ServerConfig) bool {
	if serverConfig.Type != SpawnerTypeHTTP {
		return true // Skip health check for non-HTTP servers
	}

	// Check cache first
	if health, exists := m.healthStatus[serverConfig.ID]; exists {
		if time.Since(health.LastChecked) < 5*time.Minute && health.ErrorCount < 3 {
			return health.IsHealthy
		}
	}

	// Perform health check
	endpoint := serverConfig.Command
	if !strings.HasPrefix(endpoint, "http") {
		return false
	}

	// Try a simple HEAD request with short timeout
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Head(endpoint)

	isHealthy := false
	errorMsg := ""

	if err != nil {
		errorMsg = err.Error()
	} else {
		_ = resp.Body.Close()
		// Accept any response (even 404) as "server is reachable"
		isHealthy = resp.StatusCode < 500
	}

	// Update health status
	health := m.healthStatus[serverConfig.ID]
	if health == nil {
		health = &ServerHealth{}
		m.healthStatus[serverConfig.ID] = health
	}

	health.LastChecked = time.Now()
	health.IsHealthy = isHealthy
	health.LastError = errorMsg

	if !isHealthy {
		health.ErrorCount++
	} else {
		health.ErrorCount = 0
	}

	if !isHealthy && health.ErrorCount <= 1 {
		fmt.Printf("[WARN] MCP server %s health check failed: %s\n", serverConfig.ID, errorMsg)
	}

	return isHealthy
}
