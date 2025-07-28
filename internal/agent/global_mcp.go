package agent

import (
	"context"
	"log"
	"sync"
	"time"

	"alex/internal/config"
	"alex/internal/tools/builtin"
	"alex/internal/tools/mcp"
)

// GlobalMCPManager - 全局MCP管理器单例
type GlobalMCPManager struct {
	mu           sync.RWMutex
	manager      *mcp.Manager
	tools        []builtin.Tool
	initialized  bool
	initializing bool
	initOnce     sync.Once
}

var (
	globalMCP     *GlobalMCPManager
	globalMCPOnce sync.Once
)

// GetGlobalMCPManager - 获取全局MCP管理器实例
func GetGlobalMCPManager() *GlobalMCPManager {
	globalMCPOnce.Do(func() {
		globalMCP = &GlobalMCPManager{
			tools: []builtin.Tool{},
		}
	})
	return globalMCP
}

// InitializeAsync - 异步初始化MCP管理器
func (g *GlobalMCPManager) InitializeAsync(configManager *config.Manager) {
	g.initOnce.Do(func() {
		go g.initialize(configManager)
	})
}

// initialize - 内部初始化方法
func (g *GlobalMCPManager) initialize(configManager *config.Manager) {
	g.mu.Lock()
	g.initializing = true
	g.mu.Unlock()

	defer func() {
		g.mu.Lock()
		g.initializing = false
		g.initialized = true
		g.mu.Unlock()
	}()

	// 检查MCP配置
	if configManager == nil {
		log.Printf("[INFO] GlobalMCP: ConfigManager is nil, skipping MCP initialization")
		return
	}

	configMCP := configManager.GetMCPConfig()
	if !configMCP.Enabled {
		log.Printf("[INFO] GlobalMCP: MCP integration is disabled")
		return
	}

	log.Printf("[INFO] GlobalMCP: Starting MCP initialization...")

	// 转换配置格式
	mcpConfig := convertConfigToMCP(configMCP)

	// 创建MCP管理器
	manager := mcp.NewManager(mcpConfig)

	// 启动MCP管理器
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		log.Printf("[WARN] GlobalMCP: Failed to start MCP manager: %v", err)
		return
	}

	// 集成工具
	mcpTools := manager.IntegrateWithBuiltinTools([]builtin.Tool{})

	// 更新全局状态
	g.mu.Lock()
	g.manager = manager
	g.tools = mcpTools
	g.mu.Unlock()

	log.Printf("[INFO] GlobalMCP: MCP initialization completed with %d tools", len(mcpTools))
}

// convertConfigToMCP - 转换配置格式（从tool_registry.go移过来的辅助函数）
func convertConfigToMCP(configMCP *config.MCPConfig) *mcp.MCPConfig {
	mcpConfig := &mcp.MCPConfig{
		Enabled: configMCP.Enabled,
		Servers: make(map[string]*mcp.ServerConfig),
	}

	for name, server := range configMCP.Servers {
		mcpConfig.Servers[name] = &mcp.ServerConfig{
			Command: server.Command,
			Args:    server.Args,
			Env:     server.Env,
		}
	}

	return mcpConfig
}

// GetTools - 获取已初始化的MCP工具
func (g *GlobalMCPManager) GetTools() []builtin.Tool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	// 返回工具的副本，避免并发修改
	tools := make([]builtin.Tool, len(g.tools))
	copy(tools, g.tools)
	return tools
}

// IsInitialized - 检查是否已初始化完成
func (g *GlobalMCPManager) IsInitialized() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.initialized
}

// IsInitializing - 检查是否正在初始化
func (g *GlobalMCPManager) IsInitializing() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.initializing
}

// WaitForInitialization - 等待初始化完成
func (g *GlobalMCPManager) WaitForInitialization(ctx context.Context, timeout time.Duration) bool {
	g.mu.RLock()
	if g.initialized || !g.initializing {
		g.mu.RUnlock()
		return g.initialized
	}
	g.mu.RUnlock()

	log.Printf("[INFO] GlobalMCP: Waiting for MCP initialization to complete (timeout: %v)", timeout)

	// 创建带超时的context
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 轮询检查初始化状态
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-waitCtx.Done():
			log.Printf("[WARN] GlobalMCP: MCP initialization wait timeout after %v", timeout)
			return false
		case <-ticker.C:
			g.mu.RLock()
			initialized := g.initialized
			initializing := g.initializing
			g.mu.RUnlock()

			if initialized {
				log.Printf("[INFO] GlobalMCP: MCP initialization completed")
				return true
			}

			if !initializing {
				// 初始化已结束但未成功
				return false
			}
		}
	}
}