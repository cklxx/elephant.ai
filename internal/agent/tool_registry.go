package agent

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"alex/internal/config"
	"alex/internal/llm"
	"alex/internal/session"
	"alex/internal/tools/builtin"
)

// CacheMetrics - 缓存性能指标
type CacheMetrics struct {
	staticHits    int64
	dynamicHits   int64
	mcpCacheHits  int64
	mcpCacheMiss  int64
	lastUpdate    int64 // Unix timestamp
}

// ToolRegistry - 优化的工具注册器，支持智能缓存
type ToolRegistry struct {
	// 静态工具缓存 - 永不更改（builtin tools）
	staticTools       map[string]builtin.Tool
	staticToolsMu     sync.RWMutex
	
	// MCP动态工具缓存 - TTL based
	mcpTools          map[string]builtin.Tool
	mcpToolsMu        sync.RWMutex
	lastMCPUpdate     int64  // Unix timestamp
	mcpUpdateInterval int64  // 默认30秒
	
	// 动态工具提供者 - 保持原有逻辑
	dynamicProviders  map[string]DynamicToolProvider
	dynamicMu         sync.RWMutex
	
	// 其他字段保持不变
	configManager     *config.Manager
	sessionManager    *session.Manager
	
	// 性能监控
	metrics           CacheMetrics
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

// NewToolRegistryWithSubAgentMode - 创建优化的工具注册器，支持sub-agent模式
func NewToolRegistryWithSubAgentMode(configManager *config.Manager, sessionManager *session.Manager, isSubAgent bool) *ToolRegistry {
	registry := &ToolRegistry{
		staticTools:       make(map[string]builtin.Tool),
		mcpTools:          make(map[string]builtin.Tool),
		dynamicProviders:  make(map[string]DynamicToolProvider),
		configManager:     configManager,
		sessionManager:    sessionManager,
		mcpUpdateInterval: 30, // 30秒TTL
		metrics:           CacheMetrics{},
	}
	
	// 注册所有内置工具到静态缓存（永不更改）
	registry.registerBuiltinTools()
	
	// 初始加载MCP工具到动态缓存
	registry.loadMCPToolsToCache()
	
	// 注意：sub-agent模式下不注册subagent工具，防止递归
	if isSubAgent {
		log.Printf("[INFO] ToolRegistry: Sub-agent mode - subagent tool disabled to prevent recursion")
	}
	
	log.Printf("[INFO] ToolRegistry: Initialized with smart caching - %d static tools, %d MCP tools", 
		len(registry.staticTools), len(registry.mcpTools))
	
	return registry
}

// registerBuiltinTools - 注册内置工具到静态缓存（永不更改）
func (r *ToolRegistry) registerBuiltinTools() {
	builtinTools := builtin.GetAllBuiltinToolsWithAgent(r.configManager, r.sessionManager)
	
	r.staticToolsMu.Lock()
	defer r.staticToolsMu.Unlock()
	
	for _, tool := range builtinTools {
		r.staticTools[tool.Name()] = tool
		// Static builtin tool cached permanently
	}
	
	log.Printf("[INFO] ToolRegistry: Registered %d static builtin tools", len(builtinTools))
}

// loadMCPToolsToCache - 初始加载MCP工具到缓存
func (r *ToolRegistry) loadMCPToolsToCache() {
	globalMCP := GetGlobalMCPManager()
	
	// 获取已初始化的MCP工具（如果有的话）
	mcpTools := globalMCP.GetTools()
	
	if len(mcpTools) > 0 {
		r.mcpToolsMu.Lock()
		for _, tool := range mcpTools {
			r.mcpTools[tool.Name()] = tool
		}
		atomic.StoreInt64(&r.lastMCPUpdate, time.Now().Unix())
		r.mcpToolsMu.Unlock()
		
		log.Printf("[INFO] ToolRegistry: Loaded %d MCP tools to cache", len(mcpTools))
	}
}

// RegisterDynamicToolProvider - 注册动态工具提供者
func (r *ToolRegistry) RegisterDynamicToolProvider(name string, provider DynamicToolProvider) {
	r.dynamicMu.Lock()
	defer r.dynamicMu.Unlock()
	
	r.dynamicProviders[name] = provider
	log.Printf("[INFO] ToolRegistry: Registered dynamic tool provider: %s", name)
}

// RegisterSubAgentTool - 注册sub-agent工具（仅在主agent中）
func (r *ToolRegistry) RegisterSubAgentTool(reactCore *ReactCore) {
	provider := &SubAgentToolProvider{reactCore: reactCore}
	r.RegisterDynamicToolProvider("subagent", provider)
}

// GetTool - 高性能工具查找，使用智能缓存
func (r *ToolRegistry) GetTool(ctx context.Context, name string) (builtin.Tool, error) {
	// 1. 首先查找静态工具（99%的查找，无锁读取）
	r.staticToolsMu.RLock()
	if tool, exists := r.staticTools[name]; exists {
		r.staticToolsMu.RUnlock()
		atomic.AddInt64(&r.metrics.staticHits, 1)
		return tool, nil
	}
	r.staticToolsMu.RUnlock()
	
	// 2. 然后查找MCP工具（智能TTL缓存）
	if r.shouldUpdateMCPCache() {
		r.refreshMCPCache()
	}
	
	r.mcpToolsMu.RLock()
	if tool, exists := r.mcpTools[name]; exists {
		r.mcpToolsMu.RUnlock()
		atomic.AddInt64(&r.metrics.mcpCacheHits, 1)
		return tool, nil
	}
	r.mcpToolsMu.RUnlock()
	
	// 3. 最后查找动态工具（保持原有逻辑）
	r.dynamicMu.RLock()
	provider, exists := r.dynamicProviders[name]
	r.dynamicMu.RUnlock()
	
	if exists {
		if provider.IsAvailable() {
			atomic.AddInt64(&r.metrics.dynamicHits, 1)
			return provider.GetTool(ctx)
		}
		return nil, fmt.Errorf("dynamic tool %s is not available", name)
	}
	
	return nil, fmt.Errorf("tool %s not found", name)
}

// shouldUpdateMCPCache - 检查是否需要更新MCP缓存（基于TTL）
func (r *ToolRegistry) shouldUpdateMCPCache() bool {
	lastUpdate := atomic.LoadInt64(&r.lastMCPUpdate)
	currentTime := time.Now().Unix()
	return (currentTime - lastUpdate) > r.mcpUpdateInterval
}

// refreshMCPCache - 刷新MCP工具缓存（仅在TTL过期时调用）
func (r *ToolRegistry) refreshMCPCache() {
	// 双检查锁定模式 - 避免多个goroutine同时更新缓存
	if !r.shouldUpdateMCPCache() {
		return
	}
	
	r.mcpToolsMu.Lock()
	defer r.mcpToolsMu.Unlock()
	
	// 再次检查（可能其他goroutine已经更新了）
	if !r.shouldUpdateMCPCache() {
		return
	}
	
	globalMCP := GetGlobalMCPManager()
	mcpTools := globalMCP.GetTools()
	
	// 统计变化
	oldCount := len(r.mcpTools)
	newCount := len(mcpTools)
	
	// 清空并重新填充MCP工具缓存
	r.mcpTools = make(map[string]builtin.Tool)
	for _, tool := range mcpTools {
		r.mcpTools[tool.Name()] = tool
	}
	
	// 更新时间戳
	atomic.StoreInt64(&r.lastMCPUpdate, time.Now().Unix())
	atomic.AddInt64(&r.metrics.mcpCacheMiss, 1)
	atomic.StoreInt64(&r.metrics.lastUpdate, time.Now().Unix())
	
	if newCount != oldCount {
		log.Printf("[INFO] ToolRegistry: MCP cache refreshed - %d tools (was %d)", newCount, oldCount)
	}
}

// GetAllToolDefinitions - 获取所有工具定义（用于LLM）- 优化版本
func (r *ToolRegistry) GetAllToolDefinitions(ctx context.Context) []llm.Tool {
	// 使用智能缓存更新MCP工具
	if r.shouldUpdateMCPCache() {
		r.refreshMCPCache()
	}
	
	var tools []llm.Tool
	
	// 添加静态工具（无锁读取）
	r.staticToolsMu.RLock()
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
	r.staticToolsMu.RUnlock()
	
	// 添加MCP工具（缓存读取）
	r.mcpToolsMu.RLock()
	for _, tool := range r.mcpTools {
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
	r.mcpToolsMu.RUnlock()
	
	// 添加可用的动态工具
	r.dynamicMu.RLock()
	dynamicProviders := make(map[string]DynamicToolProvider)
	for name, provider := range r.dynamicProviders {
		dynamicProviders[name] = provider
	}
	r.dynamicMu.RUnlock()
	
	for name, provider := range dynamicProviders {
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
	
	return tools
}

// ListTools - 列出所有可用工具名称 - 优化版本
func (r *ToolRegistry) ListTools(ctx context.Context) []string {
	var names []string
	
	// 添加静态工具名称
	r.staticToolsMu.RLock()
	for name := range r.staticTools {
		names = append(names, name)
	}
	r.staticToolsMu.RUnlock()
	
	// 添加MCP工具名称
	r.mcpToolsMu.RLock()
	for name := range r.mcpTools {
		names = append(names, name)
	}
	r.mcpToolsMu.RUnlock()
	
	// 添加可用的动态工具名称
	r.dynamicMu.RLock()
	for name, provider := range r.dynamicProviders {
		if provider.IsAvailable() {
			names = append(names, name)
		}
	}
	r.dynamicMu.RUnlock()
	
	return names
}

// HasTool - 检查工具是否存在 - 优化版本
func (r *ToolRegistry) HasTool(ctx context.Context, name string) bool {
	// 检查静态工具
	r.staticToolsMu.RLock()
	if _, exists := r.staticTools[name]; exists {
		r.staticToolsMu.RUnlock()
		return true
	}
	r.staticToolsMu.RUnlock()
	
	// 检查MCP工具
	r.mcpToolsMu.RLock()
	if _, exists := r.mcpTools[name]; exists {
		r.mcpToolsMu.RUnlock()
		return true
	}
	r.mcpToolsMu.RUnlock()
	
	// 检查动态工具
	r.dynamicMu.RLock()
	provider, exists := r.dynamicProviders[name]
	r.dynamicMu.RUnlock()
	
	if exists {
		return provider.IsAvailable()
	}
	
	return false
}

// GetCacheMetrics - 获取缓存性能指标
func (r *ToolRegistry) GetCacheMetrics() CacheMetrics {
	return CacheMetrics{
		staticHits:    atomic.LoadInt64(&r.metrics.staticHits),
		dynamicHits:   atomic.LoadInt64(&r.metrics.dynamicHits),
		mcpCacheHits:  atomic.LoadInt64(&r.metrics.mcpCacheHits),
		mcpCacheMiss:  atomic.LoadInt64(&r.metrics.mcpCacheMiss),
		lastUpdate:    atomic.LoadInt64(&r.metrics.lastUpdate),
	}
}

// LogCacheStats - 记录缓存统计信息
func (r *ToolRegistry) LogCacheStats() {
	metrics := r.GetCacheMetrics()
	totalLookups := metrics.staticHits + metrics.dynamicHits + metrics.mcpCacheHits
	
	if totalLookups > 0 {
		staticHitRate := float64(metrics.staticHits) / float64(totalLookups) * 100
		mcpHitRate := float64(metrics.mcpCacheHits) / float64(totalLookups) * 100
		
		log.Printf("[STATS] ToolRegistry: Cache Stats - Total: %d, Static: %d (%.1f%%), MCP: %d (%.1f%%), Dynamic: %d, MCP Refreshes: %d", 
			totalLookups, 
			metrics.staticHits, staticHitRate,
			metrics.mcpCacheHits, mcpHitRate,
			metrics.dynamicHits,
			metrics.mcpCacheMiss)
	}
}

// SetMCPUpdateInterval - 设置MCP缓存更新间隔（测试用）
func (r *ToolRegistry) SetMCPUpdateInterval(seconds int64) {
	r.mcpUpdateInterval = seconds
}