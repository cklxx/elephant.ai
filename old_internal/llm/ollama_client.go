package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// OllamaClient implements the Client interface for Ollama API
type OllamaClient struct {
	httpClient       *http.Client
	baseURL          string
	cacheManager     *CacheManager
	enableUltraThink bool
}

// OllamaRequest represents a request to Ollama API
type OllamaRequest struct {
	Model    string          `json:"model"`
	Messages []OllamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  OllamaOptions   `json:"options,omitempty"`
}

// OllamaMessage represents a message in Ollama format
type OllamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OllamaOptions represents Ollama-specific options
type OllamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
	TopK        int     `json:"top_k,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
}

// OllamaResponse represents a response from Ollama API
type OllamaResponse struct {
	Model              string        `json:"model"`
	CreatedAt          string        `json:"created_at"`
	Message            OllamaMessage `json:"message"`
	Done               bool          `json:"done"`
	TotalDuration      int64         `json:"total_duration,omitempty"`
	LoadDuration       int64         `json:"load_duration,omitempty"`
	PromptEvalCount    int           `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64         `json:"prompt_eval_duration,omitempty"`
	EvalCount          int           `json:"eval_count,omitempty"`
	EvalDuration       int64         `json:"eval_duration,omitempty"`
}

// OllamaStreamResponse represents a streaming response chunk
type OllamaStreamResponse struct {
	Model     string        `json:"model"`
	CreatedAt string        `json:"created_at"`
	Message   OllamaMessage `json:"message"`
	Done      bool          `json:"done"`
}

// NewOllamaClient creates a new Ollama API client
func NewOllamaClient(baseURL string, enableUltraThink bool) (*OllamaClient, error) {
	if baseURL == "" {
		baseURL = "http://localhost:11434" // Default Ollama server
	}

	client := &OllamaClient{
		httpClient: &http.Client{
			Timeout: 5 * time.Minute, // Longer timeout for local models
		},
		baseURL:          baseURL,
		cacheManager:     GetGlobalCacheManager(),
		enableUltraThink: enableUltraThink,
	}

	// Test connection
	if err := client.testConnection(); err != nil {
		return nil, fmt.Errorf("failed to connect to Ollama server at %s: %w", baseURL, err)
	}

	return client, nil
}

// testConnection tests if Ollama server is reachable
func (c *OllamaClient) testConnection() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/tags", nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Error closing response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	return nil
}

// Chat sends a chat request to Ollama and returns the response
func (c *OllamaClient) Chat(ctx context.Context, req *ChatRequest, sessionID string) (*ChatResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	// Apply ultra think if enabled
	messages := req.Messages
	if c.enableUltraThink && len(messages) > 0 {
		messages = c.applyUltraThink(messages)
	}

	// Optimize messages using cache
	if sessionID != "" {
		messages = c.cacheManager.GetOptimizedMessages(sessionID, messages)
	}

	// Convert to Ollama format
	ollamaReq := &OllamaRequest{
		Model:    req.Model,
		Messages: c.convertToOllamaMessages(messages),
		Stream:   false,
		Options: OllamaOptions{
			Temperature: req.Temperature,
			NumPredict:  req.MaxTokens,
		},
	}

	// Default model if not specified
	if ollamaReq.Model == "" {
		ollamaReq.Model = "llama3.2" // Default local model
	}

	jsonData, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/chat", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Error closing response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama API error %d: %s", resp.StatusCode, string(body))
	}

	// Read streaming response (Ollama always streams)
	var fullContent strings.Builder
	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		var streamResp OllamaStreamResponse
		if err := json.Unmarshal(scanner.Bytes(), &streamResp); err != nil {
			continue // Skip invalid JSON lines
		}

		fullContent.WriteString(streamResp.Message.Content)

		if streamResp.Done {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	// Convert to standard ChatResponse
	chatResp := &ChatResponse{
		ID:      fmt.Sprintf("ollama-%d", time.Now().Unix()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   ollamaReq.Model,
		Choices: []Choice{
			{
				Index: 0,
				Message: Message{
					Role:    "assistant",
					Content: fullContent.String(),
				},
				FinishReason: "stop",
			},
		},
		Usage: Usage{
			PromptTokens:     len(req.Messages) * 100,       // Rough estimate
			CompletionTokens: len(fullContent.String()) / 4, // Rough estimate
			TotalTokens:      0,                             // Will be calculated
		},
	}

	chatResp.Usage.TotalTokens = chatResp.Usage.PromptTokens + chatResp.Usage.CompletionTokens

	// Update cache
	if sessionID != "" && len(chatResp.Choices) > 0 {
		newMessages := []Message{chatResp.Choices[0].Message}
		c.cacheManager.UpdateCache(sessionID, newMessages, chatResp.Usage.TotalTokens)
	}

	return chatResp, nil
}

// ChatStream sends a streaming chat request to Ollama
func (c *OllamaClient) ChatStream(ctx context.Context, req *ChatRequest, sessionID string) (<-chan StreamDelta, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	// Apply ultra think if enabled
	messages := req.Messages
	if c.enableUltraThink && len(messages) > 0 {
		messages = c.applyUltraThink(messages)
	}

	// Optimize messages using cache
	if sessionID != "" {
		messages = c.cacheManager.GetOptimizedMessages(sessionID, messages)
	}

	// Convert to Ollama format
	ollamaReq := &OllamaRequest{
		Model:    req.Model,
		Messages: c.convertToOllamaMessages(messages),
		Stream:   true,
		Options: OllamaOptions{
			Temperature: req.Temperature,
			NumPredict:  req.MaxTokens,
		},
	}

	// Default model if not specified
	if ollamaReq.Model == "" {
		ollamaReq.Model = "llama3.2"
	}

	jsonData, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/chat", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if err := resp.Body.Close(); err != nil {
			log.Printf("Error closing response body: %v", err)
		}
		return nil, fmt.Errorf("ollama API error %d: %s", resp.StatusCode, string(body))
	}

	// Create streaming channel
	deltaChan := make(chan StreamDelta, 100)

	go func() {
		defer close(deltaChan)
		defer func() {
			if err := resp.Body.Close(); err != nil {
				log.Printf("Error closing response body: %v", err)
			}
		}()

		scanner := bufio.NewScanner(resp.Body)
		var fullContent strings.Builder

		for scanner.Scan() {
			var streamResp OllamaStreamResponse
			if err := json.Unmarshal(scanner.Bytes(), &streamResp); err != nil {
				continue
			}

			fullContent.WriteString(streamResp.Message.Content)

			// Convert to StreamDelta
			delta := StreamDelta{
				ID:      fmt.Sprintf("ollama-%d", time.Now().UnixNano()),
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Model:   ollamaReq.Model,
				Choices: []Choice{
					{
						Index: 0,
						Delta: Message{
							Content: streamResp.Message.Content,
						},
					},
				},
			}

			select {
			case deltaChan <- delta:
			case <-ctx.Done():
				return
			}

			if streamResp.Done {
				// Update cache with complete message
				if sessionID != "" {
					completeMsg := Message{
						Role:    "assistant",
						Content: fullContent.String(),
					}
					c.cacheManager.UpdateCache(sessionID, []Message{completeMsg}, len(fullContent.String())/4)
				}
				break
			}
		}

		if err := scanner.Err(); err != nil {
			log.Printf("Error reading stream: %v", err)
		}
	}()

	return deltaChan, nil
}

