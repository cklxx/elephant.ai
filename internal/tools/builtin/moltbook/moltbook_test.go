package moltbook

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"alex/internal/agent/ports"
	moltbookclient "alex/internal/moltbook"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *moltbookclient.RateLimitedClient {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return moltbookclient.NewRateLimitedClient(moltbookclient.Config{
		BaseURL: srv.URL,
		APIKey:  "test-key",
	})
}

func TestMoltbookPost_Execute(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/posts" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"post":    moltbookclient.Post{ID: "p1", Title: "Test"},
		})
	})

	tool := NewMoltbookPost(client)

	// Verify definition.
	def := tool.Definition()
	if def.Name != "moltbook_post" {
		t.Errorf("expected name moltbook_post, got %s", def.Name)
	}

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:   "call-1",
		Name: "moltbook_post",
		Arguments: map[string]any{
			"title":   "Hello World",
			"content": "First post!",
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.CallID != "call-1" {
		t.Errorf("expected CallID call-1, got %s", result.CallID)
	}
}

func TestMoltbookPost_MissingFields(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach server")
	})

	tool := NewMoltbookPost(client)
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "call-2",
		Name:      "moltbook_post",
		Arguments: map[string]any{"title": "only title"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error == nil {
		t.Error("expected error for missing content")
	}
}

func TestMoltbookFeed_Execute(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/feed" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"posts": []moltbookclient.Post{
				{ID: "p1", Title: "First"},
				{ID: "p2", Title: "Second"},
				{ID: "p3", Title: "Third"},
			},
		})
	})

	tool := NewMoltbookFeed(client)
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:   "call-1",
		Name: "moltbook_feed",
		Arguments: map[string]any{
			"page":        float64(1),
			"max_results": float64(2),
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	// Should be truncated to max_results=2.
	var posts []moltbookclient.Post
	// The content includes a prefix line; extract JSON after the first newline.
	raw := result.Content[len("Moltbook feed (page 1, 2 posts):\n"):]
	if err := json.Unmarshal([]byte(raw), &posts); err != nil {
		t.Fatalf("parse posts: %v", err)
	}
	if len(posts) != 2 {
		t.Errorf("expected 2 posts, got %d", len(posts))
	}
}

func TestMoltbookComment_Execute(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/posts/post-42/comments" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"comment": moltbookclient.Comment{ID: "c1", PostID: "post-42"},
		})
	})

	tool := NewMoltbookComment(client)
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:   "call-1",
		Name: "moltbook_comment",
		Arguments: map[string]any{
			"post_id": "post-42",
			"content": "Great post!",
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
}

func TestMoltbookComment_MissingFields(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach server")
	})

	tool := NewMoltbookComment(client)
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "call-2",
		Name:      "moltbook_comment",
		Arguments: map[string]any{"post_id": "p1"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error == nil {
		t.Error("expected error for missing content")
	}
}

func TestMoltbookVote_Upvote(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/posts/p1/upvote" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	tool := NewMoltbookVote(client)
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:   "call-1",
		Name: "moltbook_vote",
		Arguments: map[string]any{
			"post_id":   "p1",
			"direction": "up",
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
}

func TestMoltbookVote_Downvote(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/posts/p1/downvote" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	tool := NewMoltbookVote(client)
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:   "call-1",
		Name: "moltbook_vote",
		Arguments: map[string]any{
			"post_id":   "p1",
			"direction": "down",
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
}

func TestMoltbookVote_InvalidDirection(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach server")
	})

	tool := NewMoltbookVote(client)
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:   "call-1",
		Name: "moltbook_vote",
		Arguments: map[string]any{
			"post_id":   "p1",
			"direction": "sideways",
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error == nil {
		t.Error("expected error for invalid direction")
	}
}

func TestMoltbookSearch_Execute(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/search" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("q") != "AI agents" {
			t.Errorf("unexpected query: %s", r.URL.Query().Get("q"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"posts":   []moltbookclient.Post{{ID: "s1"}},
			"agents":  []moltbookclient.AgentProfile{{Name: "bot"}},
		})
	})

	tool := NewMoltbookSearch(client)
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:   "call-1",
		Name: "moltbook_search",
		Arguments: map[string]any{
			"query": "AI agents",
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
}

func TestMoltbookSearch_MissingQuery(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach server")
	})

	tool := NewMoltbookSearch(client)
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "call-1",
		Name:      "moltbook_search",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error == nil {
		t.Error("expected error for missing query")
	}
}
