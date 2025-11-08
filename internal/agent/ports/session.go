package ports

import (
	"context"
	"time"
)

// SessionStore persists agent sessions
type SessionStore interface {
	// Create creates a new session
	Create(ctx context.Context) (*Session, error)

	// Get retrieves a session by ID
	Get(ctx context.Context, id string) (*Session, error)

	// Save persists session state
	Save(ctx context.Context, session *Session) error

	// List returns all session IDs
	List(ctx context.Context) ([]string, error)

	// Delete removes a session
	Delete(ctx context.Context, id string) error
}

// Session represents an agent session
type Session struct {
	ID        string            `json:"id"`
	UserID    string            `json:"user_id"`
	Messages  []Message         `json:"messages"`
	Todos     []Todo            `json:"todos"`
	Metadata  map[string]string `json:"metadata"`
	Artifacts []Artifact        `json:"artifacts,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// Artifact represents a binary asset stored separately from the session payload.
type Artifact struct {
	ID          string    `json:"id"`
	SessionID   string    `json:"session_id"`
	UserID      string    `json:"user_id"`
	MessageID   string    `json:"message_id,omitempty"`
	Name        string    `json:"name"`
	MediaType   string    `json:"media_type"`
	StorageKey  string    `json:"storage_key"`
	URI         string    `json:"uri,omitempty"`
	Description string    `json:"description,omitempty"`
	Size        int64     `json:"size,omitempty"`
	Checksum    string    `json:"checksum,omitempty"`
	Source      string    `json:"source,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Todo represents a task item
type Todo struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}
