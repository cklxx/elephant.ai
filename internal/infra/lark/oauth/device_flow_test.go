package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestResolveEndpoints(t *testing.T) {
	tests := []struct {
		name            string
		brand           string
		wantDeviceAuth  string
		wantToken       string
	}{
		{
			name:           "empty defaults to feishu",
			brand:          "",
			wantDeviceAuth: "https://accounts.feishu.cn/oauth/v1/device_authorization",
			wantToken:      "https://open.feishu.cn/open-apis/authen/v2/oauth/token",
		},
		{
			name:           "feishu explicit",
			brand:          "feishu",
			wantDeviceAuth: "https://accounts.feishu.cn/oauth/v1/device_authorization",
			wantToken:      "https://open.feishu.cn/open-apis/authen/v2/oauth/token",
		},
		{
			name:           "feishu with whitespace",
			brand:          "  Feishu  ",
			wantDeviceAuth: "https://accounts.feishu.cn/oauth/v1/device_authorization",
			wantToken:      "https://open.feishu.cn/open-apis/authen/v2/oauth/token",
		},
		{
			name:           "lark",
			brand:          "lark",
			wantDeviceAuth: "https://accounts.larksuite.com/oauth/v1/device_authorization",
			wantToken:      "https://open.larksuite.com/open-apis/authen/v2/oauth/token",
		},
		{
			name:           "custom domain with https",
			brand:          "https://custom.example.com",
			wantDeviceAuth: "https://custom.example.com/oauth/v1/device_authorization",
			wantToken:      "https://custom.example.com/open-apis/authen/v2/oauth/token",
		},
		{
			name:           "custom domain without scheme",
			brand:          "custom.example.com",
			wantDeviceAuth: "https://custom.example.com/oauth/v1/device_authorization",
			wantToken:      "https://custom.example.com/open-apis/authen/v2/oauth/token",
		},
		{
			name:           "custom domain with trailing slash",
			brand:          "https://custom.example.com/",
			wantDeviceAuth: "https://custom.example.com/oauth/v1/device_authorization",
			wantToken:      "https://custom.example.com/open-apis/authen/v2/oauth/token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAuth, gotToken := resolveEndpoints(tt.brand)
			if gotAuth != tt.wantDeviceAuth {
				t.Errorf("deviceAuthURL = %q, want %q", gotAuth, tt.wantDeviceAuth)
			}
			if gotToken != tt.wantToken {
				t.Errorf("tokenURL = %q, want %q", gotToken, tt.wantToken)
			}
		})
	}
}

