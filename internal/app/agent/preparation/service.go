package preparation

import (
	"context"
	"time"

	appconfig "alex/internal/app/agent/config"
	"alex/internal/app/agent/cost"
	"alex/internal/app/agent/llmclient"
	domain "alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
	llm "alex/internal/domain/agent/ports/llm"
	storage "alex/internal/domain/agent/ports/storage"
	tools "alex/internal/domain/agent/ports/tools"
	toolspolicy "alex/internal/infra/tools"
	"alex/internal/shared/async"
	utils "alex/internal/shared/utils"
	id "alex/internal/shared/utils/id"
)

const (
	historyComposeSnippetLimit = 600
	historySummaryMaxTokens    = 320
	historySummaryLLMTimeout   = 4 * time.Second
	historySummaryIntent       = "user_history_summary"
	DefaultSystemPrompt        = `You are eli, a helpful AI coding assistant. Use plan() to set a visible goal header (optional).

Output quality (priority: Clear > Coherent > Concise > Concrete): Lead with result first, key evidence second, supporting detail only on demand. Avoid emojis in responses unless the user explicitly requests them.

Execution: Always execute first and exhaust safe deterministic attempts before asking questions. If intent is unclear, inspect memory and thread context first (memory_search, then memory_get/memory_related, then local chat context snapshots when available). For explicit low-risk read-only inspection asks (view/check/list/inspect project state, files, branch, workspace), execute directly with read/list/shell tools and report findings; do not ask for reconfirmation. Use clarify(needs_user_input=true) only when requirements are missing/contradictory after all viable attempts fail; ask one minimal blocking question only then, and do not use clarify for explicit operational asks. For explicit approval/consent/manual gates (login, 2FA, CAPTCHA, external confirmation), call request_user with clear steps and wait. Treat explicit user delegation signals ("you decide", "anything works", "use your judgment") as authorization for low-risk reversible actions: choose a sensible default, execute, and report instead of asking again.

Tools: Use web_search when no URL is fixed and source discovery is needed; use web_fetch after a URL is chosen. Avoid assuming interactive browser automation capabilities unless matching browser tools are explicitly present in the runtime tool list. When capability is missing, proactively search/install suitable skills or tools from trusted sources before escalating. For Lark/Feishu operations, run local skill CLIs via shell_exec (for example python3 skills/feishu-cli/run.py); do not assume a dedicated channel tool exists. Use /tmp as the default location for temporary/generated files unless the user specifies another path. Use artifacts_list for inventory and artifacts_write for creating/updating durable outputs. Use write_attachment only to materialize an existing attachment into a downloadable file path.`
)

// CredentialRefresher resolves fresh API credentials for a given LLM provider.
// Returns the api key, base URL, and whether resolution succeeded.
// Used by long-running servers (e.g. Lark) to re-resolve CLI credentials
// that may have been refreshed since startup.
type CredentialRefresher func(provider string) (apiKey, baseURL string, ok bool)

// ExecutionPreparationDeps enumerates the dependencies required by the preparation service.
type ExecutionPreparationDeps struct {
	LLMFactory          llm.LLMClientFactory
	ToolRegistry        tools.ToolRegistry
	SessionStore        storage.SessionStore
	ContextMgr          agent.ContextManager
	HistoryMgr          storage.HistoryManager
	Parser              agent.FunctionCallParser
	Config              appconfig.Config
	Logger              agent.Logger
	Clock               agent.Clock
	CostDecorator       *cost.CostTrackingDecorator
	PresetResolver      *PresetResolver // Optional: if nil, one will be created
	EventEmitter        agent.EventListener
	CostTracker         storage.CostTracker
	OKRContextProvider  OKRContextProvider  // Optional: provides OKR context for system prompt
	CredentialRefresher CredentialRefresher // Optional: re-resolves CLI credentials at task time
	ChannelHints        map[string]string   // Optional: channel-name → formatting hint text
}

// ExecutionPreparationService prepares everything needed before executing a task.
type ExecutionPreparationService struct {
	llmFactory          llm.LLMClientFactory
	toolRegistry        tools.ToolRegistry
	sessionStore        storage.SessionStore
	contextMgr          agent.ContextManager
	historyMgr          storage.HistoryManager
	parser              agent.FunctionCallParser
	config              appconfig.Config
	logger              agent.Logger
	clock               agent.Clock
	costDecorator       *cost.CostTrackingDecorator
	toolPolicy          toolspolicy.ToolPolicy
	presetResolver      *PresetResolver
	eventEmitter        agent.EventListener
	costTracker         storage.CostTracker
	okrContextProvider  OKRContextProvider
	credentialRefresher CredentialRefresher
	channelHints        map[string]string
}

