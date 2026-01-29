package id

import (
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/segmentio/ksuid"
)

// Strategy identifies the identifier generation algorithm to use.
type Strategy int

const (
	// StrategyKSUID generates lexicographically sortable identifiers using KSUID.
	StrategyKSUID Strategy = iota
	// StrategyUUIDv7 generates time-ordered identifiers using UUID version 7.
	StrategyUUIDv7
)

var (
	defaultGenerator = &Generator{strategy: StrategyKSUID}
)

const runIDSuffixLength = 12

// Generator produces identifiers for sessions and tasks.
type Generator struct {
	mu       sync.RWMutex
	strategy Strategy
}

// SetStrategy configures the generation strategy for the default generator.
func SetStrategy(strategy Strategy) {
	defaultGenerator.setStrategy(strategy)
}

func (g *Generator) setStrategy(strategy Strategy) {
	g.mu.Lock()
	g.strategy = strategy
	g.mu.Unlock()
}

// NewSessionID generates a new session identifier with a stable prefix for display.
func NewSessionID() string {
	return defaultGenerator.newIdentifier("session")
}

// NewRunID generates a new run identifier with a stable prefix for display.
func NewRunID() string {
	return defaultGenerator.newShortIdentifier("run", runIDSuffixLength)
}

// NewEventID generates a unique event identifier.
func NewEventID() string {
	return defaultGenerator.newIdentifier("evt")
}

// NewRequestID generates a new identifier for LLM requests and correlated logs.
func NewRequestID() string {
	return defaultGenerator.newIdentifier("llm")
}

// NewRequestIDWithLogID generates a request identifier that embeds the log id for correlation.
func NewRequestIDWithLogID(logID string) string {
	requestID := NewRequestID()
	logID = strings.TrimSpace(logID)
	if logID == "" {
		return requestID
	}
	return fmt.Sprintf("%s:%s", logID, requestID)
}

// NewLogID generates a new identifier for log correlation.
func NewLogID() string {
	return defaultGenerator.newIdentifier("log")
}

func (g *Generator) generateBody() string {
	g.mu.RLock()
	strategy := g.strategy
	g.mu.RUnlock()

	switch strategy {
	case StrategyUUIDv7:
		uuidv7, err := uuid.NewV7()
		if err == nil {
			return uuidv7.String()
		}
		fallthrough
	case StrategyKSUID:
		return ksuid.New().String()
	default:
		return ksuid.New().String()
	}
}

// NewKSUID exposes raw KSUID generation for callers that need unprefixed identifiers.
func NewKSUID() string {
	return ksuid.New().String()
}

// NewUUIDv7 exposes raw UUIDv7 generation for callers that need unprefixed identifiers.
func NewUUIDv7() string {
	uuidv7, err := uuid.NewV7()
	if err != nil {
		return ""
	}
	return uuidv7.String()
}

func (g *Generator) newIdentifier(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, g.generateBody())
}

func (g *Generator) newShortIdentifier(prefix string, tailLen int) string {
	body := g.generateBody()
	if tailLen > 0 && len(body) > tailLen {
		body = body[len(body)-tailLen:]
	}
	return fmt.Sprintf("%s-%s", prefix, body)
}
