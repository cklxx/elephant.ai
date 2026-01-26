package llm

import (
	"net/http"
	"strings"
	"time"

	"alex/internal/agent/ports"
	portsllm "alex/internal/agent/ports/llm"
	"alex/internal/httpclient"
	"alex/internal/logging"
	"alex/internal/utils"
)

const defaultOpenAIResponsesBaseURL = "https://api.openai.com/v1"

type openAIResponsesClient struct {
	model         string
	apiKey        string
	baseURL       string
	httpClient    *http.Client
	logger        logging.Logger
	headers       map[string]string
	maxRetries    int
	usageCallback func(usage ports.TokenUsage, model string, provider string)
}

func NewOpenAIResponsesClient(model string, config Config) (portsllm.LLMClient, error) {
	if config.BaseURL == "" {
		config.BaseURL = defaultOpenAIResponsesBaseURL
	}

	timeout := 120 * time.Second
	if config.Timeout > 0 {
		timeout = time.Duration(config.Timeout) * time.Second
	}

	logger := utils.NewCategorizedLogger(utils.LogCategoryLLM, "openai-responses")

	return &openAIResponsesClient{
		model:      model,
		apiKey:     config.APIKey,
		baseURL:    strings.TrimRight(config.BaseURL, "/"),
		httpClient: httpclient.New(timeout, logger),
		logger:     logger,
		headers:    config.Headers,
		maxRetries: config.MaxRetries,
	}, nil
}

func (c *openAIResponsesClient) Model() string {
	return c.model
}

func (c *openAIResponsesClient) isCodexEndpoint() bool {
	return strings.Contains(c.baseURL, "/backend-api/codex")
}

func (c *openAIResponsesClient) SetUsageCallback(callback func(usage ports.TokenUsage, model string, provider string)) {
	c.usageCallback = callback
}
