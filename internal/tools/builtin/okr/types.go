package okr

// GoalMeta holds the YAML frontmatter of a goal file.
type GoalMeta struct {
	ID            string               `yaml:"id"`
	Owner         string               `yaml:"owner"`
	Created       string               `yaml:"created"`
	Updated       string               `yaml:"updated"`
	Status        string               `yaml:"status"` // active | completed | paused | abandoned
	TimeWindow    TimeWindow           `yaml:"time_window"`
	ReviewCadence string               `yaml:"review_cadence"` // cron expression
	Notifications NotificationConfig   `yaml:"notifications"`
	KeyResults    map[string]KRMeta    `yaml:"key_results"`
}

// TimeWindow defines the start and end of the goal period.
type TimeWindow struct {
	Start string `yaml:"start"`
	End   string `yaml:"end"`
}

// NotificationConfig describes how proactive results are delivered.
type NotificationConfig struct {
	Channel    string `yaml:"channel"`      // lark | wechat
	LarkChatID string `yaml:"lark_chat_id"` // Lark chat_id for messages
}

// KRMeta holds a single key result's tracking data.
type KRMeta struct {
	Metric      string  `yaml:"metric"`
	Baseline    float64 `yaml:"baseline"`
	Target      float64 `yaml:"target"`
	Current     float64 `yaml:"current"`
	ProgressPct float64 `yaml:"progress_pct"`
	Confidence  string  `yaml:"confidence"` // high | medium | low
	Updated     string  `yaml:"updated"`
	Source      string  `yaml:"source"`
}

// GoalFile represents a parsed goal file with its frontmatter metadata
// and the markdown body content.
type GoalFile struct {
	Meta GoalMeta
	Body string // markdown content after frontmatter
}
