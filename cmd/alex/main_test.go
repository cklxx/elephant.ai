package main

import "testing"

func TestHandleStandaloneArgsConfigShow(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")

	handled, exitCode := handleStandaloneArgs([]string{"config", "show"})
	if !handled {
		t.Fatalf("expected config show to be handled")
	}
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
}

func TestHandleStandaloneArgsConfigRejectsExtraArgs(t *testing.T) {
	handled, exitCode := handleStandaloneArgs([]string{"config", "show", "extra"})
	if !handled {
		t.Fatalf("expected config command to be handled even with invalid args")
	}
	if exitCode == 0 {
		t.Fatalf("expected non-zero exit code for invalid args")
	}
}
