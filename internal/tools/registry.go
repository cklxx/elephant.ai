package tools

import (
	"fmt"
	"sync"

	"alex/internal/agent/ports"
	"alex/internal/tools/builtin"
)

// registry implements ToolRegistry with three-tier caching
type registry struct {
	static  map[string]ports.ToolExecutor
	dynamic map[string]ports.ToolExecutor
	mcp     map[string]ports.ToolExecutor
	mu      sync.RWMutex
}

func NewRegistry() ports.ToolRegistry {
	r := &registry{
		static:  make(map[string]ports.ToolExecutor),
		dynamic: make(map[string]ports.ToolExecutor),
		mcp:     make(map[string]ports.ToolExecutor),
	}
	r.registerBuiltins()
	return r
}

func (r *registry) Register(tool ports.ToolExecutor) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := tool.Metadata().Name
	if _, exists := r.static[name]; exists {
		return fmt.Errorf("tool already exists: %s", name)
	}
	r.dynamic[name] = tool
	return nil
}

func (r *registry) Get(name string) (ports.ToolExecutor, error) {
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

func (r *registry) List() []ports.ToolDefinition {
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

func (r *registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.static[name]; ok {
		return fmt.Errorf("cannot unregister built-in tool: %s", name)
	}
	delete(r.dynamic, name)
	delete(r.mcp, name)
	return nil
}

func (r *registry) registerBuiltins() {
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
	r.static["web_search"] = builtin.NewWebSearch()
	r.static["web_fetch"] = builtin.NewWebFetch()
}
