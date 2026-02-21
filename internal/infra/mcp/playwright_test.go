package mcp

import (
	"testing"
	"time"
)

func TestPlaywrightServerConfig_ExtensionDefault(t *testing.T) {
	cfg := PlaywrightBrowserConfig{} // empty → defaults to extension
	sc := PlaywrightServerConfig(cfg)

	if sc.Command != "npx" {
		t.Errorf("Command = %q, want npx", sc.Command)
	}
	assertContains(t, sc.Args, "--extension")
	assertNotContains(t, sc.Args, "--headless")
}

func TestPlaywrightServerConfig_ExtensionWithToken(t *testing.T) {
	cfg := PlaywrightBrowserConfig{
		Connector:   "extension",
		BridgeToken: "tok-123",
	}
	sc := PlaywrightServerConfig(cfg)

	assertContains(t, sc.Args, "--extension")
	if sc.Env["PLAYWRIGHT_MCP_EXTENSION_TOKEN"] != "tok-123" {
		t.Errorf("token env = %q, want tok-123", sc.Env["PLAYWRIGHT_MCP_EXTENSION_TOKEN"])
	}
}

func TestPlaywrightServerConfig_Headless(t *testing.T) {
	cfg := PlaywrightBrowserConfig{Connector: "headless"}
	sc := PlaywrightServerConfig(cfg)

	assertContains(t, sc.Args, "--headless")
	assertContains(t, sc.Args, "--isolated")
	assertNotContains(t, sc.Args, "--extension")
}

func TestPlaywrightServerConfig_CDP(t *testing.T) {
	cfg := PlaywrightBrowserConfig{
		Connector: "cdp",
		CDPURL:    "ws://127.0.0.1:9222",
	}
	sc := PlaywrightServerConfig(cfg)

	assertContains(t, sc.Args, "--cdp-endpoint")
	assertContains(t, sc.Args, "ws://127.0.0.1:9222")
	assertNotContains(t, sc.Args, "--extension")
}

func TestPlaywrightServerConfig_ExtraOptions(t *testing.T) {
	cfg := PlaywrightBrowserConfig{
		Connector:   "headless",
		ChromePath:  "/usr/bin/chromium",
		UserDataDir: "/tmp/profile",
		Timeout:     30 * time.Second,
		Browser:     "chrome",
		ExtraCaps:   []string{"vision", "pdf"},
	}
	sc := PlaywrightServerConfig(cfg)

	assertContains(t, sc.Args, "--executable-path")
	assertContains(t, sc.Args, "/usr/bin/chromium")
	assertContains(t, sc.Args, "--user-data-dir")
	assertContains(t, sc.Args, "/tmp/profile")
	assertContains(t, sc.Args, "--timeout-action")
	assertContains(t, sc.Args, "30000")
	assertContains(t, sc.Args, "--browser")
	assertContains(t, sc.Args, "chrome")
	// Two --caps flags
	capsCount := 0
	for _, a := range sc.Args {
		if a == "--caps" {
			capsCount++
		}
	}
	if capsCount != 2 {
		t.Errorf("expected 2 --caps flags, got %d", capsCount)
	}
}

func assertContains(t *testing.T, args []string, want string) {
	t.Helper()
	for _, a := range args {
		if a == want {
			return
		}
	}
	t.Errorf("args %v does not contain %q", args, want)
}

func assertNotContains(t *testing.T, args []string, unwant string) {
	t.Helper()
	for _, a := range args {
		if a == unwant {
			t.Errorf("args %v should not contain %q", args, unwant)
			return
		}
	}
}
