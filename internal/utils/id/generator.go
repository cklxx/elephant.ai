package id

import (
	"fmt"
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

// NewTaskID generates a new task identifier with a stable prefix for display.
func NewTaskID() string {
	return defaultGenerator.newIdentifier("task")
}

// NewArtifactID generates a unique identifier for artifacts stored in blob storage.
func NewArtifactID() string {
	return defaultGenerator.newIdentifier("artifact")
}

func (g *Generator) newIdentifier(prefix string) string {
	g.mu.RLock()
	strategy := g.strategy
	g.mu.RUnlock()

	var body string
	switch strategy {
	case StrategyUUIDv7:
		uuidv7, err := uuid.NewV7()
		if err == nil {
			body = uuidv7.String()
			break
		}
		fallthrough
	case StrategyKSUID:
		body = ksuid.New().String()
	default:
		body = ksuid.New().String()
	}

	return fmt.Sprintf("%s-%s", prefix, body)
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
