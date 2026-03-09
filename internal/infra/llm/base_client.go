package llm

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"alex/internal/domain/agent/ports"
	"alex/internal/shared/httpclient"
	"alex/internal/shared/logging"
	"alex/internal/shared/utils"
	id "alex/internal/shared/utils/id"
)

// baseClient holds fields and helpers shared by HTTP-based LLM clients
// (OpenAI, Anthropic, OpenAI Responses).
type baseClient struct {
	model         string
	apiKey        string
	apiKeyMu      sync.RWMutex
	baseURL       string
	httpClient    *http.Client
	logger        logging.Logger
	headers       map[string]string
	maxRetries    int
	usageCallback func(usage ports.TokenUsage, model string, provider string)
}

// APIKeyUpdatable is implemented by LLM clients that support hot-swapping
// the API key at runtime (e.g. after an OAuth token refresh).
type APIKeyUpdatable interface {
	SetAPIKey(key string)
}

// getAPIKey returns the current API key, safe for concurrent reads.
func (c *baseClient) getAPIKey() string {
	c.apiKeyMu.RLock()
	defer c.apiKeyMu.RUnlock()
	return c.apiKey
}

// SetAPIKey atomically replaces the API key used for subsequent requests.
func (c *baseClient) SetAPIKey(key string) {
	c.apiKeyMu.Lock()
	defer c.apiKeyMu.Unlock()
	c.apiKey = key
}

// baseClientOpts configures provider-specific defaults for newBaseClient.
type baseClientOpts struct {
	defaultBaseURL string
	defaultTimeout time.Duration
	logCategory    utils.LogCategory
	logComponent   string
}

// Model returns the model name used by this client.
func (c *baseClient) Model() string {
	return c.model
}

// newBaseClient constructs the shared fields for an HTTP-based LLM client.
func newBaseClient(model string, config Config, opts baseClientOpts) baseClient {
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if baseURL == "" {
		baseURL = opts.defaultBaseURL
	}
	timeout := opts.defaultTimeout
	if timeout == 0 {
		timeout = 30 * time.Minute
	}
	if config.Timeout > 0 {
		timeout = time.Duration(config.Timeout) * time.Second
	}
	logger := utils.NewCategorizedLogger(opts.logCategory, opts.logComponent)
	return baseClient{
		model:      model,
		apiKey:     config.APIKey,
		baseURL:    baseURL,
		httpClient: httpclient.New(timeout, logger),
		logger:     logger,
		headers:    config.Headers,
		maxRetries: config.MaxRetries,
	}
}

// buildLogPrefix extracts the request ID from metadata and builds the
// structured log prefix used across all LLM request/response logging.
func (c *baseClient) buildLogPrefix(ctx context.Context, metadata map[string]any) (requestID, prefix string) {
	requestID = extractRequestID(metadata)
	if requestID == "" {
		requestID = id.NewRequestIDWithLogID(id.LogIDFromContext(ctx))
	}
	logID := id.LogIDFromContext(ctx)
	prefix = fmt.Sprintf("[req:%s] ", requestID)
	if logID != "" {
		prefix = fmt.Sprintf("[log_id=%s] %s", logID, prefix)
	}
	return requestID, prefix
}

// doPost sends an HTTP POST request with standard headers (Content-Type,
// Authorization via Bearer, X-Retry-Limit, and any custom headers).
// Caller is responsible for closing resp.Body.
func (c *baseClient) doPost(ctx context.Context, endpoint string, body []byte) (*http.Response, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if key := c.getAPIKey(); key != "" {
		httpReq.Header.Set("Authorization", "Bearer "+key)
	}
	if c.maxRetries > 0 {
		httpReq.Header.Set("X-Retry-Limit", strconv.Itoa(c.maxRetries))
	}
	for k, v := range c.headers {
		httpReq.Header.Set(k, v)
	}
	return c.httpClient.Do(httpReq)
}

// logRequestMeta logs the standard "=== LLM Request ===" header block.
func (c *baseClient) logRequestMeta(prefix, method, url string) {
	c.logger.Debug("%s=== LLM Request ===", prefix)
	c.logger.Debug("%sURL: %s %s", prefix, method, url)
	c.logger.Debug("%sModel: %s", prefix, c.model)
}

// logRequestHeaders logs request headers with authorization values masked.
func (c *baseClient) logRequestHeaders(prefix string, header http.Header) {
	c.logger.Debug("%sRequest Headers:", prefix)
	for k, v := range header {
		if strings.EqualFold(k, "Authorization") || strings.EqualFold(k, "x-api-key") {
			c.logger.Debug("%s  %s: (hidden)", prefix, k)
		} else {
			c.logger.Debug("%s  %s: %s", prefix, k, strings.Join(v, ", "))
		}
	}
}

// logResponseStatus logs the standard "=== LLM Response ===" header block.
func (c *baseClient) logResponseStatus(prefix string, resp *http.Response) {
	c.logger.Debug("%s=== LLM Response ===", prefix)
	c.logger.Debug("%sStatus: %d %s", prefix, resp.StatusCode, resp.Status)
	c.logger.Debug("%sResponse Headers:", prefix)
	for k, v := range resp.Header {
		c.logger.Debug("%s  %s: %s", prefix, k, strings.Join(v, ", "))
	}
}

// logResponseSummary logs the standard "=== LLM Response Summary ===" block.
func (c *baseClient) logResponseSummary(prefix string, result *ports.CompletionResponse) {
	c.logger.Debug("%s=== LLM Response Summary ===", prefix)
	c.logger.Debug("%sStop Reason: %s", prefix, result.StopReason)
	c.logger.Debug("%sContent Length: %d chars", prefix, len(result.Content))
	c.logger.Debug("%sTool Calls: %d", prefix, len(result.ToolCalls))
	c.logger.Debug("%sUsage: %d prompt + %d completion = %d total tokens",
		prefix,
		result.Usage.PromptTokens,
		result.Usage.CompletionTokens,
		result.Usage.TotalTokens)
	c.logger.Debug("%s==================", prefix)
}

// fireUsageCallback invokes the usage callback if configured.
func (c *baseClient) fireUsageCallback(usage ports.TokenUsage, provider string) {
	if c.usageCallback != nil {
		c.usageCallback(usage, c.model, provider)
	}
}
