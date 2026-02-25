package toolregistry

import (
	"alex/internal/shared/utils"
	"context"
	"fmt"
	"sort"
	"sync"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/memory"
	toolspolicy "alex/internal/infra/tools"
	runtimeconfig "alex/internal/shared/config"

	"alex/internal/infra/tools/builtin/orchestration"
)

// Registry implements ToolRegistry with three-tier caching
type Registry struct {
	static       map[string]tools.ToolExecutor
	dynamic      map[string]tools.ToolExecutor
	mcp          map[string]tools.ToolExecutor
	mu           sync.RWMutex
	cachedDefs   []ports.ToolDefinition
	defsDirty    bool
	policy       toolspolicy.ToolPolicy
	breakers     *circuitBreakerStore
	degradation  DegradationConfig
	SLACollector *toolspolicy.SLACollector
}

// filteredRegistry wraps a parent registry and excludes certain tools
type filteredRegistry struct {
	parent  *Registry
	exclude map[string]bool
}

type Config struct {
	Profile string

	TavilyAPIKey string

	ArkAPIKey    string
	MemoryEngine memory.Engine
	HTTPLimits     runtimeconfig.HTTPLimitsConfig
	ToolPolicy     toolspolicy.ToolPolicy
	BreakerConfig  CircuitBreakerConfig
	SLACollector   *toolspolicy.SLACollector
	// DegradationConfig, when provided, overrides the registry defaults.
	// When nil, DefaultRegistryDegradationConfig is used.
	DegradationConfig *DegradationConfig
	Toolset           Toolset
	BrowserConfig     BrowserConfig
	// DisabledTools allows callers to explicitly suppress specific tools by name.
	// When nil, registry derives quickstart gating from runtime config.
	DisabledTools map[string]string
}

func NewRegistry(config Config) (*Registry, error) {
	policy := config.ToolPolicy
	if policy == nil {
		policy = toolspolicy.NewToolPolicy(toolspolicy.DefaultToolPolicyConfigWithRules())
	}
	breakers := newCircuitBreakerStore(normalizeCircuitBreakerConfig(config.BreakerConfig))
	degradation := DefaultRegistryDegradationConfig()
	if config.DegradationConfig != nil {
		degradation = *config.DegradationConfig
		if degradation.FallbackMap == nil {
			degradation.FallbackMap = make(map[string][]string)
		}
		if degradation.MaxFallbackAttempts <= 0 {
			degradation.MaxFallbackAttempts = defaultMaxFallbackAttempts
		}
	}
	if degradation.SLARouter == nil && config.SLACollector != nil {
		degradation.SLARouter = toolspolicy.NewSLARouter(config.SLACollector, toolspolicy.DefaultSLARouterConfig())
	}

	r := &Registry{
		static:       make(map[string]tools.ToolExecutor),
		dynamic:      make(map[string]tools.ToolExecutor),
		mcp:          make(map[string]tools.ToolExecutor),
		defsDirty:    true,
		policy:       policy,
		breakers:     breakers,
		degradation:  degradation,
		SLACollector: config.SLACollector,
	}

	if config.MemoryEngine == nil {
		return nil, fmt.Errorf("memory engine is required")
	}

	if err := r.registerBuiltins(config); err != nil {
		return nil, err
	}

	return r, nil
}

func (r *Registry) Register(tool tools.ToolExecutor) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := tool.Metadata().Name
	if _, exists := r.static[name]; exists {
		return fmt.Errorf("tool already exists: %s", name)
	}

	// Check if this is an MCP tool (tools with mcp__ prefix go to mcp map)
	wrapped := wrapTool(tool, r.policy, r.breakers, r.SLACollector)
	wrapped = r.wrapDegradation(name, wrapped)
	if len(name) > 5 && name[:5] == "mcp__" {
		r.mcp[name] = wrapped
	} else {
		r.dynamic[name] = wrapped
	}
	r.defsDirty = true
	return nil
}

func (r *Registry) Get(name string) (tools.ToolExecutor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if tool, ok := r.getRawLocked(name); ok {
		return tool, nil
	}
	return nil, fmt.Errorf("tool not found: %s", name)
}

func (r *Registry) getRawLocked(name string) (tools.ToolExecutor, bool) {
	if tool, ok := r.static[name]; ok {
		return tool, true
	}
	if tool, ok := r.dynamic[name]; ok {
		return tool, true
	}
	if tool, ok := r.mcp[name]; ok {
		return tool, true
	}
	return nil, false
}

// wrapTool ensures tools are wrapped with approval, retry, ID propagation,
// and optional SLA measurement. The SLA executor is the outermost layer so
// it measures total time including retries and approval.
func wrapTool(tool tools.ToolExecutor, policy toolspolicy.ToolPolicy, breakers *circuitBreakerStore, sla *toolspolicy.SLACollector) tools.ToolExecutor {
	if tool == nil {
		return nil
	}
	base := unwrapTool(tool)
	validated := &validatingExecutor{delegate: base}
	approval := toolspolicy.NewApprovalExecutor(validated)
	retry := newRetryExecutor(approval, policy, breakers)
	id := &idAwareExecutor{delegate: retry}
	if sla == nil {
		return id
	}
	return toolspolicy.NewSLAExecutor(id, sla)
}

func unwrapTool(tool tools.ToolExecutor) tools.ToolExecutor {
	for {
		switch typed := tool.(type) {
		case *degradationExecutor:
			tool = typed.delegate
		case *toolspolicy.SLAExecutor:
			tool = typed.Delegate()
		case *idAwareExecutor:
			tool = typed.delegate
		case *retryExecutor:
			tool = typed.delegate
		case *toolspolicy.ApprovalExecutor:
			tool = typed.Delegate()
		case *validatingExecutor:
			tool = typed.delegate
		default:
			return tool
		}
	}
}

