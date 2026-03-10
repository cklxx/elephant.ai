package moltbook

import (
	"fmt"
	"time"
)

// PostAuthor represents the author object embedded in a post.
type PostAuthor struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	Karma         int    `json:"karma"`
	FollowerCount int    `json:"follower_count"`
}

// PostSubmolt represents the submolt object embedded in a post.
type PostSubmolt struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

// Post represents a Moltbook post.
type Post struct {
	ID           string     `json:"id"`
	Title        string     `json:"title"`
	Content      string     `json:"content"`
	URL          string     `json:"url,omitempty"`
	Author       PostAuthor `json:"author"`
	Submolt      PostSubmolt `json:"submolt"`
	CreatedAt    time.Time  `json:"created_at"`
	Upvotes      int        `json:"upvotes"`
	Downvotes    int        `json:"downvotes"`
	CommentCount int        `json:"comment_count"`
}

// Comment represents a Moltbook comment.
type Comment struct {
	ID        string     `json:"id"`
	PostID    string     `json:"post_id"`
	Content   string     `json:"content"`
	Author    PostAuthor `json:"author"`
	CreatedAt time.Time  `json:"created_at"`
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
