package tools

import (
	"fmt"
	"sync"

	"alex/internal/agent/ports"
	"alex/internal/tools/builtin"
)

// Registry implements ToolRegistry with three-tier caching
type Registry struct {
	static  map[string]ports.ToolExecutor
	dynamic map[string]ports.ToolExecutor
	mcp     map[string]ports.ToolExecutor
	mu      sync.RWMutex
}

// filteredRegistry wraps a parent registry and excludes certain tools
type filteredRegistry struct {
	parent  *Registry
	exclude map[string]bool
}

type Config struct {
	TavilyAPIKey string
}

func NewRegistry(config Config) *Registry {
	r := &Registry{
		static:  make(map[string]ports.ToolExecutor),
		dynamic: make(map[string]ports.ToolExecutor),
		mcp:     make(map[string]ports.ToolExecutor),
	}
	r.registerBuiltins(config)
	return r
}

func (r *Registry) Register(tool ports.ToolExecutor) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := tool.Metadata().Name
	if _, exists := r.static[name]; exists {
		return fmt.Errorf("tool already exists: %s", name)
	}

	// Check if this is an MCP tool (tools with mcp__ prefix go to mcp map)
	if len(name) > 5 && name[:5] == "mcp__" {
		r.mcp[name] = tool
	} else {
		r.dynamic[name] = tool
	}
	return nil
}

func (r *Registry) Get(name string) (ports.ToolExecutor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if tool, ok := r.static[name]; ok {
		return tool, nil
	}
	if tool, ok := r.dynamic[name]; ok {
		return tool, nil
	}
	if tool, ok := r.mcp[name]; ok {
		return tool, nil
	}
	return nil, fmt.Errorf("tool not found: %s", name)
}

// WithoutSubagent returns a filtered registry that excludes the subagent tool
// This prevents nested subagent calls at registration level
func (r *Registry) WithoutSubagent() ports.ToolRegistry {
	return &filteredRegistry{
		parent:  r,
		exclude: map[string]bool{"subagent": true},
	}
}

// filteredRegistry implements ports.ToolRegistry with exclusions

func (f *filteredRegistry) Get(name string) (ports.ToolExecutor, error) {
	if f.exclude[name] {
		return nil, fmt.Errorf("tool not available: %s", name)
	}
	return f.parent.Get(name)
}

func (f *filteredRegistry) List() []ports.ToolDefinition {
	allTools := f.parent.List()
	filtered := make([]ports.ToolDefinition, 0, len(allTools))
	for _, tool := range allTools {
		if !f.exclude[tool.Name] {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}

func (f *filteredRegistry) Register(tool ports.ToolExecutor) error {
	// Delegate to parent, but exclude from own filter
	name := tool.Metadata().Name
	if f.exclude[name] {
		return fmt.Errorf("tool registration blocked: %s", name)
	}
	return f.parent.Register(tool)
}

func (f *filteredRegistry) Unregister(name string) error {
	if f.exclude[name] {
		return fmt.Errorf("tool unregistration blocked: %s", name)
	}
	return f.parent.Unregister(name)
}

func (r *Registry) List() []ports.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var defs []ports.ToolDefinition
	for _, tool := range r.static {
		defs = append(defs, tool.Definition())
	}
	for _, tool := range r.dynamic {
		defs = append(defs, tool.Definition())
	}
	for _, tool := range r.mcp {
		defs = append(defs, tool.Definition())
	}
	return defs
}

func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.static[name]; ok {
		return fmt.Errorf("cannot unregister built-in tool: %s", name)
	}
	delete(r.dynamic, name)
	delete(r.mcp, name)
	return nil
}

func (r *Registry) registerBuiltins(config Config) {
	// File operations
	r.static["file_read"] = builtin.NewFileRead()
	r.static["file_write"] = builtin.NewFileWrite()
	r.static["file_edit"] = builtin.NewFileEdit()
	r.static["list_files"] = builtin.NewListFiles()

	// Shell & search
	r.static["bash"] = builtin.NewBash()
	r.static["grep"] = builtin.NewGrep()
	r.static["ripgrep"] = builtin.NewRipgrep()
	r.static["find"] = builtin.NewFind()

	// Task management
	r.static["todo_read"] = builtin.NewTodoRead()
	r.static["todo_update"] = builtin.NewTodoUpdate()

	// Execution & reasoning
	r.static["code_execute"] = builtin.NewCodeExecute()
	r.static["think"] = builtin.NewThink()

	// Web tools
	r.static["web_search"] = builtin.NewWebSearch(config.TavilyAPIKey)
	r.static["web_fetch"] = builtin.NewWebFetch()

	// Git tools (without LLM - will be registered separately if needed)
	r.static["git_history"] = builtin.NewGitHistory()

	// Note: code_search tool is not registered (feature not ready)
}

// RegisterGitTools registers Git tools that require an LLM client
func (r *Registry) RegisterGitTools(llmClient ports.LLMClient) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if llmClient != nil {
		r.static["git_commit"] = builtin.NewGitCommit(llmClient)
		r.static["git_pr"] = builtin.NewGitPR(llmClient)
	}
}

// RegisterSubAgent registers the subagent tool that requires a coordinator
func (r *Registry) RegisterSubAgent(coordinator ports.AgentCoordinator) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if coordinator != nil {
		r.static["subagent"] = builtin.NewSubAgent(coordinator, 3)
	}
}
