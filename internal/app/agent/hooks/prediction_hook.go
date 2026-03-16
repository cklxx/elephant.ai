package hooks

import (
	"context"
	"fmt"
	"strings"
	"time"

	appcontext "alex/internal/app/agent/context"
	"alex/internal/app/agent/llmclient"
	"alex/internal/domain/agent/ports"
	portsllm "alex/internal/domain/agent/ports/llm"
	"alex/internal/infra/memory"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/logging"
	"alex/internal/shared/utils"
)

const (
	defaultPredictionTimeout   = 5 * time.Second
	defaultPredictionMaxTokens = 150
	maxPredictions             = 3
)

// PredictionConfig controls LLM-driven session prediction behavior.
type PredictionConfig struct {
	Enabled   bool
	Profile   runtimeconfig.LLMProfile
	MaxTokens int
	Timeout   time.Duration
}

// PredictionHook predicts likely next user needs after each task and records
// query category distribution.
type PredictionHook struct {
	engine       memory.Engine
	factory      portsllm.LLMClientFactory
	queryTracker *memory.QueryTracker
	config       PredictionConfig
	logger       logging.Logger
}

// NewPredictionHook constructs a PredictionHook.
func NewPredictionHook(
	engine memory.Engine,
	factory portsllm.LLMClientFactory,
	tracker *memory.QueryTracker,
	logger logging.Logger,
	config PredictionConfig,
) *PredictionHook {
	if config.Timeout <= 0 {
		config.Timeout = defaultPredictionTimeout
	}
	if config.MaxTokens <= 0 {
		config.MaxTokens = defaultPredictionMaxTokens
	}
	return &PredictionHook{
		engine:       engine,
		factory:      factory,
		queryTracker: tracker,
		config:       config,
		logger:       logging.OrNop(logger),
	}
}

func (h *PredictionHook) Name() string {
	return "prediction"
}

func (h *PredictionHook) OnTaskStart(_ context.Context, _ TaskInfo) []Injection {
	return nil
}

func (h *PredictionHook) OnTaskCompleted(ctx context.Context, result TaskResultInfo) error {
	if h == nil || h.engine == nil || h.factory == nil {
		return nil
	}
	if !h.config.Enabled {
		return nil
	}
	policy := appcontext.ResolveMemoryPolicy(ctx)
	if !policy.Enabled {
		return nil
	}
	userID := strings.TrimSpace(result.UserID)

	// Record query category (non-blocking heuristic, no LLM call).
	if h.queryTracker != nil {
		queryText := strings.TrimSpace(result.TaskInput)
		if queryText != "" {
			cat := h.queryTracker.Classify(queryText)
			if err := h.queryTracker.Record(ctx, userID, cat); err != nil {
				h.logger.Warn("Prediction hook: query tracker record failed: %v", err)
			}
		}
	}

	// Predict next session needs via LLM.
	predictions := h.predictWithLLM(ctx, result)
	if len(predictions) == 0 {
		return nil
	}
	if err := h.engine.SavePredictions(ctx, userID, predictions); err != nil {
		h.logger.Warn("Prediction hook: save predictions failed: %v", err)
		return fmt.Errorf("save predictions: %w", err)
	}
	return nil
}

