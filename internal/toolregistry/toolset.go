package toolregistry

import (
	"strings"
	"time"
)

// Toolset controls which builtin tool implementations are registered.
type Toolset string

const (
	ToolsetDefault   Toolset = "default"
	ToolsetLarkLocal Toolset = "lark-local"
)

// NormalizeToolset coerces a raw string into a supported toolset.
func NormalizeToolset(value string) Toolset {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(ToolsetLarkLocal), "local":
		return ToolsetLarkLocal
	case string(ToolsetDefault), "":
		return ToolsetDefault
	default:
		return ToolsetDefault
	}
}

// BrowserConfig configures local browser tooling when sandbox is disabled.
type BrowserConfig struct {
	Connector        string
	CDPURL           string
	ChromePath       string
	Headless         bool
	UserDataDir      string
	Timeout          time.Duration
	BridgeListenAddr string
	BridgeToken      string
}
