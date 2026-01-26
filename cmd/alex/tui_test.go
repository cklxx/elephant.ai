package main

import "testing"

func TestShouldUseFullscreenTUIHonorsIMEEnv(t *testing.T) {
	t.Setenv("ALEX_TUI_MODE", "fullscreen")
	t.Setenv("ALEX_TUI_IME", "1")

	if shouldUseFullscreenTUI() {
		t.Fatalf("expected IME flag to force line input")
	}
}

func TestShouldUseFullscreenTUIHonorsInputMode(t *testing.T) {
	t.Setenv("ALEX_TUI_MODE", "fullscreen")
	t.Setenv("ALEX_TUI_INPUT", "ime")

	if shouldUseFullscreenTUI() {
		t.Fatalf("expected ALEX_TUI_INPUT=ime to force line input")
	}
}

func TestShouldUseFullscreenTUIAllowsExplicitRaw(t *testing.T) {
	t.Setenv("ALEX_TUI_MODE", "fullscreen")
	t.Setenv("ALEX_TUI_INPUT", "raw")

	if !shouldUseFullscreenTUI() {
		t.Fatalf("expected ALEX_TUI_INPUT=raw to allow fullscreen")
	}
}

func TestShouldUseFullscreenTUIForcesLineInputForCJKLocale(t *testing.T) {
	t.Setenv("ALEX_TUI_MODE", "fullscreen")
	t.Setenv("LANG", "zh_CN.UTF-8")

	if shouldUseFullscreenTUI() {
		t.Fatalf("expected CJK locale to force line input")
	}
}

func TestShouldUseFullscreenTUIAllowsOverrideForCJKLocale(t *testing.T) {
	t.Setenv("ALEX_TUI_MODE", "fullscreen")
	t.Setenv("LANG", "zh_CN.UTF-8")
	t.Setenv("ALEX_TUI_INPUT", "raw")

	if !shouldUseFullscreenTUI() {
		t.Fatalf("expected explicit raw mode to override CJK locale")
	}
}
