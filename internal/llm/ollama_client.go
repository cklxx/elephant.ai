package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"alex/internal/agent/ports"
	"alex/internal/logging"
)

var _ ports.StreamingLLMClient = (*ollamaClient)(nil)

// ollamaClient implements both streaming and non-streaming chat completions against an Ollama server.
type ollamaClient struct {
	model      string
	baseURL    string
	httpClient *http.Client
	logger     logging.Logger
}

func NewOllamaClient(model string, config Config) (ports.LLMClient, error) {
	baseURL := strings.TrimRight(config.BaseURL, "/")
	if baseURL == "" {
		baseURL = "http://localhost:11434/api"
	}
	if !strings.HasSuffix(baseURL, "/api") {
		baseURL = baseURL + "/api"
	}

	timeout := time.Duration(config.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	client := &ollamaClient{
		model:   model,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger: logging.NewComponentLogger("ollama-client"),
	}

	return client, nil
}

func (c *ollamaClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	payload, err := c.buildRequestPayload(req, false)
	if err != nil {
		return nil, err
	}

	resp, err := c.doRequest(ctx, payload)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama request failed: %s", strings.TrimSpace(string(body)))
	}

	var response ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decode ollama response: %w", err)
	}
	if response.Error != "" {
		return nil, fmt.Errorf("ollama error: %s", response.Error)
	}

	return c.buildCompletionResponse(response, response.Message.Content), nil
}

func (c *ollamaClient) StreamComplete(
	ctx context.Context,
	req ports.CompletionRequest,
	callbacks ports.CompletionStreamCallbacks,
) (*ports.CompletionResponse, error) {
	payload, err := c.buildRequestPayload(req, true)
	if err != nil {
		return nil, err
	}

	resp, err := c.doRequest(ctx, payload)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama request failed: %s", strings.TrimSpace(string(body)))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)

	var builder strings.Builder
	var finalResponse *ports.CompletionResponse
	finalSent := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var chunk ollamaResponse
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			return nil, fmt.Errorf("decode ollama stream chunk: %w", err)
		}
		if chunk.Error != "" {
			return nil, fmt.Errorf("ollama error: %s", chunk.Error)
		}

		delta := chunk.Message.Content
		if delta != "" {
			builder.WriteString(delta)
			if cb := callbacks.OnContentDelta; cb != nil {
				cb(ports.ContentDelta{Delta: delta})
			}
		}

		if chunk.Done && !finalSent {
			finalSent = true
			if cb := callbacks.OnContentDelta; cb != nil {
				cb(ports.ContentDelta{Final: true})
			}
			finalResponse = c.buildCompletionResponse(chunk, builder.String())
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read ollama stream: %w", err)
	}

	if finalResponse == nil {
		// The stream may have ended without an explicit final chunk; synthesize a response.
		finalResponse = &ports.CompletionResponse{
			Content:    builder.String(),
			StopReason: "unknown",
		}
	}

	return finalResponse, nil
}

func (c *ollamaClient) Model() string {
	return c.model
}

func (c *ollamaClient) buildRequestPayload(req ports.CompletionRequest, stream bool) ([]byte, error) {
	request := ollamaRequest{
		Model:    c.model,
		Messages: convertOllamaMessages(req.Messages),
		Stream:   stream,
	}

	options := make(map[string]any)
	if req.Temperature > 0 {
		options["temperature"] = req.Temperature
	}
	if req.TopP > 0 {
		options["top_p"] = req.TopP
	}
	if req.MaxTokens > 0 {
		options["num_predict"] = req.MaxTokens
	}
	if len(req.StopSequences) > 0 {
		options["stop"] = append([]string(nil), req.StopSequences...)
	}
	if len(options) > 0 {
		request.Options = options
	}

	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal ollama request: %w", err)
	}
	return body, nil
}

func (c *ollamaClient) doRequest(ctx context.Context, body []byte) (*http.Response, error) {
	endpoint := fmt.Sprintf("%s/chat", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	return c.httpClient.Do(httpReq)
}

func (c *ollamaClient) buildCompletionResponse(resp ollamaResponse, content string) *ports.CompletionResponse {
	stopReason := strings.TrimSpace(resp.DoneReason)
	if stopReason == "" {
		stopReason = "stop"
	}

	return &ports.CompletionResponse{
		Content:    content,
		StopReason: stopReason,
		Usage: ports.TokenUsage{
			PromptTokens:     resp.PromptEvalCount,
			CompletionTokens: resp.EvalCount,
			TotalTokens:      resp.PromptEvalCount + resp.EvalCount,
		},
		Metadata: map[string]any{
			"model": resp.Model,
		},
	}
}

type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  map[string]any  `json:"options,omitempty"`
}

type ollamaMessage struct {
	Role    string   `json:"role"`
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"`
}

type ollamaResponse struct {
	Model           string        `json:"model"`
	Message         ollamaMessage `json:"message"`
	Done            bool          `json:"done"`
	DoneReason      string        `json:"done_reason"`
	PromptEvalCount int           `json:"prompt_eval_count"`
	EvalCount       int           `json:"eval_count"`
	Error           string        `json:"error"`
}

func convertOllamaMessages(msgs []ports.Message) []ollamaMessage {
	result := make([]ollamaMessage, 0, len(msgs))
	for _, msg := range msgs {
		if msg.Source == ports.MessageSourceDebug || msg.Source == ports.MessageSourceEvaluation {
			continue
		}
		role := strings.TrimSpace(msg.Role)
		if role == "" {
			continue
		}

		content := msg.Content

		var images []string
		if strings.EqualFold(role, "user") && len(msg.Attachments) > 0 {
			ordered := orderedImageAttachments(content, msg.Attachments)
			if len(ordered) > 0 {
				images = make([]string, 0, len(ordered))
				for _, desc := range ordered {
					if b64 := ports.AttachmentInlineBase64(desc.Attachment); b64 != "" {
						images = append(images, b64)
					}
				}
			}
		}

		if strings.TrimSpace(content) == "" && len(images) == 0 {
			continue
		}

		result = append(result, ollamaMessage{
			Role:    role,
			Content: content,
			Images:  images,
		})
	}
	return result
}
