package mcp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"alex/internal/tools/mcp/protocol"
	"alex/internal/tools/mcp/transport"
)

// SpawnerType represents different types of server spawners
type SpawnerType string

const (
	SpawnerTypeNPX        SpawnerType = "npx"
	SpawnerTypeExecutable SpawnerType = "executable"
	SpawnerTypeDocker     SpawnerType = "docker"
	SpawnerTypeHTTP       SpawnerType = "http"
)

// ServerConfig represents configuration for an MCP server
type ServerConfig struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        SpawnerType       `json:"type"`
	Command     string            `json:"command"`
	Args        []string          `json:"args"`
	Env         map[string]string `json:"env"`
	WorkDir     string            `json:"workDir"`
	AutoStart   bool              `json:"autoStart"`
	AutoRestart bool              `json:"autoRestart"`
	Timeout     time.Duration     `json:"timeout"`
	Enabled     bool              `json:"enabled"`
}

// Spawner interface for different server spawning strategies
type Spawner interface {
	Spawn(ctx context.Context, config *ServerConfig) (*transport.StdioTransport, error)
	Stop(ctx context.Context, transport *transport.StdioTransport) error
	IsRunning(transport *transport.StdioTransport) bool
}

// NPXSpawner implements spawning MCP servers via npx
type NPXSpawner struct {
	mu            sync.RWMutex
	activeServers map[string]*transport.StdioTransport
}

// NewNPXSpawner creates a new NPX spawner
func NewNPXSpawner() *NPXSpawner {
	return &NPXSpawner{
		activeServers: make(map[string]*transport.StdioTransport),
	}
}

// Spawn spawns an MCP server using npx
func (s *NPXSpawner) Spawn(ctx context.Context, config *ServerConfig) (*transport.StdioTransport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if server is already running
	if existing, exists := s.activeServers[config.ID]; exists {
		if existing.IsConnected() {
			return existing, nil
		}
		// Clean up dead connection
		delete(s.activeServers, config.ID)
	}

	// Validate NPX availability
	if err := s.validateNPX(); err != nil {
		return nil, fmt.Errorf("npx validation failed: %w", err)
	}

	// Prepare command and args
	var command string
	var args []string

	switch config.Type {
	case SpawnerTypeNPX:
		command = "npx"
		args = append([]string{"-y"}, config.Args...)
		if config.Command != "" {
			args = append(args, config.Command)
		}
	case SpawnerTypeExecutable:
		command = config.Command
		args = config.Args
	default:
		return nil, fmt.Errorf("unsupported spawner type: %s", config.Type)
	}

	// Prepare environment
	env := os.Environ()
	for k, v := range config.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// Set working directory
	workDir := config.WorkDir
	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	// Create transport configuration
	transportConfig := &transport.StdioTransportConfig{
		Command: command,
		Args:    args,
		Env:     env,
		WorkDir: workDir,
	}

	// Create and connect transport
	stdioTransport := transport.NewStdioTransport(transportConfig)
	if err := stdioTransport.ConnectWithConfig(ctx, transportConfig); err != nil {
		return nil, fmt.Errorf("failed to connect to MCP server: %w", err)
	}

	// Store active server
	s.activeServers[config.ID] = stdioTransport

	return stdioTransport, nil
}

// Stop stops an MCP server
func (s *NPXSpawner) Stop(ctx context.Context, transport *transport.StdioTransport) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if transport == nil {
		return nil
	}

	// Find and remove from active servers
	for id, t := range s.activeServers {
		if t == transport {
			delete(s.activeServers, id)
			break
		}
	}

	return transport.Disconnect()
}

// IsRunning checks if a transport is still running
func (s *NPXSpawner) IsRunning(transport *transport.StdioTransport) bool {
	if transport == nil {
		return false
	}
	return transport.IsConnected()
}

// validateNPX validates that npx is available
func (s *NPXSpawner) validateNPX() error {
	cmd := exec.Command("npx", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npx not found or not executable: %w", err)
	}
	return nil
}

// GetActiveServers returns a copy of active servers
func (s *NPXSpawner) GetActiveServers() map[string]*transport.StdioTransport {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*transport.StdioTransport)
	for id, transport := range s.activeServers {
		result[id] = transport
	}
	return result
}

// ExecutableSpawner implements spawning MCP servers via local executables
type ExecutableSpawner struct {
	mu            sync.RWMutex
	activeServers map[string]*transport.StdioTransport
}

// NewExecutableSpawner creates a new executable spawner
func NewExecutableSpawner() *ExecutableSpawner {
	return &ExecutableSpawner{
		activeServers: make(map[string]*transport.StdioTransport),
	}
}

