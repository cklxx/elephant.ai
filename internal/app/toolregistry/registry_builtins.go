package toolregistry

import (
	"fmt"
	"runtime"
	"strings"

	portsllm "alex/internal/domain/agent/ports/llm"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/llm"
	runtimeconfig "alex/internal/shared/config"

	"alex/internal/infra/tools/builtin/aliases"
	applescripttools "alex/internal/infra/tools/builtin/applescript"
	"alex/internal/infra/tools/builtin/artifacts"
	"alex/internal/infra/tools/builtin/browser"
	"alex/internal/infra/tools/builtin/chromebridge"
	configtool "alex/internal/infra/tools/builtin/config"
	"alex/internal/infra/tools/builtin/diagram"
	"alex/internal/infra/tools/builtin/execution"
	"alex/internal/infra/tools/builtin/larktools"
	"alex/internal/infra/tools/builtin/media"
	memorytools "alex/internal/infra/tools/builtin/memory"
	okrtools "alex/internal/infra/tools/builtin/okr"
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

func (r *Registry) registerSearchTools(shellConfig shared.ShellToolConfig) {
	r.static["grep"] = search.NewGrep(shellConfig)
	r.static["ripgrep"] = search.NewRipgrep(shellConfig)
	r.static["find"] = search.NewFind(shellConfig)
}

func (r *Registry) registerSessionTools(httpLimits runtimeconfig.HTTPLimitsConfig) {
	r.static["todo_read"] = sessiontools.NewTodoRead()
	r.static["todo_update"] = sessiontools.NewTodoUpdate()
	r.static["skills"] = sessiontools.NewSkills()
	r.static["apps"] = sessiontools.NewApps()
	r.static["music_play"] = media.NewMusicPlayWithConfig(media.MusicPlayConfig{
		MaxResponseBytes: httpLimits.MusicSearchMaxResponseBytes,
	})
}

func (r *Registry) registerArtifactTools() {
	r.static["artifacts_write"] = artifacts.NewArtifactsWrite()
	r.static["artifacts_list"] = artifacts.NewArtifactsList()
	r.static["artifacts_delete"] = artifacts.NewArtifactsDelete()
	r.static["a2ui_emit"] = artifacts.NewA2UIEmit()
	r.static["artifact_manifest"] = artifacts.NewArtifactManifest()
	r.static["pptx_from_images"] = artifacts.NewPPTXFromImages()
}

func (r *Registry) registerExecutionTools(config Config) {
	r.static["acp_executor"] = execution.NewACPExecutor(execution.ACPExecutorConfig{
		Addr:                    config.ACPExecutorAddr,
		CWD:                     config.ACPExecutorCWD,
		Mode:                    config.ACPExecutorMode,
		AutoApprove:             config.ACPExecutorAutoApprove,
		MaxCLICalls:             config.ACPExecutorMaxCLICalls,
		MaxDurationSeconds:      config.ACPExecutorMaxDuration,
		RequireArtifactManifest: config.ACPExecutorRequireManifest,
	})
	r.static["config_manage"] = configtool.NewConfigManage()
}

func (r *Registry) registerUITools(config Config) {
	r.static["plan"] = ui.NewPlan(config.MemoryEngine)
	r.static["clarify"] = ui.NewClarify()
	r.static["memory_search"] = memorytools.NewMemorySearch(config.MemoryEngine)
	r.static["memory_get"] = memorytools.NewMemoryGet(config.MemoryEngine)
	r.static["request_user"] = ui.NewRequestUser()
}

func (r *Registry) registerWebTools(config Config) error {
	r.static["web_search"] = web.NewWebSearch(config.TavilyAPIKey, web.WebSearchConfig{
		MaxResponseBytes: config.HTTPLimits.WebSearchMaxResponseBytes,
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
		MaxResponseBytes: config.HTTPLimits.DefaultMaxResponseBytes,
	})
	r.static["web_fetch"] = web.NewWebFetch(shared.WebFetchConfig{
		MaxResponseBytes: config.HTTPLimits.WebFetchMaxResponseBytes,
	})
	r.static["douyin_hot"] = web.NewDouyinHot()
	return nil
}

// registerMediaTools registers image/video generation tools and returns the
// vision tool (may be nil) for use by platform tools.
func (r *Registry) registerMediaTools(config Config) tools.ToolExecutor {
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
	return visionTool
}

func (r *Registry) registerPlatformTools(config Config, visionTool tools.ToolExecutor) error {
	fileConfig := shared.FileToolConfig{}
	shellConfig := shared.ShellToolConfig{}
	toolset := NormalizeToolset(string(config.Toolset))

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
		r.browserMgr = browserMgr
		r.static["browser_action"] = browser.NewBrowserAction(browserMgr)
		r.static["browser_info"] = browser.NewBrowserInfo(browserMgr)
		r.static["browser_screenshot"] = browser.NewBrowserScreenshot(browserMgr)
		r.static["browser_dom"] = browser.NewBrowserDOM(browserMgr)
		r.static["diagram_render"] = diagram.NewDiagramRenderLocal(diagram.LocalConfig{
			ChromePath:  config.BrowserConfig.ChromePath,
			Headless:    config.BrowserConfig.Headless,
			UserDataDir: config.BrowserConfig.UserDataDir,
			Timeout:     config.BrowserConfig.Timeout,
		}, browserMgr)
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
		r.static["write_attachment"] = aliases.NewWriteAttachment(fileConfig)
		if runtime.GOOS == "darwin" {
			r.static["peekaboo_exec"] = peekabootools.NewPeekabooExec()
			r.static["atlas"] = applescripttools.NewAtlas()
			r.static["chrome"] = applescripttools.NewChrome()
		}
	default:
		sandboxConfig := sandbox.SandboxConfig{
			BaseURL:          config.SandboxBaseURL,
			VisionTool:       visionTool,
			VisionPrompt:     "",
			MaxResponseBytes: config.HTTPLimits.SandboxMaxResponseBytes,
		}
		r.static["browser_action"] = sandbox.NewSandboxBrowser(sandboxConfig)
		r.static["browser_info"] = sandbox.NewSandboxBrowserInfo(sandboxConfig)
		r.static["browser_screenshot"] = sandbox.NewSandboxBrowserScreenshot(sandboxConfig)
		r.static["browser_dom"] = sandbox.NewSandboxBrowserDOM(sandboxConfig)
		r.static["diagram_render"] = diagram.NewDiagramRenderSandbox(diagram.SandboxConfig{
			BaseURL:          config.SandboxBaseURL,
			MaxResponseBytes: config.HTTPLimits.SandboxMaxResponseBytes,
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
	return nil
}

func (r *Registry) registerLarkTools() {
	r.static["channel"] = larktools.NewLarkChannel()
}

func (r *Registry) registerOKRTools(okrGoalsRoot string) {
	okrCfg := okrtools.DefaultOKRConfig()
	if okrGoalsRoot != "" {
		okrCfg.GoalsRoot = okrGoalsRoot
	}
	r.static["okr_read"] = okrtools.NewOKRRead(okrCfg)
	r.static["okr_write"] = okrtools.NewOKRWrite(okrCfg)
}

// registerSkillModePlatformTools registers only the essential platform tools
// for skill mode. Toolset branching (local vs sandbox) is preserved.
func (r *Registry) registerSkillModePlatformTools(config Config) error {
	fileConfig := shared.FileToolConfig{}
	shellConfig := shared.ShellToolConfig{}
	toolset := NormalizeToolset(string(config.Toolset))

	switch toolset {
	case ToolsetLarkLocal:
		browserCfg := browser.Config{
			CDPURL:      config.BrowserConfig.CDPURL,
			ChromePath:  config.BrowserConfig.ChromePath,
			Headless:    config.BrowserConfig.Headless,
			UserDataDir: config.BrowserConfig.UserDataDir,
			Timeout:     config.BrowserConfig.Timeout,
		}
		browserMgr := browser.NewManager(browserCfg)
		r.browserMgr = browserMgr
		r.static["browser_action"] = browser.NewBrowserAction(browserMgr)
		r.static["read_file"] = aliases.NewReadFile(fileConfig)
		r.static["write_file"] = aliases.NewWriteFile(fileConfig)
		r.static["replace_in_file"] = aliases.NewReplaceInFile(fileConfig)
		r.static["shell_exec"] = aliases.NewShellExec(shellConfig)
		r.static["execute_code"] = aliases.NewExecuteCode(shellConfig)
	default:
		sandboxConfig := sandbox.SandboxConfig{
			BaseURL:          config.SandboxBaseURL,
			MaxResponseBytes: config.HTTPLimits.SandboxMaxResponseBytes,
		}
		r.static["browser_action"] = sandbox.NewSandboxBrowser(sandboxConfig)
		r.static["read_file"] = sandbox.NewSandboxFileRead(sandboxConfig)
		r.static["write_file"] = sandbox.NewSandboxFileWrite(sandboxConfig)
		r.static["replace_in_file"] = sandbox.NewSandboxFileReplace(sandboxConfig)
		r.static["shell_exec"] = sandbox.NewSandboxShellExec(sandboxConfig)
		r.static["execute_code"] = sandbox.NewSandboxCodeExecute(sandboxConfig)
	}
	return nil
}

func (r *Registry) registerTimerSchedulerTools() {
	r.static["set_timer"] = timertools.NewSetTimer()
	r.static["list_timers"] = timertools.NewListTimers()
	r.static["cancel_timer"] = timertools.NewCancelTimer()
	r.static["scheduler_create_job"] = schedulertools.NewSchedulerCreate()
	r.static["scheduler_list_jobs"] = schedulertools.NewSchedulerList()
	r.static["scheduler_delete_job"] = schedulertools.NewSchedulerDelete()
}

// registerSkillModeCoreTools registers only the essential core tools when
// SkillMode is enabled. All other functionality is provided by Python skills
// invoked through shell_exec (e.g. python3 skills/<name>/run.py '{...}').
//
// Core tools kept:
//   - Platform (6): read_file, write_file, replace_in_file, shell_exec, browser_action, execute_code
//   - UI (3): plan, clarify, request_user
//   - Memory (2): memory_search, memory_get
//   - Web (1): web_search
//   - Session (1): skills
//   - Lark (8): kept individually (channel consolidation deferred)
//
// Tools removed (~50): grep, ripgrep, find, todo_*, apps, music_play,
//   artifacts_*, a2ui_emit, pptx_from_images, acp_executor, config_manage,
//   html_edit, web_fetch, douyin_hot, text_to_image, image_to_image,
//   vision_analyze, video_generate, diagram_render, okr_*, set_timer,
//   list_timers, cancel_timer, scheduler_*, browser_info/screenshot/dom,
//   list_dir, search_file, write_attachment, peekaboo_exec, atlas, chrome
func (r *Registry) registerSkillModeCoreTools(config Config) error {
	// UI tools (plan, clarify, request_user) + memory tools
	r.registerUITools(config)

	// Web search (essential for agent reasoning)
	r.static["web_search"] = web.NewWebSearch(config.TavilyAPIKey, web.WebSearchConfig{
		MaxResponseBytes: config.HTTPLimits.WebSearchMaxResponseBytes,
	})

	// Skills matcher (discovers and activates Python skills)
	r.static["skills"] = sessiontools.NewSkills()

	// Platform tools (file read/write/edit, shell, browser) — toolset-dependent
	if err := r.registerSkillModePlatformTools(config); err != nil {
		return err
	}

	// Lark tools (kept individually; channel consolidation is Phase 2)
	r.registerLarkTools()

	// Pre-wrap all static tools with approval, retry, ID propagation, and SLA.
	for name, tool := range r.static {
		wrapped := wrapTool(tool, r.policy, r.breakers, r.SLACollector)
		r.static[name] = r.wrapDegradation(name, wrapped)
	}

	return nil
}
