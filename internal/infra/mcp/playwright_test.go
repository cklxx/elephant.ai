package mcp

import (
	"testing"
	"time"
)

func TestPlaywrightServerConfig_ExtensionDefault(t *testing.T) {
	cfg := PlaywrightBrowserConfig{}
	sc := PlaywrightServerConfig(cfg)

	if sc.Command != "npx" {
		t.Errorf("Command = %q, want npx", sc.Command)
	}
	wantArgs := []string{"-y", "@playwright/mcp@latest", "--extension"}
	if len(sc.Args) != len(wantArgs) {
		t.Fatalf("Args = %v, want %v", sc.Args, wantArgs)
	}
	for i, a := range wantArgs {
		if sc.Args[i] != a {
			t.Errorf("Args[%d] = %q, want %q", i, sc.Args[i], a)
		}
	}
}

func TestPlaywrightServerConfig_ExtensionWithToken(t *testing.T) {
	cfg := PlaywrightBrowserConfig{
		BridgeToken: "test-token-123",
	}
	sc := PlaywrightServerConfig(cfg)

	if sc.Env["PLAYWRIGHT_MCP_EXTENSION_TOKEN"] != "test-token-123" {
		t.Errorf("Env token = %q, want test-token-123", sc.Env["PLAYWRIGHT_MCP_EXTENSION_TOKEN"])
	}
}

func TestPlaywrightServerConfig_Headless(t *testing.T) {
	cfg := PlaywrightBrowserConfig{Connector: "headless"}
	sc := PlaywrightServerConfig(cfg)

	found := map[string]bool{}
	for _, a := range sc.Args {
		found[a] = true
	}
	if !found["--headless"] {
		t.Error("missing --headless flag")
	}
	if !found["--isolated"] {
		t.Error("missing --isolated flag")
	}
	if found["--extension"] {
		t.Error("should not have --extension in headless mode")
	}
}

func TestPlaywrightServerConfig_CDP(t *testing.T) {
	cfg := PlaywrightBrowserConfig{
		Connector: "cdp",
		CDPURL:    "ws://localhost:9222",
	}
	sc := PlaywrightServerConfig(cfg)

	for i, a := range sc.Args {
		if a == "--cdp-endpoint" {
			if i+1 >= len(sc.Args) || sc.Args[i+1] != "ws://localhost:9222" {
				t.Errorf("--cdp-endpoint value = %q, want ws://localhost:9222", sc.Args[i+1])
			}
			return
		}
	}
	t.Error("missing --cdp-endpoint flag")
}

func TestPlaywrightServerConfig_AllOptions(t *testing.T) {
	cfg := PlaywrightBrowserConfig{
		Connector:   "extension",
		ChromePath:  "/usr/bin/chromium",
		UserDataDir: "/tmp/browser-profile",
		Timeout:     30 * time.Second,
		BridgeToken: "tok",
	}
	sc := PlaywrightServerConfig(cfg)

	argMap := map[string]string{}
	for i := 0; i < len(sc.Args)-1; i++ {
		argMap[sc.Args[i]] = sc.Args[i+1]
	}

	if argMap["--executable-path"] != "/usr/bin/chromium" {
		t.Errorf("--executable-path = %q, want /usr/bin/chromium", argMap["--executable-path"])
	}
	if argMap["--user-data-dir"] != "/tmp/browser-profile" {
		t.Errorf("--user-data-dir = %q, want /tmp/browser-profile", argMap["--user-data-dir"])
	}
	if argMap["--timeout"] != "30000" {
		t.Errorf("--timeout = %q, want 30000", argMap["--timeout"])
	}
	if sc.Env["PLAYWRIGHT_MCP_EXTENSION_TOKEN"] != "tok" {
		t.Errorf("env token = %q, want tok", sc.Env["PLAYWRIGHT_MCP_EXTENSION_TOKEN"])
	}
}
