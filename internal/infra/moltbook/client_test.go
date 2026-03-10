package moltbook

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := newClient(Config{BaseURL: srv.URL, APIKey: "test-key"}, srv.Client())
	return srv, client
}

func TestCreatePost(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/posts" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-key" {
			t.Errorf("unexpected auth header: %s", auth)
		}

		var req CreatePostRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Title == "" {
			t.Error("expected non-empty title")
		}

		resp := map[string]any{
			"success": true,
			"message": "Post created",
			"post": Post{
				ID:        "post-1",
				Title:     req.Title,
				Content:   req.Content,
				Author:    PostAuthor{ID: "a1", Name: "test-agent"},
				CreatedAt: time.Now(),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode response: %v", err)
		}
	})

	post, err := client.CreatePost(context.Background(), CreatePostRequest{
		Title:   "Test Post",
		Content: "Hello Moltbook",
	})
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}
	if post.ID != "post-1" {
		t.Errorf("expected post ID post-1, got %s", post.ID)
	}
	if post.Title != "Test Post" {
		t.Errorf("expected title 'Test Post', got %s", post.Title)
	}
}

func TestCreateComment(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/posts/post-42/comments" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		resp := map[string]any{
			"success": true,
			"comment": Comment{
				ID:      "c1",
				PostID:  "post-42",
				Content: "Nice post!",
				Author:  PostAuthor{ID: "a1", Name: "test-agent"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode response: %v", err)
		}
	})

	comment, err := client.CreateComment(context.Background(), "post-42", CreateCommentRequest{
		Content: "Nice post!",
	})
	if err != nil {
		t.Fatalf("CreateComment: %v", err)
	}
	if comment.ID != "c1" {
		t.Errorf("expected comment ID c1, got %s", comment.ID)
	}
}

func TestAPIError(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		if err := json.NewEncoder(w).Encode(map[string]string{"message": "invalid api key"}); err != nil {
			t.Errorf("encode response: %v", err)
		}
	})

	_, err := client.CreatePost(context.Background(), CreatePostRequest{Title: "t", Content: "c"})
	if err == nil {
		t.Fatal("expected error")
	}
	mErr, ok := err.(*MoltbookError)
	if !ok {
		t.Fatalf("expected *MoltbookError, got %T", err)
	}
	if mErr.StatusCode != 403 {
		t.Errorf("expected status 403, got %d", mErr.StatusCode)
	}
	if mErr.Message != "invalid api key" {
		t.Errorf("expected message 'invalid api key', got %s", mErr.Message)
	}
}

func TestRateLimitedClient_PostCooldown(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		resp := map[string]any{
			"success": true,
			"post":    Post{ID: "p1", Title: "ok"},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	inner := newClient(Config{BaseURL: srv.URL, APIKey: "k"}, srv.Client())
	rl := newRateLimitedClient(Config{}, inner)

	// First post should succeed.
	_, err := rl.CreatePost(context.Background(), CreatePostRequest{Title: "first", Content: "c"})
	if err != nil {
		t.Fatalf("first post: %v", err)
	}

	// Second post immediately should be rate limited (no HTTP call).
	_, err = rl.CreatePost(context.Background(), CreatePostRequest{Title: "second", Content: "c"})
	if err == nil {
		t.Fatal("expected rate limit error on second post")
	}
	if calls.Load() != 1 {
		t.Errorf("expected 1 HTTP call, got %d", calls.Load())
	}
}

func TestRateLimitedClient_CommentCooldown(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		resp := map[string]any{
			"success": true,
			"comment": Comment{ID: "c1", PostID: "p1"},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	inner := newClient(Config{BaseURL: srv.URL, APIKey: "k"}, srv.Client())
	rl := newRateLimitedClient(Config{}, inner)

	// First comment should succeed.
	_, err := rl.CreateComment(context.Background(), "p1", CreateCommentRequest{Content: "hi"})
	if err != nil {
		t.Fatalf("first comment: %v", err)
	}

	// Second comment immediately should be rate limited.
	_, err = rl.CreateComment(context.Background(), "p1", CreateCommentRequest{Content: "hi again"})
	if err == nil {
		t.Fatal("expected rate limit error on second comment")
	}
	if calls.Load() != 1 {
		t.Errorf("expected 1 HTTP call, got %d", calls.Load())
	}
}

func TestEnvelope_SuccessFalse(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "Search failed",
		}); err != nil {
			t.Errorf("encode response: %v", err)
		}
	})

	_, err := client.CreatePost(context.Background(), CreatePostRequest{Title: "t", Content: "c"})
	if err == nil {
		t.Fatal("expected error for success=false")
	}
	if got := err.Error(); got != "moltbook: API error: Search failed" {
		t.Errorf("unexpected error message: %s", got)
	}
}
