package lark

import (
	"context"
	"strings"
	"time"

	ports "alex/internal/domain/agent/ports"
)

// DeliveryMode controls how terminal replies are sent.
type DeliveryMode string

const (
	DeliveryModeDirect DeliveryMode = "direct"
	DeliveryModeShadow DeliveryMode = "shadow"
	DeliveryModeOutbox DeliveryMode = "outbox"
)

func normalizeDeliveryMode(mode string) DeliveryMode {
	switch DeliveryMode(strings.TrimSpace(strings.ToLower(mode))) {
	case DeliveryModeOutbox:
		return DeliveryModeOutbox
	case DeliveryModeShadow:
		return DeliveryModeShadow
	default:
		return DeliveryModeDirect
	}
}

// DeliveryIntentStatus tracks the persistence state of an outbound message.
type DeliveryIntentStatus string

const (
	DeliveryIntentPending  DeliveryIntentStatus = "pending"
	DeliveryIntentSending  DeliveryIntentStatus = "sending"
	DeliveryIntentSent     DeliveryIntentStatus = "sent"
	DeliveryIntentRetrying DeliveryIntentStatus = "retrying"
	DeliveryIntentDead     DeliveryIntentStatus = "dead"
)

// DeliveryIntent records a terminal message delivery task.
type DeliveryIntent struct {
	IntentID          string
	Channel           string
	ChatID            string
	ReplyToMessageID  string
	ProgressMessageID string
	SessionID         string
	RunID             string
	EventType         string
	Sequence          int64
	IdempotencyKey    string
	MsgType           string
	Content           string
	Attachments       map[string]ports.Attachment
	Status            DeliveryIntentStatus
	AttemptCount      int
	NextAttemptAt     time.Time
	LastError         string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	SentAt            time.Time
}

// ReplayFilter selects dead-letter intents to replay.
type ReplayFilter struct {
	IntentIDs []string
	ChatID    string
	RunID     string
	Limit     int
}

// DeliveryOutboxStore persists delivery intents for async retries.
type DeliveryOutboxStore interface {
	EnsureSchema(ctx context.Context) error
	Enqueue(ctx context.Context, intents []DeliveryIntent) ([]DeliveryIntent, error)
	ClaimPending(ctx context.Context, limit int, now time.Time) ([]DeliveryIntent, error)
	MarkSent(ctx context.Context, intentID string, sentAt time.Time) error
	MarkRetry(ctx context.Context, intentID string, nextAttemptAt time.Time, lastErr string) error
	MarkDead(ctx context.Context, intentID string, lastErr string) error
	GetByIdempotencyKey(ctx context.Context, key string) (DeliveryIntent, bool, error)
	Replay(ctx context.Context, filter ReplayFilter) (int, error)
}
