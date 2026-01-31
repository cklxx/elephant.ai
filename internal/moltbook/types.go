package moltbook

import (
	"fmt"
	"time"
)

// Post represents a Moltbook post.
type Post struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	URL       string    `json:"url,omitempty"`
	Author    string    `json:"author"`
	Submolt   string    `json:"submolt,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Upvotes   int       `json:"upvotes"`
	Comments  int       `json:"comments"`
}

// Comment represents a Moltbook comment.
type Comment struct {
	ID        string    `json:"id"`
	PostID    string    `json:"post_id"`
	Content   string    `json:"content"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"created_at"`
}

// AgentProfile represents an agent's Moltbook profile.
type AgentProfile struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Followers   int    `json:"followers"`
	Following   int    `json:"following"`
	PostCount   int    `json:"post_count"`
}

// SearchResult contains Moltbook search results.
type SearchResult struct {
	Posts  []Post        `json:"posts"`
	Agents []AgentProfile `json:"agents"`
}

// CreatePostRequest is the payload for creating a new post.
type CreatePostRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	URL     string `json:"url,omitempty"`
	Submolt string `json:"submolt,omitempty"`
}

// CreateCommentRequest is the payload for creating a comment.
type CreateCommentRequest struct {
	Content string `json:"content"`
}

// MoltbookError represents an API error from Moltbook.
type MoltbookError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
}

func (e *MoltbookError) Error() string {
	return fmt.Sprintf("moltbook: HTTP %d: %s", e.StatusCode, e.Message)
}
