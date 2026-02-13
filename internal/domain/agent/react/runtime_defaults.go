package react

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
)

type fallbackContextKey string

const (
	fallbackSessionKey     fallbackContextKey = "session_id"
	fallbackRunKey         fallbackContextKey = "run_id"
	fallbackParentRunKey   fallbackContextKey = "parent_run_id"
	fallbackLogKey         fallbackContextKey = "log_id"
	fallbackCorrelationKey fallbackContextKey = "correlation_id"
	fallbackCausationKey   fallbackContextKey = "causation_id"
)

var fallbackIDSequence atomic.Uint64

type defaultIDGenerator struct{}

func (defaultIDGenerator) NewEventID() string { return defaultIdentifier("evt") }
func (defaultIDGenerator) NewRunID() string   { return defaultIdentifier("run") }
func (defaultIDGenerator) NewRequestIDWithLogID(logID string) string {
	requestID := defaultIdentifier("llm")
	if logID == "" {
		return requestID
	}
	return fmt.Sprintf("%s:%s", logID, requestID)
}
func (defaultIDGenerator) NewLogID() string  { return defaultIdentifier("log") }
func (defaultIDGenerator) NewKSUID() string  { return defaultIdentifier("ksuid") }
func (defaultIDGenerator) NewUUIDv7() string { return defaultIdentifier("uuid") }

type defaultIDContextReader struct{}

func (defaultIDContextReader) LogIDFromContext(ctx context.Context) string {
	return stringFromContext(ctx, fallbackLogKey)
}
func (defaultIDContextReader) CorrelationIDFromContext(ctx context.Context) string {
	return stringFromContext(ctx, fallbackCorrelationKey)
}
func (defaultIDContextReader) CausationIDFromContext(ctx context.Context) string {
	return stringFromContext(ctx, fallbackCausationKey)
}
func (defaultIDContextReader) IDsFromContext(ctx context.Context) agent.IDContextValues {
	return agent.IDContextValues{
		SessionID:     stringFromContext(ctx, fallbackSessionKey),
		RunID:         stringFromContext(ctx, fallbackRunKey),
		ParentRunID:   stringFromContext(ctx, fallbackParentRunKey),
		LogID:         stringFromContext(ctx, fallbackLogKey),
		CorrelationID: stringFromContext(ctx, fallbackCorrelationKey),
		CausationID:   stringFromContext(ctx, fallbackCausationKey),
	}
}
func (defaultIDContextReader) WithSessionID(ctx context.Context, sessionID string) context.Context {
	return withContextValue(ctx, fallbackSessionKey, sessionID)
}
func (defaultIDContextReader) WithRunID(ctx context.Context, runID string) context.Context {
	return withContextValue(ctx, fallbackRunKey, runID)
}
func (defaultIDContextReader) WithParentRunID(ctx context.Context, parentRunID string) context.Context {
	return withContextValue(ctx, fallbackParentRunKey, parentRunID)
}
func (defaultIDContextReader) WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return withContextValue(ctx, fallbackCorrelationKey, correlationID)
}
func (defaultIDContextReader) WithCausationID(ctx context.Context, causationID string) context.Context {
	return withContextValue(ctx, fallbackCausationKey, causationID)
}
func (defaultIDContextReader) WithLogID(ctx context.Context, logID string) context.Context {
	return withContextValue(ctx, fallbackLogKey, logID)
}

func defaultIdentifier(prefix string) string {
	sequence := fallbackIDSequence.Add(1)
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixNano(), sequence)
}

func stringFromContext(ctx context.Context, key fallbackContextKey) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(key).(string)
	return value
}

func withContextValue(ctx context.Context, key fallbackContextKey, value string) context.Context {
	if value == "" {
		return ctx
	}
	return context.WithValue(ctx, key, value)
}
