package scheduler

// Trigger represents a scheduled task trigger.
type Trigger struct {
	Name     string // unique trigger name (e.g. "daily_briefing" or "okr:q1-2026-revenue")
	Schedule string // cron expression
	Task     string // task text for agent execution
	Channel  string // delivery channel: lark | web
	UserID   string // internal user_id
	ChatID   string // channel-specific chat_id for notifications
	GoalID   string // non-empty for OKR-derived triggers
}

// IsOKRTrigger returns true if the trigger was derived from an OKR goal.
func (t Trigger) IsOKRTrigger() bool {
	return t.GoalID != ""
}
