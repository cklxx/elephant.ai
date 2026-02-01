package http

import (
	"time"

	"alex/internal/attachments"
	authapp "alex/internal/auth/app"
	"alex/internal/memory"
	"alex/internal/observability"
	"alex/internal/server/app"
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
	Evaluation              *app.EvaluationService
	Obs                     *observability.Observability
	MemoryService           memory.Service
	AttachmentCfg           attachments.StoreConfig
	SandboxBaseURL          string
	SandboxMaxResponseBytes int
	DataCache               *DataCache
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
