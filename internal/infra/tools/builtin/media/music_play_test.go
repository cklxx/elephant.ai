package media

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"alex/internal/agent/ports"
)

func TestMusicPlayUsesRequestQuery(t *testing.T) {
	var got url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		payload := itunesSearchResponse{
			ResultCount: 1,
			Results: []struct {
				TrackName      string `json:"trackName"`
				ArtistName     string `json:"artistName"`
				CollectionName string `json:"collectionName"`
				PreviewURL     string `json:"previewUrl"`
				TrackViewURL   string `json:"trackViewUrl"`
				ArtworkURL100  string `json:"artworkUrl100"`
			}{
				{
					TrackName:      "Test Song",
					ArtistName:     "Test Artist",
					CollectionName: "Test Album",
					PreviewURL:     "https://example.com/preview.mp3",
					TrackViewURL:   "https://example.com/track",
					ArtworkURL100:  "https://example.com/cover.jpg",
				},
			},
		}
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	tool := newMusicPlay(server.Client(), server.URL, MusicPlayConfig{})
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"request": "lofi beats",
			"limit":   3,
			"country": "jp",
		},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("tool error: %v", result.Error)
	}
	if got.Get("term") != "lofi beats" {
		t.Fatalf("unexpected term: %s", got.Get("term"))
	}
	if got.Get("limit") != "3" {
		t.Fatalf("unexpected limit: %s", got.Get("limit"))
	}
	if got.Get("country") != "JP" {
		t.Fatalf("unexpected country: %s", got.Get("country"))
	}
	if len(result.Attachments) == 0 {
		t.Fatalf("expected HTML attachment")
	}
}

func TestMusicPlayFallsBackToMood(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload := itunesSearchResponse{
			ResultCount: 0,
			Results:     nil,
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	tool := newMusicPlay(server.Client(), server.URL, MusicPlayConfig{})
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-2",
		Arguments: map[string]any{
			"mood": "happy",
		},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("tool error: %v", result.Error)
	}
	if result.Metadata["query"] == "" {
		t.Fatalf("expected query metadata")
	}
}
