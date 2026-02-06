package toolregistry

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	portsllm "alex/internal/domain/agent/ports/llm"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/llm"
	"alex/internal/infra/memory"
	toolspolicy "alex/internal/infra/tools"
	runtimeconfig "alex/internal/shared/config"

	"alex/internal/infra/tools/builtin/aliases"
	"alex/internal/infra/tools/builtin/artifacts"
	"alex/internal/infra/tools/builtin/browser"
	"alex/internal/infra/tools/builtin/chromebridge"
	"alex/internal/infra/tools/builtin/diagram"
	"alex/internal/infra/tools/builtin/execution"
	"alex/internal/infra/tools/builtin/fileops"
	"alex/internal/infra/tools/builtin/larktools"
	"alex/internal/infra/tools/builtin/media"
	memorytools "alex/internal/infra/tools/builtin/memory"

	okrtools "alex/internal/infra/tools/builtin/okr"
	"alex/internal/infra/tools/builtin/orchestration"
	peekabootools "alex/internal/infra/tools/builtin/peekaboo"
	"alex/internal/infra/tools/builtin/sandbox"
	schedulertools "alex/internal/infra/tools/builtin/scheduler"
	"alex/internal/infra/tools/builtin/search"
	sessiontools "alex/internal/infra/tools/builtin/session"
	"alex/internal/infra/tools/builtin/shared"
	timertools "alex/internal/infra/tools/builtin/timer"
	"alex/internal/infra/tools/builtin/ui"
	"alex/internal/infra/tools/builtin/web"
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
	SLACollector *toolspolicy.SLACollector
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
	Toolset       Toolset
	BrowserConfig BrowserConfig
}

