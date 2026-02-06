package lark

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"alex/internal/app/subscription"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/logging"
)

func TestBuildModelListIncludesLlamaServerWhenAvailable(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/v1/models" {
			t.Fatalf("expected /v1/models path, got %q", got)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[{"id":"llama-3.2-local"}]}`))
	}))
	defer srv.Close()

	gw := &Gateway{
		logger: logging.OrNop(nil),
		cliCredsLoader: func() runtimeconfig.CLICredentials {
			return runtimeconfig.CLICredentials{}
		},
		llamaResolver: func(context.Context) (subscription.LlamaServerTarget, bool) {
			return subscription.LlamaServerTarget{
				BaseURL: srv.URL,
				Source:  "llama_server",
			}, true
		},
	}

	out := gw.buildModelList(context.Background(), &incomingMessage{chatID: "oc_test", senderID: "ou_test"})
	if !strings.Contains(out, "- llama_server (llama_server)") {
		t.Fatalf("expected llama_server provider in output, got:\n%s", out)
	}
	if !strings.Contains(out, "llama-3.2-local") {
		t.Fatalf("expected llama model in output, got:\n%s", out)
	}
}

func TestResolveLlamaServerTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		lookup  runtimeconfig.EnvLookup
		wantURL string
		wantSrc string
	}{
		{
			name: "base url from env",
			lookup: func(key string) (string, bool) {
				if key == "LLAMA_SERVER_BASE_URL" {
					return "http://127.0.0.1:8082", true
				}
				return "", false
			},
			wantURL: "http://127.0.0.1:8082",
			wantSrc: string(runtimeconfig.SourceEnv),
		},
		{
			name: "host from env without scheme",
			lookup: func(key string) (string, bool) {
				if key == "LLAMA_SERVER_HOST" {
					return "127.0.0.1:8082", true
				}
				return "", false
			},
			wantURL: "http://127.0.0.1:8082",
			wantSrc: string(runtimeconfig.SourceEnv),
		},
		{
			name: "host from env with scheme",
			lookup: func(key string) (string, bool) {
				if key == "LLAMA_SERVER_HOST" {
					return "https://llama.local:8082", true
				}
				return "", false
			},
			wantURL: "https://llama.local:8082",
			wantSrc: string(runtimeconfig.SourceEnv),
		},
		{
			name: "fallback source",
			lookup: func(string) (string, bool) {
				return "", false
			},
			wantURL: "",
			wantSrc: "llama_server",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := resolveLlamaServerTarget(tt.lookup)
			if !ok {
				t.Fatalf("expected resolver to return enabled")
			}
			if got.BaseURL != tt.wantURL {
				t.Fatalf("expected base url %q, got %q", tt.wantURL, got.BaseURL)
			}
			if got.Source != tt.wantSrc {
				t.Fatalf("expected source %q, got %q", tt.wantSrc, got.Source)
			}
		})
	}
}
