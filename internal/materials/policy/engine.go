package policy

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	materialapi "alex/internal/materials/api"
)

// DefaultArtifactRetention is the standard TTL applied to final artifacts when
// no explicit override is provided.
const DefaultArtifactRetention = 90 * 24 * time.Hour

var defaultEngine = NewEngine()

// DefaultEngine returns a singleton engine configured with the baked-in
// retention policy. Callers that need custom behavior should construct their
// own engine with NewEngine and the relevant options.
func DefaultEngine() *Engine {
	return defaultEngine
}

// RetentionUpdater captures the ability to mutate the retention window for a
// material that already exists in the registry.
type RetentionUpdater interface {
	UpdateRetention(ctx context.Context, materialID string, ttlSeconds uint64) error
}

// AuditEntry records a retention change for governance reviews.
type AuditEntry struct {
	MaterialID string
	Action     string
	Actor      string
	Reason     string
	TTLSeconds uint64
	Timestamp  time.Time
}

// AuditLogger is notified whenever the policy engine mutates retention values.
type AuditLogger interface {
	Record(ctx context.Context, entry AuditEntry)
}

// Engine encapsulates lifecycle policy decisions (default TTLs, pin/unpin
// semantics, and audit logging).
type Engine struct {
	artifactTTL time.Duration
	audit       AuditLogger
	now         func() time.Time
}

// Option customizes the policy engine.
type Option func(*Engine)

// WithArtifactRetention overrides the default TTL for final artifacts.
func WithArtifactRetention(ttl time.Duration) Option {
	return func(e *Engine) {
		if ttl > 0 {
			e.artifactTTL = ttl
		}
	}
}

// WithAuditLogger sets the audit logger used to capture retention mutations.
func WithAuditLogger(logger AuditLogger) Option {
	return func(e *Engine) {
		if logger != nil {
			e.audit = logger
		}
	}
}

// WithNow overrides the clock source (useful for deterministic tests).
func WithNow(now func() time.Time) Option {
	return func(e *Engine) {
		if now != nil {
			e.now = now
		}
	}
}

// NewEngine builds a retention policy engine with optional overrides.
func NewEngine(opts ...Option) *Engine {
	engine := &Engine{
		artifactTTL: DefaultArtifactRetention,
		audit:       logAuditLogger{},
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(engine)
		}
	}
	return engine
}

// ResolveRetention mirrors the broker's legacy logic: callers may supply an
// explicit TTL (in seconds), an override duration, or rely on the policy
// engine's defaults based on status/kind.
func (e *Engine) ResolveRetention(explicitTTLSeconds uint64, status materialapi.MaterialStatus, kind materialapi.MaterialKind, override time.Duration) time.Duration {
	if explicitTTLSeconds > 0 {
		return time.Duration(explicitTTLSeconds) * time.Second
	}
	return e.retentionWindow(status, kind, override)
}

// PinMaterial marks a material as permanent (retention=0) and records the
// mutation for auditing purposes.
func (e *Engine) PinMaterial(ctx context.Context, updater RetentionUpdater, materialID, actor, reason string) error {
	return e.setRetention(ctx, updater, materialID, 0, "pin", actor, reason)
}

// UnpinMaterial reapplies the policy defaults (or override) for the supplied
// status/kind combination and logs the action.
func (e *Engine) UnpinMaterial(ctx context.Context, updater RetentionUpdater, materialID string, status materialapi.MaterialStatus, kind materialapi.MaterialKind, override time.Duration, actor, reason string) error {
	ttl := e.retentionWindow(status, kind, override)
	ttlSeconds := uint64(ttl / time.Second)
	return e.setRetention(ctx, updater, materialID, ttlSeconds, "unpin", actor, reason)
}

func (e *Engine) setRetention(ctx context.Context, updater RetentionUpdater, materialID string, ttlSeconds uint64, action, actor, reason string) error {
	if updater == nil {
		return errors.New("retention updater is required")
	}
	if strings.TrimSpace(materialID) == "" {
		return errors.New("material id is required")
	}
	if err := updater.UpdateRetention(ctx, materialID, ttlSeconds); err != nil {
		return err
	}
	e.recordAudit(ctx, AuditEntry{
		MaterialID: materialID,
		Action:     action,
		Actor:      actor,
		Reason:     reason,
		TTLSeconds: ttlSeconds,
		Timestamp:  e.now(),
	})
	return nil
}

func (e *Engine) retentionWindow(status materialapi.MaterialStatus, kind materialapi.MaterialKind, override time.Duration) time.Duration {
	if override > 0 {
		return override
	}
	switch status {
	case materialapi.MaterialStatusInput:
		return 30 * 24 * time.Hour
	case materialapi.MaterialStatusIntermediate:
		return 7 * 24 * time.Hour
	case materialapi.MaterialStatusFinal:
		if kind == materialapi.MaterialKindArtifact {
			if e.artifactTTL > 0 {
				return e.artifactTTL
			}
			return DefaultArtifactRetention
		}
		return 0
	default:
		return 0
	}
}

func (e *Engine) recordAudit(ctx context.Context, entry AuditEntry) {
	if e.audit == nil {
		return
	}
	e.audit.Record(ctx, entry)
}

type logAuditLogger struct {
	logger *slog.Logger
}

func (l logAuditLogger) Record(ctx context.Context, entry AuditEntry) {
	logger := l.logger
	if logger == nil {
		logger = slog.Default()
	}
	if logger == nil {
		return
	}
	logger.InfoContext(ctx, "material retention updated",
		"material_id", entry.MaterialID,
		"action", entry.Action,
		"actor", entry.Actor,
		"reason", entry.Reason,
		"ttl_seconds", entry.TTLSeconds,
	)
}

// InMemoryAuditLogger captures audit entries for tests or dev tooling.
type InMemoryAuditLogger struct {
	mu      sync.Mutex
	entries []AuditEntry
}

// NewInMemoryAuditLogger builds an audit logger that simply stores entries in
// memory for later inspection.
func NewInMemoryAuditLogger() *InMemoryAuditLogger {
	return &InMemoryAuditLogger{}
}

// Record implements AuditLogger.
func (l *InMemoryAuditLogger) Record(ctx context.Context, entry AuditEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, entry)
}

// Entries returns a copy of the recorded audit entries.
func (l *InMemoryAuditLogger) Entries() []AuditEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]AuditEntry, len(l.entries))
	copy(out, l.entries)
	return out
}
