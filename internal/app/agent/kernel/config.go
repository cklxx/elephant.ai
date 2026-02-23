package kernel

const (
	// DefaultStateRootDir is the fixed storage root for kernel markdown artifacts.
	DefaultStateRootDir = "~/.alex/kernel"
	// DefaultSeedStateContent initializes STATE.md on first boot.
	DefaultSeedStateContent = "# Kernel State\n## identity\nelephant.ai autonomous kernel — founder mindset.\n永不询问、永不等待、只派发任务、记录状态、做总结、思考规划。\n## recent_actions\n(none yet)\n"

	DefaultKernelID              = "default"
	DefaultKernelSchedule        = "0,30 * * * *"
	DefaultKernelTimeoutSeconds  = 900
	DefaultKernelLeaseSeconds    = 1800
	DefaultKernelMaxConcurrent   = 3
	DefaultKernelMaxCycleHistory = 5
	DefaultKernelChannel         = "lark"
	DefaultKernelUserID          = "cklxx"

	DefaultKernelPlannerMaxDispatches = 5
	DefaultKernelPlannerTimeoutSec    = 30
)

const defaultKernelOperatorPrompt = `你是 elephant.ai kernel 的执行代理。请直接行动，不提问、不等待。

基于以下状态推进：
{STATE}

要求：
1. 必须完成至少一个真实工具动作并产出可验证证据（文件路径/命令输出/检索结果）。
2. 遇到阻塞立即改走备选路径，并记录决策理由。
3. 输出“## 执行总结”，包含：完成事项、证据、风险与下一步。`

// RuntimeSettings defines code-owned kernel runtime behavior.
// It intentionally does not come from user YAML config.
type RuntimeSettings struct {
	KernelID        string
	Schedule        string
	SeedState       string
	TimeoutSeconds  int
	LeaseSeconds    int
	MaxConcurrent   int
	MaxCycleHistory int
	Channel         string
	ChatID          string
	UserID          string
	Planner         PlannerSettings
	Agents          []AgentConfig
}

// PlannerSettings controls built-in LLM planner defaults.
type PlannerSettings struct {
	Enabled        bool
	Provider       string
	Model          string
	APIKey         string
	BaseURL        string
	MaxDispatches  int
	GoalFile       string
	TimeoutSeconds int
}

// DefaultRuntimeSettings returns the kernel runtime defaults owned by code.
func DefaultRuntimeSettings() RuntimeSettings {
	agents := []AgentConfig{
		{
			AgentID:  "founder-operator",
			Prompt:   defaultKernelOperatorPrompt,
			Priority: 10,
			Enabled:  true,
			Metadata: map[string]string{"source": "kernel_default"},
		},
	}
	return RuntimeSettings{
		KernelID:        DefaultKernelID,
		Schedule:        DefaultKernelSchedule,
		SeedState:       DefaultSeedStateContent,
		TimeoutSeconds:  DefaultKernelTimeoutSeconds,
		LeaseSeconds:    DefaultKernelLeaseSeconds,
		MaxConcurrent:   DefaultKernelMaxConcurrent,
		MaxCycleHistory: DefaultKernelMaxCycleHistory,
		Channel:         DefaultKernelChannel,
		UserID:          DefaultKernelUserID,
		Planner: PlannerSettings{
			Enabled:        true,
			MaxDispatches:  DefaultKernelPlannerMaxDispatches,
			TimeoutSeconds: DefaultKernelPlannerTimeoutSec,
		},
		Agents: CloneAgentConfigs(agents),
	}
}

// KernelConfig holds only the fields the Engine reads at runtime.
type KernelConfig struct {
	KernelID        string
	Schedule        string
	SeedState       string
	MaxConcurrent   int
	MaxCycleHistory int // rolling history rows; default 5
	Channel         string
	ChatID          string
	UserID          string
}

// AgentConfig defines a single agent that the kernel dispatches.
type AgentConfig struct {
	AgentID  string
	Prompt   string // may contain {STATE} placeholder
	Priority int
	Enabled  bool
	Metadata map[string]string
}

// CloneAgentConfigs deep-copies agent config slices and metadata maps.
func CloneAgentConfigs(agents []AgentConfig) []AgentConfig {
	if len(agents) == 0 {
		return nil
	}
	cloned := make([]AgentConfig, 0, len(agents))
	for _, agentCfg := range agents {
		var metadata map[string]string
		if len(agentCfg.Metadata) > 0 {
			metadata = make(map[string]string, len(agentCfg.Metadata))
			for key, value := range agentCfg.Metadata {
				metadata[key] = value
			}
		}
		cloned = append(cloned, AgentConfig{
			AgentID:  agentCfg.AgentID,
			Prompt:   agentCfg.Prompt,
			Priority: agentCfg.Priority,
			Enabled:  agentCfg.Enabled,
			Metadata: metadata,
		})
	}
	return cloned
}
