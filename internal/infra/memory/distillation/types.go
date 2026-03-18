package distillation

import "time"

// ExtractedFact is a single piece of information extracted from conversations.
type ExtractedFact struct {
	ID         string    `json:"id"`
	Content    string    `json:"content"`
	Category   string    `json:"category"`   // "decision", "preference", "fact", "pattern"
	Confidence float64   `json:"confidence"` // 0.0-1.0
	Source     string    `json:"source"`     // daily entry path
	CreatedAt  time.Time `json:"created_at"`
}

// DailyExtraction is the output of one day's distillation.
type DailyExtraction struct {
	Date   string          `json:"date"` // "2026-03-18"
	Facts  []ExtractedFact `json:"facts"`
	Tokens int             `json:"tokens_used"`
}

// WeeklyPattern is a higher-level pattern derived from daily extractions.
type WeeklyPattern struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	Evidence    []string  `json:"evidence"` // fact IDs that support this pattern
	Confidence  float64   `json:"confidence"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// PersonalityModel is a long-term profile built from patterns.
type PersonalityModel struct {
	UserID      string            `json:"user_id"`
	Patterns    []WeeklyPattern   `json:"patterns"`
	Preferences map[string]string `json:"preferences"`
	LastUpdated time.Time         `json:"last_updated"`
}
