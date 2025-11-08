package main

import "testing"

func TestSanitizeUserID(t *testing.T) {
	cases := map[string]string{
		"Alice":            "alice",
		"  Bob Smith  ":    "bob-smith",
		"carl@example.com": "carl-example-com",
		"user/name":        "user-name",
		"":                 "",
		"--":               "",
		"MiXeD_Case":       "mixed-case",
	}

	for input, want := range cases {
		if got := sanitizeUserID(input); got != want {
			t.Fatalf("sanitizeUserID(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestResolveCLIUserIDPrefersExplicitEnv(t *testing.T) {
	t.Setenv("ALEX_USER_ID", "Primary User")
	t.Setenv("ALEX_CLI_USER_ID", "Secondary")
	t.Setenv("USER", "fallback")

	got := resolveCLIUserID()
	if got != "primary-user" {
		t.Fatalf("expected env override to be used, got %q", got)
	}
}

func TestResolveCLIUserIDFallsBackToUserEnv(t *testing.T) {
	t.Setenv("ALEX_USER_ID", "")
	t.Setenv("ALEX_CLI_USER_ID", "")
	t.Setenv("USER", "CLI Tester")
	t.Setenv("USERNAME", "")

	got := resolveCLIUserID()
	if got != "cli-tester" {
		t.Fatalf("expected USER env to provide id, got %q", got)
	}
}
