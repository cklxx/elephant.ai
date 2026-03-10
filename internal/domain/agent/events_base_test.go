package domain

import (
	"testing"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
)

func TestBaseEvent_Getters(t *testing.T) {
	ts := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	base := NewBaseEvent(agent.LevelCore, "sess-1", "run-1", "parent-1", ts)

	if base.GetAgentLevel() != agent.LevelCore {
		t.Errorf("expected LevelCore, got %v", base.GetAgentLevel())
	}
	if base.GetSessionID() != "sess-1" {
		t.Errorf("expected sess-1, got %s", base.GetSessionID())
	}
	if base.GetRunID() != "run-1" {
		t.Errorf("expected run-1, got %s", base.GetRunID())
	}
	if base.GetParentRunID() != "parent-1" {
		t.Errorf("expected parent-1, got %s", base.GetParentRunID())
	}
	if !base.Timestamp().Equal(ts) {
		t.Errorf("expected %v, got %v", ts, base.Timestamp())
	}
	if base.GetEventID() == "" {
		t.Error("expected non-empty event ID")
	}
}

func TestBaseEvent_SettersAndMutators(t *testing.T) {
	ts := time.Now()
	base := NewBaseEvent(agent.LevelSubagent, "s", "r", "p", ts)

	base.SetLogID("log-42")
	if base.GetLogID() != "log-42" {
		t.Errorf("expected log-42, got %s", base.GetLogID())
	}

	base.SetSeq(99)
	if base.GetSeq() != 99 {
		t.Errorf("expected 99, got %d", base.GetSeq())
	}

	base.SetCorrelationID("corr-1")
	if base.GetCorrelationID() != "corr-1" {
		t.Errorf("expected corr-1, got %s", base.GetCorrelationID())
	}

	base.SetCausationID("cause-1")
	if base.GetCausationID() != "cause-1" {
		t.Errorf("expected cause-1, got %s", base.GetCausationID())
	}
}

func TestNewBaseEventFull(t *testing.T) {
	ts := time.Now()
	base := NewBaseEventFull(agent.LevelParallel, "s", "r", "p", "corr", "cause", 7, ts)

	if base.GetCorrelationID() != "corr" {
		t.Errorf("expected corr, got %s", base.GetCorrelationID())
	}
	if base.GetCausationID() != "cause" {
		t.Errorf("expected cause, got %s", base.GetCausationID())
	}
	if base.GetSeq() != 7 {
		t.Errorf("expected 7, got %d", base.GetSeq())
	}
}

func TestSeqCounter_Monotonic(t *testing.T) {
	var sc SeqCounter
	seen := make(map[uint64]bool)
	for i := 0; i < 100; i++ {
		n := sc.Next()
		if seen[n] {
			t.Fatalf("duplicate seq %d", n)
		}
		seen[n] = true
	}
	if sc.Next() != 101 {
		t.Errorf("expected 101 after 100 calls")
	}
}

func TestSetEventIDGenerator_Nil(t *testing.T) {
	SetEventIDGenerator(nil)
	base := NewBaseEvent(agent.LevelCore, "", "", "", time.Now())
	if base.GetEventID() == "" {
		t.Error("default generator should still produce IDs")
	}
}

func TestSetEventIDGenerator_Custom(t *testing.T) {
	SetEventIDGenerator(staticIDGen("custom-id"))
	defer SetEventIDGenerator(staticIDGen("evt-restore"))

	base := NewBaseEvent(agent.LevelCore, "", "", "", time.Now())
	if base.GetEventID() != "custom-id" {
		t.Errorf("expected custom-id, got %s", base.GetEventID())
	}
}

func TestNextEventID_DefaultFormat(t *testing.T) {
	// Reset to a known generator to avoid cross-test pollution
	SetEventIDGenerator(nil) // nil is ignored, default stays
	id := nextEventID()
	if id == "" {
		t.Error("expected non-empty event ID")
	}
}

type staticIDGen string

func (s staticIDGen) NewEventID() string                     { return string(s) }
func (s staticIDGen) NewRunID() string                       { return string(s) }
func (s staticIDGen) NewRequestIDWithLogID(_ string) string  { return string(s) }
func (s staticIDGen) NewLogID() string                       { return string(s) }
func (s staticIDGen) NewKSUID() string                       { return string(s) }
func (s staticIDGen) NewUUIDv7() string                      { return string(s) }
