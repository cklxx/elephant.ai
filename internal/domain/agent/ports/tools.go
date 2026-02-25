package ports

import (
	"encoding/json"
	"errors"
	"strings"
)

// ToolCall represents a request to execute a tool
type ToolCall struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Arguments    map[string]any `json:"arguments"`
	SessionID    string         `json:"session_id,omitempty"`
	TaskID       string         `json:"task_id,omitempty"`
	ParentTaskID string         `json:"parent_task_id,omitempty"`
}

// ToolResult is the execution result
type ToolResult struct {
	CallID       string                `json:"call_id"`
	Content      string                `json:"content"`
	Error        error                 `json:"error,omitempty"`
	Metadata     map[string]any        `json:"metadata,omitempty"`
	SessionID    string                `json:"session_id,omitempty"`
	TaskID       string                `json:"task_id,omitempty"`
	ParentTaskID string                `json:"parent_task_id,omitempty"`
	Attachments  map[string]Attachment `json:"attachments,omitempty"`
}

// MarshalJSON customizes ToolResult JSON encoding to support the error interface.
func (r ToolResult) MarshalJSON() ([]byte, error) {
	type Alias struct {
		CallID      string                `json:"call_id"`
		Content     string                `json:"content"`
		Error       any                   `json:"error,omitempty"`
		Metadata    map[string]any        `json:"metadata,omitempty"`
		Attachments map[string]Attachment `json:"attachments,omitempty"`
	}

	alias := Alias{
		CallID:      r.CallID,
		Content:     r.Content,
		Metadata:    r.Metadata,
		Attachments: r.Attachments,
	}

	if r.Error != nil {
		alias.Error = r.Error.Error()
	}

	return json.Marshal(alias)
}

// UnmarshalJSON customizes ToolResult decoding to accept both string and object error representations.
func (r *ToolResult) UnmarshalJSON(data []byte) error {
	type Alias struct {
		CallID      string                `json:"call_id"`
		Content     string                `json:"content"`
		Error       json.RawMessage       `json:"error"`
		Metadata    map[string]any        `json:"metadata,omitempty"`
		Attachments map[string]Attachment `json:"attachments,omitempty"`
	}

	var aux Alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	r.CallID = aux.CallID
	r.Content = aux.Content
	r.Metadata = aux.Metadata
	r.Attachments = aux.Attachments
	r.Error = nil

	raw := strings.TrimSpace(string(aux.Error))
	if raw == "" || raw == "null" {
		return nil
	}

	var errStr string
	if err := json.Unmarshal(aux.Error, &errStr); err == nil {
		if errStr != "" {
			r.Error = errors.New(errStr)
		}
		return nil
	}

	var errObj map[string]any
	if err := json.Unmarshal(aux.Error, &errObj); err == nil {
		if msg, ok := errObj["message"].(string); ok && msg != "" {
			r.Error = errors.New(msg)
			return nil
		}
		if msg, ok := errObj["error"].(string); ok && msg != "" {
			r.Error = errors.New(msg)
			return nil
		}
	}

	// Fallback: use the raw JSON string as the error message
	if raw != "" {
		r.Error = errors.New(raw)
	}

	return nil
}

// ToolDefinition describes a tool for the LLM
type ToolMaterialCapabilities struct {
	Consumes          []string `json:"consumes,omitempty"`
	Produces          []string `json:"produces,omitempty"`
	ProducesArtifacts []string `json:"produces_artifacts,omitempty"`
}

// IsZero allows ToolMaterialCapabilities to honor json omitempty semantics.
func (c ToolMaterialCapabilities) IsZero() bool {
	return len(c.Consumes) == 0 && len(c.Produces) == 0 && len(c.ProducesArtifacts) == 0
}

// ToolDefinition describes a tool for the LLM
type ToolDefinition struct {
	Name                 string                   `json:"name"`
	Description          string                   `json:"description"`
	Parameters           ParameterSchema          `json:"parameters"`
	MaterialCapabilities ToolMaterialCapabilities `json:"material_capabilities,omitempty"`
}

// Safety levels for tool classification (L1=read-only, L4=high-impact irreversible).
const (
	SafetyLevelUnset       = 0 // Use Dangerous flag fallback: false→L1, true→L3
	SafetyLevelReadOnly    = 1 // L1: read-only operations
	SafetyLevelReversible  = 2 // L2: reversible write operations
	SafetyLevelHighImpact  = 3 // L3: high-impact but reversible (requires confirmation)
	SafetyLevelIrreversible = 4 // L4: high-impact irreversible (requires confirmation + alternative plan)
)

// ToolMetadata contains tool information
type ToolMetadata struct {
	Name                 string                   `json:"name"`
	Version              string                   `json:"version"`
	Category             string                   `json:"category"`
	Tags                 []string                 `json:"tags"`
	Dangerous            bool                     `json:"dangerous"`
	SafetyLevel          int                      `json:"safety_level,omitempty"`
	MaterialCapabilities ToolMaterialCapabilities `json:"material_capabilities,omitempty"`
}

// EffectiveSafetyLevel returns the safety level, falling back to Dangerous flag
// when SafetyLevel is unset.
func (m ToolMetadata) EffectiveSafetyLevel() int {
	if m.SafetyLevel > 0 {
		return m.SafetyLevel
	}
	if m.Dangerous {
		return SafetyLevelHighImpact
	}
	return SafetyLevelReadOnly
}

// ParameterSchema defines tool parameters (JSON Schema format)
type ParameterSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

// Property defines a single parameter
type Property struct {
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Enum        []any     `json:"enum,omitempty"`
	Items       *Property `json:"items,omitempty"`
}