// convertToOllamaMessages converts standard messages to Ollama format
func (c *OllamaClient) convertToOllamaMessages(messages []Message) []OllamaMessage {
	ollamaMessages := make([]OllamaMessage, 0, len(messages))

	for _, msg := range messages {
		// Handle tool calls by converting to text
		content := msg.Content
		if len(msg.ToolCalls) > 0 {
			content += "\n[Tool Calls]:\n"
			for _, tc := range msg.ToolCalls {
				content += fmt.Sprintf("- %s(%s)\n", tc.Function.Name, tc.Function.Arguments)
			}
		}

		ollamaMessages = append(ollamaMessages, OllamaMessage{
			Role:    msg.Role,
			Content: content,
		})
	}

	return ollamaMessages
}

// applyUltraThink applies ultra thinking enhancement to messages
func (c *OllamaClient) applyUltraThink(messages []Message) []Message {
	if len(messages) == 0 {
		return messages
	}

	// Get the last user message
	lastUserIdx := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			lastUserIdx = i
			break
		}
	}

	if lastUserIdx == -1 {
		return messages
	}

	// Create enhanced messages with ultra think prompt
	enhanced := make([]Message, len(messages))
	copy(enhanced, messages)

	// Add ultra think enhancement to the last user message
	ultraThinkPrompt := `
[ULTRA THINK MODE ACTIVATED]
Please engage in deep, comprehensive reasoning about this request:

1. **Problem Analysis**: Break down the request into its core components
2. **Solution Exploration**: Consider multiple approaches and their trade-offs
3. **Implementation Strategy**: Think through the detailed steps needed
4. **Edge Cases & Error Handling**: Identify potential issues and solutions
5. **Optimization Opportunities**: Consider performance and quality improvements
6. **Verification Strategy**: Plan how to test and validate the solution

After thorough thinking, provide your response with the best approach.

Original Request:
`

	enhanced[lastUserIdx].Content = ultraThinkPrompt + enhanced[lastUserIdx].Content

	// Add a system message to reinforce ultra thinking
	if len(enhanced) > 0 && enhanced[0].Role != "system" {
		systemMsg := Message{
			Role: "system",
			Content: `You are an AI assistant with ULTRA THINK capabilities. 
When processing requests, engage in deep, systematic reasoning before responding. 
Consider multiple perspectives, identify edge cases, and provide thorough, well-reasoned solutions.
Show your thinking process when it adds value to the response.`,
		}
		enhanced = append([]Message{systemMsg}, enhanced...)
	}

	return enhanced
}

// EnableUltraThink enables or disables ultra think mode
func (c *OllamaClient) EnableUltraThink(enable bool) {
	c.enableUltraThink = enable
	if enable {
		log.Println("[OLLAMA] Ultra Think mode ENABLED - Enhanced reasoning activated")
	} else {
		log.Println("[OLLAMA] Ultra Think mode DISABLED")
	}
}

// Close closes the client
func (c *OllamaClient) Close() error {
	// No persistent connections to close
	return nil
}

// GetAvailableModels returns list of available Ollama models
func (c *OllamaClient) GetAvailableModels() ([]string, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/tags", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Error closing response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get models: status %d", resp.StatusCode)
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	models := make([]string, len(result.Models))
	for i, m := range result.Models {
		models[i] = m.Name
	}

	return models, nil
}

// IsOllamaAPI checks if the base URL is for Ollama
func IsOllamaAPI(baseURL string) bool {
	return strings.Contains(baseURL, "localhost:11434") ||
		strings.Contains(baseURL, "127.0.0.1:11434") ||
		strings.Contains(baseURL, "ollama")
}
