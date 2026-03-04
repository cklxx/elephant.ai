package teamruntime

import (
	"time"

	"alex/internal/infra/coding"
)

// CapabilitySnapshot persists dynamic CLI probe results with TTL metadata.
type CapabilitySnapshot struct {
	GeneratedAt  time.Time                        `yaml:"generated_at"`
	TTLSeconds   int                              `yaml:"ttl_seconds"`
	Capabilities []coding.DiscoveredCLICapability `yaml:"capabilities"`
}

// RoleBinding captures one role's runtime CLI selection and tmux binding.
type RoleBinding struct {
	RoleID            string   `yaml:"role_id"`
	CapabilityProfile string   `yaml:"capability_profile,omitempty"`
	TargetCLI         string   `yaml:"target_cli,omitempty"`
	SelectedCLI       string   `yaml:"selected_cli,omitempty"`
	SelectedPath      string   `yaml:"selected_path,omitempty"`
	SelectedAgentType string   `yaml:"selected_agent_type,omitempty"`
	FallbackCLIs      []string `yaml:"fallback_clis,omitempty"`
	TmuxPane          string   `yaml:"tmux_pane,omitempty"`
	RoleLogPath       string   `yaml:"role_log_path,omitempty"`
}

// RoleRegistry is persisted for recovery and future retries.
type RoleRegistry struct {
	Roles []RoleBinding `yaml:"roles"`
}

// RoleRuntimeState captures mutable execution state for one role.
type RoleRuntimeState struct {
	RoleID       string    `yaml:"role_id"`
	Status       string    `yaml:"status"`
	UpdatedAt    time.Time `yaml:"updated_at"`
	SelectedCLI  string    `yaml:"selected_cli,omitempty"`
	FallbackCLIs []string  `yaml:"fallback_clis,omitempty"`
}

// RuntimeState tracks team bootstrap and per-role runtime progression.
type RuntimeState struct {
	SessionID string                      `yaml:"session_id"`
	TeamID    string                      `yaml:"team_id"`
	Status    string                      `yaml:"status"`
	UpdatedAt time.Time                   `yaml:"updated_at"`
	Roles     map[string]RoleRuntimeState `yaml:"roles"`
}

// BootstrapState records immutable bootstrap artifacts.
type BootstrapState struct {
	SessionID        string    `yaml:"session_id"`
	TeamID           string    `yaml:"team_id"`
	Template         string    `yaml:"template"`
	Goal             string    `yaml:"goal"`
	InitializedAt    time.Time `yaml:"initialized_at"`
	CapabilitiesPath string    `yaml:"capabilities_path"`
	RoleRegistryPath string    `yaml:"role_registry_path"`
	RuntimeStatePath string    `yaml:"runtime_state_path"`
	EventLogPath     string    `yaml:"event_log_path"`
	TmuxSession      string    `yaml:"tmux_session,omitempty"`
}

// EnsureRequest describes team bootstrap requirements for one run.
type EnsureRequest struct {
	SessionID string
	Template  string
	Goal      string
	RoleIDs   []string
	Profiles  map[string]string
	Targets   map[string]string
}

// EnsureResult returns bootstrap artifacts used by orchestration dispatch.
type EnsureResult struct {
	BaseDir      string
	Bootstrap    BootstrapState
	Capabilities []coding.DiscoveredCLICapability
	RoleBindings map[string]RoleBinding
	RoleLogPaths map[string]string
	EventLogPath string
}
