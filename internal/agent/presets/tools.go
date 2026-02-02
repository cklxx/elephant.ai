package presets

import (
	"fmt"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
)

// ToolMode defines the runtime surface the agent runs under.
type ToolMode string

const (
	ToolModeCLI ToolMode = "cli"
	ToolModeWeb ToolMode = "web"
)

// ToolPreset defines tool access levels for CLI and web agents.
type ToolPreset string

const (
	ToolPresetFull      ToolPreset = "full"
	ToolPresetReadOnly  ToolPreset = "read-only"
	ToolPresetSafe      ToolPreset = "safe"
	ToolPresetSandbox   ToolPreset = "sandbox"
	ToolPresetArchitect ToolPreset = "architect"
	ToolPresetLarkLocal ToolPreset = "lark-local"
)

// ToolConfig contains tool access configuration for a preset
type ToolConfig struct {
	Name         string
	Description  string
	AllowedTools map[string]bool // nil means all tools allowed
	DeniedTools  map[string]bool // Tools explicitly denied
}

var (
	cliRestrictedTools = func() map[string]bool {
		return map[string]bool{
			"artifacts_write":  true,
			"artifacts_list":   true,
			"artifacts_delete": true,
			"acp_executor":     true,
			// Memory tools (Web UI only)
			"memory_write":  true,
			"memory_recall": true,
			// Media generation tools (Web UI only)
			"text_to_image":    true,
			"image_to_image":   true,
			"video_generate":   true,
			"pptx_from_images": true,
		}
	}()
	cliDeniedTools = func() map[string]bool {
		tools := cloneToolSet(cliRestrictedTools)
		addSandboxTools(tools)
		return tools
	}()
	larkLocalDeniedTools = func() map[string]bool {
		tools := cloneToolSet(cliRestrictedTools)
		tools["write_attachment"] = true
		return tools
	}()
	webDeniedTools = map[string]bool{
		"file_read":    true,
		"file_write":   true,
		"file_edit":    true,
		"list_files":   true,
		"grep":         true,
		"ripgrep":      true,
		"find":         true,
		"bash":         true,
		"code_execute": true,
		"skills":       true,
		"todo_read":    true,
		"todo_update":  true,
	}
	readOnlyDeniedTools = func() map[string]bool {
		tools := map[string]bool{
			"file_write":   true,
			"file_edit":    true,
			"bash":         true,
			"code_execute": true,
			"todo_update":  true,
		}
		addSandboxTools(tools)
		return tools
	}()
	safeDeniedTools = func() map[string]bool {
		tools := map[string]bool{
			"bash":         true,
			"code_execute": true,
		}
		addSandboxTools(tools)
		return tools
	}()
	sandboxDeniedTools = map[string]bool{
		"file_read":    true,
		"file_write":   true,
		"file_edit":    true,
		"list_files":   true,
		"grep":         true,
		"ripgrep":      true,
		"find":         true,
		"bash":         true,
		"code_execute": true,
		"skills":       true,
		"todo_read":    true,
		"todo_update":  true,
	}
	architectAllowedToolsCLI = map[string]bool{
		"plan":         true,
		"clarify":      true,
		"web_search":   true,
		"web_fetch":    true,
		"request_user": true,
	}
	sandboxToolNames = []string{
		"browser_action",
		"browser_info",
		"browser_screenshot",
		"browser_dom",
		"read_file",
		"write_file",
		"list_dir",
		"search_file",
		"replace_in_file",
		"shell_exec",
		"execute_code",
		"write_attachment",
	}
)

func addSandboxTools(dst map[string]bool) {
	for _, name := range sandboxToolNames {
		dst[name] = true
	}
}