// Spawn spawns an MCP server using a local executable
func (s *ExecutableSpawner) Spawn(ctx context.Context, config *ServerConfig) (*transport.StdioTransport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if server is already running
	if existing, exists := s.activeServers[config.ID]; exists {
		if existing.IsConnected() {
			return existing, nil
		}
		// Clean up dead connection
		delete(s.activeServers, config.ID)
	}

	// Validate executable
	if err := s.validateExecutable(config.Command); err != nil {
		return nil, fmt.Errorf("executable validation failed: %w", err)
	}

	// Prepare environment
	env := os.Environ()
	for k, v := range config.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// Set working directory
	workDir := config.WorkDir
	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	// Create transport configuration
	transportConfig := &transport.StdioTransportConfig{
		Command: config.Command,
		Args:    config.Args,
		Env:     env,
		WorkDir: workDir,
	}

	// Create and connect transport
	stdioTransport := transport.NewStdioTransport(transportConfig)
	if err := stdioTransport.ConnectWithConfig(ctx, transportConfig); err != nil {
		return nil, fmt.Errorf("failed to connect to MCP server: %w", err)
	}

	// Store active server
	s.activeServers[config.ID] = stdioTransport

	return stdioTransport, nil
}

// Stop stops an MCP server
func (s *ExecutableSpawner) Stop(ctx context.Context, transport *transport.StdioTransport) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if transport == nil {
		return nil
	}

	// Find and remove from active servers
	for id, t := range s.activeServers {
		if t == transport {
			delete(s.activeServers, id)
			break
		}
	}

	return transport.Disconnect()
}

// IsRunning checks if a transport is still running
func (s *ExecutableSpawner) IsRunning(transport *transport.StdioTransport) bool {
	if transport == nil {
		return false
	}
	return transport.IsConnected()
}

// validateExecutable validates that the executable exists and is executable
func (s *ExecutableSpawner) validateExecutable(command string) error {
	// Check if it's an absolute path
	if filepath.IsAbs(command) {
		if _, err := os.Stat(command); err != nil {
			return fmt.Errorf("executable not found: %s", command)
		}
		return nil
	}

	// Check if it's in PATH
	if _, err := exec.LookPath(command); err != nil {
		return fmt.Errorf("executable not found in PATH: %s", command)
	}

	return nil
}

// GetActiveServers returns a copy of active servers
func (s *ExecutableSpawner) GetActiveServers() map[string]*transport.StdioTransport {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*transport.StdioTransport)
	for id, transport := range s.activeServers {
		result[id] = transport
	}
	return result
}

// SSETransportAdapter adapts SSE transport to StdioTransport interface
type SSETransportAdapter struct {
	sseTransport *transport.SSETransport
	mu           sync.RWMutex
	connected    bool
}

// Connect establishes the SSE connection
func (a *SSETransportAdapter) Connect(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := a.sseTransport.Connect(ctx); err != nil {
		return err
	}

	a.connected = true
	return nil
}

// Disconnect closes the SSE connection
func (a *SSETransportAdapter) Disconnect() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := a.sseTransport.Disconnect(); err != nil {
		return err
	}

	a.connected = false
	return nil
}

// IsConnected returns the connection status
func (a *SSETransportAdapter) IsConnected() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.connected && a.sseTransport.IsConnected()
}

// SendRequest forwards to SSE transport
func (a *SSETransportAdapter) SendRequest(req *protocol.JSONRPCRequest) (*protocol.JSONRPCResponse, error) {
	return a.sseTransport.SendRequest(req)
}

// SendNotification forwards to SSE transport
func (a *SSETransportAdapter) SendNotification(notification *protocol.JSONRPCNotification) error {
	return a.sseTransport.SendNotification(notification)
}

// ReceiveMessages forwards to SSE transport
func (a *SSETransportAdapter) ReceiveMessages() <-chan []byte {
	return a.sseTransport.ReceiveMessages()
}

// ReceiveErrors forwards to SSE transport
func (a *SSETransportAdapter) ReceiveErrors() <-chan error {
	return a.sseTransport.ReceiveErrors()
}

// NextRequestID forwards to SSE transport
func (a *SSETransportAdapter) NextRequestID() int64 {
	return a.sseTransport.NextRequestID()
}

// HTTPSpawner implements spawning MCP servers via HTTP/SSE transport
type HTTPSpawner struct {
	mu            sync.RWMutex
	activeServers map[string]*transport.StdioTransport
}

// NewHTTPSpawner creates a new HTTP spawner
func NewHTTPSpawner() *HTTPSpawner {
	return &HTTPSpawner{
		activeServers: make(map[string]*transport.StdioTransport),
	}
}

// Spawn connects to an MCP server via HTTP/SSE
func (s *HTTPSpawner) Spawn(ctx context.Context, config *ServerConfig) (*transport.StdioTransport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if server is already connected
	if existing, exists := s.activeServers[config.ID]; exists {
		if existing.IsConnected() {
			return existing, nil
		}
		// Clean up dead connection
		delete(s.activeServers, config.ID)
	}

	// For HTTP transport, the Command field contains the URL
	endpoint := config.Command
	if endpoint == "" {
		return nil, fmt.Errorf("HTTP spawner requires URL in command field")
	}

	// For HTTP transport, we create a special marker that the manager can recognize
	// This is a temporary solution until we refactor the transport system
	httpMarker := &transport.StdioTransport{}

	// Store the endpoint in a way the manager can access it later
	// We'll modify the manager to handle HTTP servers specially
	s.activeServers[config.ID] = httpMarker

	fmt.Printf("[INFO] HTTP spawner: Created HTTP server marker for %s\n", endpoint)

	return httpMarker, nil
}

