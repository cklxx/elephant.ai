package http

import (
	"net/http"
	"time"

	authapp "alex/internal/app/auth"
	"alex/internal/delivery/server/app"
	"alex/internal/infra/attachments"
	"alex/internal/infra/observability"
)

// RouterDeps holds all service dependencies needed to construct the HTTP router.
type RouterDeps struct {
	Tasks                  *app.TaskExecutionService
	Sessions               *app.SessionService
	Snapshots              *app.SnapshotService
	Broadcaster            *app.EventBroadcaster
	RunTracker             app.RunTracker
	HealthChecker          *app.HealthCheckerImpl
	AuthHandler            *AuthHandler
	AuthService            *authapp.Service
	ConfigHandler          *ConfigHandler
	OnboardingStateHandler *OnboardingStateHandler
	Evaluation             *app.EvaluationService
	Obs                    *observability.Observability
	AttachmentCfg          attachments.StoreConfig
	SandboxClient          SandboxClient
	DataCache              *DataCache
	LarkOAuthHandler       *LarkOAuthHandler
	MemoryEngine           MemoryEngine
	HooksBridge            http.Handler // optional: Claude Code hooks → Lark bridge
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
