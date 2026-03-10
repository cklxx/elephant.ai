package di

import (
	"context"
	"fmt"
	"strings"
	"time"

	agentcoordinator "alex/internal/app/agent/coordinator"
	"alex/internal/app/lifecycle"
	"alex/internal/app/toolregistry"
	lark "alex/internal/delivery/channels/lark"
	portsllm "alex/internal/domain/agent/ports/llm"
	agentstorage "alex/internal/domain/agent/ports/storage"
	react "alex/internal/domain/agent/react"
	signalports "alex/internal/domain/signal/ports"
	taskdomain "alex/internal/domain/task"
	"alex/internal/infra/filestore"
	larkoauth "alex/internal/infra/lark/oauth"
	"alex/internal/infra/llm"
	"alex/internal/infra/memory"
	sessionstate "alex/internal/infra/session/state_store"
	toolspolicy "alex/internal/infra/tools"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/logging"
)

// LarkGateway is the minimal gateway surface needed outside delivery/channel.
type LarkGateway interface {
	NoticeLoader() func() (string, bool, error)
	SendNotification(ctx context.Context, chatID, text string) error
	InjectMessageSync(ctx context.Context, req lark.InjectSyncRequest) *lark.InjectSyncResponse
}

// Container holds all application dependencies
type Container struct {
	AgentCoordinator *agentcoordinator.AgentCoordinator
	SessionStore     agentstorage.SessionStore
	StateStore       sessionstate.Store
	HistoryStore     sessionstate.Store
	HistoryManager   agentstorage.HistoryManager
	CostTracker      agentstorage.CostTracker
	MemoryEngine     memory.Engine
	CheckpointStore  react.CheckpointStore
	TaskStore        taskdomain.Store // Unified durable task store (nil when unavailable)
	LarkGateway      LarkGateway
	LarkOAuth         *larkoauth.Service
	GitSignalProvider signalports.GitSignalProvider

	// Drainables holds subsystems that support graceful drain.
	Drainables []lifecycle.Drainable

	// Lazy initialization state
	config       Config
	toolRegistry *toolregistry.Registry
	llmFactory   *llm.Factory
	bgCancel     context.CancelFunc // cancels background goroutines (e.g. memory cleanup)
}

// Config holds the dependency injection configuration
type Config struct {
	// LLM Configuration
	LLMProvider                string
	LLMModel                   string
	LLMVisionModel             string
	APIKey                     string
	ArkAPIKey                  string
	BaseURL                    string
	ACPExecutorAddr            string
	ACPExecutorCWD             string
	ACPExecutorMode            string
	ACPExecutorAutoApprove     bool
	ACPExecutorMaxCLICalls     int
	ACPExecutorMaxDuration     int
	ACPExecutorRequireManifest bool
	TavilyAPIKey               string
	MoltbookAPIKey             string
	MoltbookBaseURL            string
	SeedreamTextEndpointID     string
	SeedreamImageEndpointID    string
	SeedreamTextModel          string
	SeedreamImageModel         string
	SeedreamVisionModel        string
	SeedreamVideoModel         string
	MaxTokens                  int
	MaxIterations              int
	ToolMaxConcurrent          int
	LLMCacheSize               int
	LLMCacheTTL                time.Duration
	UserRateLimitRPS           float64
	UserRateLimitBurst         int
	KimiRateLimitRPS           float64
	KimiRateLimitBurst         int
	Temperature                float64
	TemperatureProvided        bool
	TopP                       float64
	StopSequences              []string
	AgentPreset                string
	ToolPreset                 string
	ToolMode                   string
	Toolset                    toolregistry.Toolset
	Profile                    string
	Environment                string
	Verbose                    bool
	DisableTUI                 bool
	FollowTranscript           bool
	FollowStream               bool

	EnvironmentSummary         string
	EnvironmentSummaryProvider func() string // lazy; overrides EnvironmentSummary when set

	// Storage Configuration
	SessionDir        string // Directory for session storage (default: ~/.alex/sessions)
	CostDir           string // Directory for cost tracking (default: ~/.alex/costs)
	MemoryDir         string // Directory for file-based memory storage (default: ~/.alex/memory)
	SessionStaleAfter time.Duration
	ToolPolicy        toolspolicy.ToolPolicyConfig
	BrowserConfig     toolregistry.BrowserConfig

	HTTPLimits       runtimeconfig.HTTPLimitsConfig
	Proactive        runtimeconfig.ProactiveConfig
	ExternalAgents   runtimeconfig.ExternalAgentsConfig
	LLMFallbackRules []runtimeconfig.LLMFallbackRuleConfig
}

// Start initializes container lifecycle hooks.
func (c *Container) Start() error {
	logger := logging.NewComponentLogger("DI")
	logger.Info("Starting container lifecycle...")
	logger.Info("Container lifecycle started")
	return nil
}

// drainTimeout is the per-subsystem timeout for graceful drain.
const drainTimeout = 5 * time.Second

// Drain gracefully drains all registered Drainable subsystems (with a
// per-subsystem timeout), then performs the hard shutdown of remaining
// resources. If the parent context expires, drain still attempts Shutdown.
func (c *Container) Drain(ctx context.Context) error {
	logger := logging.NewComponentLogger("DI")

	if len(c.Drainables) > 0 {
		logger.Info("Draining %d subsystem(s)...", len(c.Drainables))
		errs := lifecycle.DrainAll(ctx, drainTimeout, c.Drainables...)
		for _, err := range errs {
			logger.Warn("Drain error: %v", err)
		}
	}

	return c.Shutdown()
}

