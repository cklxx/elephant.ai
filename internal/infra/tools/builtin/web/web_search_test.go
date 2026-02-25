package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"alex/internal/domain/agent/ports"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestWebSearchMissingAPIKey(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		html := `<div class="result"><a class="result__a" href="https://example.com">Example</a><a class="result__snippet">Snippet</a></div>`
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(html)),
			Header:     make(http.Header),
		}, nil
	})}
	tool := newWebSearch("", client, WebSearchConfig{})

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "call-1",
		Arguments: map[string]any{"query": "test"},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result when API key missing")
	}
	if result.Error != nil {
		t.Fatalf("expected no tool error, got %v", result.Error)
	}
	if result.Metadata == nil {
		t.Fatalf("expected metadata, got nil")
	}
	if result.Metadata["source"] != "duckduckgo" {
		t.Fatalf("expected fallback source, got %v", result.Metadata["source"])
	}
	if !bytes.Contains([]byte(result.Content), []byte("Search (fallback): test")) {
		t.Fatalf("expected fallback content to include query, got %s", result.Content)
	}
}

func TestWebSearchExecutesWithAPIKey(t *testing.T) {
	t.Helper()

	var requestPayload map[string]any
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://api.tavily.com/search" {
			return nil, fmt.Errorf("unexpected URL: %s", req.URL.String())
		}
		defer func() { _ = req.Body.Close() }()
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(body, &requestPayload); err != nil {
			return nil, err
		}
		response := map[string]any{
			"query":  "golang",
			"answer": "Go is a programming language",
			"results": []map[string]any{{
				"title":   "Go Programming Language",
				"url":     "https://go.dev",
				"content": "Go is expressive, concise, clean, and efficient.",
				"score":   0.9,
			}},
		}
		data, err := json.Marshal(response)
		if err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader(data)),
			Header:     make(http.Header),
		}, nil
	})}

	tool := newWebSearch("token", client, WebSearchConfig{})
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-2",
		Arguments: map[string]any{
			"query": "golang",
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if requestPayload == nil {
		t.Fatal("expected request payload to be captured")
	}
	if requestPayload["api_key"] != "token" {
		t.Fatalf("expected api_key=token, got %v", requestPayload["api_key"])
	}
	if requestPayload["query"] != "golang" {
		t.Fatalf("expected query=golang, got %v", requestPayload["query"])
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}
	if result.Metadata == nil {
		t.Fatalf("expected metadata, got nil")
	}
	count, ok := result.Metadata["results_count"].(int)
	if !ok || count != 1 {
		t.Fatalf("expected results_count=1, got %+v", result.Metadata["results_count"])
	}
	if result.Metadata["source"] != "tavily" {
		t.Fatalf("expected tavily source, got %+v", result.Metadata["source"])
	}
	if !bytes.Contains([]byte(result.Content), []byte("Search: golang")) {
		t.Fatalf("expected content to include query, got %s", result.Content)
	}
}
