package react

import (
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
)

func TestHandleNoTools_StewardCorrectionPendingContinues(t *testing.T) {
	runtime := &reactRuntime{
		engine:                   &ReactEngine{logger: agent.NoopLogger{}},
		state:                    &TaskState{},
		stewardCorrectionPending: true,
	}
	iteration := &reactIteration{
		runtime: runtime,
		thought: Message{Content: "final answer"},
	}

	result, done, err := iteration.handleNoTools()
	if err != nil {
		t.Fatalf("handleNoTools returned error: %v", err)
	}
	if done {
		t.Fatal("expected iteration to continue when steward correction is pending")
	}
	if result != nil {
		t.Fatalf("expected nil result while awaiting correction, got %+v", result)
	}
	if runtime.stewardCorrectionPending {
		t.Fatal("expected stewardCorrectionPending to be reset")
	}
}

func TestRecordThought_CompressesOversizeStewardState(t *testing.T) {
	longGoal := strings.Repeat("目标", agent.MaxStewardStateChars)
	content := "<NEW_STATE>{\"version\":2,\"goal\":\"" + longGoal + "\",\"decisions\":[{\"conclusion\":\"ok\",\"evidence_ref\":\"r1\"}],\"evidence_index\":[{\"ref\":\"r1\",\"source\":\"tool://web_search\",\"summary\":\"source\"}]}</NEW_STATE>"

	state := &TaskState{
		StewardMode: true,
	}
	iteration := &reactIteration{
		runtime: &reactRuntime{
			engine: &ReactEngine{logger: agent.NoopLogger{}},
			state:  state,
		},
	}
	thought := &Message{Role: "assistant", Content: content}

	iteration.recordThought(thought)
	if state.StewardState == nil {
		t.Fatal("expected steward state to be updated after compression")
	}
	if iteration.runtime.stewardCorrectionPending {
		t.Fatal("did not expect correction pending for compressible state")
	}
	if strings.Contains(thought.Content, "<NEW_STATE>") {
		t.Fatalf("expected NEW_STATE block to be removed from thought content: %q", thought.Content)
	}
}

func TestRecordThought_InjectsCorrectionWhenEvidenceMissing(t *testing.T) {
	content := "<NEW_STATE>{\"version\":1,\"decisions\":[{\"conclusion\":\"ok\",\"evidence_ref\":\"missing\"}],\"evidence_index\":[{\"ref\":\"other\",\"source\":\"tool://x\",\"summary\":\"x\"}]}</NEW_STATE>"

	state := &TaskState{
		StewardMode: true,
		Messages:    []ports.Message{},
	}
	iteration := &reactIteration{
		runtime: &reactRuntime{
			engine: &ReactEngine{logger: agent.NoopLogger{}},
			state:  state,
		},
	}
	thought := &Message{Role: "assistant", Content: content}

	iteration.recordThought(thought)
	if !iteration.runtime.stewardCorrectionPending {
		t.Fatal("expected correction pending when evidence refs are invalid")
	}
	if len(state.Messages) == 0 {
		t.Fatal("expected correction prompt to be injected into state messages")
	}
	last := state.Messages[len(state.Messages)-1]
	if last.Role != "system" {
		t.Fatalf("expected system correction message, got role=%q", last.Role)
	}
	if !strings.Contains(last.Content, stewardEvidencePromptPrefix) {
		t.Fatalf("expected correction message to contain steward evidence prefix, got %q", last.Content)
	}
	if state.StewardState != nil {
		t.Fatal("expected invalid steward state not to be applied")
	}
}
