package ports

import "time"

// UserPersonaProfile captures the user's core personality configuration.
type UserPersonaProfile struct {
	Version           string                 `json:"version"`
	UpdatedAt         time.Time              `json:"updated_at"`
	InitiativeSources []string               `json:"initiative_sources,omitempty"`
	CoreDrives        []UserPersonaDrive     `json:"core_drives,omitempty"`
	TopDrives         []string               `json:"top_drives,omitempty"`
	Values            []string               `json:"values,omitempty"`
	Goals             UserPersonaGoals       `json:"goals,omitempty"`
	Traits            map[string]int         `json:"traits,omitempty"`
	DecisionStyle     string                 `json:"decision_style,omitempty"`
	RiskProfile       string                 `json:"risk_profile,omitempty"`
	ConflictStyle     string                 `json:"conflict_style,omitempty"`
	KeyChoices        []string               `json:"key_choices,omitempty"`
	NonNegotiables    string                 `json:"non_negotiables,omitempty"`
	Summary           string                 `json:"summary,omitempty"`
	ConstructionRules []string               `json:"construction_rules,omitempty"`
	RawAnswers        map[string]interface{} `json:"raw_answers,omitempty"`
}

// UserPersonaDrive represents a core motivational driver and its intensity.
type UserPersonaDrive struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Score int    `json:"score"`
}

// UserPersonaGoals captures user goals across time horizons.
type UserPersonaGoals struct {
	CurrentFocus string `json:"current_focus,omitempty"`
	OneYear      string `json:"one_year,omitempty"`
	ThreeYear    string `json:"three_year,omitempty"`
}
