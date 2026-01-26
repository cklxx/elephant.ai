package main

import "testing"

func TestShouldUseFullscreenTUIDefaultsToFullscreen(t *testing.T) {
	// Clear all relevant env vars
	t.Setenv("ALEX_TUI_MODE", "")
	t.Setenv("ALEX_TUI_FULLSCREEN", "")

	if !shouldUseFullscreenTUI() {
		t.Fatalf("expected default to be fullscreen")
	}
}

func TestShouldUseFullscreenTUIExplicitFullscreen(t *testing.T) {
	t.Setenv("ALEX_TUI_MODE", "fullscreen")

	if !shouldUseFullscreenTUI() {
		t.Fatalf("expected explicit fullscreen mode")
	}
}

func TestShouldUseFullscreenTUIExplicitLineMode(t *testing.T) {
	t.Setenv("ALEX_TUI_MODE", "line")

	if shouldUseFullscreenTUI() {
		t.Fatalf("expected explicit line mode to disable fullscreen")
	}
}

func TestShouldUseFullscreenTUIExplicitTerminalMode(t *testing.T) {
	t.Setenv("ALEX_TUI_MODE", "terminal")

	if shouldUseFullscreenTUI() {
		t.Fatalf("expected terminal mode to disable fullscreen")
	}
}

func TestShouldUseFullscreenTUIFullscreenEnvOff(t *testing.T) {
	t.Setenv("ALEX_TUI_FULLSCREEN", "0")

	if shouldUseFullscreenTUI() {
		t.Fatalf("expected ALEX_TUI_FULLSCREEN=0 to disable fullscreen")
	}
}

func TestShouldUseFullscreenTUIFullscreenEnvOn(t *testing.T) {
	t.Setenv("ALEX_TUI_FULLSCREEN", "1")

	if !shouldUseFullscreenTUI() {
		t.Fatalf("expected ALEX_TUI_FULLSCREEN=1 to enable fullscreen")
	}
}

func TestShouldUseFullscreenTUICJKLocaleStillFullscreen(t *testing.T) {
	// CJK locale now defaults to fullscreen (IME input is supported)
	t.Setenv("LANG", "zh_CN.UTF-8")
	t.Setenv("ALEX_TUI_MODE", "")
	t.Setenv("ALEX_TUI_FULLSCREEN", "")

	if !shouldUseFullscreenTUI() {
		t.Fatalf("expected CJK locale to use fullscreen (IME is supported)")
	}
}

func TestShouldUseFullscreenTUICJKLocaleCanOptOutToLine(t *testing.T) {
	t.Setenv("LANG", "zh_CN.UTF-8")
	t.Setenv("ALEX_TUI_MODE", "line")

	if shouldUseFullscreenTUI() {
		t.Fatalf("expected explicit line mode to override default")
	}
}
