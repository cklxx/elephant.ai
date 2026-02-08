package http

import (
	"time"

	authapp "alex/internal/app/auth"
	"alex/internal/delivery/server/app"
	"alex/internal/infra/attachments"
	"alex/internal/infra/memory"
	"alex/internal/infra/observability"
)

// RouterDeps holds all service dependencies needed to construct the HTTP router.
type RouterDeps struct {
	Coordinator             *app.ServerCoordinator
	Broadcaster             *app.EventBroadcaster
	RunTracker              app.RunTracker
	HealthChecker           *app.HealthCheckerImpl
	AuthHandler             *AuthHandler
	AuthService             *authapp.Service
	ConfigHandler           *ConfigHandler
	OnboardingStateHandler  *OnboardingStateHandler
	Evaluation              *app.EvaluationService
	Obs                     *observability.Observability
	AttachmentCfg           attachments.StoreConfig
	SandboxBaseURL          string
	SandboxMaxResponseBytes int
	DataCache        *DataCache
	LarkOAuthHandler *LarkOAuthHandler
	MemoryEngine            memory.Engine
}

// RouterConfig holds configuration values for the HTTP router.
type RouterConfig struct {
	Environment      string
	AllowedOrigins   []string
	MaxTaskBodyBytes int64
	StreamGuard      StreamGuardConfig
	RateLimit        RateLimitConfig
	NonStreamTimeout time.Duration
}
