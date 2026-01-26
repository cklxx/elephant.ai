package web

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"alex/internal/agent/ports"
)

func TestDouyinHotReturnsKeywords(t *testing.T) {
	t.Helper()

	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		resp := douyinHotResponse{
			StatusCode: 0,
			WordList: []struct {
				Word     string `json:"word"`
				HotValue int64  `json:"hot_value"`
				Label    int    `json:"label"`
			}{
				{Word: "小游戏", HotValue: 1200},
				{Word: "热点", HotValue: 1100},
			},
		}
		data, _ := json.Marshal(resp)
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader(data)),
			Header:     make(http.Header),
		}, nil
	})}

	tool := &douyinHot{client: client}
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "call-1",
		Arguments: map[string]any{"limit": 1},
	})
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}
	if result.Metadata == nil {
		t.Fatalf("expected metadata with keywords")
	}
	if count, ok := result.Metadata["results_count"].(int); !ok || count != 1 {
		t.Fatalf("expected results_count=1, got %v", result.Metadata["results_count"])
	}
	if !bytes.Contains([]byte(result.Content), []byte("Douyin hot keywords")) {
		t.Fatalf("expected content to include heading, got %s", result.Content)
	}
}

func TestDouyinHotHandlesEmptyResponse(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		resp := douyinHotResponse{StatusCode: 1}
		data, _ := json.Marshal(resp)
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader(data)),
			Header:     make(http.Header),
		}, nil
	})}

	tool := &douyinHot{client: client}
	result, err := tool.Execute(context.Background(), ports.ToolCall{ID: "call-2"})
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if result == nil || result.Content == "" {
		t.Fatalf("expected graceful content, got nil/empty")
	}
}
