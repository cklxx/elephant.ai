package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

// EmbedderConfig holds embedding configuration
type EmbedderConfig struct {
	Provider  string // "openai"
	Model     string // "text-embedding-3-small"
	APIKey    string
	BaseURL   string // Optional, defaults to OpenAI
	CacheSize int    // LRU cache size, default 10000
}

// Embedder generates text embeddings
type Embedder interface {
	// Embed generates embedding for a single text
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch generates embeddings for multiple texts (up to 100)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

	// Dimensions returns the embedding dimension
	Dimensions() int
}

// openaiEmbedder implements Embedder using OpenAI API
type openaiEmbedder struct {
	config     EmbedderConfig
	httpClient *http.Client
	cache      *lru.Cache[string, []float32]
}

// NewEmbedder creates a new embedder
func NewEmbedder(config EmbedderConfig) (Embedder, error) {
	if config.Model == "" {
		config.Model = "text-embedding-3-small"
	}
	if config.BaseURL == "" {
		config.BaseURL = "https://api.openai.com/v1"
	}
	if config.CacheSize == 0 {
		config.CacheSize = 10000
	}

	cache, err := lru.New[string, []float32](config.CacheSize)
	if err != nil {
		return nil, fmt.Errorf("create cache: %w", err)
	}

	return &openaiEmbedder{
		config: config,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		cache: cache,
	}, nil
}

// Embed generates embedding for a single text
func (e *openaiEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// Check cache first
	if cached, ok := e.cache.Get(text); ok {
		return cached, nil
	}

	// Generate embedding
	embeddings, err := e.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}

	return embeddings[0], nil
}

// EmbedBatch generates embeddings for multiple texts
func (e *openaiEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("no texts provided")
	}
	if len(texts) > 100 {
		return nil, fmt.Errorf("batch size exceeds limit: %d > 100", len(texts))
	}

	// Check cache and collect uncached texts
	results := make([][]float32, len(texts))
	uncachedIndices := []int{}
	uncachedTexts := []string{}

	for i, text := range texts {
		if cached, ok := e.cache.Get(text); ok {
			results[i] = cached
		} else {
			uncachedIndices = append(uncachedIndices, i)
			uncachedTexts = append(uncachedTexts, text)
		}
	}

	// If all cached, return immediately
	if len(uncachedTexts) == 0 {
		return results, nil
	}

	// Call API with exponential backoff
	var embeddings [][]float32
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		embeddings, err = e.callAPI(ctx, uncachedTexts)
		if err == nil {
			break
		}

		// Exponential backoff
		if attempt < 2 {
			backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			time.Sleep(backoff)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("embed batch after retries: %w", err)
	}

	// Cache and populate results
	for i, idx := range uncachedIndices {
		e.cache.Add(texts[idx], embeddings[i])
		results[idx] = embeddings[i]
	}

	return results, nil
}

// Dimensions returns embedding dimension (1536 for text-embedding-3-small)
func (e *openaiEmbedder) Dimensions() int {
	// text-embedding-3-small: 1536 dimensions
	return 1536
}

// callAPI calls OpenAI Embeddings API
func (e *openaiEmbedder) callAPI(ctx context.Context, texts []string) ([][]float32, error) {
	reqBody := map[string]any{
		"model": e.config.Model,
		"input": texts,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.config.BaseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.config.APIKey)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var apiResp struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Sort by index and extract embeddings
	embeddings := make([][]float32, len(texts))
	for _, item := range apiResp.Data {
		if item.Index >= len(embeddings) {
			return nil, fmt.Errorf("invalid index: %d", item.Index)
		}
		embeddings[item.Index] = item.Embedding
	}

	return embeddings, nil
}
