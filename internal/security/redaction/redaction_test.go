package redaction

import "testing"

func TestIsSensitiveKeyAllowsUsageFields(t *testing.T) {
	keys := []string{
		"tokens",
		"token_count",
		"tokens_used",
		"total_tokens",
		"input_tokens",
		"output_tokens",
		"prompt_tokens",
		"completion_tokens",
		"max_tokens",
		"remaining_tokens",
	}

	for _, key := range keys {
		if IsSensitiveKey(key) {
			t.Fatalf("expected %q to be treated as non-sensitive", key)
		}
		if got := RedactStringValue(key, "123"); got != "123" {
			t.Fatalf("expected %q to pass through for key %q, got %q", "123", key, got)
		}
	}

	// Even for usage keys, secret-looking values should still be redacted.
	if got := RedactStringValue("total_tokens", "sk-secret-value"); got != Placeholder {
		t.Fatalf("expected secret-looking value to be redacted, got %q", got)
	}
}

func TestIsSensitiveKeyFlagsSecrets(t *testing.T) {
	cases := []struct {
		key   string
		value string
	}{
		{"token", "short"},
		{"access_token", "value"},
		{"refresh_token", "value"},
		{"id_token", "value"},
		{"auth_token", "value"},
		{"session_token", "value"},
		{"api_key", "value"},
		{"apikey", "value"},
		{"private_key", "value"},
		{"ssh_key", "value"},
	}

	for _, tc := range cases {
		if !IsSensitiveKey(tc.key) {
			t.Fatalf("expected %q to be treated as sensitive", tc.key)
		}
		if got := RedactStringValue(tc.key, tc.value); got != Placeholder {
			t.Fatalf("expected key %q to be redacted, got %q", tc.key, got)
		}
	}
}
