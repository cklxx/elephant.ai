package toolregistry

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"alex/internal/agent/ports"
	runtimeconfig "alex/internal/config"
	"alex/internal/llm"
	"alex/internal/memory"
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

	ArkAPIKey               string
	SeedreamTextEndpointID  string
	SeedreamImageEndpointID string
	SeedreamTextModel       string
	SeedreamImageModel      string
	SeedreamVisionModel     string
	SeedreamVideoModel      string
	LLMVisionModel          string

	LLMFactory    ports.LLMClientFactory
	LLMProvider   string
	LLMModel      string
	APIKey        string
	BaseURL       string
	MemoryService memory.Service
}

func NewRegistry(config Config) (*Registry, error) {
	r := &Registry{
		static:  make(map[string]ports.ToolExecutor),
		dynamic: make(map[string]ports.ToolExecutor),
		mcp:     make(map[string]ports.ToolExecutor),
	}

	if config.MemoryService == nil {
		config.MemoryService = memory.NewService(memory.NewInMemoryStore())
	}

	if err := r.registerBuiltins(Config{
		TavilyAPIKey:            config.TavilyAPIKey,
		ArkAPIKey:               config.ArkAPIKey,
		LLMFactory:              config.LLMFactory,
		LLMProvider:             config.LLMProvider,
		LLMModel:                config.LLMModel,
		LLMVisionModel:          config.LLMVisionModel,
		APIKey:                  config.APIKey,
		BaseURL:                 config.BaseURL,
		SeedreamTextEndpointID:  config.SeedreamTextEndpointID,
		SeedreamImageEndpointID: config.SeedreamImageEndpointID,
		SeedreamTextModel:       config.SeedreamTextModel,
		SeedreamImageModel:      config.SeedreamImageModel,
		SeedreamVisionModel:     config.SeedreamVisionModel,
		SeedreamVideoModel:      config.SeedreamVideoModel,
		MemoryService:           config.MemoryService,
	}); err != nil {
		return nil, err
	}

	return r, nil
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
func wrapWithIDPropagation(tool ports.ToolExecutor) ports.ToolExecutor {
	if tool == nil {
		return nil
	}
	if _, ok := tool.(*idAwareExecutor); ok {
		return tool
	}
	return &idAwareExecutor{delegate: tool}
}

type idAwareExecutor struct {
	delegate ports.ToolExecutor
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

// WithoutSubagent returns a filtered registry that excludes the subagent tool
// This prevents nested subagent calls at registration level
func (r *Registry) WithoutSubagent() ports.ToolRegistry {
	return &filteredRegistry{
		parent: r,
		// Exclude both subagent and explore (which wraps subagent) to prevent
		// recursive delegation chains inside subagents.
		exclude: map[string]bool{"subagent": true, "explore": true},
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

func (r *Registry) registerBuiltins(config Config) error {
	fileConfig := builtin.FileToolConfig{}
	shellConfig := builtin.ShellToolConfig{}

	// File operations
	r.static["file_read"] = builtin.NewFileRead(fileConfig)
	r.static["file_write"] = builtin.NewFileWrite(fileConfig)
	r.static["file_edit"] = builtin.NewFileEdit(fileConfig)
	r.static["list_files"] = builtin.NewListFiles(fileConfig)

	// Shell & search
	r.static["bash"] = builtin.NewBash(shellConfig)
	r.static["grep"] = builtin.NewGrep(shellConfig)
	r.static["ripgrep"] = builtin.NewRipgrep(shellConfig)
	r.static["find"] = builtin.NewFind(shellConfig)
	// TODO: full impl code search
	// r.static["code_search"] = builtin.NewCodeSearch()

	// Task management
	r.static["todo_read"] = builtin.NewTodoRead()
	r.static["todo_update"] = builtin.NewTodoUpdate()
	r.static["skills"] = builtin.NewSkills()

	// Attachment and artifact operations
	r.static["artifacts_write"] = builtin.NewArtifactsWrite()
	r.static["artifacts_list"] = builtin.NewArtifactsList()
	r.static["artifacts_delete"] = builtin.NewArtifactsDelete()
	r.static["a2ui_emit"] = builtin.NewA2UIEmit()

	// Execution & reasoning
	r.static["code_execute"] = builtin.NewCodeExecute(builtin.CodeExecuteConfig{})

	// UI orchestration
	r.static["plan"] = builtin.NewPlan(config.MemoryService)
	r.static["clearify"] = builtin.NewClearify()
	r.static["memory_recall"] = builtin.NewMemoryRecall(config.MemoryService)
	r.static["memory_write"] = builtin.NewMemoryWrite(config.MemoryService)
	r.static["attention"] = builtin.NewAttention()

	// Web tools
	r.static["web_search"] = builtin.NewWebSearch(config.TavilyAPIKey)
	r.static["flow_search"] = builtin.NewFlowSearch(config.TavilyAPIKey)
	writeLLM := llm.NewMockClient()
	provider := strings.TrimSpace(config.LLMProvider)
	model := strings.TrimSpace(config.LLMModel)
	if provider != "" && provider != "mock" {
		if config.LLMFactory == nil {
			return fmt.Errorf("flow_write: LLMFactory is required when provider is %q", provider)
		}
		if model == "" {
			return fmt.Errorf("flow_write: model is required when provider is %q", provider)
		}
		client, err := config.LLMFactory.GetClient(provider, model, ports.LLMConfig{
			APIKey:  config.APIKey,
			BaseURL: config.BaseURL,
		})
		if err != nil {
			return fmt.Errorf("flow_write: failed to create LLM client: %w", err)
		}
		writeLLM = client
	}
	r.static["flow_write"] = builtin.NewFlowWrite(writeLLM)
	r.static["html_edit"] = builtin.NewHTMLEdit(writeLLM)
	r.static["web_fetch"] = builtin.NewWebFetch(builtin.WebFetchConfig{
		// Reserved for future config.
	})
	r.static["douyin_hot"] = builtin.NewDouyinHot()
	miniappLLM := writeLLM
	if provider != "" && provider != "mock" {
		if config.LLMFactory == nil {
			return fmt.Errorf("miniapp_html: LLMFactory is required when provider is %q", provider)
		}
		if model == "" {
			return fmt.Errorf("miniapp_html: model is required when provider is %q", provider)
		}
		client, err := config.LLMFactory.GetClient(provider, model, ports.LLMConfig{
			APIKey:  config.APIKey,
			BaseURL: config.BaseURL,
		})
		if err != nil {
			return fmt.Errorf("miniapp_html: failed to create LLM client: %w", err)
		}
		miniappLLM = client
	}
	r.static["miniapp_html"] = builtin.NewMiniAppHTMLWithLLM(miniappLLM)

	// Document generation
	r.static["pptx_from_images"] = builtin.NewPPTXFromImages()

	seedreamBase := builtin.SeedreamConfig{
		APIKey: config.ArkAPIKey,
	}
	if config.SeedreamTextModel != "" {
		textConfig := seedreamBase
		textConfig.Model = config.SeedreamTextModel
		textConfig.ModelDescriptor = "Seedream 4.5 text-to-image"
		textConfig.ModelEnvVar = "SEEDREAM_TEXT_MODEL"
		r.static["text_to_image"] = builtin.NewSeedreamTextToImage(textConfig)
	}
	if config.SeedreamImageModel != "" {
		imageConfig := seedreamBase
		imageConfig.Model = config.SeedreamImageModel
		imageConfig.ModelDescriptor = "Seedream 4.5 image-to-image"
		imageConfig.ModelEnvVar = "SEEDREAM_IMAGE_MODEL"
		r.static["image_to_image"] = builtin.NewSeedreamImageToImage(imageConfig)
	}
	var visionTool ports.ToolExecutor
	if config.SeedreamVisionModel != "" {
		visionConfig := seedreamBase
		visionConfig.Model = config.SeedreamVisionModel
		visionConfig.ModelDescriptor = "Seedream vision analysis"
		visionConfig.ModelEnvVar = "SEEDREAM_VISION_MODEL"
		visionTool = builtin.NewVisionAnalyze(builtin.VisionConfig{
			Provider: builtin.VisionProviderSeedream,
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
		r.static["video_generate"] = builtin.NewSeedreamVideoGenerate(videoConfig)
	}

	r.static["browser"] = builtin.NewBrowser(builtin.BrowserConfig{
		LLMFactory:     config.LLMFactory,
		LLMProvider:    config.LLMProvider,
		LLMModel:       config.LLMModel,
		LLMVisionModel: config.LLMVisionModel,
		APIKey:         config.APIKey,
		BaseURL:        config.BaseURL,
		VisionTool:     visionTool,
	})

	return nil
}

// RegisterSubAgent registers the subagent tool that requires a coordinator
func (r *Registry) RegisterSubAgent(coordinator ports.AgentCoordinator) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if coordinator == nil {
		return
	}

	if _, exists := r.static["subagent"]; exists {
		if _, ok := r.static["explore"]; !ok {
			r.static["explore"] = builtin.NewExplore(r.static["subagent"])
		}
		return
	}

	subTool := builtin.NewSubAgent(coordinator, 3)
	r.static["subagent"] = subTool
	r.static["explore"] = builtin.NewExplore(subTool)
}
