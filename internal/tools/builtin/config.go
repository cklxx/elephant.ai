package builtin

import "alex/internal/tools"

// FileToolConfig propagates execution mode settings to file-based tools.
type FileToolConfig struct {
	Mode           tools.ExecutionMode
	SandboxManager *tools.SandboxManager
}

// ShellToolConfig propagates execution mode settings to shell-based tools.
type ShellToolConfig struct {
        Mode           tools.ExecutionMode
        SandboxManager *tools.SandboxManager
}

// BrowserToolConfig propagates execution mode settings to browser automation tools.
type BrowserToolConfig struct {
        Mode           tools.ExecutionMode
        SandboxManager *tools.SandboxManager
}
