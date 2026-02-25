package kernel

const (
	// DefaultStateRootDir is the fixed storage root for kernel markdown artifacts.
	DefaultStateRootDir = "~/.alex/kernel"
	// DefaultSeedStateContent initializes STATE.md on first boot.
	DefaultSeedStateContent = "# Kernel State\n## identity\nelephant.ai autonomous kernel — founder mindset.\nNever ask, never wait — only dispatch tasks, record state, summarize, and plan.\n## recent_actions\n(none yet)\n"

	DefaultKernelID              = "default"
	DefaultKernelSchedule        = "8,38 * * * *"
	DefaultKernelTimeoutSeconds  = 900
	DefaultKernelLeaseSeconds    = 1800
	DefaultKernelMaxConcurrent   = 3
	DefaultKernelMaxCycleHistory = 5
	DefaultKernelChannel         = "lark"
	DefaultKernelUserID          = "cklxx"

	DefaultKernelPlannerMaxDispatches = 5
	DefaultKernelPlannerTimeoutSec = 60
)

const defaultKernelOperatorPrompt = `You are an execution agent for the elephant.ai kernel. Act immediately — never ask, never wait.

Proceed based on the following state:
{STATE}

Requirements:
1. Complete at least one real tool action and produce verifiable evidence (file path / command output / search result).
2. If blocked, immediately switch to an alternative path and record the decision rationale.
3. Output an “## Execution Summary” section containing: completed items, evidence, risks, and next steps.`

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
// LLM provider/model/credentials come from the shared runtime config;
// this struct holds only planner-specific knobs.
type PlannerSettings struct {
	Enabled        bool
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
