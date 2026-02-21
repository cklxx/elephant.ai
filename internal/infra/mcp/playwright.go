package mcp

import (
	"fmt"
	"time"
)

// PlaywrightBrowserConfig holds the subset of browser configuration used to
// build a Playwright MCP ServerConfig. The fields mirror
// toolregistry.BrowserConfig so callers can translate directly.
type PlaywrightBrowserConfig struct {
	Connector   string        // "extension" (default), "headless", "cdp"
	CDPURL      string        // CDP endpoint for connector="cdp"
	ChromePath  string        // Custom Chrome/Edge binary path
	Headless    bool          // Explicit headless flag (used by "headless" connector)
	UserDataDir string        // Browser profile directory
	Timeout     time.Duration // Per-action timeout
	BridgeToken string        // PLAYWRIGHT_MCP_EXTENSION_TOKEN for auto-approval
	ExtraCaps   []string      // Additional --caps values (e.g. "vision", "pdf")
	Browser     string        // Browser channel: chrome, firefox, webkit, msedge
}

const playwrightMCPPackage = "@playwright/mcp@latest"

// PlaywrightServerName is the canonical MCP server name used when registering
// the Playwright browser server. Tools will be prefixed mcp__playwright__*.
const PlaywrightServerName = "playwright"

// PlaywrightServerConfig translates a PlaywrightBrowserConfig into an MCP
// ServerConfig that can be passed to Registry.StartServerWithConfig.
func PlaywrightServerConfig(cfg PlaywrightBrowserConfig) ServerConfig {
	args := []string{"-y", playwrightMCPPackage}
	env := map[string]string{}

	connector := cfg.Connector
	if connector == "" {
		connector = "extension"
	}

	switch connector {
	case "extension":
		args = append(args, "--extension")
		if cfg.BridgeToken != "" {
			env["PLAYWRIGHT_MCP_EXTENSION_TOKEN"] = cfg.BridgeToken
		}
	case "headless":
		args = append(args, "--headless", "--isolated")
	case "cdp":
		if cfg.CDPURL != "" {
			args = append(args, "--cdp-endpoint", cfg.CDPURL)
		}
	}

	if cfg.ChromePath != "" {
		args = append(args, "--executable-path", cfg.ChromePath)
	}
	if cfg.UserDataDir != "" {
		args = append(args, "--user-data-dir", cfg.UserDataDir)
	}
	if cfg.Timeout > 0 {
		ms := fmt.Sprintf("%d", cfg.Timeout.Milliseconds())
		args = append(args, "--timeout-action", ms)
	}
	if cfg.Browser != "" {
		args = append(args, "--browser", cfg.Browser)
	}
	for _, cap := range cfg.ExtraCaps {
		args = append(args, "--caps", cap)
	}

	return ServerConfig{
		Command: "npx",
		Args:    args,
		Env:     env,
	}
}

// WithPlaywrightBrowser returns a RegistryOption that pre-registers a
// Playwright MCP server configuration to be started during Initialize().
func WithPlaywrightBrowser(cfg PlaywrightBrowserConfig) RegistryOption {
	return func(r *Registry) {
		r.playwrightConfig = &cfg
	}
}