func TestRequestDeviceAuthorization(t *testing.T) {
	t.Run("missing credentials", func(t *testing.T) {
		_, err := RequestDeviceAuthorization(context.Background(), DeviceFlowConfig{})
		if err == nil {
			t.Fatal("expected error for missing credentials")
		}
	})

	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", r.Method)
			}
			user, pass, ok := r.BasicAuth()
			if !ok || user != "app1" || pass != "secret1" {
				t.Fatalf("bad basic auth: user=%q pass=%q ok=%v", user, pass, ok)
			}
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}
			scope := r.FormValue("scope")
			if scope == "" {
				t.Fatal("expected scope in form")
			}
			resp := DeviceAuthResponse{
				DeviceCode:              "dev-code-123",
				UserCode:                "ABCD-1234",
				VerificationURI:         "https://example.com/verify",
				VerificationURIComplete: "https://example.com/verify?code=ABCD-1234",
				ExpiresIn:               600,
				Interval:                5,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		got, err := RequestDeviceAuthorization(context.Background(), DeviceFlowConfig{
			AppID:     "app1",
			AppSecret: "secret1",
			Brand:     server.URL,
			Scopes:    []string{"contact:user.id:readonly"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.DeviceCode != "dev-code-123" {
			t.Fatalf("device_code = %q, want %q", got.DeviceCode, "dev-code-123")
		}
		if got.UserCode != "ABCD-1234" {
			t.Fatalf("user_code = %q, want %q", got.UserCode, "ABCD-1234")
		}
	})

	t.Run("offline_access already present", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = r.ParseForm()
			scope := r.FormValue("scope")
			// Should not duplicate offline_access
			count := 0
			for _, s := range splitScope(scope) {
				if s == "offline_access" {
					count++
				}
			}
			if count != 1 {
				t.Fatalf("expected exactly 1 offline_access, got %d in %q", count, scope)
			}
			resp := DeviceAuthResponse{
				DeviceCode: "dc",
				ExpiresIn:  600,
				Interval:   5,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		_, err := RequestDeviceAuthorization(context.Background(), DeviceFlowConfig{
			AppID:     "app1",
			AppSecret: "secret1",
			Brand:     server.URL,
			Scopes:    []string{"offline_access", "user:read"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("internal error"))
		}))
		defer server.Close()

		_, err := RequestDeviceAuthorization(context.Background(), DeviceFlowConfig{
			AppID:     "app1",
			AppSecret: "secret1",
			Brand:     server.URL,
		})
		if err == nil {
			t.Fatal("expected error for server error response")
		}
	})

	t.Run("empty device code in response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := DeviceAuthResponse{DeviceCode: "", ExpiresIn: 600}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		_, err := RequestDeviceAuthorization(context.Background(), DeviceFlowConfig{
			AppID:     "app1",
			AppSecret: "secret1",
			Brand:     server.URL,
		})
		if err == nil {
			t.Fatal("expected error for empty device_code")
		}
	})

	t.Run("zero interval defaults to 5", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := DeviceAuthResponse{
				DeviceCode: "dc",
				ExpiresIn:  600,
				Interval:   0,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		got, err := RequestDeviceAuthorization(context.Background(), DeviceFlowConfig{
			AppID:     "app1",
			AppSecret: "secret1",
			Brand:     server.URL,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Interval != 5 {
			t.Fatalf("interval = %d, want 5", got.Interval)
		}
	})
}

func TestPollOnce(t *testing.T) {
	cfg := DeviceFlowConfig{AppID: "app1", AppSecret: "secret1"}

	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := DeviceTokenResponse{
				AccessToken:  "at-123",
				RefreshToken: "rt-123",
				TokenType:    "Bearer",
				ExpiresIn:    3600,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		got, err := pollOnce(context.Background(), cfg, server.URL, "dc-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.AccessToken != "at-123" {
			t.Fatalf("access_token = %q, want %q", got.AccessToken, "at-123")
		}
	})

	t.Run("authorization_pending", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "authorization_pending"})
		}))
		defer server.Close()

		_, err := pollOnce(context.Background(), cfg, server.URL, "dc-1")
		if !errors.Is(err, ErrDeviceFlowPending) {
			t.Fatalf("expected ErrDeviceFlowPending, got %v", err)
		}
	})

	t.Run("slow_down", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "slow_down"})
		}))
		defer server.Close()

		_, err := pollOnce(context.Background(), cfg, server.URL, "dc-1")
		if !errors.Is(err, ErrDeviceFlowSlowDown) {
			t.Fatalf("expected ErrDeviceFlowSlowDown, got %v", err)
		}
	})

	t.Run("access_denied", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "access_denied"})
		}))
		defer server.Close()

		_, err := pollOnce(context.Background(), cfg, server.URL, "dc-1")
		if !errors.Is(err, ErrDeviceFlowDenied) {
			t.Fatalf("expected ErrDeviceFlowDenied, got %v", err)
		}
	})

	t.Run("expired_token", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "expired_token"})
		}))
		defer server.Close()

		_, err := pollOnce(context.Background(), cfg, server.URL, "dc-1")
		if !errors.Is(err, ErrDeviceFlowExpired) {
			t.Fatalf("expected ErrDeviceFlowExpired, got %v", err)
		}
	})

	t.Run("unknown error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "server_error"})
		}))
		defer server.Close()

		_, err := pollOnce(context.Background(), cfg, server.URL, "dc-1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("non-200 without error field", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("bad gateway"))
		}))
		defer server.Close()

		_, err := pollOnce(context.Background(), cfg, server.URL, "dc-1")
		if err == nil {
			t.Fatal("expected error for non-200 status")
		}
	})

	t.Run("empty access_token", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := DeviceTokenResponse{AccessToken: ""}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		_, err := pollOnce(context.Background(), cfg, server.URL, "dc-1")
		if err == nil {
			t.Fatal("expected error for empty access_token")
		}
	})
}

