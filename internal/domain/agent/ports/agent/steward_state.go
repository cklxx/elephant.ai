package agent

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// StewardState captures the cross-turn structured state document maintained by
// the steward mode. It is injected as SYSTEM_REMINDER each turn and updated by
// parsing the model's <NEW_STATE> output block.
type StewardState struct {
	Version       int              `json:"version"`
	Goal          string           `json:"goal"`
	Context       string           `json:"context"`
	Plan          []StewardAction  `json:"plan"`
	TaskGraph     []TaskGraphNode  `json:"task_graph"`
	Decisions     []Decision       `json:"decisions"`
	Artifacts     []ArtifactRef    `json:"artifacts"`
	Risks         []Risk           `json:"risks"`
	EvidenceIndex []EvidenceRef    `json:"evidence_index"`
}

// StewardAction represents a planned next step with tool binding.
type StewardAction struct {
	Input      string `json:"input"`
	Tool       string `json:"tool"`
	OutputID   string `json:"output_id"`
	Checkpoint string `json:"checkpoint"`
}

// TaskGraphNode represents a node in the task DAG.
type TaskGraphNode struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Status        string   `json:"status"`
	DependsOn     []string `json:"depends_on"`
	Reversibility int      `json:"reversibility"`
}

// Decision records a conclusion with evidence backing.
type Decision struct {
	Conclusion   string `json:"conclusion"`
	EvidenceRef  string `json:"evidence_ref"`
	Alternatives string `json:"alternatives,omitempty"`
}

// ArtifactRef references a produced artifact.
type ArtifactRef struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	URL  string `json:"url"`
}

// Risk captures a trigger-action pair for rollback mapping.
type Risk struct {
	Trigger string `json:"trigger"`
	Action  string `json:"action"`
}

// EvidenceRef indexes a source reference used in decisions.
type EvidenceRef struct {
	Ref     string `json:"ref"`
	Source  string `json:"source"`
	Summary string `json:"summary"`
}

const (
	// MaxStewardStateChars is the default character budget for the rendered
	// state (approx 1400 CJK characters ≈ 4200 bytes).
	MaxStewardStateChars = 1400

	// MaxTaskGraphNodes caps the task DAG to prevent unbounded growth.
	MaxTaskGraphNodes = 8

	// MaxPlanActions caps the number of planned next steps.
	MaxPlanActions = 5
)

// ValidateStewardState checks structural invariants. It returns nil when the
// state is valid.
func ValidateStewardState(s *StewardState) error {
	if s == nil {
		return fmt.Errorf("steward state is nil")
	}
	if len(s.TaskGraph) > MaxTaskGraphNodes {
		return fmt.Errorf("task_graph has %d nodes, max %d", len(s.TaskGraph), MaxTaskGraphNodes)
	}
	if len(s.Plan) > MaxPlanActions {
		return fmt.Errorf("plan has %d actions, max %d", len(s.Plan), MaxPlanActions)
	}
	rendered := RenderAsReminder(s)
	charCount := utf8.RuneCountInString(rendered)
	if charCount > MaxStewardStateChars {
		return fmt.Errorf("rendered state is %d chars, max %d", charCount, MaxStewardStateChars)
	}
	return nil
}

// RenderAsReminder formats the StewardState as a markdown section suitable for
// injection into the system prompt as a SYSTEM_REMINDER block.
func RenderAsReminder(s *StewardState) string {
	if s == nil {
		return ""
	}
	var b strings.Builder

	b.WriteString(fmt.Sprintf("**STATE v%d**\n\n", s.Version))

	if s.Goal != "" {
		b.WriteString("**Goal**: ")
		b.WriteString(strings.TrimSpace(s.Goal))
		b.WriteString("\n\n")
	}

	if s.Context != "" {
		b.WriteString("**Context**: ")
		b.WriteString(strings.TrimSpace(s.Context))
		b.WriteString("\n\n")
	}

	if len(s.Plan) > 0 {
		b.WriteString("**Plan**:\n")
		for i, action := range s.Plan {
			b.WriteString(fmt.Sprintf("%d. [%s] %s", i+1, action.Tool, action.Input))
			if action.Checkpoint != "" {
				b.WriteString(fmt.Sprintf(" (checkpoint: %s)", action.Checkpoint))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if len(s.TaskGraph) > 0 {
		b.WriteString("**Task Graph**:\n")
		for _, node := range s.TaskGraph {
			deps := ""
			if len(node.DependsOn) > 0 {
				deps = fmt.Sprintf(" ← [%s]", strings.Join(node.DependsOn, ", "))
			}
			b.WriteString(fmt.Sprintf("- %s: %s [%s] (L%d)%s\n", node.ID, node.Title, node.Status, node.Reversibility, deps))
		}
		b.WriteString("\n")
	}

	if len(s.Decisions) > 0 {
		b.WriteString("**Decisions**:\n")
		for _, d := range s.Decisions {
			b.WriteString(fmt.Sprintf("- %s [ref:%s]", d.Conclusion, d.EvidenceRef))
			if d.Alternatives != "" {
				b.WriteString(fmt.Sprintf(" (alt: %s)", d.Alternatives))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if len(s.Artifacts) > 0 {
		b.WriteString("**Artifacts**:\n")
		for _, a := range s.Artifacts {
			b.WriteString(fmt.Sprintf("- [%s] %s: %s\n", a.Type, a.ID, a.URL))
		}
		b.WriteString("\n")
	}

	if len(s.Risks) > 0 {
		b.WriteString("**Risks**:\n")
		for _, r := range s.Risks {
			b.WriteString(fmt.Sprintf("- IF %s → %s\n", r.Trigger, r.Action))
		}
		b.WriteString("\n")
	}

	if len(s.EvidenceIndex) > 0 {
		b.WriteString("**Evidence**:\n")
		for _, e := range s.EvidenceIndex {
			b.WriteString(fmt.Sprintf("- [%s] %s: %s\n", e.Ref, e.Source, e.Summary))
		}
		b.WriteString("\n")
	}

	return strings.TrimSpace(b.String())
}
