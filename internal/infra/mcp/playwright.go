package mcp

import (
	"fmt"
	"strconv"
	"time"
)

// PlaywrightBrowserConfig holds the subset of browser settings needed to
// construct a Playwright MCP server configuration.
type PlaywrightBrowserConfig struct {
	Connector   string        // "extension" (default), "headless", "cdp"
	CDPURL      string        // CDP endpoint URL (used when Connector="cdp")
	ChromePath  string        // Custom Chrome/Chromium binary path
	Headless    bool          // Force headless even in extension mode
	UserDataDir string        // Persistent browser profile directory
	Timeout     time.Duration // Per-action timeout
	BridgeToken string        // PLAYWRIGHT_MCP_EXTENSION_TOKEN for auto-approval
}

// PlaywrightServerConfig translates a PlaywrightBrowserConfig into an MCP
// ServerConfig suitable for spawning the @playwright/mcp server via npx.
func PlaywrightServerConfig(cfg PlaywrightBrowserConfig) ServerConfig {
	args := []string{"-y", "@playwright/mcp@latest"}
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
		ms := int(cfg.Timeout / time.Millisecond)
		args = append(args, "--timeout", strconv.Itoa(ms))
	}

	return ServerConfig{
		Command: "npx",
		Args:    args,
		Env:     env,
	}
}

// PlaywrightServerName is the canonical MCP server name used when registering
// the Playwright browser server. Tools will be prefixed mcp__playwright__*.
const PlaywrightServerName = "playwright"

// WithPlaywrightBrowser returns a RegistryOption that pre-registers a
// Playwright MCP server configuration to be started during Initialize().
func WithPlaywrightBrowser(cfg PlaywrightBrowserConfig) RegistryOption {
	return func(r *Registry) {
		if r.playwrightConfig == nil {
			r.playwrightConfig = &cfg
		}
	}
}

// startPlaywrightIfConfigured starts the Playwright MCP server when a config
// was provided via WithPlaywrightBrowser. Called from Initialize().
func (r *Registry) startPlaywrightIfConfigured() error {
	if r.playwrightConfig == nil {
		return nil
	}

	serverCfg := PlaywrightServerConfig(*r.playwrightConfig)
	r.logger.Info("Starting Playwright MCP server (connector=%s)", r.playwrightConfig.Connector)

	if err := r.startServer(PlaywrightServerName, serverCfg); err != nil {
		return fmt.Errorf("playwright MCP server: %w", err)
	}

	return nil
}
