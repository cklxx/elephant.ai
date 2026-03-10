package domain

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
)

// BaseEvent provides common fields for all events
type BaseEvent struct {
	// Identity
	eventID string // Unique per event: "evt-{ksuid}"
	seq     uint64 // Monotonic within a run

	// Temporal
	timestamp time.Time

	// Hierarchy
	sessionID   string           // Conversation scope
	runID       string           // This agent execution
	parentRunID string           // Parent agent's runID (empty for core)
	agentLevel  agent.AgentLevel // "core" or "subagent"

	// Causal chain
	correlationID string // Root runID of the causal chain
	causationID   string // call_id that spawned this run

	// Operational
	logID string // Log correlation
}

func (e *BaseEvent) Timestamp() time.Time            { return e.timestamp }
func (e *BaseEvent) GetAgentLevel() agent.AgentLevel { return e.agentLevel }
func (e *BaseEvent) GetSessionID() string            { return e.sessionID }
func (e *BaseEvent) GetRunID() string                { return e.runID }
func (e *BaseEvent) GetParentRunID() string          { return e.parentRunID }
func (e *BaseEvent) GetCorrelationID() string        { return e.correlationID }
func (e *BaseEvent) GetCausationID() string          { return e.causationID }
func (e *BaseEvent) GetEventID() string              { return e.eventID }
func (e *BaseEvent) GetSeq() uint64                  { return e.seq }
func (e *BaseEvent) GetLogID() string                { return e.logID }

// SetLogID attaches a log identifier for correlation.
func (e *BaseEvent) SetLogID(logID string) { e.logID = logID }

// SetSeq assigns a sequence number to the event (typically called by emitter).
func (e *BaseEvent) SetSeq(seq uint64) { e.seq = seq }

// SetCorrelationID assigns the correlation ID.
func (e *BaseEvent) SetCorrelationID(cid string) { e.correlationID = cid }

// SetCausationID assigns the causation ID.
func (e *BaseEvent) SetCausationID(cid string) { e.causationID = cid }

// SeqCounter provides monotonic event sequence numbering within a run.
type SeqCounter struct {
	counter atomic.Uint64
}

// Next returns the next sequence number.
func (s *SeqCounter) Next() uint64 {
	return s.counter.Add(1)
}

type eventIDProvider interface {
	NewEventID() string
}

type defaultEventIDProvider struct{}

func (defaultEventIDProvider) NewEventID() string {
	return fmt.Sprintf("evt-%d", time.Now().UnixNano())
}

var (
	eventIDProviderMu sync.RWMutex
	currentEventIDGen eventIDProvider = defaultEventIDProvider{}
)

// SetEventIDGenerator installs the event ID generator used by domain events.
func SetEventIDGenerator(generator agent.IDGenerator) {
	if generator == nil {
		return
	}
	eventIDProviderMu.Lock()
	currentEventIDGen = generator
	eventIDProviderMu.Unlock()
}

func newBaseEventWithIDs(level agent.AgentLevel, sessionID, runID, parentRunID string, ts time.Time) BaseEvent {
	return BaseEvent{
		eventID:     nextEventID(),
		timestamp:   ts,
		agentLevel:  level,
		sessionID:   sessionID,
		runID:       runID,
		parentRunID: parentRunID,
	}
}

// NewBaseEvent exposes construction of BaseEvent for adapters that need to bridge
// external lifecycle systems (e.g., workflows) into the agent event stream while
// preserving field encapsulation.
func NewBaseEvent(level agent.AgentLevel, sessionID, runID, parentRunID string, ts time.Time) BaseEvent {
	return newBaseEventWithIDs(level, sessionID, runID, parentRunID, ts)
}

// NewBaseEventFull constructs a BaseEvent with all fields including causal chain.
func NewBaseEventFull(level agent.AgentLevel, sessionID, runID, parentRunID, correlationID, causationID string, seq uint64, ts time.Time) BaseEvent {
	return BaseEvent{
		eventID:       nextEventID(),
		seq:           seq,
		timestamp:     ts,
		agentLevel:    level,
		sessionID:     sessionID,
		runID:         runID,
		parentRunID:   parentRunID,
		correlationID: correlationID,
		causationID:   causationID,
	}
}

func nextEventID() string {
	eventIDProviderMu.RLock()
	generator := currentEventIDGen
	eventIDProviderMu.RUnlock()
	if generator == nil {
		return defaultEventIDProvider{}.NewEventID()
	}
	return generator.NewEventID()
}
