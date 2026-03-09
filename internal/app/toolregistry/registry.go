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
)

// Registry implements ToolRegistry with three-tier caching
type Registry struct {
	static       map[string]tools.ToolExecutor
	dynamic      map[string]tools.ToolExecutor
	mu           sync.RWMutex
	cachedDefs   []ports.ToolDefinition
	defsDirty    bool
	policy       tools.ToolPolicy
	breakers     *circuitBreakerStore
	degradation  DegradationConfig
	SLACollector *toolspolicy.SLACollector
}

type Config struct {
	Profile string

	TavilyAPIKey string

	ArkAPIKey     string
	MemoryEngine  memory.Engine
	HTTPLimits    runtimeconfig.HTTPLimitsConfig
	ToolPolicy    tools.ToolPolicy
	BreakerConfig CircuitBreakerConfig
	SLACollector  *toolspolicy.SLACollector
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

	wrapped := wrapTool(tool, r.policy, r.breakers, r.SLACollector)
	wrapped = r.wrapDegradationLocked(name, wrapped)
	r.dynamic[name] = wrapped
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
	return nil, false
}

// wrapTool ensures tools are wrapped with validation, approval, retry, ID
// propagation, and optional SLA measurement. The SLA executor is the outermost
// layer so it measures total time including retries and approval.
func wrapTool(tool tools.ToolExecutor, policy tools.ToolPolicy, breakers *circuitBreakerStore, sla *toolspolicy.SLACollector) tools.ToolExecutor {
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

// unwrapTool peels off all decorator layers to reach the base tool
// implementation using the Unwrappable interface.
func unwrapTool(tool tools.ToolExecutor) tools.ToolExecutor {
	for {
		u, ok := tool.(tools.Unwrappable)
		if !ok {
			return tool
		}
		inner := u.Unwrap()
		if inner == nil {
			return tool
		}
		tool = inner
	}
}

// wrapDegradationLocked wraps a tool with degradation logic. Caller must
// hold r.mu (read or write). The lookup closure uses getRawLocked to avoid
// re-acquiring the lock.
func (r *Registry) wrapDegradationLocked(toolName string, tool tools.ToolExecutor) tools.ToolExecutor {
	if tool == nil {
		return nil
	}
	fallbacks, ok := r.degradation.FallbackMap[toolName]
	if !ok || len(fallbacks) == 0 {
		return tool
	}
	lookup := func(name string) (tools.ToolExecutor, bool) {
		executor, ok := r.getRawLocked(name)
		return executor, ok
	}
	return NewDegradationExecutor(tool, lookup, r.degradation)
}

type idAwareExecutor struct {
	delegate tools.ToolExecutor
}

// Unwrap returns the inner executor (implements tools.Unwrappable).
func (w *idAwareExecutor) Unwrap() tools.ToolExecutor {
	return w.delegate
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

// WithoutOrchestration returns the registry view used by subagents and other
// delegated execution paths. Orchestration now runs through CLI services,
// so this is intentionally a no-op that returns the full registry.
func (r *Registry) WithoutOrchestration() tools.ToolRegistry {
	return r
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
	defs := make([]ports.ToolDefinition, 0, len(r.static)+len(r.dynamic))
	for _, tool := range r.static {
		defs = append(defs, tool.Definition())
	}
	for _, tool := range r.dynamic {
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
	// No-op.
}

func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.static[name]; ok {
		return fmt.Errorf("cannot unregister built-in tool: %s", name)
	}
	delete(r.dynamic, name)
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
	r.pruneDisabledTools(disabled)

	for name, tool := range r.static {
		wrapped := wrapTool(tool, r.policy, r.breakers, r.SLACollector)
		r.static[name] = r.wrapDegradationLocked(name, wrapped)
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

// Compile-time interface checks.
var _ tools.ToolExecutor = (*idAwareExecutor)(nil)