func (r *Registry) wrapDegradation(toolName string, tool tools.ToolExecutor) tools.ToolExecutor {
	if tool == nil {
		return nil
	}
	fallbacks, ok := r.degradation.FallbackMap[toolName]
	if !ok || len(fallbacks) == 0 {
		return tool
	}
	lookup := func(name string) (tools.ToolExecutor, bool) {
		executor, err := r.Get(name)
		if err != nil {
			return nil, false
		}
		return executor, true
	}
	return NewDegradationExecutor(tool, lookup, r.degradation)
}

type idAwareExecutor struct {
	delegate tools.ToolExecutor
}

func (w *idAwareExecutor) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	result, err := w.delegate.Execute(ctx, call)
	if result != nil {
		if result.CallID == "" {
			result.CallID = call.ID
		}
		if result.SessionID == "" {
			result.SessionID = call.SessionID
		}
		if result.TaskID == "" {
			result.TaskID = call.TaskID
		}
		if result.ParentTaskID == "" {
			result.ParentTaskID = call.ParentTaskID
		}
	}
	return result, err
}

func (w *idAwareExecutor) Definition() ports.ToolDefinition {
	return w.delegate.Definition()
}

func (w *idAwareExecutor) Metadata() ports.ToolMetadata {
	return w.delegate.Metadata()
}

// WithoutOrchestration returns a filtered registry that excludes orchestration
// tools. This prevents recursive delegation chains inside background tasks.
func (r *Registry) WithoutOrchestration() tools.ToolRegistry {
	return &filteredRegistry{
		parent: r,
		exclude: map[string]bool{
			"run_tasks":   true,
			"reply_agent": true,
		},
	}
}

// filteredRegistry implements tools.ToolRegistry with exclusions

func (f *filteredRegistry) Get(name string) (tools.ToolExecutor, error) {
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

func (f *filteredRegistry) Register(tool tools.ToolExecutor) error {
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
	if !r.defsDirty && r.cachedDefs != nil {
		defs := r.cachedDefs
		r.mu.RUnlock()
		return defs
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	// Double-check after acquiring write lock.
	if !r.defsDirty && r.cachedDefs != nil {
		return r.cachedDefs
	}
	defs := make([]ports.ToolDefinition, 0, len(r.static)+len(r.dynamic)+len(r.mcp))
	for _, tool := range r.static {
		defs = append(defs, tool.Definition())
	}
	for _, tool := range r.dynamic {
		defs = append(defs, tool.Definition())
	}
	for _, tool := range r.mcp {
		defs = append(defs, tool.Definition())
	}
	sort.Slice(defs, func(i, j int) bool {
		return defs[i].Name < defs[j].Name
	})
	r.cachedDefs = defs
	r.defsDirty = false
	return defs
}

// Close releases managed resources.
func (r *Registry) Close() {
	// No-op: browser lifecycle now managed by MCP registry.
}

func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.static[name]; ok {
		return fmt.Errorf("cannot unregister built-in tool: %s", name)
	}
	delete(r.dynamic, name)
	delete(r.mcp, name)
	r.defsDirty = true
	return nil
}

func (r *Registry) registerBuiltins(config Config) error {
	disabled := resolveDisabledTools(config)

	r.registerUITools(config)
	r.registerWebTools(config)
	r.registerSessionTools()
	if err := r.registerPlatformTools(config); err != nil {
		return err
	}
	r.registerLarkTools()
	r.pruneDisabledTools(disabled)

	for name, tool := range r.static {
		wrapped := wrapTool(tool, r.policy, r.breakers, r.SLACollector)
		r.static[name] = r.wrapDegradation(name, wrapped)
	}
	return nil
}

func resolveDisabledTools(config Config) map[string]string {
	if len(config.DisabledTools) > 0 {
		cloned := make(map[string]string, len(config.DisabledTools))
		for name, reason := range config.DisabledTools {
			cloned[name] = reason
		}
		return cloned
	}

	if runtimeconfig.NormalizeRuntimeProfile(config.Profile) != runtimeconfig.RuntimeProfileQuickstart {
		return nil
	}

	disabled := map[string]string{}
	if utils.IsBlank(config.TavilyAPIKey) {
		disabled["web_search"] = "missing TAVILY_API_KEY in quickstart profile"
	}
	if utils.IsBlank(config.ArkAPIKey) {
		disabled["text_to_image"] = "missing ARK_API_KEY in quickstart profile"
		disabled["image_to_image"] = "missing ARK_API_KEY in quickstart profile"
		disabled["vision_analyze"] = "missing ARK_API_KEY in quickstart profile"
		disabled["video_generate"] = "missing ARK_API_KEY in quickstart profile"
	}

	if len(disabled) == 0 {
		return nil
	}

	return disabled
}

func (r *Registry) pruneDisabledTools(disabled map[string]string) {
	if len(disabled) == 0 {
		return
	}
	for name := range disabled {
		delete(r.static, name)
	}
}

// RegisterOrchestration registers file-based orchestration tools.
// Dispatcher is injected via context at runtime.
func (r *Registry) RegisterOrchestration() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.static["run_tasks"]; exists {
		return
	}

	r.static["run_tasks"] = r.wrapDegradation("run_tasks", wrapTool(orchestration.NewRunTasks(), r.policy, r.breakers, r.SLACollector))
	r.static["reply_agent"] = r.wrapDegradation("reply_agent", wrapTool(orchestration.NewReplyAgent(), r.policy, r.breakers, r.SLACollector))
	r.defsDirty = true
}
