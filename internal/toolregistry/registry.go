package toolregistry

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	portsllm "alex/internal/agent/ports/llm"
	tools "alex/internal/agent/ports/tools"
	runtimeconfig "alex/internal/config"
	"alex/internal/llm"
	"alex/internal/memory"
	"alex/internal/tools/builtin/artifacts"
	"alex/internal/tools/builtin/execution"
	"alex/internal/tools/builtin/fileops"
	"alex/internal/tools/builtin/larktools"
	"alex/internal/tools/builtin/media"
	memorytools "alex/internal/tools/builtin/memory"
	"alex/internal/tools/builtin/orchestration"
	"alex/internal/tools/builtin/sandbox"
	"alex/internal/tools/builtin/search"
	sessiontools "alex/internal/tools/builtin/session"
	"alex/internal/tools/builtin/shared"
	"alex/internal/tools/builtin/ui"
	"alex/internal/tools/builtin/web"
)

// Registry implements ToolRegistry with three-tier caching
type Registry struct {
	static  map[string]tools.ToolExecutor
	dynamic map[string]tools.ToolExecutor
	mcp     map[string]tools.ToolExecutor
	mu      sync.RWMutex
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
	MemoryService memory.Service
}

func NewRegistry(config Config) (*Registry, error) {
	r := &Registry{
		static:  make(map[string]tools.ToolExecutor),
		dynamic: make(map[string]tools.ToolExecutor),
		mcp:     make(map[string]tools.ToolExecutor),
	}

	if config.MemoryService == nil {
		return nil, fmt.Errorf("memory service is required")
	}

	if err := r.registerBuiltins(Config{
		TavilyAPIKey:               config.TavilyAPIKey,
		ArkAPIKey:                  config.ArkAPIKey,
		LLMFactory:                 config.LLMFactory,
		LLMProvider:                config.LLMProvider,
		LLMModel:                   config.LLMModel,
		LLMVisionModel:             config.LLMVisionModel,
		APIKey:                     config.APIKey,
		BaseURL:                    config.BaseURL,
		SeedreamTextEndpointID:     config.SeedreamTextEndpointID,
		SeedreamImageEndpointID:    config.SeedreamImageEndpointID,
		SeedreamTextModel:          config.SeedreamTextModel,
		SeedreamImageModel:         config.SeedreamImageModel,
		SeedreamVisionModel:        config.SeedreamVisionModel,
		SeedreamVideoModel:         config.SeedreamVideoModel,
		ACPExecutorAddr:            config.ACPExecutorAddr,
		ACPExecutorCWD:             config.ACPExecutorCWD,
		ACPExecutorMode:            config.ACPExecutorMode,
		ACPExecutorAutoApprove:     config.ACPExecutorAutoApprove,
		ACPExecutorMaxCLICalls:     config.ACPExecutorMaxCLICalls,
		ACPExecutorMaxDuration:     config.ACPExecutorMaxDuration,
		ACPExecutorRequireManifest: config.ACPExecutorRequireManifest,
		MemoryService:              config.MemoryService,
	}); err != nil {
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
	if len(name) > 5 && name[:5] == "mcp__" {
		r.mcp[name] = tool
	} else {
		r.dynamic[name] = tool
	}
	return nil
}

func (r *Registry) Get(name string) (tools.ToolExecutor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if tool, ok := r.static[name]; ok {
		return wrapWithIDPropagation(tool), nil
	}
	if tool, ok := r.dynamic[name]; ok {
		return wrapWithIDPropagation(tool), nil
	}
	if tool, ok := r.mcp[name]; ok {
		return wrapWithIDPropagation(tool), nil
	}
	return nil, fmt.Errorf("tool not found: %s", name)
}

// wrapWithIDPropagation ensures that tool results always include the originating call's lineage identifiers.
func wrapWithIDPropagation(tool tools.ToolExecutor) tools.ToolExecutor {
	if tool == nil {
		return nil
	}
	tool = ensureApprovalWrapper(tool)
	if _, ok := tool.(*idAwareExecutor); ok {
		return tool
	}
	return &idAwareExecutor{delegate: tool}
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

func ensureApprovalWrapper(tool tools.ToolExecutor) tools.ToolExecutor {
	if tool == nil {
		return nil
	}
	switch typed := tool.(type) {
	case *approvalExecutor:
		return tool
	case *idAwareExecutor:
		if _, ok := typed.delegate.(*approvalExecutor); ok {
			return tool
		}
		typed.delegate = &approvalExecutor{delegate: typed.delegate}
		return tool
	default:
		return &approvalExecutor{delegate: tool}
	}
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
		// Exclude both subagent and explore (which wraps subagent) to prevent
		// recursive delegation chains inside subagents.
		exclude: map[string]bool{"subagent": true, "explore": true, "acp_executor": true},
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
	sort.Slice(defs, func(i, j int) bool {
		return defs[i].Name < defs[j].Name
	})
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

func (r *Registry) registerBuiltins(config Config) error {
	fileConfig := shared.FileToolConfig{}
	shellConfig := shared.ShellToolConfig{}

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
	// TODO: full impl code search
	// r.static["code_search"] = search.NewCodeSearch()

	// Task management
	r.static["todo_read"] = sessiontools.NewTodoRead()
	r.static["todo_update"] = sessiontools.NewTodoUpdate()
	r.static["skills"] = sessiontools.NewSkills()
	r.static["apps"] = sessiontools.NewApps()
	r.static["music_play"] = media.NewMusicPlay()

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
	r.static["plan"] = ui.NewPlan(config.MemoryService)
	r.static["clarify"] = ui.NewClarify()
	r.static["memory_recall"] = memorytools.NewMemoryRecall(config.MemoryService)
	r.static["memory_write"] = memorytools.NewMemoryWrite(config.MemoryService)
	r.static["attention"] = ui.NewAttention()
	r.static["request_user"] = ui.NewRequestUser()

	// Web tools
	r.static["web_search"] = web.NewWebSearch(config.TavilyAPIKey)
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
	r.static["html_edit"] = web.NewHTMLEdit(writeLLM)
	r.static["web_fetch"] = web.NewWebFetch(shared.WebFetchConfig{
		// Reserved for future config.
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
	sandboxConfig := sandbox.SandboxConfig{
		BaseURL:      config.SandboxBaseURL,
		VisionTool:   visionTool,
		VisionPrompt: "",
	}
	r.static["sandbox_browser"] = sandbox.NewSandboxBrowser(sandboxConfig)
	r.static["sandbox_browser_info"] = sandbox.NewSandboxBrowserInfo(sandboxConfig)
	r.static["sandbox_browser_screenshot"] = sandbox.NewSandboxBrowserScreenshot(sandboxConfig)
	r.static["sandbox_browser_dom"] = sandbox.NewSandboxBrowserDOM(sandboxConfig)
	r.static["sandbox_file_read"] = sandbox.NewSandboxFileRead(sandboxConfig)
	r.static["sandbox_file_write"] = sandbox.NewSandboxFileWrite(sandboxConfig)
	r.static["sandbox_file_list"] = sandbox.NewSandboxFileList(sandboxConfig)
	r.static["sandbox_file_search"] = sandbox.NewSandboxFileSearch(sandboxConfig)
	r.static["sandbox_file_replace"] = sandbox.NewSandboxFileReplace(sandboxConfig)
	r.static["sandbox_shell_exec"] = sandbox.NewSandboxShellExec(sandboxConfig)
	r.static["sandbox_code_execute"] = sandbox.NewSandboxCodeExecute(sandboxConfig)
	r.static["sandbox_write_attachment"] = sandbox.NewSandboxWriteAttachment(sandboxConfig)

	// Lark tools
	r.static["lark_chat_history"] = larktools.NewLarkChatHistory()

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
			r.static["explore"] = orchestration.NewExplore(r.static["subagent"])
		}
		return
	}

	subTool := orchestration.NewSubAgent(coordinator, 3)
	r.static["subagent"] = subTool
	r.static["explore"] = orchestration.NewExplore(subTool)
}
