package agent

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"alex/internal/config"
	"alex/internal/session"
	"alex/internal/tools/builtin"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockTool - Mock implementation of builtin.Tool for testing
type MockTool struct {
	name        string
	description string
}

func (m *MockTool) Name() string                       { return m.name }
func (m *MockTool) Description() string                { return m.description }
func (m *MockTool) Parameters() map[string]interface{} { return make(map[string]interface{}) }
func (m *MockTool) Execute(ctx context.Context, args map[string]interface{}) (*builtin.ToolResult, error) {
	return &builtin.ToolResult{
		Content: "mock result",
	}, nil
}

func (m *MockTool) Validate(args map[string]interface{}) error {
	return nil
}

// MockDynamicToolProvider - Mock implementation for testing
type MockDynamicToolProvider struct {
	tool      builtin.Tool
	available bool
}

func (m *MockDynamicToolProvider) GetTool(ctx context.Context) (builtin.Tool, error) {
	return m.tool, nil
}

func (m *MockDynamicToolProvider) IsAvailable() bool {
	return m.available
}

func TestCacheMetrics(t *testing.T) {
	// Test CacheMetrics structure
	metrics := CacheMetrics{
		staticHits:   10,
		dynamicHits:  5,
		mcpCacheHits: 8,
		mcpCacheMiss: 2,
		lastUpdate:   time.Now().Unix(),
	}

	assert.Equal(t, int64(10), metrics.staticHits)
	assert.Equal(t, int64(5), metrics.dynamicHits)
	assert.Equal(t, int64(8), metrics.mcpCacheHits)
	assert.Equal(t, int64(2), metrics.mcpCacheMiss)
	assert.Greater(t, metrics.lastUpdate, int64(0))
}

func TestNewToolRegistry(t *testing.T) {
	// Test ToolRegistry creation
	configManager, err := config.NewManager()
	require.NoError(t, err)

	sessionManager, err := session.NewManager()
	require.NoError(t, err)

	registry := NewToolRegistry(configManager, sessionManager)

	assert.NotNil(t, registry)
	assert.NotNil(t, registry.staticTools)
	assert.NotNil(t, registry.mcpTools)
	assert.NotNil(t, registry.dynamicProviders)
	assert.Equal(t, configManager, registry.configManager)
	assert.Equal(t, sessionManager, registry.sessionManager)
	assert.Equal(t, int64(30), registry.mcpUpdateInterval)
}

func TestNewToolRegistryWithSubAgentMode(t *testing.T) {
	// Test ToolRegistry creation with sub-agent mode
	configManager, err := config.NewManager()
	require.NoError(t, err)

	sessionManager, err := session.NewManager()
	require.NoError(t, err)

	// Test normal mode
	registry := NewToolRegistryWithSubAgentMode(configManager, sessionManager, false)
	assert.NotNil(t, registry)

	// Test sub-agent mode
	subAgentRegistry := NewToolRegistryWithSubAgentMode(configManager, sessionManager, true)
	assert.NotNil(t, subAgentRegistry)
}

func TestToolRegistryStructure(t *testing.T) {
	// Test ToolRegistry internal structure
	registry := &ToolRegistry{
		staticTools:       make(map[string]builtin.Tool),
		mcpTools:          make(map[string]builtin.Tool),
		dynamicProviders:  make(map[string]DynamicToolProvider),
		mcpUpdateInterval: 30,
		metrics:           CacheMetrics{},
	}

	assert.NotNil(t, registry.staticTools)
	assert.NotNil(t, registry.mcpTools)
	assert.NotNil(t, registry.dynamicProviders)
	assert.Equal(t, int64(30), registry.mcpUpdateInterval)
	assert.Empty(t, registry.staticTools)
	assert.Empty(t, registry.mcpTools)
	assert.Empty(t, registry.dynamicProviders)
}

func TestDynamicToolProviderInterface(t *testing.T) {
	// Test DynamicToolProvider interface implementation
	mockTool := &MockTool{name: "test_tool", description: "Test tool"}
	provider := &MockDynamicToolProvider{
		tool:      mockTool,
		available: true,
	}

	// Test that mock implements the interface
	var _ DynamicToolProvider = provider

	// Test GetTool method
	ctx := context.Background()
	tool, err := provider.GetTool(ctx)

	assert.NoError(t, err)
	assert.Equal(t, mockTool, tool)

	// Test IsAvailable method
	assert.True(t, provider.IsAvailable())

	// Test with unavailable provider
	provider.available = false
	assert.False(t, provider.IsAvailable())
}

