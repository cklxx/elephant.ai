package context

import (
	"path/filepath"
	"strings"
	"sync"
	"time"

	agent "alex/internal/agent/ports/agent"
	"alex/internal/analytics/journal"
	"alex/internal/logging"
	"alex/internal/observability"
	sessionstate "alex/internal/session/state_store"
)

type manager struct {
	threshold  float64
	configRoot string
	logger     logging.Logger
	stateStore sessionstate.Store
	metrics    *observability.ContextMetrics
	journal    journal.Writer

	static      *staticRegistry
	sopResolver *SOPResolver
	flushHook   FlushBeforeCompactionHook
	preloadOnce sync.Once
	preloadErr  error
}

func (m *manager) compressionThreshold() float64 {
	if m.threshold <= 0 {
		return defaultThreshold
	}
	return m.threshold
}

const (
	defaultThreshold    = 0.8
	defaultStaticTTL    = 30 * time.Minute
	contextConfigEnvVar = "ALEX_CONTEXT_CONFIG_DIR"
)

// Option configures the context manager.
type Option func(*manager)

// WithConfigRoot overrides the directory used for static context files.
func WithConfigRoot(root string) Option {
	return func(m *manager) {
		if root != "" {
			m.configRoot = root
		}
	}
}

// WithStateStore attaches a dynamic state store implementation.
func WithStateStore(store sessionstate.Store) Option {
	return func(m *manager) {
		m.stateStore = store
	}
}

// WithLogger injects a custom logger (used by tests).
func WithLogger(logger logging.Logger) Option {
	return func(m *manager) {
		if !logging.IsNil(logger) {
			m.logger = logger
		}
	}
}

// WithJournalWriter wires a turn journal writer for replay and meta-context jobs.
func WithJournalWriter(writer journal.Writer) Option {
	return func(m *manager) {
		if writer != nil {
			m.journal = writer
		}
	}
}

// WithMetrics allows overriding the metrics recorder.
func WithMetrics(metrics *observability.ContextMetrics) Option {
	return func(m *manager) {
		if metrics != nil {
			m.metrics = metrics
		}
	}
}

// WithSOPResolver injects a custom SOP resolver (useful for tests).
func WithSOPResolver(resolver *SOPResolver) Option {
	return func(m *manager) {
		m.sopResolver = resolver
	}
}

// WithFlushHook attaches a hook that is called before context compaction to
// persist key information from the about-to-be-compressed messages.
func WithFlushHook(hook FlushBeforeCompactionHook) Option {
	return func(m *manager) {
		if hook != nil {
			m.flushHook = hook
		}
	}
}

// NewManager constructs a layered context manager implementation.
func NewManager(opts ...Option) agent.ContextManager {
	root := resolveContextConfigRoot()

	m := &manager{
		threshold:  defaultThreshold,
		configRoot: root,
		logger:     logging.NewComponentLogger("ContextManager"),
		metrics:    observability.NewContextMetrics(),
		journal:    journal.NopWriter(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(m)
		}
	}
	if m.static == nil {
		cfgRoot := m.configRoot
		if cfgRoot == "" {
			cfgRoot = root
		}
		m.static = newStaticRegistry(cfgRoot, defaultStaticTTL, m.logger, m.metrics)
	}
	if m.sopResolver == nil {
		repoRoot := deriveRepoRoot(m.configRoot)
		m.sopResolver = NewSOPResolver(repoRoot, m.logger)
	}
	return m
}

// deriveRepoRoot strips the "configs/context" suffix from the config root to
// obtain the repository root directory.
func deriveRepoRoot(configRoot string) string {
	cleaned := filepath.Clean(configRoot)
	suffix := filepath.Join("configs", "context")
	if strings.HasSuffix(cleaned, suffix) {
		return strings.TrimSuffix(cleaned, suffix)
	}
	// Fallback: walk up two levels if the path ends with the expected dirs.
	return filepath.Dir(filepath.Dir(cleaned))
}
