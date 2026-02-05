package admin

import (
	"strings"

	runtimeconfig "alex/internal/config"
)

// TaskSeverity indicates the urgency of a readiness task.
type TaskSeverity string

const (
	// TaskSeverityCritical represents a blocking configuration gap.
	TaskSeverityCritical TaskSeverity = "critical"
	// TaskSeverityWarning represents a recommended but non-blocking item.
	TaskSeverityWarning TaskSeverity = "warning"
)

// ReadinessTask captures outstanding configuration work surfaced to operators.
type ReadinessTask struct {
	ID       string       `json:"id"`
	Label    string       `json:"label"`
	Hint     string       `json:"hint,omitempty"`
	Severity TaskSeverity `json:"severity"`
}

// DeriveReadinessTasks inspects the effective runtime configuration and determines
// which critical values are still missing. This keeps the UI and operators aligned
// on exactly what is left before the system is production ready.
func DeriveReadinessTasks(cfg runtimeconfig.RuntimeConfig) []ReadinessTask {
	var tasks []ReadinessTask

	provider := strings.TrimSpace(cfg.LLMProvider)
	model := strings.TrimSpace(cfg.LLMModel)
	apiKey := strings.TrimSpace(cfg.APIKey)
	tavilyKey := strings.TrimSpace(cfg.TavilyAPIKey)

	providerLower := strings.ToLower(strings.TrimSpace(provider))
	providerNeedsKey := providerLower != "" && providerLower != "mock" && providerLower != "ollama" && providerLower != "llama.cpp" && providerLower != "llamacpp" && providerLower != "llama-cpp"

	if provider == "" {
		tasks = append(tasks, ReadinessTask{
			ID:       "llm-provider",
			Label:    "选择默认的 LLM 提供方",
			Hint:     "此项会影响所有任务的推理入口，请在保存前确保已确定供应商。",
			Severity: TaskSeverityCritical,
		})
	}

	if model == "" {
		tasks = append(tasks, ReadinessTask{
			ID:       "llm-model",
			Label:    "设置默认推理模型",
			Hint:     "例如 deepseek/deepseek-chat、gpt-4.1 等模型名称。",
			Severity: TaskSeverityCritical,
		})
	}

	if providerNeedsKey && apiKey == "" {
		tasks = append(tasks, ReadinessTask{
			ID:       "llm-api-key",
			Label:    "提供对应的 API Key",
			Hint:     "未配置密钥时所有请求都会失败，可以暂时切换为 mock/ollama/llama.cpp 以继续调试。",
			Severity: TaskSeverityCritical,
		})
	}

	if tavilyKey == "" {
		tasks = append(tasks, ReadinessTask{
			ID:       "tavily-key",
			Label:    "设置 Tavily API Key",
			Hint:     "缺少该密钥时外部搜索/检索能力会受限，可在 Tavily 控制台申请。",
			Severity: TaskSeverityWarning,
		})
	}

	return tasks
}
