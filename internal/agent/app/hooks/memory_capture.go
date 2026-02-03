package hooks

import (
	"context"
	"fmt"
	"strings"
	"time"

	appcontext "alex/internal/agent/app/context"
	"alex/internal/agent/ports"
	portsllm "alex/internal/agent/ports/llm"
	"alex/internal/logging"
	"alex/internal/memory"
)

const (
	defaultCaptureTitle     = "Task Summary"
	defaultCaptureTimeout   = 5 * time.Second
	defaultCaptureMaxTokens = 200
	maxCaptureLines         = 3
	maxPromptChars          = 2000
	maxLineChars            = 240
)

// MemoryCaptureConfig controls LLM-driven memory capture behavior.
type MemoryCaptureConfig struct {
	Enabled       bool
	Provider      string
	Model         string
	SmallProvider string
	SmallModel    string
	APIKey        string
	BaseURL       string
	MaxTokens     int
	Timeout       time.Duration
}

// MemoryCaptureHook appends compact memories after successful tasks.
type MemoryCaptureHook struct {
	engine  memory.Engine
	factory portsllm.LLMClientFactory
	config  MemoryCaptureConfig
	logger  logging.Logger
	clock   func() time.Time
}

// NewMemoryCaptureHook constructs a MemoryCaptureHook.
func NewMemoryCaptureHook(engine memory.Engine, factory portsllm.LLMClientFactory, logger logging.Logger, config MemoryCaptureConfig) *MemoryCaptureHook {
	if config.Timeout <= 0 {
		config.Timeout = defaultCaptureTimeout
	}
	if config.MaxTokens <= 0 {
		config.MaxTokens = defaultCaptureMaxTokens
	}
	return &MemoryCaptureHook{
		engine:  engine,
		factory: factory,
		config:  config,
		logger:  logging.OrNop(logger),
		clock:   time.Now,
	}
}

func (h *MemoryCaptureHook) Name() string {
	return "memory_capture"
}

func (h *MemoryCaptureHook) OnTaskStart(_ context.Context, _ TaskInfo) []Injection {
	return nil
}

func (h *MemoryCaptureHook) OnTaskCompleted(ctx context.Context, result TaskResultInfo) error {
	if h == nil || h.engine == nil || h.factory == nil {
		return nil
	}
	if !h.config.Enabled {
		return nil
	}
	policy := appcontext.ResolveMemoryPolicy(ctx)
	if !policy.Enabled || !policy.AutoCapture {
		return nil
	}
	userID := strings.TrimSpace(result.UserID)

	lines := h.captureWithLLM(ctx, result)
	if len(lines) == 0 {
		lines = h.fallbackLines(result)
	}
	if len(lines) == 0 {
		return nil
	}
	content := strings.Join(lines, "\n")
	_, err := h.engine.AppendDaily(ctx, userID, memory.DailyEntry{
		Title:     defaultCaptureTitle,
		Content:   content,
		CreatedAt: h.clock(),
	})
	if err != nil {
		h.logger.Warn("Memory capture append failed: %v", err)
	}
	return nil
}

