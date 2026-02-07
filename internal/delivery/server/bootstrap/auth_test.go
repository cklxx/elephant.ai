package bootstrap

import (
	"testing"

	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/logging"
)

func TestBuildAuthServiceDevelopmentFallbackSecret(t *testing.T) {
	cfg := Config{
		Port:    "8080",
		Runtime: runtimeconfig.RuntimeConfig{Environment: "development"},
	}

	svc, cleanup, err := BuildAuthService(cfg, logging.OrNop(nil))
	if err != nil {
		t.Fatalf("expected auth service with development fallback secret, got error: %v", err)
	}
	if svc == nil {
		t.Fatalf("expected non-nil auth service")
	}
	if cleanup != nil {
		cleanup()
	}
}

func TestBuildAuthServiceRequiresSecretOutsideDevelopment(t *testing.T) {
	cfg := Config{
		Port:    "8080",
		Runtime: runtimeconfig.RuntimeConfig{Environment: "production"},
	}

	svc, cleanup, err := BuildAuthService(cfg, logging.OrNop(nil))
	if cleanup != nil {
		cleanup()
	}
	if err == nil {
		t.Fatalf("expected error when auth secret is missing in production")
	}
	if svc != nil {
		t.Fatalf("expected nil auth service on missing production secret")
	}
}

func TestBuildAuthServiceFallbackWhenDatabaseUnavailableInDevelopment(t *testing.T) {
	cfg := Config{
		Port: "8080",
		Runtime: runtimeconfig.RuntimeConfig{
			Environment: "development",
		},
		Auth: runtimeconfig.AuthConfig{
			JWTSecret:   "dev-secret",
			DatabaseURL: "postgres://127.0.0.1:1/auth?sslmode=disable&connect_timeout=1",
		},
	}

	svc, cleanup, err := BuildAuthService(cfg, logging.OrNop(nil))
	if err != nil {
		t.Fatalf("expected development fallback to memory stores, got error: %v", err)
	}
	if svc == nil {
		t.Fatalf("expected non-nil auth service")
	}
	if cleanup != nil {
		cleanup()
	}
}

func TestBuildAuthServiceDatabaseUnavailableFailsInProduction(t *testing.T) {
	cfg := Config{
		Port: "8080",
		Runtime: runtimeconfig.RuntimeConfig{
			Environment: "production",
		},
		Auth: runtimeconfig.AuthConfig{
			JWTSecret:   "prod-secret",
			DatabaseURL: "postgres://127.0.0.1:1/auth?sslmode=disable&connect_timeout=1",
		},
	}

	svc, cleanup, err := BuildAuthService(cfg, logging.OrNop(nil))
	if cleanup != nil {
		cleanup()
	}
	if err == nil {
		t.Fatalf("expected error when production auth db is unavailable")
	}
	if svc != nil {
		t.Fatalf("expected nil auth service on production db failure")
	}
}
