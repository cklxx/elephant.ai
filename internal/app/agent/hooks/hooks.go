package hooks

import (
	"context"
	"sort"
	"sync"

	agent "alex/internal/agent/ports/agent"
	"alex/internal/logging"
)

// InjectionType classifies the kind of content being injected into context.
type InjectionType string

const (
	InjectionSkillActivation InjectionType = "skill_activation"
	InjectionSuggestion      InjectionType = "suggestion"
	InjectionWarning         InjectionType = "warning"
	InjectionOKRContext      InjectionType = "okr_context"
)

// Injection describes content to be injected into the agent context.
type Injection struct {
	Type     InjectionType // Classification of the injection
	Content  string        // Text content to inject
	Source   string        // Name of the hook that produced this injection
	Priority int           // Higher priority injections appear first
}

// TaskInfo provides task context to hooks without coupling to domain types.
type TaskInfo struct {
	TaskInput   string
	SessionID   string
	RunID       string
	UserID      string
	ToolResults []ToolResultInfo
}

// ToolResultInfo is a simplified tool result for hook consumption.
type ToolResultInfo struct {
	ToolName string
	Success  bool
	Output   string
}

// TaskResultInfo provides task completion context to hooks.
type TaskResultInfo struct {
	TaskInput  string
	Answer     string
	SessionID  string
	RunID      string
	UserID     string
	Iterations int
	StopReason string
	ToolCalls  []ToolResultInfo
}

// ProactiveHook defines the interface for proactive behavior hooks.
// Implementations can inject content into agent context at various lifecycle points.
type ProactiveHook interface {
	// Name returns a unique identifier for this hook (used in observability).
	Name() string

	// OnTaskStart is called before task execution begins.
	// Returns injections to prepend to the agent context.
	OnTaskStart(ctx context.Context, task TaskInfo) []Injection

	// OnTaskCompleted is called after task execution finishes.
	// Used for post-processing such as audit or metrics.
	OnTaskCompleted(ctx context.Context, result TaskResultInfo) error
}

// Registry manages registered hooks and dispatches lifecycle events.
type Registry struct {
	mu     sync.RWMutex
	hooks  []ProactiveHook
	logger logging.Logger
}

// NewRegistry creates a new hook registry.
func NewRegistry(logger logging.Logger) *Registry {
	return &Registry{
		logger: logging.OrNop(logger),
	}
}

// Register adds a hook to the registry. Hooks are called in registration order
// within the same priority level.
func (r *Registry) Register(hook ProactiveHook) {
	if hook == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hooks = append(r.hooks, hook)
	r.logger.Info("Registered proactive hook: %s", hook.Name())
}

// RunOnTaskStart executes all hooks' OnTaskStart methods and returns
// aggregated injections sorted by priority (highest first).
func (r *Registry) RunOnTaskStart(ctx context.Context, task TaskInfo) []Injection {
	r.mu.RLock()
	hooks := make([]ProactiveHook, len(r.hooks))
	copy(hooks, r.hooks)
	r.mu.RUnlock()

	var all []Injection
	for _, hook := range hooks {
		injections := hook.OnTaskStart(ctx, task)
		if len(injections) > 0 {
			r.logger.Debug("Hook %s produced %d injections on task start", hook.Name(), len(injections))
			all = append(all, injections...)
		}
	}

	sort.SliceStable(all, func(i, j int) bool {
		return all[i].Priority > all[j].Priority
	})

	return all
}

// RunOnTaskCompleted executes all hooks' OnTaskCompleted methods.
// Errors are logged but do not stop subsequent hooks from running.
func (r *Registry) RunOnTaskCompleted(ctx context.Context, result TaskResultInfo) {
	r.mu.RLock()
	hooks := make([]ProactiveHook, len(r.hooks))
	copy(hooks, r.hooks)
	r.mu.RUnlock()

	for _, hook := range hooks {
		if err := hook.OnTaskCompleted(ctx, result); err != nil {
			r.logger.Warn("Hook %s OnTaskCompleted failed: %v", hook.Name(), err)
		}
	}
}

// HookCount returns the number of registered hooks.
func (r *Registry) HookCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.hooks)
}

// FormatInjectionsAsContext formats injections into a text block suitable
// for injection into the agent system prompt or as a user message.
func FormatInjectionsAsContext(injections []Injection) string {
	if len(injections) == 0 {
		return ""
	}

	var buf []byte
	buf = append(buf, "## Proactive Context\n\n"...)
	for _, inj := range injections {
		buf = append(buf, "### "...)
		buf = append(buf, string(inj.Type)...)
		buf = append(buf, " (from "...)
		buf = append(buf, inj.Source...)
		buf = append(buf, ")\n\n"...)
		buf = append(buf, inj.Content...)
		buf = append(buf, "\n\n"...)
	}
	return string(buf)
}

// Ensure NoopEventListener compatibility for embedding.
var _ agent.EventListener = agent.NoopEventListener{}
