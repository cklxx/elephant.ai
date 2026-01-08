package builtin

import "alex/internal/sandbox"

type SandboxConfig struct {
	BaseURL string
}

func newSandboxClient(cfg SandboxConfig) *sandbox.Client {
	return sandbox.NewClient(sandbox.Config{BaseURL: cfg.BaseURL})
}
