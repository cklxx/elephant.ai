package config

import (
	"errors"
	"testing"
)

func TestResolveLLMProfile(t *testing.T) {
	tests := []struct {
		name      string
		cfg       RuntimeConfig
		wantErr   bool
		errSubstr string
	}{
		{
			name: "valid codex profile",
			cfg: RuntimeConfig{
				LLMProvider: "codex",
				LLMModel:    "gpt-5-codex",
				APIKey:      "sk-proj-abc",
				BaseURL:     codexCLIBaseURL,
			},
		},
		{
			name: "codex with kimi key and non-kimi endpoint fails",
			cfg: RuntimeConfig{
				LLMProvider: "codex",
				LLMModel:    "gpt-5-codex",
				APIKey:      "sk-kimi-abc",
				BaseURL:     codexCLIBaseURL,
			},
			wantErr:   true,
			errSubstr: "moonshot",
		},
		{
			name: "codex with kimi key and kimi endpoint passes",
			cfg: RuntimeConfig{
				LLMProvider: "codex",
				LLMModel:    "gpt-5-codex",
				APIKey:      "sk-kimi-abc",
				BaseURL:     "https://api.moonshot.cn/v1",
			},
		},
		{
			name: "anthropic with non-anthropic key fails",
			cfg: RuntimeConfig{
				LLMProvider: "anthropic",
				LLMModel:    "claude-3-5-sonnet",
				APIKey:      "sk-proj-abc",
				BaseURL:     "https://api.anthropic.com/v1",
			},
			wantErr:   true,
			errSubstr: "incompatible",
		},
		{
			name: "openai with kimi key allowed for compatible endpoints",
			cfg: RuntimeConfig{
				LLMProvider: "openai",
				LLMModel:    "kimi-k2",
				APIKey:      "sk-kimi-abc",
				BaseURL:     "https://api.moonshot.cn/v1",
			},
		},
		{
			name: "openai with anthropic key fails",
			cfg: RuntimeConfig{
				LLMProvider: "openai",
				LLMModel:    "gpt-4o-mini",
				APIKey:      "sk-ant-abc",
				BaseURL:     "https://api.openai.com/v1",
			},
			wantErr:   true,
			errSubstr: "incompatible",
		},
		{
			name: "anthropic with openai endpoint fails",
			cfg: RuntimeConfig{
				LLMProvider: "anthropic",
				LLMModel:    "claude-3-5-sonnet",
				APIKey:      "sk-ant-abc",
				BaseURL:     "https://api.openai.com/v1",
			},
			wantErr:   true,
			errSubstr: "OpenAI",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile, err := ResolveLLMProfile(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil profile=%+v", profile)
				}
				var mismatchErr *LLMProfileMismatchError
				if !errors.As(err, &mismatchErr) {
					t.Fatalf("expected LLMProfileMismatchError, got %T (%v)", err, err)
				}
				if tt.errSubstr != "" && !containsInsensitive(err.Error(), tt.errSubstr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveLLMProfile() error = %v", err)
			}
			if profile.Provider != tt.cfg.LLMProvider {
				t.Fatalf("provider mismatch: got %q want %q", profile.Provider, tt.cfg.LLMProvider)
			}
			if profile.Model != tt.cfg.LLMModel {
				t.Fatalf("model mismatch: got %q want %q", profile.Model, tt.cfg.LLMModel)
			}
		})
	}
}

func TestValidateLLMProfileAcceptsEmptyValues(t *testing.T) {
	if err := ValidateLLMProfile(LLMProfile{}); err != nil {
		t.Fatalf("ValidateLLMProfile empty profile error = %v", err)
	}
}

func containsInsensitive(s, needle string) bool {
	return len(needle) == 0 || (len(s) >= len(needle) && containsFold(s, needle))
}

func containsFold(s, substr string) bool {
	// Keep this local helper tiny to avoid importing strings in test for one call.
	if len(substr) == 0 {
		return true
	}
	for i := 0; i+len(substr) <= len(s); i++ {
		if equalFoldASCII(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		aa, bb := a[i], b[i]
		if aa >= 'A' && aa <= 'Z' {
			aa += 'a' - 'A'
		}
		if bb >= 'A' && bb <= 'Z' {
			bb += 'a' - 'A'
		}
		if aa != bb {
			return false
		}
	}
	return true
}