// Stop disconnects from an HTTP MCP server
func (s *HTTPSpawner) Stop(ctx context.Context, transport *transport.StdioTransport) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if transport == nil {
		return nil
	}

	// Find and remove from active servers
	for id, t := range s.activeServers {
		if t == transport {
			delete(s.activeServers, id)
			break
		}
	}

	return transport.Disconnect()
}

// IsRunning checks if an HTTP transport is still connected
func (s *HTTPSpawner) IsRunning(transport *transport.StdioTransport) bool {
	if transport == nil {
		return false
	}
	return transport.IsConnected()
}

// GetActiveServers returns a copy of active HTTP servers
func (s *HTTPSpawner) GetActiveServers() map[string]*transport.StdioTransport {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*transport.StdioTransport)
	for id, transport := range s.activeServers {
		result[id] = transport
	}
	return result
}

// ServerManager manages multiple MCP server spawners
type ServerManager struct {
	spawners map[SpawnerType]Spawner
	mu       sync.RWMutex
}

// NewServerManager creates a new server manager
func NewServerManager() *ServerManager {
	return &ServerManager{
		spawners: map[SpawnerType]Spawner{
			SpawnerTypeNPX:        NewNPXSpawner(),
			SpawnerTypeExecutable: NewExecutableSpawner(),
			SpawnerTypeHTTP:       NewHTTPSpawner(),
		},
	}
}

// SpawnServer spawns an MCP server based on its configuration
func (m *ServerManager) SpawnServer(ctx context.Context, config *ServerConfig) (*transport.StdioTransport, error) {
	m.mu.RLock()
	spawner, exists := m.spawners[config.Type]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unsupported spawner type: %s", config.Type)
	}

	return spawner.Spawn(ctx, config)
}

// StopServer stops an MCP server
func (m *ServerManager) StopServer(ctx context.Context, config *ServerConfig, transport *transport.StdioTransport) error {
	m.mu.RLock()
	spawner, exists := m.spawners[config.Type]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("unsupported spawner type: %s", config.Type)
	}

	return spawner.Stop(ctx, transport)
}

// IsServerRunning checks if a server is running
func (m *ServerManager) IsServerRunning(config *ServerConfig, transport *transport.StdioTransport) bool {
	m.mu.RLock()
	spawner, exists := m.spawners[config.Type]
	m.mu.RUnlock()

	if !exists {
		return false
	}

	return spawner.IsRunning(transport)
}

// GetNPXPackageCommand generates the npx command for common MCP packages
func GetNPXPackageCommand(packageName string) (string, []string) {
	// Common MCP server packages
	packages := map[string]string{
		"filesystem": "@modelcontextprotocol/server-filesystem",
		"memory":     "@modelcontextprotocol/server-memory",
		"github":     "@modelcontextprotocol/server-github",
		"gitlab":     "@modelcontextprotocol/server-gitlab",
		"sqlite":     "@modelcontextprotocol/server-sqlite",
		"postgres":   "@modelcontextprotocol/server-postgres",
		"brave":      "@modelcontextprotocol/server-brave-search",
		"youtube":    "@modelcontextprotocol/server-youtube-transcript",
		"puppeteer":  "@modelcontextprotocol/server-puppeteer",
		"docker":     "@modelcontextprotocol/server-docker",
		"kubernetes": "@modelcontextprotocol/server-kubernetes",
	}

	if fullPackage, exists := packages[packageName]; exists {
		return "npx", []string{"-y", fullPackage}
	}

	// If not a known package, assume it's a full package name
	if strings.Contains(packageName, "/") {
		return "npx", []string{"-y", packageName}
	}

	// Default to adding the MCP prefix
	return "npx", []string{"-y", "@modelcontextprotocol/server-" + packageName}
}

// ValidateServerConfig validates an MCP server configuration
func ValidateServerConfig(config *ServerConfig) error {
	if config.ID == "" {
		return fmt.Errorf("server ID is required")
	}

	if config.Name == "" {
		return fmt.Errorf("server name is required")
	}

	switch config.Type {
	case SpawnerTypeNPX:
		if len(config.Args) == 0 && config.Command == "" {
			return fmt.Errorf("NPX spawner requires either command or args")
		}
	case SpawnerTypeExecutable:
		if config.Command == "" {
			return fmt.Errorf("executable spawner requires command")
		}
	case SpawnerTypeHTTP:
		if config.Command == "" {
			return fmt.Errorf("HTTP spawner requires URL in command field")
		}
	default:
		return fmt.Errorf("unsupported spawner type: %s", config.Type)
	}

	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}

	return nil
}