func cloneToolSet(src map[string]bool) map[string]bool {
	dst := make(map[string]bool, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func mergeToolSets(dst map[string]bool, src map[string]bool) map[string]bool {
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

// GetToolConfig returns the tool configuration for a mode and preset.
func GetToolConfig(mode ToolMode, preset ToolPreset) (*ToolConfig, error) {
	if mode == "" {
		mode = ToolModeCLI
	}
	switch mode {
	case ToolModeWeb:
		if preset == "" {
			return &ToolConfig{
				Name:         "Web Mode",
				Description:  "All non-local tools (file/shell/code exec disabled)",
				AllowedTools: nil,
				DeniedTools:  cloneToolSet(webDeniedTools),
			}, nil
		}
		switch preset {
		case ToolPresetArchitect:
			return &ToolConfig{
				Name:         "Architect Access",
				Description:  "Web-safe tools (local file/shell/code exec disabled)",
				AllowedTools: nil,
				DeniedTools:  cloneToolSet(webDeniedTools),
			}, nil
		case ToolPresetFull, ToolPresetReadOnly, ToolPresetSafe, ToolPresetSandbox:
			return &ToolConfig{
				Name:         "Web Mode",
				Description:  "All non-local tools (file/shell/code exec disabled)",
				AllowedTools: nil,
				DeniedTools:  cloneToolSet(webDeniedTools),
			}, nil
		case ToolPresetLarkLocal:
			return &ToolConfig{
				Name:         "Lark Local",
				Description:  "Lark-local tools (local browser/file aliases, no sandbox attachments)",
				AllowedTools: nil,
				DeniedTools:  cloneToolSet(larkLocalDeniedTools),
			}, nil
		default:
			return nil, fmt.Errorf("unknown tool preset: %s", preset)
		}
	case ToolModeCLI:
		if preset == "" {
			preset = ToolPresetFull
		}
		switch preset {
		case ToolPresetFull:
			return &ToolConfig{
				Name:         "Full Access",
				Description:  "All tools available - unrestricted access",
				AllowedTools: nil,
				DeniedTools:  cloneToolSet(cliDeniedTools),
			}, nil
		case ToolPresetReadOnly:
			return &ToolConfig{
				Name:         "Read-Only Access",
				Description:  "No local writes or shell/code execution",
				AllowedTools: nil,
				DeniedTools:  mergeToolSets(cloneToolSet(readOnlyDeniedTools), cliDeniedTools),
			}, nil
		case ToolPresetSafe:
			return &ToolConfig{
				Name:         "Safe Mode",
				Description:  "Excludes potentially dangerous tools (bash, code execution)",
				AllowedTools: nil,
				DeniedTools:  mergeToolSets(cloneToolSet(safeDeniedTools), cliDeniedTools),
			}, nil
		case ToolPresetSandbox:
			return &ToolConfig{
				Name:         "Sandbox Access",
				Description:  "No local file/shell tools; sandbox_* tools are web-only",
				AllowedTools: nil,
				DeniedTools:  mergeToolSets(cloneToolSet(sandboxDeniedTools), cliDeniedTools),
			}, nil
		case ToolPresetArchitect:
			return &ToolConfig{
				Name:         "Architect Access",
				Description:  "Architect-only tools (search/plan/clarify + executor dispatch)",
				AllowedTools: cloneToolSet(architectAllowedToolsCLI),
				DeniedTools:  cloneToolSet(cliDeniedTools),
			}, nil
		case ToolPresetLarkLocal:
			return &ToolConfig{
				Name:         "Lark Local",
				Description:  "Lark-local tools (local browser/file aliases, no sandbox attachments)",
				AllowedTools: nil,
				DeniedTools:  cloneToolSet(larkLocalDeniedTools),
			}, nil
		default:
			return nil, fmt.Errorf("unknown tool preset: %s", preset)
		}
	default:
		return nil, fmt.Errorf("unknown tool mode: %s", mode)
	}
}

// FilteredToolRegistry wraps a tool registry with preset-based filtering
type FilteredToolRegistry struct {
	parent tools.ToolRegistry
	config *ToolConfig
}

// NewFilteredToolRegistry creates a filtered registry based on tool mode and preset.
func NewFilteredToolRegistry(parent tools.ToolRegistry, mode ToolMode, preset ToolPreset) (*FilteredToolRegistry, error) {
	config, err := GetToolConfig(mode, preset)
	if err != nil {
		return nil, err
	}

	return &FilteredToolRegistry{
		parent: parent,
		config: config,
	}, nil
}

// Get retrieves a tool if allowed by the preset
func (f *FilteredToolRegistry) Get(name string) (tools.ToolExecutor, error) {
	// Check if tool is denied
	if f.config.DeniedTools[name] {
		return nil, fmt.Errorf("tool not available in %s preset: %s", f.config.Name, name)
	}

	// If AllowedTools is nil, all tools are allowed (unless explicitly denied)
	if f.config.AllowedTools == nil {
		return f.parent.Get(name)
	}

	// Check if tool is in allowed list
	if !f.config.AllowedTools[name] {
		return nil, fmt.Errorf("tool not available in %s preset: %s", f.config.Name, name)
	}

	return f.parent.Get(name)
}

// List returns only tools allowed by the preset
func (f *FilteredToolRegistry) List() []ports.ToolDefinition {
	allTools := f.parent.List()
	filtered := make([]ports.ToolDefinition, 0)

	for _, tool := range allTools {
		// Skip denied tools
		if f.config.DeniedTools[tool.Name] {
			continue
		}

		// If AllowedTools is nil, include all (unless denied)
		if f.config.AllowedTools == nil {
			filtered = append(filtered, tool)
			continue
		}

		// Include only allowed tools
		if f.config.AllowedTools[tool.Name] {
			filtered = append(filtered, tool)
		}
	}

	return filtered
}

// Register delegates to parent registry
func (f *FilteredToolRegistry) Register(tool tools.ToolExecutor) error {
	return f.parent.Register(tool)
}

// Unregister delegates to parent registry
func (f *FilteredToolRegistry) Unregister(name string) error {
	return f.parent.Unregister(name)
}

// GetAllToolPresets returns all available tool presets
func GetAllToolPresets() []ToolPreset {
	return []ToolPreset{
		ToolPresetFull,
		ToolPresetReadOnly,
		ToolPresetSafe,
		ToolPresetSandbox,
		ToolPresetArchitect,
		ToolPresetLarkLocal,
	}
}

// IsValidToolPreset checks if a tool preset is valid
func IsValidToolPreset(preset string) bool {
	switch ToolPreset(preset) {
	case ToolPresetFull, ToolPresetReadOnly, ToolPresetSafe, ToolPresetSandbox, ToolPresetArchitect, ToolPresetLarkLocal:
		return true
	default:
		return false
	}
}
