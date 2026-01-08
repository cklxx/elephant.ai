package builtin

import (
	"alex/internal/agent/ports"
	"alex/internal/sandbox"
)

type SandboxConfig struct {
	BaseURL      string
	VisionTool   ports.ToolExecutor
	VisionPrompt string
}

func newSandboxClient(cfg SandboxConfig) *sandbox.Client {
	return sandbox.NewClient(sandbox.Config{BaseURL: cfg.BaseURL})
}
