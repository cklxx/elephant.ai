package signals

import "time"

// SignalEvent represents a normalized event from any external source.
type SignalEvent struct {
	ID        string
	Source    SignalSource
	Type      string
	Content   string
	EntityID  string
	ChatID    string
	UserID    string
	Metadata  map[string]string
	Timestamp time.Time
	Score     int
	Route     AttentionRoute
}

// SignalSource identifies the external system that produced a signal.
type SignalSource string

const (
	SourceLark     SignalSource = "lark"
	SourceGit      SignalSource = "git"
	SourceCalendar SignalSource = "calendar"
)

// AttentionRoute describes how a scored signal should be handled.
type AttentionRoute string

const (
	RouteSuppress  AttentionRoute = "suppress"
	RouteSummarize AttentionRoute = "summarize"
	RouteQueue     AttentionRoute = "queue"
	RouteNotifyNow AttentionRoute = "notify_now"
	RouteEscalate  AttentionRoute = "escalate"
)
