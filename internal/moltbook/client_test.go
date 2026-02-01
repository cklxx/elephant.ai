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

func TestGetFeed(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/feed" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		page := r.URL.Query().Get("page")
		if page != "2" {
			t.Errorf("expected page=2, got %s", page)
		}

		resp := map[string]any{
			"success": true,
			"posts": []Post{
				{ID: "p1", Title: "First"},
				{ID: "p2", Title: "Second"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode response: %v", err)
		}
	})

	posts, err := client.GetFeed(context.Background(), 2)
	if err != nil {
		t.Fatalf("GetFeed: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("expected 2 posts, got %d", len(posts))
	}
	if posts[0].ID != "p1" {
		t.Errorf("expected first post ID p1, got %s", posts[0].ID)
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

func TestUpvote(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/posts/post-42/upvote" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	if err := client.Upvote(context.Background(), "post-42"); err != nil {
		t.Fatalf("Upvote: %v", err)
	}
}

func TestDownvote(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/posts/post-42/downvote" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	if err := client.Downvote(context.Background(), "post-42"); err != nil {
		t.Fatalf("Downvote: %v", err)
	}
}

func TestSearch(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/search" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		q := r.URL.Query().Get("q")
		if q != "AI agents" {
			t.Errorf("expected query 'AI agents', got %s", q)
		}

		resp := map[string]any{
			"success": true,
			"posts":   []Post{{ID: "s1", Title: "AI Agents Rock"}},
			"agents":  []AgentProfile{{Name: "agent-x"}},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode response: %v", err)
		}
	})

	result, err := client.Search(context.Background(), "AI agents")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(result.Posts) != 1 {
		t.Errorf("expected 1 post, got %d", len(result.Posts))
	}
	if len(result.Agents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(result.Agents))
	}
}

func TestGetProfile(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agents/me" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		resp := map[string]any{
			"success": true,
			"agent":   AgentProfile{Name: "elephant", PostCount: 42},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode response: %v", err)
		}
	})

	profile, err := client.GetProfile(context.Background())
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if profile.Name != "elephant" {
		t.Errorf("expected name 'elephant', got %s", profile.Name)
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

	_, err := client.GetFeed(context.Background(), 1)
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

func TestGetFeed_DefaultPage(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		if page != "1" {
			t.Errorf("expected page=1 for negative input, got %s", page)
		}
		resp := map[string]any{
			"success": true,
			"posts":   []Post{},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode response: %v", err)
		}
	})

	_, err := client.GetFeed(context.Background(), -1)
	if err != nil {
		t.Fatalf("GetFeed: %v", err)
	}
}

func TestEnvelope_SuccessFalse(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "Search failed",
		})
	})

	_, err := client.GetFeed(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error for success=false")
	}
	if got := err.Error(); got != "moltbook: API error: Search failed" {
		t.Errorf("unexpected error message: %s", got)
	}
}
