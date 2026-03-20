package tool

import "context"

// Tool describes a tool that can be executed by the agent.
type Tool struct {
	Name        string                                                       `json:"name"`
	Description string                                                       `json:"description"`
	Parameters  ParameterSchema                                              `json:"parameters"`
	Handler     func(ctx context.Context, args map[string]any) (string, error) `json:"-"`
	Metadata    Metadata                                                     `json:"metadata,omitempty"`
}

// ParameterSchema defines tool parameters in JSON Schema format.
type ParameterSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

// Property defines a single parameter property.
type Property struct {
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Enum        []any     `json:"enum,omitempty"`
	Items       *Property `json:"items,omitempty"`
}

// Metadata contains tool classification and capability information.
type Metadata struct {
	Name        string   `json:"name"`
	Version     string   `json:"version,omitempty"`
	Category    string   `json:"category,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	SafetyLevel int      `json:"safety_level,omitempty"`
}

// Safety level constants.
const (
	SafetyReadOnly     = 1 // L1: read-only
	SafetyReversible   = 2 // L2: reversible writes
	SafetyHighImpact   = 3 // L3: high-impact, reversible
	SafetyIrreversible = 4 // L4: high-impact, irreversible
)