// Shutdown gracefully shuts down all resources
func (c *Container) Shutdown() error {
	logger := logging.NewComponentLogger("DI")
	logger.Info("Shutting down container...")

	if c.bgCancel != nil {
		c.bgCancel()
	}

	if c.AgentCoordinator != nil {
		if err := c.AgentCoordinator.Close(); err != nil {
			logger.Error("Failed to close agent coordinator: %v", err)
			return err
		}
	}

	if c.toolRegistry != nil {
		c.toolRegistry.Close()
	}

	logger.Info("Container shutdown complete")
	return nil
}

// resolveStorageDir resolves a storage directory path, handling ~ expansion and environment variables.
// Delegates to filestore.ResolvePath.
func resolveStorageDir(configured, defaultPath string) string {
	return filestore.ResolvePath(configured, defaultPath)
}

// HasLLMFactory reports whether the container holds an initialised LLM factory.
func (c *Container) HasLLMFactory() bool {
	return c.llmFactory != nil
}

// LLMFactory returns the LLM client factory as a port interface.
// Returns nil when the factory has not been initialised.
func (c *Container) LLMFactory() portsllm.LLMClientFactory {
	if c.llmFactory == nil {
		return nil
	}
	return c.llmFactory
}

// GetModelHealth returns per-model health snapshots from the LLM factory.
// Returns nil if the factory is not initialized or has no health data.
func (c *Container) GetModelHealth() []llm.ProviderHealth {
	if c.llmFactory == nil {
		return nil
	}
	return c.llmFactory.GetModelHealth()
}

// AggregateModelHealth returns (healthy, message) summarising all tracked models.
// Satisfies delivery/server/ports.ModelHealthProvider structurally.
func (c *Container) AggregateModelHealth() (bool, string) {
	healths := c.GetModelHealth()
	if len(healths) == 0 {
		return true, "No models tracked yet"
	}
	var totalScore float64
	var downCount, degradedCount int
	for _, h := range healths {
		totalScore += h.HealthScore
		switch string(h.State) {
		case "down":
			downCount++
		case "degraded":
			degradedCount++
		}
	}
	avgScore := totalScore / float64(len(healths))
	healthy := downCount == 0 && degradedCount == 0
	msg := fmt.Sprintf("%d models tracked, avg health score %.0f", len(healths), avgScore)
	return healthy, msg
}

// SanitizedModelHealth returns per-model data safe for external exposure.
// Satisfies delivery/server/ports.ModelHealthProvider structurally.
func (c *Container) SanitizedModelHealth() interface{} {
	healths := c.GetModelHealth()
	if len(healths) == 0 {
		return nil
	}
	return llm.SanitizeAll(healths)
}

// DefaultLLMProfile returns the shared runtime LLM profile.
func (c *Container) DefaultLLMProfile() runtimeconfig.LLMProfile {
	return runtimeconfig.LLMProfile{
		Provider: strings.TrimSpace(c.config.LLMProvider),
		Model:    strings.TrimSpace(c.config.LLMModel),
		APIKey:   strings.TrimSpace(c.config.APIKey),
		BaseURL:  strings.TrimSpace(c.config.BaseURL),
	}
}

// SessionDir returns the resolved session directory backing file-based stores.
func (c *Container) SessionDir() string {
	return c.config.SessionDir
}

// AlternateCoordinator holds a secondary AgentCoordinator and ToolRegistry
// that share the parent Container's heavy resources (LLM Factory, Session
// Store, Memory Engine, Cost Tracker, DB Pool). Only the ToolRegistry and
// AgentCoordinator are independently owned and shut down separately.
type AlternateCoordinator struct {
	AgentCoordinator *agentcoordinator.AgentCoordinator
	toolRegistry     *toolregistry.Registry
}

// Shutdown releases only the resources owned by this alternate coordinator
// (tool registry and coordinator). It does NOT close shared resources.
func (a *AlternateCoordinator) Shutdown() error {
	if a.AgentCoordinator != nil {
		if err := a.AgentCoordinator.Close(); err != nil {
			return err
		}
	}
	if a.toolRegistry != nil {
		a.toolRegistry.Close()
	}
	return nil
}

// BuildAlternateCoordinator creates a lightweight secondary AgentCoordinator
// that shares the container's LLM Factory, Session Store, Memory Engine,
// Cost Tracker, and other heavy resources, but uses a fresh ToolRegistry
// configured with the given toolMode, toolset, and browser config.
//
// This avoids the cost of duplicating an entire DI Container when only
// the tool configuration differs (e.g. Lark gateway needing CLI-mode tools).
func (c *Container) BuildAlternateCoordinator(
	toolMode string,
	toolset toolregistry.Toolset,
	browserCfg toolregistry.BrowserConfig,
) (*AlternateCoordinator, error) {
	// Override only the tool-related fields in a copy of the config.
	altConfig := c.config
	altConfig.ToolMode = toolMode
	altConfig.Toolset = toolset
	altConfig.BrowserConfig = browserCfg

	builder := newContainerBuilder(altConfig)
	return builder.buildAlternateFrom(c)
}
