package storage

import (
	"context"
	"errors"
	"time"

	core "alex/internal/domain/agent/ports"
)

// ErrSessionNotFound indicates a session lookup failed.
var ErrSessionNotFound = errors.New("session not found")

// SessionStore persists agent sessions
type SessionStore interface {
	// Create creates a new session
	Create(ctx context.Context) (*Session, error)

	// Get retrieves a session by ID
	Get(ctx context.Context, id string) (*Session, error)

	// Save persists session state
	Save(ctx context.Context, session *Session) error

	// List returns session IDs with optional limit/offset pagination.
	List(ctx context.Context, limit int, offset int) ([]string, error)

	// Delete removes a session
	Delete(ctx context.Context, id string) error
}

// SessionListItem is a lightweight session row used by list UIs.
type SessionListItem struct {
	ID        string
	Title     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SessionItemLister is an optional SessionStore extension for lightweight list reads.
type SessionItemLister interface {
	ListSessionItems(ctx context.Context, limit int, offset int) ([]SessionListItem, error)
}

// Session represents an agent session
type Session struct {
	ID          string                        `json:"id"`
	Messages    []core.Message                `json:"messages"`
	Todos       []Todo                        `json:"todos"`
	Metadata    map[string]string             `json:"metadata"`
	Attachments map[string]core.Attachment    `json:"attachments,omitempty"`
	Important   map[string]core.ImportantNote `json:"important,omitempty"`
	UserPersona *core.UserPersonaProfile      `json:"user_persona,omitempty"`
	CreatedAt   time.Time                     `json:"created_at"`
	UpdatedAt   time.Time                     `json:"updated_at"`
}

// EnsureMetadata returns session metadata, initializing it when needed.
func EnsureMetadata(session *Session) map[string]string {
	if session == nil {
		return nil
	}
	if session.Metadata == nil {
		session.Metadata = make(map[string]string)
	}
	return session.Metadata
}

// CloneMetadata returns a shallow copy of metadata, or nil when empty.
func CloneMetadata(metadata map[string]string) map[string]string {
	if len(metadata) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(metadata))
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}

// Todo represents a task item
type Todo struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}
