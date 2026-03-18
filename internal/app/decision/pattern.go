package decision

import "time"

// ConfidenceFloor is the minimum confidence for auto-acting. Below this, escalate.
const ConfidenceFloor = 0.90

// Pattern represents a learned decision-making pattern.
type Pattern struct {
	ID          string    `json:"id"`
	Category    string    `json:"category"`
	Description string    `json:"description"`
	Condition   string    `json:"condition"`
	Action      string    `json:"action"`
	Confidence  float64   `json:"confidence"`
	SampleCount int       `json:"sample_count"`
	LastMatched time.Time `json:"last_matched,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Checksum    string    `json:"checksum"`
}
