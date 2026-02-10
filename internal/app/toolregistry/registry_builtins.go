package toolregistry

import (
	"alex/internal/infra/tools/builtin/aliases"
	"alex/internal/infra/tools/builtin/browser"
	"alex/internal/infra/tools/builtin/larktools"
	memorytools "alex/internal/infra/tools/builtin/memory"
	"alex/internal/infra/tools/builtin/sandbox"
	sessiontools "alex/internal/infra/tools/builtin/session"
	"alex/internal/infra/tools/builtin/shared"
	"alex/internal/infra/tools/builtin/ui"
	"alex/internal/infra/tools/builtin/web"
)

func (r *Registry) registerUITools(config Config) {
	r.static["plan"] = ui.NewPlan(config.MemoryEngine)
	r.static["clarify"] = ui.NewClarify()
	r.static["memory_search"] = memorytools.NewMemorySearch(config.MemoryEngine)
	r.static["memory_get"] = memorytools.NewMemoryGet(config.MemoryEngine)
	r.static["request_user"] = ui.NewRequestUser()
}

func (r *Registry) registerWebTools(config Config) {
	r.static["web_search"] = web.NewWebSearch(config.TavilyAPIKey, web.WebSearchConfig{
		MaxResponseBytes: config.HTTPLimits.WebSearchMaxResponseBytes,
	})
}

func (r *Registry) registerSessionTools() {
	r.static["skills"] = sessiontools.NewSkills()
}

// registerPlatformTools registers the essential platform tools.
// Toolset branching (local vs sandbox) is preserved.
func (r *Registry) registerPlatformTools(config Config) error {
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

func (r *Registry) registerLarkTools() {
	r.static["channel"] = larktools.NewLarkChannel()
}
