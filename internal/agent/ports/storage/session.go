package storage

import (
	"context"
	"time"

	core "alex/internal/agent/ports"
)

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

// Session represents an agent session
type Session struct {
	ID          string                   `json:"id"`
	Messages    []core.Message           `json:"messages"`
	Todos       []Todo                   `json:"todos"`
	Metadata    map[string]string        `json:"metadata"`
	Attachments map[string]core.Attachment    `json:"attachments,omitempty"`
	Important   map[string]core.ImportantNote `json:"important,omitempty"`
	UserPersona *core.UserPersonaProfile      `json:"user_persona,omitempty"`
	CreatedAt   time.Time                `json:"created_at"`
	UpdatedAt   time.Time                `json:"updated_at"`
}

// Todo represents a task item
type Todo struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}
