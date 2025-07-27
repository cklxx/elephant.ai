package agent

import (
	"context"
	"fmt"
	"log"
	"sync"

	"alex/internal/config"
	"alex/internal/llm"
	"alex/internal/session"
	"alex/internal/tools/builtin"
	"alex/internal/tools/mcp"
)

// ToolRegistry - 统一的工具注册器
type ToolRegistry struct {
	mu                sync.RWMutex
	staticTools       map[string]builtin.Tool        // 静态工具（builtin + MCP）
	dynamicProviders  map[string]DynamicToolProvider // 动态工具提供者
	configManager     *config.Manager
	sessionManager    *session.Manager
}

// DynamicToolProvider - 动态工具提供者接口
type DynamicToolProvider interface {
	GetTool(ctx context.Context) (builtin.Tool, error)
	IsAvailable() bool
}

// SubAgentToolProvider - sub-agent工具的动态提供者
type SubAgentToolProvider struct {
	reactCore *ReactCore
}

func (p *SubAgentToolProvider) GetTool(ctx context.Context) (builtin.Tool, error) {
	if p.reactCore == nil {
		return nil, fmt.Errorf("ReactCore not available")
	}
	return builtin.CreateSubAgentTool(p.reactCore), nil
}

func (p *SubAgentToolProvider) IsAvailable() bool {
	return p.reactCore != nil
}

// NewToolRegistry - 创建工具注册器
func NewToolRegistry(configManager *config.Manager, sessionManager *session.Manager) *ToolRegistry {
	return NewToolRegistryWithSubAgentMode(configManager, sessionManager, false)
}

// NewToolRegistryWithSubAgentMode - 创建工具注册器，支持sub-agent模式
func NewToolRegistryWithSubAgentMode(configManager *config.Manager, sessionManager *session.Manager, isSubAgent bool) *ToolRegistry {
	registry := &ToolRegistry{
		staticTools:      make(map[string]builtin.Tool),
		dynamicProviders: make(map[string]DynamicToolProvider),
		configManager:    configManager,
		sessionManager:   sessionManager,
	}
	
	// 注册所有内置工具
	registry.registerBuiltinTools()
	
	// 注册MCP工具
	registry.registerMCPTools()
	
	// 注意：sub-agent模式下不注册subagent工具，防止递归
	if isSubAgent {
		log.Printf("[INFO] ToolRegistry: Sub-agent mode - subagent tool disabled to prevent recursion")
	}
	
	return registry
}

// registerBuiltinTools - 注册内置工具
func (r *ToolRegistry) registerBuiltinTools() {
	builtinTools := builtin.GetAllBuiltinToolsWithAgent(r.configManager, r.sessionManager)
	
	for _, tool := range builtinTools {
		r.staticTools[tool.Name()] = tool
		log.Printf("[DEBUG] ToolRegistry: Registered builtin tool: %s", tool.Name())
	}
}

// registerMCPTools - 注册MCP工具
func (r *ToolRegistry) registerMCPTools() {
	if r.configManager == nil {
		return
	}
	
	// 获取MCP配置
	configMCP := r.configManager.GetMCPConfig()
	if !configMCP.Enabled {
		log.Printf("[INFO] MCP integration is disabled")
		return
	}

	// 转换配置格式
	mcpConfig := r.convertConfigToMCP(configMCP)

	// 创建MCP管理器
	mcpManager := mcp.NewManager(mcpConfig)

	// 启动MCP管理器
	ctx := context.Background()
	if err := mcpManager.Start(ctx); err != nil {
		log.Printf("[WARN] Failed to start MCP manager: %v", err)
		return
	}

	// 集成工具 - 获取MCP工具并注册
	mcpTools := mcpManager.IntegrateWithBuiltinTools([]builtin.Tool{})
	for _, tool := range mcpTools {
		r.staticTools[tool.Name()] = tool
		log.Printf("[DEBUG] ToolRegistry: Registered MCP tool: %s", tool.Name())
	}
}

// convertConfigToMCP - 转换配置格式从config包到mcp包
func (r *ToolRegistry) convertConfigToMCP(configMCP *config.MCPConfig) *mcp.MCPConfig {
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

// RegisterDynamicToolProvider - 注册动态工具提供者
func (r *ToolRegistry) RegisterDynamicToolProvider(name string, provider DynamicToolProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.dynamicProviders[name] = provider
	log.Printf("[DEBUG] ToolRegistry: Registered dynamic tool provider: %s", name)
}

// RegisterSubAgentTool - 注册sub-agent工具（仅在主agent中）
func (r *ToolRegistry) RegisterSubAgentTool(reactCore *ReactCore) {
	provider := &SubAgentToolProvider{reactCore: reactCore}
	r.RegisterDynamicToolProvider("subagent", provider)
}

// GetTool - 获取工具实例
func (r *ToolRegistry) GetTool(ctx context.Context, name string) (builtin.Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// 首先查找静态工具
	if tool, exists := r.staticTools[name]; exists {
		return tool, nil
	}
	
	// 然后查找动态工具
	if provider, exists := r.dynamicProviders[name]; exists {
		if provider.IsAvailable() {
			return provider.GetTool(ctx)
		}
		return nil, fmt.Errorf("dynamic tool %s is not available", name)
	}
	
	return nil, fmt.Errorf("tool %s not found", name)
}

// GetAllToolDefinitions - 获取所有工具定义（用于LLM）
func (r *ToolRegistry) GetAllToolDefinitions(ctx context.Context) []llm.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	var tools []llm.Tool
	
	// 添加静态工具
	for _, tool := range r.staticTools {
		toolDef := llm.Tool{
			Type: "function",
			Function: llm.Function{
				Name:        tool.Name(),
				Description: tool.Description(),
				Parameters:  tool.Parameters(),
			},
		}
		tools = append(tools, toolDef)
	}
	
	// 添加可用的动态工具
	for name, provider := range r.dynamicProviders {
		if provider.IsAvailable() {
			if tool, err := provider.GetTool(ctx); err == nil {
				toolDef := llm.Tool{
					Type: "function",
					Function: llm.Function{
						Name:        tool.Name(),
						Description: tool.Description(),
						Parameters:  tool.Parameters(),
					},
				}
				tools = append(tools, toolDef)
			} else {
				log.Printf("[WARN] ToolRegistry: Failed to get dynamic tool %s: %v", name, err)
			}
		}
	}
	
	log.Printf("[DEBUG] ToolRegistry: Generated %d tool definitions", len(tools))
	return tools
}

// ListTools - 列出所有可用工具名称
func (r *ToolRegistry) ListTools(ctx context.Context) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	var names []string
	
	// 添加静态工具名称
	for name := range r.staticTools {
		names = append(names, name)
	}
	
	// 添加可用的动态工具名称
	for name, provider := range r.dynamicProviders {
		if provider.IsAvailable() {
			names = append(names, name)
		}
	}
	
	return names
}

// HasTool - 检查工具是否存在
func (r *ToolRegistry) HasTool(ctx context.Context, name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// 检查静态工具
	if _, exists := r.staticTools[name]; exists {
		return true
	}
	
	// 检查动态工具
	if provider, exists := r.dynamicProviders[name]; exists {
		return provider.IsAvailable()
	}
	
	return false
}