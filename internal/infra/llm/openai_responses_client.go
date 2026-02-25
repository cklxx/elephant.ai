package llm

import (
	"strings"

	"alex/internal/domain/agent/ports"
	portsllm "alex/internal/domain/agent/ports/llm"
	"alex/internal/shared/utils"
)

const defaultOpenAIResponsesBaseURL = "https://api.openai.com/v1"

type openAIResponsesClient struct {
	baseClient
}

func NewOpenAIResponsesClient(model string, config Config) (portsllm.LLMClient, error) {
	return &openAIResponsesClient{
		baseClient: newBaseClient(model, config, baseClientOpts{
			defaultBaseURL: defaultOpenAIResponsesBaseURL,
			logCategory:    utils.LogCategoryLLM,
			logComponent:   "openai-responses",
		}),
	}, nil
}

func (c *openAIResponsesClient) isCodexEndpoint() bool {
	return strings.Contains(c.baseURL, "/backend-api/codex")
}

func (c *openAIResponsesClient) SetUsageCallback(callback func(usage ports.TokenUsage, model string, provider string)) {
	c.usageCallback = callback
}
