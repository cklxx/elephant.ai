package kernel

const (
	// DefaultStateRootDir is the fixed storage root for kernel markdown artifacts.
	DefaultStateRootDir = "~/.alex/kernel"
	// DefaultSeedStateContent initializes STATE.md on first boot.
	DefaultSeedStateContent = "# Kernel State\n## status\n- last_cycle: bootstrap | unknown | duration=unknown\n- health: green\n- blocked_on: none\n\n## active_work\n- [ ] (none)\n\n## completed_last_24h\n- [x] bootstrap initialized | completed=unknown | artifact=STATE.md\n\n## next_priority\n1. Run one autonomous kernel iteration and write at least one artifact under ./artifacts/\n\n## blocked\n(none)\n"

	DefaultKernelID               = "default"
	DefaultKernelSchedule         = "8,38 * * * *"
	DefaultKernelTimeoutSeconds   = 900
	DefaultKernelLeaseSeconds     = 1800
	DefaultKernelRetentionSeconds = 1209600
	DefaultKernelMaxConcurrent    = 3
	DefaultKernelMaxCycleHistory  = 5
	DefaultKernelChannel          = "lark"
	DefaultKernelUserID           = "cklxx"

	DefaultKernelPlannerMaxDispatches = 5
	DefaultKernelPlannerTimeoutSec    = 60
	DefaultKernelTeamTimeoutSeconds   = 900
	DefaultKernelMaxTeamsPerCycle     = 1
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
	KernelID         string
	Schedule         string
	SeedState        string
	TimeoutSeconds   int
	LeaseSeconds     int
	RetentionSeconds int
	MaxConcurrent    int
	MaxCycleHistory  int
	Channel          string
	ChatID           string
	UserID           string
	Planner          PlannerSettings
	Agents           []AgentConfig
}

// PlannerSettings controls built-in LLM planner defaults.
// LLM provider/model/credentials come from the shared runtime config;
// this struct holds only planner-specific knobs.
type PlannerSettings struct {
	Enabled             bool
	MaxDispatches       int
	GoalFile            string
	TimeoutSeconds      int
	TeamDispatchEnabled bool
	MaxTeamsPerCycle    int
	TeamTimeoutSeconds  int
}

// DefaultRuntimeSettings returns the kernel runtime defaults owned by code.
func DefaultRuntimeSettings() RuntimeSettings {
	agents := []AgentConfig{
		{
			AgentID:        "founder-operator",
			Prompt:         defaultKernelOperatorPrompt,
			Priority:       10,
			Enabled:        true,
			TimeoutSeconds: 600, // capped at 10 min; high-level orchestration should be fast
			Metadata:       map[string]string{"source": "kernel_default"},
		},
		{
			AgentID:        "build-executor",
			Prompt:         "Execute kernel build/implementation tasks with deterministic verification.\n\nProceed based on the following state:\n{STATE}",
			Priority:       9,
			Enabled:        true,
			TimeoutSeconds: 840, // build + test can be slow; allow up to 14 min
			Metadata:       map[string]string{"source": "kernel_default", "bucket": "build"},
		},
		{
			AgentID:         "research-executor",
			Prompt:          "Execute kernel research/investigation tasks and convert findings into actionable artifacts.\n\nProceed based on the following state:\n{STATE}",
			Priority:        8,
			Enabled:         true,
			CooldownMinutes: 30,
			TimeoutSeconds:  600, // research is I/O bound; 10 min is ample
			Metadata:        map[string]string{"source": "kernel_default", "bucket": "research"},
		},
		{
			AgentID:        "outreach-executor",
			Prompt:         "Execute kernel communication/outreach tasks that unblock delivery and state sync.\n\nProceed based on the following state:\n{STATE}",
			Priority:       7,
			Enabled:        false,
			TimeoutSeconds: 300,
			Metadata:       map[string]string{"source": "kernel_default", "bucket": "outreach"},
		},
		{
			AgentID:        "data-executor",
			Prompt:         "Execute kernel data/file transformation and state maintenance tasks with evidence.\n\nProceed based on the following state:\n{STATE}",
			Priority:       8,
			Enabled:        true,
			TimeoutSeconds: 300, // file/data ops should complete in 5 min; keeps cycle budget free
			Metadata:       map[string]string{"source": "kernel_default", "bucket": "data"},
		},
		{
			AgentID:        "audit-executor",
			Prompt:         "Execute kernel audit/validation tasks and record risks plus next actions.\n\nProceed based on the following state:\n{STATE}",
			Priority:       8,
			Enabled:        true,
			TimeoutSeconds: 300, // audit/validation is read-only; 5 min ceiling
			Metadata:       map[string]string{"source": "kernel_default", "bucket": "audit"},
		},
	}
	return RuntimeSettings{
		KernelID:         DefaultKernelID,
		Schedule:         DefaultKernelSchedule,
		SeedState:        DefaultSeedStateContent,
		TimeoutSeconds:   DefaultKernelTimeoutSeconds,
		LeaseSeconds:     DefaultKernelLeaseSeconds,
		RetentionSeconds: DefaultKernelRetentionSeconds,
		MaxConcurrent:    DefaultKernelMaxConcurrent,
		MaxCycleHistory:  DefaultKernelMaxCycleHistory,
		Channel:          DefaultKernelChannel,
		UserID:           DefaultKernelUserID,
		Planner: PlannerSettings{
			Enabled:             true,
			MaxDispatches:       DefaultKernelPlannerMaxDispatches,
			TimeoutSeconds:      DefaultKernelPlannerTimeoutSec,
			TeamDispatchEnabled: true,
			MaxTeamsPerCycle:    DefaultKernelMaxTeamsPerCycle,
			TeamTimeoutSeconds:  DefaultKernelTeamTimeoutSeconds,
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
	AgentID         string
	Prompt          string // may contain {STATE} placeholder
	Priority        int
	Enabled         bool
	CooldownMinutes int // skip redispatch after successful run when within cooldown window
	TimeoutSeconds  int // per-agent timeout override; 0 means use global DefaultKernelTimeoutSeconds
	Metadata        map[string]string
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
			AgentID:         agentCfg.AgentID,
			Prompt:          agentCfg.Prompt,
			Priority:        agentCfg.Priority,
			Enabled:         agentCfg.Enabled,
			CooldownMinutes: agentCfg.CooldownMinutes,
			TimeoutSeconds:  agentCfg.TimeoutSeconds,
			Metadata:        metadata,
		})
	}
	return cloned
}
