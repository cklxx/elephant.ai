package builtin

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"alex/internal/agent/ports"
)

func TestFlowSearchMissingAPIKey(t *testing.T) {
	tool := newFlowSearch("", nil)

	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "call-1",
		Arguments: map[string]any{"query": "latest ai research"},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil || result.Content == "" {
		t.Fatalf("expected instructional content when API key missing, got %+v", result)
	}
	if result.Error != nil {
		t.Fatalf("expected no tool error, got %v", result.Error)
	}
}

func TestFlowSearchExecutesWithAPIKey(t *testing.T) {
	var payload map[string]any
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		defer func() { _ = req.Body.Close() }()
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, err
		}
		resp := map[string]any{
			"query":  "flow writing",
			"answer": "Flow writing blends quick ideas with structure.",
			"results": []map[string]any{{
				"title":   "Flow Writing Basics",
				"url":     "https://example.com/flow",
				"content": "Flow writing combines brainstorming with rapid structuring for clarity.",
				"score":   0.8,
			}},
		}
		data, _ := json.Marshal(resp)
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader(data)),
			Header:     make(http.Header),
		}, nil
	})}

	tool := newFlowSearch("token", client)
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-2",
		Arguments: map[string]any{
			"query":       "flow writing",
			"max_results": 4,
			"reason":      "prepare flow mode guidance",
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if payload["api_key"] != "token" {
		t.Fatalf("expected api_key=token, got %v", payload["api_key"])
	}
	if payload["query"] != "flow writing" {
		t.Fatalf("expected query to propagate, got %v", payload["query"])
	}
	if payload["max_results"] != float64(4) {
		t.Fatalf("expected max_results=4, got %v", payload["max_results"])
	}
	if result == nil || result.Error != nil {
		t.Fatalf("expected successful result, got %+v", result)
	}
	if result.Metadata["results_count"] != 1 {
		t.Fatalf("expected results_count=1, got %v", result.Metadata["results_count"])
	}
	if !bytes.Contains([]byte(result.Content), []byte("Flow search")) {
		t.Fatalf("expected flow label in content, got %s", result.Content)
	}
}
