package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultOllamaBaseURL = "http://localhost:11434"

// EmbeddingProvider generates embeddings for text.
type EmbeddingProvider interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// OllamaEmbedder implements EmbeddingProvider using Ollama's embedding API.
type OllamaEmbedder struct {
	baseURL string
	model   string
	client  *http.Client
}

// NewOllamaEmbedder constructs an embedder with the provided base URL.
func NewOllamaEmbedder(model, baseURL string) *OllamaEmbedder {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = defaultOllamaBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &OllamaEmbedder{
		baseURL: baseURL,
		model:   strings.TrimSpace(model),
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

// Embed generates embeddings for a batch of texts.
func (o *OllamaEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	if strings.TrimSpace(o.model) == "" {
		return nil, fmt.Errorf("ollama embedder requires model name")
	}
	if o.client == nil {
		o.client = &http.Client{Timeout: 60 * time.Second}
	}

	embeddings, fallback, err := o.embedBatch(ctx, texts)
	if err != nil {
		return nil, err
	}
	if !fallback {
		return embeddings, nil
	}
	return o.embedFallback(ctx, texts)
}

func (o *OllamaEmbedder) embedBatch(ctx context.Context, texts []string) ([][]float32, bool, error) {
	reqBody := map[string]any{
		"model": o.model,
		"input": texts,
	}
	status, body, err := o.postJSON(ctx, "/api/embed", reqBody)
	if err != nil {
		return nil, false, err
	}
	if status == http.StatusNotFound {
		return nil, true, nil
	}
	if status != http.StatusOK {
		return nil, false, fmt.Errorf("ollama /api/embed failed: %s", strings.TrimSpace(body))
	}
	var resp struct {
		Embeddings [][]float32 `json:"embeddings"`
		Error      string      `json:"error"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil, false, err
	}
	if resp.Error != "" {
		return nil, false, fmt.Errorf("ollama /api/embed error: %s", resp.Error)
	}
	if len(resp.Embeddings) != len(texts) {
		return nil, false, fmt.Errorf("ollama /api/embed returned %d embeddings for %d inputs", len(resp.Embeddings), len(texts))
	}
	return resp.Embeddings, false, nil
}

func (o *OllamaEmbedder) embedFallback(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, 0, len(texts))
	for _, text := range texts {
		reqBody := map[string]any{
			"model":  o.model,
			"prompt": text,
		}
		status, body, err := o.postJSON(ctx, "/api/embeddings", reqBody)
		if err != nil {
			return nil, err
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("ollama /api/embeddings failed: %s", strings.TrimSpace(body))
		}
		var resp struct {
			Embedding []float32 `json:"embedding"`
			Error     string    `json:"error"`
		}
		if err := json.Unmarshal([]byte(body), &resp); err != nil {
			return nil, err
		}
		if resp.Error != "" {
			return nil, fmt.Errorf("ollama /api/embeddings error: %s", resp.Error)
		}
		out = append(out, resp.Embedding)
	}
	return out, nil
}

func (o *OllamaEmbedder) postJSON(ctx context.Context, path string, payload any) (int, string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.client.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("ollama request failed: %w (try `ollama serve`)", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, "", err
	}
	return resp.StatusCode, string(respBody), nil
}