func TestSubAgentToolProvider(t *testing.T) {
	// Test SubAgentToolProvider implementation
	var _ DynamicToolProvider = &SubAgentToolProvider{}

	// Test with nil ReactCore
	provider := &SubAgentToolProvider{reactCore: nil}
	assert.False(t, provider.IsAvailable())

	_, err := provider.GetTool(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ReactCore not available")
}

func TestSubAgentToolProviderWithReactCore(t *testing.T) {
	// Test SubAgentToolProvider with valid ReactCore
	// Create a minimal ReactCore for testing
	mockCore := &ReactCore{} // This would need proper initialization in real usage

	provider := &SubAgentToolProvider{reactCore: mockCore}
	assert.True(t, provider.IsAvailable())

	// Note: GetTool would require builtin.CreateSubAgentTool to be properly implemented
	// This test verifies the structure and availability check
}

func TestToolRegistryConcurrency(t *testing.T) {
	// Test ToolRegistry thread safety
	registry := &ToolRegistry{
		staticTools:      make(map[string]builtin.Tool),
		mcpTools:         make(map[string]builtin.Tool),
		dynamicProviders: make(map[string]DynamicToolProvider),
		metrics:          CacheMetrics{},
	}

	var wg sync.WaitGroup
	numGoroutines := 10

	// Test concurrent access to static tools
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Read operation
			registry.staticToolsMu.RLock()
			_ = len(registry.staticTools)
			registry.staticToolsMu.RUnlock()

			// Write operation
			registry.staticToolsMu.Lock()
			registry.staticTools[fmt.Sprintf("tool_%d", id)] = &MockTool{
				name: fmt.Sprintf("test_tool_%d", id),
			}
			registry.staticToolsMu.Unlock()
		}(i)
	}

	wg.Wait()

	registry.staticToolsMu.RLock()
	toolCount := len(registry.staticTools)
	registry.staticToolsMu.RUnlock()

	assert.Equal(t, numGoroutines, toolCount)
}

func TestCacheMetricsConcurrency(t *testing.T) {
	// Test CacheMetrics atomic operations
	var metrics CacheMetrics

	var wg sync.WaitGroup
	numGoroutines := 100
	incrementsPerGoroutine := 10

	// Test concurrent increments
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < incrementsPerGoroutine; j++ {
				atomic.AddInt64(&metrics.staticHits, 1)
				atomic.AddInt64(&metrics.dynamicHits, 1)
				atomic.AddInt64(&metrics.mcpCacheHits, 1)
				atomic.AddInt64(&metrics.mcpCacheMiss, 1)
			}
		}()
	}

	wg.Wait()

	expectedCount := int64(numGoroutines * incrementsPerGoroutine)
	assert.Equal(t, expectedCount, atomic.LoadInt64(&metrics.staticHits))
	assert.Equal(t, expectedCount, atomic.LoadInt64(&metrics.dynamicHits))
	assert.Equal(t, expectedCount, atomic.LoadInt64(&metrics.mcpCacheHits))
	assert.Equal(t, expectedCount, atomic.LoadInt64(&metrics.mcpCacheMiss))
}

func TestMCPToolsCaching(t *testing.T) {
	// Test MCP tools caching mechanism
	registry := &ToolRegistry{
		mcpTools:          make(map[string]builtin.Tool),
		mcpUpdateInterval: 1, // 1 second for testing
		lastMCPUpdate:     0,
		metrics:           CacheMetrics{},
	}

	// Test initial state
	assert.Empty(t, registry.mcpTools)
	assert.Equal(t, int64(0), registry.lastMCPUpdate)

	// Simulate adding MCP tools
	registry.mcpToolsMu.Lock()
	registry.mcpTools["mcp_tool_1"] = &MockTool{name: "mcp_tool_1"}
	registry.lastMCPUpdate = time.Now().Unix()
	registry.mcpToolsMu.Unlock()

	registry.mcpToolsMu.RLock()
	toolCount := len(registry.mcpTools)
	lastUpdate := registry.lastMCPUpdate
	registry.mcpToolsMu.RUnlock()

	assert.Equal(t, 1, toolCount)
	assert.Greater(t, lastUpdate, int64(0))
}

func TestDynamicProviderRegistration(t *testing.T) {
	// Test dynamic provider registration
	registry := &ToolRegistry{
		dynamicProviders: make(map[string]DynamicToolProvider),
	}

	mockProvider := &MockDynamicToolProvider{
		tool:      &MockTool{name: "dynamic_tool"},
		available: true,
	}

	// Test registration
	registry.dynamicMu.Lock()
	registry.dynamicProviders["dynamic_tool"] = mockProvider
	registry.dynamicMu.Unlock()

	registry.dynamicMu.RLock()
	provider, exists := registry.dynamicProviders["dynamic_tool"]
	registry.dynamicMu.RUnlock()

	assert.True(t, exists)
	assert.Equal(t, mockProvider, provider)
	assert.True(t, provider.IsAvailable())
}

// Benchmark tests for performance
func BenchmarkToolRegistryStaticAccess(b *testing.B) {
	registry := &ToolRegistry{
		staticTools: map[string]builtin.Tool{
			"test_tool": &MockTool{name: "test_tool"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry.staticToolsMu.RLock()
		_ = registry.staticTools["test_tool"]
		registry.staticToolsMu.RUnlock()
	}
}

func BenchmarkCacheMetricsIncrement(b *testing.B) {
	var metrics CacheMetrics

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		atomic.AddInt64(&metrics.staticHits, 1)
	}
}
