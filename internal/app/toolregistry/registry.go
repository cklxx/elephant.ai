package toolregistry

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	portsllm "alex/internal/domain/agent/ports/llm"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/memory"
	toolspolicy "alex/internal/infra/tools"
	runtimeconfig "alex/internal/shared/config"

	"alex/internal/infra/tools/builtin/browser"
	"alex/internal/infra/tools/builtin/orchestration"
	"alex/internal/infra/tools/builtin/shared"
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
	browserMgr   *browser.Manager
}

// filteredRegistry wraps a parent registry and excludes certain tools
type filteredRegistry struct {
	parent  *Registry
	exclude map[string]bool
}

type Config struct {
	TavilyAPIKey string

	ArkAPIKey                  string
	SeedreamTextEndpointID     string
	SeedreamImageEndpointID    string
	SeedreamTextModel          string
	SeedreamImageModel         string
	SeedreamVisionModel        string
	SeedreamVideoModel         string
	LLMVisionModel             string
	SandboxBaseURL             string
	ACPExecutorAddr            string
	ACPExecutorCWD             string
	ACPExecutorMode            string
	ACPExecutorAutoApprove     bool
	ACPExecutorMaxCLICalls     int
	ACPExecutorMaxDuration     int
	ACPExecutorRequireManifest bool

	LLMFactory    portsllm.LLMClientFactory
	LLMProvider   string
	LLMModel      string
	APIKey        string
	BaseURL       string
	MemoryEngine  memory.Engine
	OKRGoalsRoot  string
	HTTPLimits    runtimeconfig.HTTPLimitsConfig
	ToolPolicy    toolspolicy.ToolPolicy
	BreakerConfig CircuitBreakerConfig
	SLACollector  *toolspolicy.SLACollector
	// DegradationConfig, when provided, overrides the registry defaults.
	// When nil, DefaultRegistryDegradationConfig is used.
	DegradationConfig *DegradationConfig
	Toolset           Toolset
	BrowserConfig     BrowserConfig
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
	if aliasTool := r.resolveLegacyAliasLocked(name); aliasTool != nil {
		return aliasTool, nil
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
	approval := &approvalExecutor{delegate: validated}
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
		case *approvalExecutor:
			tool = typed.delegate
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

type approvalExecutor struct {
	delegate tools.ToolExecutor
}

func (a *approvalExecutor) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	if a.delegate == nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("tool executor missing")}, nil
	}
	meta := a.delegate.Metadata()
	if !meta.Dangerous {
		return a.delegate.Execute(ctx, call)
	}
	approver := shared.GetApproverFromContext(ctx)
	if approver == nil || shared.GetAutoApproveFromContext(ctx) {
		return a.delegate.Execute(ctx, call)
	}

	req := &tools.ApprovalRequest{
		Operation:   meta.Name,
		FilePath:    extractFilePath(call.Arguments),
		Summary:     buildApprovalSummary(meta, call),
		AutoApprove: shared.GetAutoApproveFromContext(ctx),
		ToolCallID:  call.ID,
		ToolName:    call.Name,
		Arguments:   call.Arguments,
		SafetyLevel: meta.EffectiveSafetyLevel(),
	}
	if req.SafetyLevel >= ports.SafetyLevelHighImpact {
		req.RollbackSteps = buildRollbackSteps(meta, req.FilePath)
	}
	if req.SafetyLevel >= ports.SafetyLevelIrreversible {
		req.AlternativePlan = buildAlternativePlan(meta)
	}
	resp, err := approver.RequestApproval(ctx, req)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}
	if resp == nil || !resp.Approved {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("operation rejected")}, nil
	}

	return a.delegate.Execute(ctx, call)
}

func (a *approvalExecutor) Definition() ports.ToolDefinition {
	return a.delegate.Definition()
}

func (a *approvalExecutor) Metadata() ports.ToolMetadata {
	return a.delegate.Metadata()
}

func extractFilePath(args map[string]any) string {
	if args == nil {
		return ""
	}
	for _, key := range []string{"file_path", "path", "resolved_path"} {
		if val, ok := args[key].(string); ok {
			return strings.TrimSpace(val)
		}
	}
	return ""
}

func buildApprovalSummary(meta ports.ToolMetadata, call ports.ToolCall) string {
	parts := []string{fmt.Sprintf("Approval required for %s (L%d)", meta.Name, meta.EffectiveSafetyLevel())}
	if filePath := extractFilePath(call.Arguments); filePath != "" {
		parts = append(parts, fmt.Sprintf("path=%s", filePath))
	}
	if keys := extractArgumentKeys(call.Arguments); len(keys) > 0 {
		parts = append(parts, fmt.Sprintf("args=%s", strings.Join(keys, ", ")))
	}
	return strings.Join(parts, "; ")
}

