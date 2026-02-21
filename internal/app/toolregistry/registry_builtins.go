package toolregistry

import (
	"alex/internal/infra/tools/builtin/aliases"
	"alex/internal/infra/tools/builtin/larktools"
	memorytools "alex/internal/infra/tools/builtin/memory"
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
	r.static["context_checkpoint"] = ui.NewContextCheckpoint()
}

func (r *Registry) registerWebTools(config Config) {
	r.static["web_search"] = web.NewWebSearch(config.TavilyAPIKey, web.WebSearchConfig{
		MaxResponseBytes: config.HTTPLimits.WebSearchMaxResponseBytes,
	})
}

func (r *Registry) registerSessionTools() {
	r.static["skills"] = sessiontools.NewSkills()
}

// registerPlatformTools registers the essential platform tools (local only).
// Browser automation is provided by the Playwright MCP server registered in the
// MCP registry; no static browser tool is needed here.
func (r *Registry) registerPlatformTools(config Config) error {
	fileConfig := shared.FileToolConfig{}
	shellConfig := shared.ShellToolConfig{}

	r.static["read_file"] = aliases.NewReadFile(fileConfig)
	r.static["write_file"] = aliases.NewWriteFile(fileConfig)
	r.static["replace_in_file"] = aliases.NewReplaceInFile(fileConfig)
	r.static["shell_exec"] = aliases.NewShellExec(shellConfig)
	r.static["execute_code"] = aliases.NewExecuteCode(shellConfig)
	return nil
}

func (r *Registry) registerLarkTools() {
	r.static["channel"] = larktools.NewLarkChannel()
}