func (h *MemoryCaptureHook) captureWithLLM(ctx context.Context, result TaskResultInfo) []string {
	provider, model := h.selectModel()
	if provider == "" || model == "" {
		return nil
	}
	client, err := h.factory.GetIsolatedClient(provider, model, portsllm.LLMConfig{
		APIKey:  h.config.APIKey,
		BaseURL: h.config.BaseURL,
	})
	if err != nil {
		h.logger.Warn("Memory capture LLM client init failed: %v", err)
		return nil
	}
	prompt := buildMemoryCapturePrompt(result)
	if prompt == "" {
		return nil
	}
	ctxTimeout, cancel := context.WithTimeout(ctx, h.config.Timeout)
	defer cancel()
	resp, err := client.Complete(ctxTimeout, ports.CompletionRequest{
		Messages: []ports.Message{
			{
				Role: "system",
				Content: "You extract durable memory facts. Output 1-3 bullet points only, no preamble. " +
					"Each bullet should be a short, reusable fact, decision, preference, or constraint that matters later.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Temperature: 0.2,
		MaxTokens:   h.config.MaxTokens,
		Metadata: map[string]any{
			"intent": "memory_capture",
		},
	})
	if err != nil {
		h.logger.Warn("Memory capture LLM request failed: %v", err)
		return nil
	}
	if resp == nil || strings.TrimSpace(resp.Content) == "" {
		return nil
	}
	return normalizeMemoryLines(resp.Content)
}

func (h *MemoryCaptureHook) selectModel() (string, string) {
	model := strings.TrimSpace(h.config.SmallModel)
	provider := strings.TrimSpace(h.config.SmallProvider)
	if model == "" {
		model = strings.TrimSpace(h.config.Model)
		provider = strings.TrimSpace(h.config.Provider)
	}
	if provider == "" {
		provider = strings.TrimSpace(h.config.Provider)
	}
	return provider, model
}

func buildMemoryCapturePrompt(result TaskResultInfo) string {
	if strings.TrimSpace(result.TaskInput) == "" && strings.TrimSpace(result.Answer) == "" && len(result.ToolCalls) == 0 {
		return ""
	}
	var b strings.Builder
	appendField(&b, "Task", result.TaskInput, 600)
	appendField(&b, "Answer", result.Answer, 800)
	if len(result.ToolCalls) > 0 {
		var toolLines []string
		for _, tool := range result.ToolCalls {
			out := truncateText(tool.Output, 200)
			status := "ok"
			if !tool.Success {
				status = "fail"
			}
			if out != "" {
				toolLines = append(toolLines, fmt.Sprintf("%s (%s): %s", tool.ToolName, status, out))
			} else {
				toolLines = append(toolLines, fmt.Sprintf("%s (%s)", tool.ToolName, status))
			}
			if len(toolLines) >= 3 {
				break
			}
		}
		appendField(&b, "Tools", strings.Join(toolLines, "\n"), 600)
	}
	prompt := strings.TrimSpace(b.String())
	if len(prompt) > maxPromptChars {
		prompt = truncateText(prompt, maxPromptChars)
	}
	return prompt
}

func appendField(b *strings.Builder, label, value string, limit int) {
	value = truncateText(value, limit)
	if value == "" {
		return
	}
	b.WriteString(label)
	b.WriteString(":\n")
	b.WriteString(value)
	b.WriteString("\n\n")
}

func truncateText(value string, limit int) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || limit <= 0 {
		return ""
	}
	runes := []rune(trimmed)
	if len(runes) <= limit {
		return trimmed
	}
	return strings.TrimSpace(string(runes[:limit])) + "..."
}

func normalizeMemoryLines(content string) []string {
	lines := strings.Split(content, "\n")
	var out []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line = stripBulletPrefix(line)
		if line == "" {
			continue
		}
		line = truncateText(line, maxLineChars)
		out = append(out, "- "+line)
		if len(out) >= maxCaptureLines {
			break
		}
	}
	return out
}

func stripBulletPrefix(line string) string {
	if line == "" {
		return ""
	}
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "*") {
		return strings.TrimSpace(strings.TrimLeft(trimmed, "-*"))
	}
	if strings.HasPrefix(trimmed, "•") {
		return strings.TrimSpace(strings.TrimPrefix(trimmed, "•"))
	}
	for i, r := range trimmed {
		if r < '0' || r > '9' {
			if i == 0 {
				return trimmed
			}
			if r == '.' || r == ')' {
				return strings.TrimSpace(trimmed[i+1:])
			}
			return trimmed
		}
	}
	return trimmed
}

func (h *MemoryCaptureHook) fallbackLines(result TaskResultInfo) []string {
	var lines []string
	if task := truncateText(result.TaskInput, 200); task != "" {
		lines = append(lines, "- Task: "+task)
	}
	if answer := truncateText(result.Answer, 260); answer != "" {
		lines = append(lines, "- Outcome: "+answer)
	}
	if len(result.ToolCalls) > 0 {
		var toolNames []string
		for _, tool := range result.ToolCalls {
			status := "ok"
			if !tool.Success {
				status = "fail"
			}
			toolNames = append(toolNames, fmt.Sprintf("%s(%s)", tool.ToolName, status))
			if len(toolNames) >= 3 {
				break
			}
		}
		if len(toolNames) > 0 {
			lines = append(lines, "- Tools: "+strings.Join(toolNames, ", "))
		}
	}
	if len(lines) > maxCaptureLines {
		lines = lines[:maxCaptureLines]
	}
	return lines
}