func extractArgumentKeys(args map[string]any) []string {
	if len(args) == 0 {
		return nil
	}
	keys := make([]string, 0, len(args))
	for key := range args {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return nil
	}
	sort.Strings(keys)
	if len(keys) > 8 {
		keys = append(keys[:8], "...")
	}
	return keys
}

func buildRollbackSteps(meta ports.ToolMetadata, filePath string) string {
	if filePath != "" {
		return fmt.Sprintf("If outcome is incorrect, restore %s from VCS/backups and rerun the last known-good step.", filePath)
	}
	return fmt.Sprintf("If outcome is incorrect, revert the %s operation via VCS/rollback tooling and rerun the last known-good step.", meta.Name)
}

func buildAlternativePlan(meta ports.ToolMetadata) string {
	if strings.Contains(strings.ToLower(meta.Name), "delete") {
		return "Prefer archive/disable first; verify impact in read-only mode before irreversible deletion."
	}
	return "Run a read-only or dry-run check first, then apply the smallest reversible change."
}

// WithoutSubagent returns a filtered registry that excludes the subagent tool
// This prevents nested subagent calls at registration level
func (r *Registry) WithoutSubagent() tools.ToolRegistry {
	return &filteredRegistry{
		parent: r,
		// Exclude delegation tools to prevent recursive delegation chains inside subagents.
		exclude: map[string]bool{
			"subagent":     true,
			"explore":      true,
			"acp_executor": true,
			"bg_dispatch":  true,
			"bg_status":    true,
			"bg_collect":   true,
			"ext_reply":    true,
			"ext_merge":    true,
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

// Close releases managed resources (e.g. the shared Chrome process).
func (r *Registry) Close() {
	if r == nil {
		return
	}
	if r.browserMgr != nil {
		r.browserMgr.Close()
		r.browserMgr = nil
	}
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
	shellConfig := shared.ShellToolConfig{}

	r.registerSearchTools(shellConfig)
	r.registerSessionTools(config.HTTPLimits)
	r.registerArtifactTools()
	r.registerExecutionTools(config)
	r.registerUITools(config)
	if err := r.registerWebTools(config); err != nil {
		return err
	}
	visionTool := r.registerMediaTools(config)
	if err := r.registerPlatformTools(config, visionTool); err != nil {
		return err
	}
	r.registerLarkTools()
	r.registerOKRTools(config.OKRGoalsRoot)
	r.registerTimerSchedulerTools()

	// Pre-wrap all static tools with approval, retry, ID propagation, and SLA.
	for name, tool := range r.static {
		wrapped := wrapTool(tool, r.policy, r.breakers, r.SLACollector)
		r.static[name] = r.wrapDegradation(name, wrapped)
	}

	return nil
}

// RegisterSubAgent registers the subagent tool that requires a coordinator
func (r *Registry) RegisterSubAgent(coordinator agent.AgentCoordinator) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if coordinator == nil {
		return
	}

	if _, exists := r.static["subagent"]; exists {
		if _, ok := r.static["explore"]; !ok {
			wrapped := wrapTool(orchestration.NewExplore(r.static["subagent"]), r.policy, r.breakers, r.SLACollector)
			r.static["explore"] = r.wrapDegradation("explore", wrapped)
			r.defsDirty = true
		}
		return
	}

	subTool := orchestration.NewSubAgent(coordinator, 3)
	subWrapped := wrapTool(subTool, r.policy, r.breakers, r.SLACollector)
	r.static["subagent"] = r.wrapDegradation("subagent", subWrapped)
	exploreWrapped := wrapTool(orchestration.NewExplore(subTool), r.policy, r.breakers, r.SLACollector)
	r.static["explore"] = r.wrapDegradation("explore", exploreWrapped)

	// Register background task tools (dispatcher injected via context at runtime).
	r.static["bg_dispatch"] = r.wrapDegradation("bg_dispatch", wrapTool(orchestration.NewBGDispatch(), r.policy, r.breakers, r.SLACollector))
	r.static["bg_status"] = r.wrapDegradation("bg_status", wrapTool(orchestration.NewBGStatus(), r.policy, r.breakers, r.SLACollector))
	r.static["bg_collect"] = r.wrapDegradation("bg_collect", wrapTool(orchestration.NewBGCollect(), r.policy, r.breakers, r.SLACollector))
	r.static["ext_reply"] = r.wrapDegradation("ext_reply", wrapTool(orchestration.NewExtReply(), r.policy, r.breakers, r.SLACollector))
	r.static["ext_merge"] = r.wrapDegradation("ext_merge", wrapTool(orchestration.NewExtMerge(), r.policy, r.breakers, r.SLACollector))
	r.defsDirty = true
}
