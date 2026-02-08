package preparation

import "testing"

func TestSafeKeyPrefix(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{"", "***"},
		{"short", "***"},
		{"12345678", "***"},
		{"sk-kimi-abcdef123456", "sk-kimi-..."},
		{"sk-ant-api03-xxxx", "sk-ant-a..."},
		{"sess-codex-xxxx", "sess-cod..."},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := safeKeyPrefix(tt.key)
			if got != tt.want {
				t.Errorf("safeKeyPrefix(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestDetectKeyProviderMismatch(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		apiKey   string
		wantMM   bool
	}{
		{
			name:     "kimi key sent to codex",
			provider: "codex",
			apiKey:   "sk-kimi-abcdef123456",
			wantMM:   true,
		},
		{
			name:     "anthropic key sent to openai",
			provider: "openai",
			apiKey:   "sk-ant-api03-xxxxxxxxxxxx",
			wantMM:   true,
		},
		{
			name:     "deepseek key sent to openai-responses",
			provider: "openai-responses",
			apiKey:   "sk-deepseek-xxxxxxxxxxxx",
			wantMM:   true,
		},
		{
			name:     "valid openai key with codex provider",
			provider: "codex",
			apiKey:   "sk-proj-xxxxxxxx",
			wantMM:   false,
		},
		{
			name:     "valid session key with codex provider",
			provider: "codex",
			apiKey:   "sess-codex-xxxxxxxx",
			wantMM:   false,
		},
		{
			name:     "valid anthropic key with anthropic provider",
			provider: "anthropic",
			apiKey:   "sk-ant-api03-xxxxxxxxxxxx",
			wantMM:   false,
		},
		{
			name:     "non-anthropic key with anthropic provider",
			provider: "anthropic",
			apiKey:   "sk-proj-xxxxxxxxxxxx",
			wantMM:   true,
		},
		{
			name:     "empty key returns no mismatch",
			provider: "codex",
			apiKey:   "",
			wantMM:   false,
		},
		{
			name:     "kimi provider with kimi key is fine",
			provider: "kimi",
			apiKey:   "sk-kimi-abcdef123456",
			wantMM:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMM, detail := detectKeyProviderMismatch(tt.provider, tt.apiKey)
			if gotMM != tt.wantMM {
				t.Errorf("detectKeyProviderMismatch(%q, %q) mismatch = %v, want %v (detail: %s)",
					tt.provider, tt.apiKey, gotMM, tt.wantMM, detail)
			}
		})
	}
}
