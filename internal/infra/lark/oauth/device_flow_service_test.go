package oauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestService_ResolveBrand(t *testing.T) {
	tests := []struct {
		name       string
		baseDomain string
		want       string
	}{
		{"empty defaults to feishu", "", "feishu"},
		{"feishu domain", "https://open.feishu.cn", "feishu"},
		{"lark domain", "https://open.larksuite.com", "lark"},
		{"custom domain", "https://custom.example.com", "https://custom.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use feishu default for empty baseDomain since NewService sets it
			bd := tt.baseDomain
			if bd == "" {
				bd = "https://open.feishu.cn"
			}
			svc, err := NewService(ServiceConfig{
				AppID:        "app",
				AppSecret:    "secret",
				BaseDomain:   bd,
				RedirectBase: "http://localhost:8080",
			}, newMemoryTokenStore(), NewMemoryStateStore())
			if err != nil {
				t.Fatalf("NewService: %v", err)
			}

			got := svc.resolveBrand()
			if got != tt.want {
				t.Errorf("resolveBrand() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestService_StartDeviceFlow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(DeviceAuthResponse{
			DeviceCode:              "dc-svc",
			UserCode:                "UC-1234",
			VerificationURI:         "https://example.com/verify",
			VerificationURIComplete: "https://example.com/verify?code=UC-1234",
			ExpiresIn:               600,
			Interval:                5,
		})
	}))
	defer server.Close()

	svc, err := NewService(ServiceConfig{
		AppID:        "app",
		AppSecret:    "secret",
		BaseDomain:   server.URL,
		RedirectBase: "http://localhost:8080",
	}, newMemoryTokenStore(), NewMemoryStateStore())
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	result, err := svc.StartDeviceFlow(context.Background(), []string{"user:read"})
	if err != nil {
		t.Fatalf("StartDeviceFlow: %v", err)
	}
	if result.DeviceCode != "dc-svc" {
		t.Fatalf("device_code = %q, want %q", result.DeviceCode, "dc-svc")
	}
	if result.UserCode != "UC-1234" {
		t.Fatalf("user_code = %q, want %q", result.UserCode, "UC-1234")
	}
	if result.VerificationURI != "https://example.com/verify" {
		t.Fatalf("verification_uri = %q, want %q", result.VerificationURI, "https://example.com/verify")
	}
}

func TestService_PollAndStoreDeviceToken(t *testing.T) {
	tokenStore := newMemoryTokenStore()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(DeviceTokenResponse{
			AccessToken:  "at-device",
			RefreshToken: "rt-device",
			TokenType:    "Bearer",
			ExpiresIn:    3600,
			Scope:        "user:read",
		})
	}))
	defer server.Close()

	svc, err := NewService(ServiceConfig{
		AppID:        "app",
		AppSecret:    "secret",
		BaseDomain:   server.URL,
		RedirectBase: "http://localhost:8080",
	}, tokenStore, NewMemoryStateStore())
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	var callbackToken Token
	err = svc.PollAndStoreDeviceToken(
		context.Background(),
		"dc-1", 1, 30,
		"ou_device_user",
		func(token Token) { callbackToken = token },
	)
	if err != nil {
		t.Fatalf("PollAndStoreDeviceToken: %v", err)
	}

	if callbackToken.AccessToken != "at-device" {
		t.Fatalf("callback token access_token = %q, want %q", callbackToken.AccessToken, "at-device")
	}
	if callbackToken.OpenID != "ou_device_user" {
		t.Fatalf("callback token open_id = %q, want %q", callbackToken.OpenID, "ou_device_user")
	}

	stored, err := tokenStore.Get(context.Background(), "ou_device_user")
	if err != nil {
		t.Fatalf("get stored token: %v", err)
	}
	if stored.AccessToken != "at-device" {
		t.Fatalf("stored access_token = %q, want %q", stored.AccessToken, "at-device")
	}
	if stored.Scope != "user:read" {
		t.Fatalf("stored scope = %q, want %q", stored.Scope, "user:read")
	}
}

func TestService_PollAndStoreDeviceToken_NilCallback(t *testing.T) {
	tokenStore := newMemoryTokenStore()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(DeviceTokenResponse{
			AccessToken:  "at-nil-cb",
			RefreshToken: "rt-nil-cb",
			ExpiresIn:    3600,
		})
	}))
	defer server.Close()

	svc, err := NewService(ServiceConfig{
		AppID:        "app",
		AppSecret:    "secret",
		BaseDomain:   server.URL,
		RedirectBase: "http://localhost:8080",
	}, tokenStore, NewMemoryStateStore())
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	err = svc.PollAndStoreDeviceToken(
		context.Background(),
		"dc-2", 1, 30,
		"ou_nil_cb",
		nil, // nil callback should not panic
	)
	if err != nil {
		t.Fatalf("PollAndStoreDeviceToken: %v", err)
	}
}

func TestService_StartURL(t *testing.T) {
	svc, err := NewService(ServiceConfig{
		AppID:        "app",
		AppSecret:    "secret",
		BaseDomain:   "https://open.feishu.cn",
		RedirectBase: "http://localhost:8080",
	}, newMemoryTokenStore(), NewMemoryStateStore())
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	got := svc.StartURL()
	want := "http://localhost:8080/api/lark/oauth/start"
	if got != want {
		t.Fatalf("StartURL() = %q, want %q", got, want)
	}
}

func TestNeedUserAuthError(t *testing.T) {
	t.Run("with url", func(t *testing.T) {
		e := &NeedUserAuthError{AuthURL: "https://example.com/auth"}
		got := e.Error()
		if got != "lark user authorization required: https://example.com/auth" {
			t.Fatalf("Error() = %q", got)
		}
	})

	t.Run("empty url", func(t *testing.T) {
		e := &NeedUserAuthError{}
		got := e.Error()
		if got != "lark user authorization required" {
			t.Fatalf("Error() = %q", got)
		}
	})

	t.Run("nil receiver", func(t *testing.T) {
		var e *NeedUserAuthError
		got := e.Error()
		if got != "lark user authorization required" {
			t.Fatalf("Error() = %q", got)
		}
	})
}

func TestAccessValidAt(t *testing.T) {
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)

	t.Run("valid token", func(t *testing.T) {
		tok := Token{AccessToken: "at", ExpiresAt: now.Add(10 * time.Minute)}
		if !tok.AccessValidAt(now, time.Minute) {
			t.Fatal("expected valid")
		}
	})

	t.Run("expired token", func(t *testing.T) {
		tok := Token{AccessToken: "at", ExpiresAt: now.Add(-1 * time.Minute)}
		if tok.AccessValidAt(now, 0) {
			t.Fatal("expected invalid")
		}
	})

	t.Run("empty access token", func(t *testing.T) {
		tok := Token{AccessToken: "", ExpiresAt: now.Add(10 * time.Minute)}
		if tok.AccessValidAt(now, 0) {
			t.Fatal("expected invalid for empty access token")
		}
	})

	t.Run("zero expires", func(t *testing.T) {
		tok := Token{AccessToken: "at"}
		if tok.AccessValidAt(now, 0) {
			t.Fatal("expected invalid for zero expires")
		}
	})

	t.Run("negative leeway treated as zero", func(t *testing.T) {
		tok := Token{AccessToken: "at", ExpiresAt: now.Add(1 * time.Second)}
		if !tok.AccessValidAt(now, -5*time.Minute) {
			t.Fatal("expected valid with negative leeway")
		}
	})
}