func (h *PredictionHook) predictWithLLM(ctx context.Context, result TaskResultInfo) []string {
	profile, fallbackProfile, canFallback, ok := h.resolveProfile(ctx)
	if !ok {
		return nil
	}
	prompt := buildPredictionPrompt(result)
	if prompt == "" {
		return nil
	}
	req := ports.CompletionRequest{
		Messages: []ports.Message{
			{
				Role: "system",
				Content: "You predict what the user will need next session. " +
					"Output exactly 2-3 bullet points, no preamble. " +
					"Each bullet should be a specific, actionable prediction about what the user will likely ask or need next.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Temperature: 0.3,
		MaxTokens:   h.config.MaxTokens,
		Metadata: map[string]any{
			"intent": "prediction",
		},
	}

	resp, err := h.completeWithProfile(ctx, profile, req)
	if err != nil {
		if canFallback && llmclient.IsRateLimitError(err) {
			h.logger.Warn("Prediction hook pinned model rate-limited; retrying with default profile")
			if fallbackResp, fallbackErr := h.completeWithProfile(ctx, fallbackProfile, req); fallbackErr == nil {
				resp = fallbackResp
				err = nil
			}
		}
	}
	if err != nil {
		h.logger.Warn("Prediction hook LLM request failed: %v", err)
		return nil
	}
	if resp == nil || utils.IsBlank(resp.Content) {
		return nil
	}
	return parsePredictionLines(resp.Content)
}

func (h *PredictionHook) completeWithProfile(
	ctx context.Context,
	profile runtimeconfig.LLMProfile,
	req ports.CompletionRequest,
) (*ports.CompletionResponse, error) {
	client, _, err := llmclient.GetIsolatedClientFromProfile(h.factory, profile, nil, false)
	if err != nil {
		return nil, err
	}
	ctxTimeout, cancel := context.WithTimeout(ctx, h.config.Timeout)
	defer cancel()
	return client.Complete(ctxTimeout, req)
}

func (h *PredictionHook) resolveProfile(ctx context.Context) (runtimeconfig.LLMProfile, runtimeconfig.LLMProfile, bool, bool) {
	defaultProfile := h.config.Profile
	defaultOK := !utils.IsBlank(defaultProfile.Provider) && !utils.IsBlank(defaultProfile.Model)

	if selection, ok := appcontext.GetLLMSelection(ctx); ok {
		provider := strings.TrimSpace(selection.Provider)
		model := strings.TrimSpace(selection.Model)
		if provider != "" && model != "" {
			profile := runtimeconfig.LLMProfile{
				Provider: provider,
				Model:    model,
				APIKey:   strings.TrimSpace(selection.APIKey),
				BaseURL:  strings.TrimSpace(selection.BaseURL),
				Headers:  llmclient.CloneHeaders(selection.Headers),
			}
			if selection.Pinned && defaultOK && !sameProviderModel(profile, defaultProfile) {
				return profile, defaultProfile, true, true
			}
			return profile, runtimeconfig.LLMProfile{}, false, true
		}
	}

	if !defaultOK {
		return runtimeconfig.LLMProfile{}, runtimeconfig.LLMProfile{}, false, false
	}
	return defaultProfile, runtimeconfig.LLMProfile{}, false, true
}

func buildPredictionPrompt(result TaskResultInfo) string {
	if utils.IsBlank(result.TaskInput) && utils.IsBlank(result.Answer) {
		return ""
	}
	var b strings.Builder
	if task := strings.TrimSpace(result.TaskInput); task != "" {
		b.WriteString("Task just completed:\n")
		b.WriteString(ports.TruncateRuneSnippet(task, 600))
		b.WriteString("\n\n")
	}
	if answer := strings.TrimSpace(result.Answer); answer != "" {
		b.WriteString("Outcome:\n")
		b.WriteString(ports.TruncateRuneSnippet(answer, 800))
		b.WriteString("\n\n")
	}
	if len(result.ToolCalls) > 0 {
		b.WriteString("Tools used: ")
		var names []string
		for _, tc := range result.ToolCalls {
			names = append(names, tc.ToolName)
			if len(names) >= 3 {
				break
			}
		}
		b.WriteString(strings.Join(names, ", "))
		b.WriteString("\n")
	}
	b.WriteString("\nPredict 2-3 things the user will likely need next session.")
	return strings.TrimSpace(b.String())
}

func parsePredictionLines(content string) []string {
	var predictions []string
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Strip bullet prefixes.
		text := stripBulletPrefix(trimmed)
		if text == "" {
			continue
		}
		predictions = append(predictions, text)
		if len(predictions) >= maxPredictions {
			break
		}
	}
	return predictions
}