func TestPollDeviceToken(t *testing.T) {
	cfg := DeviceFlowConfig{AppID: "app1", AppSecret: "secret1"}

	t.Run("empty device code", func(t *testing.T) {
		_, err := PollDeviceToken(context.Background(), cfg, "", 5, 600)
		if err == nil {
			t.Fatal("expected error for empty device_code")
		}
	})

	t.Run("context cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "authorization_pending"})
		}))
		defer server.Close()

		_, err := PollDeviceToken(ctx, cfg, "dc-1", 1, 60)
		if err == nil {
			t.Fatal("expected error for cancelled context")
		}
	})

	t.Run("success after pending", func(t *testing.T) {
		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			n := calls.Add(1)
			if n <= 2 {
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "authorization_pending"})
				return
			}
			_ = json.NewEncoder(w).Encode(DeviceTokenResponse{
				AccessToken:  "at-final",
				RefreshToken: "rt-final",
				TokenType:    "Bearer",
				ExpiresIn:    3600,
			})
		}))
		defer server.Close()

		got, err := PollDeviceToken(context.Background(), DeviceFlowConfig{
			AppID:     "app1",
			AppSecret: "secret1",
			Brand:     server.URL,
		}, "dc-1", 1, 30)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.AccessToken != "at-final" {
			t.Fatalf("access_token = %q, want %q", got.AccessToken, "at-final")
		}
	})

	t.Run("denied stops polling", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "access_denied"})
		}))
		defer server.Close()

		_, err := PollDeviceToken(context.Background(), DeviceFlowConfig{
			AppID:     "app1",
			AppSecret: "secret1",
			Brand:     server.URL,
		}, "dc-1", 1, 30)
		if !errors.Is(err, ErrDeviceFlowDenied) {
			t.Fatalf("expected ErrDeviceFlowDenied, got %v", err)
		}
	})

	t.Run("slow_down increases interval", func(t *testing.T) {
		var calls atomic.Int32
		var timestamps []time.Time
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			timestamps = append(timestamps, time.Now())
			w.Header().Set("Content-Type", "application/json")
			n := calls.Add(1)
			if n == 1 {
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "slow_down"})
				return
			}
			_ = json.NewEncoder(w).Encode(DeviceTokenResponse{
				AccessToken: "at-ok",
				ExpiresIn:   3600,
			})
		}))
		defer server.Close()

		got, err := PollDeviceToken(context.Background(), DeviceFlowConfig{
			AppID:     "app1",
			AppSecret: "secret1",
			Brand:     server.URL,
		}, "dc-1", 1, 30)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.AccessToken != "at-ok" {
			t.Fatalf("access_token = %q, want %q", got.AccessToken, "at-ok")
		}
	})

	t.Run("defaults for zero interval and expiresIn", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(DeviceTokenResponse{
				AccessToken: "at-default",
				ExpiresIn:   3600,
			})
		}))
		defer server.Close()

		got, err := PollDeviceToken(context.Background(), DeviceFlowConfig{
			AppID:     "app1",
			AppSecret: "secret1",
			Brand:     server.URL,
		}, "dc-1", 0, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.AccessToken != "at-default" {
			t.Fatalf("access_token = %q, want %q", got.AccessToken, "at-default")
		}
	})
}

// splitScope is a test helper to split space-separated scopes.
func splitScope(s string) []string {
	var result []string
	for _, part := range strings.Split(s, " ") {
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