// NewExecutionPreparationService creates a service instance.
func NewExecutionPreparationService(deps ExecutionPreparationDeps) *ExecutionPreparationService {
	logger := deps.Logger
	if logger == nil {
		logger = agent.NoopLogger{}
	}
	clock := deps.Clock
	if clock == nil {
		clock = agent.SystemClock{}
	}

	costDecorator := deps.CostDecorator
	if costDecorator == nil {
		costDecorator = cost.NewCostTrackingDecorator(nil, logger, clock)
	}

	eventEmitter := deps.EventEmitter
	if eventEmitter == nil {
		eventEmitter = agent.NoopEventListener{}
	}

	presetResolver := deps.PresetResolver
	if presetResolver == nil {
		presetResolver = newPresetResolverWithDeps(presetResolverDeps{
			Logger:       logger,
			Clock:        clock,
			EventEmitter: eventEmitter,
		})
	}

	toolPolicy := toolspolicy.NewToolPolicy(deps.Config.ToolPolicy)

	return &ExecutionPreparationService{
		llmFactory:          deps.LLMFactory,
		toolRegistry:        deps.ToolRegistry,
		sessionStore:        deps.SessionStore,
		contextMgr:          deps.ContextMgr,
		historyMgr:          deps.HistoryMgr,
		parser:              deps.Parser,
		config:              deps.Config,
		logger:              logger,
		clock:               clock,
		costDecorator:       costDecorator,
		toolPolicy:          toolPolicy,
		presetResolver:      presetResolver,
		eventEmitter:        eventEmitter,
		costTracker:         deps.CostTracker,
		okrContextProvider:  deps.OKRContextProvider,
		credentialRefresher: deps.CredentialRefresher,
		channelHints:        deps.ChannelHints,
	}
}

// preAnalyzeTaskAsync fires preAnalyzeTask in a background goroutine and
// persists the resulting title to the session store when done. This removes
// the LLM round-trip from the critical path of Prepare().
func (s *ExecutionPreparationService) preAnalyzeTaskAsync(ctx context.Context, session *storage.Session, task string) {
	if session == nil || utils.IsBlank(session.ID) {
		return
	}
	if session.Metadata != nil && utils.HasContent(session.Metadata["title"]) {
		return
	}

	sessionID := session.ID
	ids := id.IDsFromContext(ctx)

	// Snapshot a minimal session for the goroutine so it doesn't race with the
	// caller mutating the original session object.
	sessionSnapshot := &storage.Session{
		ID:       sessionID,
		Metadata: map[string]string{},
	}

	bgCtx := context.Background()
	if logID := id.LogIDFromContext(ctx); logID != "" {
		bgCtx = id.WithLogID(bgCtx, logID)
	}

	async.Go(s.logger, "preanalysis-title", func() {
		analysis := s.preAnalyzeTask(bgCtx, sessionSnapshot, task)
		if analysis == nil {
			return
		}
		if analysis.ReactEmoji != "" {
			s.eventEmitter.OnEvent(domain.NewPreAnalysisEmojiEvent(
				agent.LevelCore,
				sessionID,
				ids.RunID,
				ids.ParentRunID,
				analysis.ReactEmoji,
				s.clock.Now(),
			))
		}
		title := utils.NormalizeSessionTitle(analysis.ActionName)
		if title == "" {
			return
		}

		persistCtx, cancel := context.WithTimeout(bgCtx, 2*time.Second)
		defer cancel()

		sess, err := s.sessionStore.Get(persistCtx, sessionID)
		if err != nil {
			s.logger.Warn("Async title: failed to load session: %v", err)
			return
		}
		if sess.Metadata == nil {
			sess.Metadata = make(map[string]string)
		}
		if utils.HasContent(sess.Metadata["title"]) {
			return // Title already set by plan tool or elsewhere.
		}
		sess.Metadata["title"] = title
		if err := s.sessionStore.Save(persistCtx, sess); err != nil {
			s.logger.Warn("Async title: failed to persist: %v", err)
		}
	})
}

func cloneHeaders(headers map[string]string) map[string]string {
	return llmclient.CloneHeaders(headers)
}
