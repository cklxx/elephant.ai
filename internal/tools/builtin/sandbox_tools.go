package builtin

import (
	tools "alex/internal/agent/ports/tools"
	materialports "alex/internal/materials/ports"
	"alex/internal/sandbox"
)

type SandboxConfig struct {
	BaseURL            string
	VisionTool         tools.ToolExecutor
	VisionPrompt       string
	AttachmentUploader materialports.Migrator
}

func newSandboxClient(cfg SandboxConfig) *sandbox.Client {
	return sandbox.NewClient(sandbox.Config{BaseURL: cfg.BaseURL})
}