func NewRegistry(config Config) (*Registry, error) {
	policy := config.ToolPolicy
	if policy == nil {
		policy = toolspolicy.NewToolPolicy(toolspolicy.DefaultToolPolicyConfigWithRules())
	}
	breakers := newCircuitBreakerStore(normalizeCircuitBreakerConfig(config.BreakerConfig))

	r := &Registry{
		static:       make(map[string]tools.ToolExecutor),
		dynamic:      make(map[string]tools.ToolExecutor),
		mcp:          make(map[string]tools.ToolExecutor),
		defsDirty:    true,
		policy:       policy,
		breakers:     breakers,
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

// wrapTool ensures tools are wrapped with approval, retry, ID propagation,
// and optional SLA measurement. The SLA executor is the outermost layer so
// it measures total time including retries and approval.
func wrapTool(tool tools.ToolExecutor, policy toolspolicy.ToolPolicy, breakers *circuitBreakerStore, sla *toolspolicy.SLACollector) tools.ToolExecutor {
	if tool == nil {
		return nil
	}
	base := unwrapTool(tool)
	approval := &approvalExecutor{delegate: base}
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
		case *toolspolicy.SLAExecutor:
			tool = typed.Delegate()
		case *idAwareExecutor:
			tool = typed.delegate
		case *retryExecutor:
			tool = typed.delegate
		case *approvalExecutor:
			tool = typed.delegate
		default:
			return tool
		}
	}
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
		Summary:     fmt.Sprintf("Approval required for %s", meta.Name),
		AutoApprove: shared.GetAutoApproveFromContext(ctx),
		ToolCallID:  call.ID,
		ToolName:    call.Name,
		Arguments:   call.Arguments,
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
	fileConfig := shared.FileToolConfig{}
	shellConfig := shared.ShellToolConfig{}
	httpLimits := config.HTTPLimits
	toolset := NormalizeToolset(string(config.Toolset))

	// File operations
	r.static["file_read"] = fileops.NewFileRead(fileConfig)
	r.static["file_write"] = fileops.NewFileWrite(fileConfig)
	r.static["file_edit"] = fileops.NewFileEdit(fileConfig)
	r.static["list_files"] = fileops.NewListFiles(fileConfig)

	// Shell & search
	if execution.LocalExecEnabled {
		r.static["bash"] = execution.NewBash(shellConfig)
	}
	r.static["grep"] = search.NewGrep(shellConfig)
	r.static["ripgrep"] = search.NewRipgrep(shellConfig)
	r.static["find"] = search.NewFind(shellConfig)

	// Task management
	r.static["todo_read"] = sessiontools.NewTodoRead()
	r.static["todo_update"] = sessiontools.NewTodoUpdate()
	r.static["skills"] = sessiontools.NewSkills()
	r.static["apps"] = sessiontools.NewApps()
	r.static["music_play"] = media.NewMusicPlayWithConfig(media.MusicPlayConfig{
		MaxResponseBytes: httpLimits.MusicSearchMaxResponseBytes,
	})

	// Attachment and artifact operations
	r.static["artifacts_write"] = artifacts.NewArtifactsWrite()
	r.static["artifacts_list"] = artifacts.NewArtifactsList()
	r.static["artifacts_delete"] = artifacts.NewArtifactsDelete()
	r.static["a2ui_emit"] = artifacts.NewA2UIEmit()
	r.static["artifact_manifest"] = artifacts.NewArtifactManifest()

	// Execution & reasoning
	if execution.LocalExecEnabled {
		r.static["code_execute"] = execution.NewCodeExecute(execution.CodeExecuteConfig{})
	}
	r.static["acp_executor"] = execution.NewACPExecutor(execution.ACPExecutorConfig{
		Addr:                    config.ACPExecutorAddr,
		CWD:                     config.ACPExecutorCWD,
		Mode:                    config.ACPExecutorMode,
		AutoApprove:             config.ACPExecutorAutoApprove,
		MaxCLICalls:             config.ACPExecutorMaxCLICalls,
		MaxDurationSeconds:      config.ACPExecutorMaxDuration,
		RequireArtifactManifest: config.ACPExecutorRequireManifest,
	})

	// UI orchestration
	r.static["plan"] = ui.NewPlan(config.MemoryEngine)
	r.static["clarify"] = ui.NewClarify()
	r.static["memory_search"] = memorytools.NewMemorySearch(config.MemoryEngine)
	r.static["memory_get"] = memorytools.NewMemoryGet(config.MemoryEngine)
	r.static["request_user"] = ui.NewRequestUser()

	// Web tools
	r.static["web_search"] = web.NewWebSearch(config.TavilyAPIKey, web.WebSearchConfig{
		MaxResponseBytes: httpLimits.WebSearchMaxResponseBytes,
	})
	writeLLM := llm.NewMockClient()
	provider := strings.TrimSpace(config.LLMProvider)
	model := strings.TrimSpace(config.LLMModel)
	if provider != "" && provider != "mock" {
		if config.LLMFactory == nil {
			return fmt.Errorf("html_edit: LLMFactory is required when provider is %q", provider)
		}
		if model == "" {
			return fmt.Errorf("html_edit: model is required when provider is %q", provider)
		}
		client, err := config.LLMFactory.GetClient(provider, model, portsllm.LLMConfig{
			APIKey:  config.APIKey,
			BaseURL: config.BaseURL,
		})
		if err != nil {
			return fmt.Errorf("html_edit: failed to create LLM client: %w", err)
		}
		writeLLM = client
	}
	r.static["html_edit"] = web.NewHTMLEdit(writeLLM, web.HTMLEditConfig{
		MaxResponseBytes: httpLimits.DefaultMaxResponseBytes,
	})
	r.static["web_fetch"] = web.NewWebFetch(shared.WebFetchConfig{
		MaxResponseBytes: httpLimits.WebFetchMaxResponseBytes,
	})
	r.static["douyin_hot"] = web.NewDouyinHot()
	// Document generation
	r.static["pptx_from_images"] = artifacts.NewPPTXFromImages()

	seedreamBase := media.SeedreamConfig{
		APIKey: config.ArkAPIKey,
	}
	if config.SeedreamTextModel != "" {
		textConfig := seedreamBase
		textConfig.Model = config.SeedreamTextModel
		textConfig.ModelDescriptor = "Seedream 4.5 text-to-image"
		textConfig.ModelEnvVar = "SEEDREAM_TEXT_MODEL"
		r.static["text_to_image"] = media.NewSeedreamTextToImage(textConfig)
	}
	if config.SeedreamImageModel != "" {
		imageConfig := seedreamBase
		imageConfig.Model = config.SeedreamImageModel
		imageConfig.ModelDescriptor = "Seedream 4.5 image-to-image"
		imageConfig.ModelEnvVar = "SEEDREAM_IMAGE_MODEL"
		r.static["image_to_image"] = media.NewSeedreamImageToImage(imageConfig)
	}
	var visionTool tools.ToolExecutor
	if config.SeedreamVisionModel != "" {
		visionConfig := seedreamBase
		visionConfig.Model = config.SeedreamVisionModel
		visionConfig.ModelDescriptor = "Seedream vision analysis"
		visionConfig.ModelEnvVar = "SEEDREAM_VISION_MODEL"
		visionTool = media.NewVisionAnalyze(media.VisionConfig{
			Provider: media.VisionProviderSeedream,
			Seedream: visionConfig,
		})
		r.static["vision_analyze"] = visionTool
	}
	videoModel := strings.TrimSpace(config.SeedreamVideoModel)
	if videoModel == "" {
		videoModel = runtimeconfig.DefaultSeedreamVideoModel
	}
	if videoModel != "" {
		videoConfig := seedreamBase
		videoConfig.Model = videoModel
		videoConfig.ModelDescriptor = "Seedance video generation"
		videoConfig.ModelEnvVar = "SEEDREAM_VIDEO_MODEL"
		r.static["video_generate"] = media.NewSeedreamVideoGenerate(videoConfig)
	}
	switch toolset {
	case ToolsetLarkLocal:
		browserCfg := browser.Config{
			CDPURL:      config.BrowserConfig.CDPURL,
			ChromePath:  config.BrowserConfig.ChromePath,
			Headless:    config.BrowserConfig.Headless,
			UserDataDir: config.BrowserConfig.UserDataDir,
			Timeout:     config.BrowserConfig.Timeout,
			VisionTool:  visionTool,
		}
		browserMgr := browser.NewManager(browserCfg)
		r.static["browser_action"] = browser.NewBrowserAction(browserMgr)
		r.static["browser_info"] = browser.NewBrowserInfo(browserMgr)
		r.static["browser_screenshot"] = browser.NewBrowserScreenshot(browserMgr)
		r.static["browser_dom"] = browser.NewBrowserDOM(browserMgr)
		r.static["diagram_render"] = diagram.NewDiagramRenderLocal(diagram.LocalConfig{
			ChromePath:  config.BrowserConfig.ChromePath,
			Headless:    config.BrowserConfig.Headless,
			UserDataDir: config.BrowserConfig.UserDataDir,
			Timeout:     config.BrowserConfig.Timeout,
		})
		if strings.EqualFold(strings.TrimSpace(config.BrowserConfig.Connector), "chrome_extension") {
			bridge := chromebridge.New(chromebridge.Config{
				ListenAddr: config.BrowserConfig.BridgeListenAddr,
				Token:      config.BrowserConfig.BridgeToken,
				Timeout:    config.BrowserConfig.Timeout,
			})
			if err := bridge.Start(); err != nil {
				return fmt.Errorf("chrome extension bridge: %w", err)
			}
			r.static["browser_session_status"] = chromebridge.NewBrowserSessionStatus(bridge)
			r.static["browser_cookies"] = chromebridge.NewBrowserCookies(bridge)
			r.static["browser_storage_local"] = chromebridge.NewBrowserStorageLocal(bridge)
		}
		r.static["read_file"] = aliases.NewReadFile(fileConfig)
		r.static["write_file"] = aliases.NewWriteFile(fileConfig)
		r.static["list_dir"] = aliases.NewListDir(fileConfig)
		r.static["search_file"] = aliases.NewSearchFile(fileConfig)
		r.static["replace_in_file"] = aliases.NewReplaceInFile(fileConfig)
		r.static["shell_exec"] = aliases.NewShellExec(shellConfig)
		r.static["execute_code"] = aliases.NewExecuteCode(shellConfig)
		if runtime.GOOS == "darwin" {
			r.static["peekaboo_exec"] = peekabootools.NewPeekabooExec()
		}
	default:
		sandboxConfig := sandbox.SandboxConfig{
			BaseURL:          config.SandboxBaseURL,
			VisionTool:       visionTool,
			VisionPrompt:     "",
			MaxResponseBytes: httpLimits.SandboxMaxResponseBytes,
		}
		r.static["browser_action"] = sandbox.NewSandboxBrowser(sandboxConfig)
		r.static["browser_info"] = sandbox.NewSandboxBrowserInfo(sandboxConfig)
		r.static["browser_screenshot"] = sandbox.NewSandboxBrowserScreenshot(sandboxConfig)
		r.static["browser_dom"] = sandbox.NewSandboxBrowserDOM(sandboxConfig)
		r.static["diagram_render"] = diagram.NewDiagramRenderSandbox(diagram.SandboxConfig{
			BaseURL:          config.SandboxBaseURL,
			MaxResponseBytes: httpLimits.SandboxMaxResponseBytes,
		})
		r.static["read_file"] = sandbox.NewSandboxFileRead(sandboxConfig)
		r.static["write_file"] = sandbox.NewSandboxFileWrite(sandboxConfig)
		r.static["list_dir"] = sandbox.NewSandboxFileList(sandboxConfig)
		r.static["search_file"] = sandbox.NewSandboxFileSearch(sandboxConfig)
		r.static["replace_in_file"] = sandbox.NewSandboxFileReplace(sandboxConfig)
		r.static["shell_exec"] = sandbox.NewSandboxShellExec(sandboxConfig)
		r.static["execute_code"] = sandbox.NewSandboxCodeExecute(sandboxConfig)
		r.static["write_attachment"] = sandbox.NewSandboxWriteAttachment(sandboxConfig)
	}

	// Lark tools
	r.static["lark_chat_history"] = larktools.NewLarkChatHistory()
	r.static["lark_send_message"] = larktools.NewLarkSendMessage()
	r.static["lark_upload_file"] = larktools.NewLarkUploadFile()
	r.static["lark_calendar_query"] = larktools.NewLarkCalendarQuery()
	r.static["lark_calendar_create"] = larktools.NewLarkCalendarCreate()
	r.static["lark_calendar_update"] = larktools.NewLarkCalendarUpdate()
	r.static["lark_calendar_delete"] = larktools.NewLarkCalendarDelete()
	r.static["lark_task_manage"] = larktools.NewLarkTaskManage()

	// OKR tools
	okrCfg := okrtools.DefaultOKRConfig()
	if config.OKRGoalsRoot != "" {
		okrCfg.GoalsRoot = config.OKRGoalsRoot
	}
	r.static["okr_read"] = okrtools.NewOKRRead(okrCfg)
	r.static["okr_write"] = okrtools.NewOKRWrite(okrCfg)

	// Timer tools (stateless; TimerManager injected via context at runtime)
	r.static["set_timer"] = timertools.NewSetTimer()
	r.static["list_timers"] = timertools.NewListTimers()
	r.static["cancel_timer"] = timertools.NewCancelTimer()

	// Scheduler tools (stateless; Scheduler injected via context at runtime)
	r.static["scheduler_create_job"] = schedulertools.NewSchedulerCreate()
	r.static["scheduler_list_jobs"] = schedulertools.NewSchedulerList()
	r.static["scheduler_delete_job"] = schedulertools.NewSchedulerDelete()

	// Moltbook interaction is skill-driven (see skills/moltbook-posting/SKILL.md).
	// The agent uses shell curl commands guided by the skill's API reference.

	// Pre-wrap all static tools with approval, retry, ID propagation, and SLA.
	for name, tool := range r.static {
		r.static[name] = wrapTool(tool, r.policy, r.breakers, r.SLACollector)
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
			r.static["explore"] = wrapTool(orchestration.NewExplore(r.static["subagent"]), r.policy, r.breakers, r.SLACollector)
			r.defsDirty = true
		}
		return
	}

	subTool := orchestration.NewSubAgent(coordinator, 3)
	r.static["subagent"] = wrapTool(subTool, r.policy, r.breakers, r.SLACollector)
	r.static["explore"] = wrapTool(orchestration.NewExplore(subTool), r.policy, r.breakers, r.SLACollector)

	// Register background task tools (dispatcher injected via context at runtime).
	r.static["bg_dispatch"] = wrapTool(orchestration.NewBGDispatch(), r.policy, r.breakers, r.SLACollector)
	r.static["bg_status"] = wrapTool(orchestration.NewBGStatus(), r.policy, r.breakers, r.SLACollector)
	r.static["bg_collect"] = wrapTool(orchestration.NewBGCollect(), r.policy, r.breakers, r.SLACollector)
	r.static["ext_reply"] = wrapTool(orchestration.NewExtReply(), r.policy, r.breakers, r.SLACollector)
	r.static["ext_merge"] = wrapTool(orchestration.NewExtMerge(), r.policy, r.breakers, r.SLACollector)
	r.defsDirty = true
}
