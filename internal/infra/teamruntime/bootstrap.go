package teamruntime

import (
	"context"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strings"
	"time"

	"alex/internal/infra/coding"
	"alex/internal/shared/logging"
	"gopkg.in/yaml.v3"
)

const defaultCapabilityTTL = 5 * time.Minute

type BootstrapManager struct {
	baseDir       string
	capabilityTTL time.Duration
	logger        logging.Logger
	tmuxManager   *TmuxManager
}

func NewBootstrapManager(baseDir string, logger logging.Logger) *BootstrapManager {
	root := strings.TrimSpace(baseDir)
	if root == "" {
		root = filepath.Join(".elephant", "team_runtime")
	}
	return &BootstrapManager{
		baseDir:       root,
		capabilityTTL: defaultCapabilityTTL,
		logger:        logging.OrNop(logger),
		tmuxManager:   NewTmuxManager(""),
	}
}

func (m *BootstrapManager) WithCapabilityTTL(ttl time.Duration) *BootstrapManager {
	if ttl > 0 {
		m.capabilityTTL = ttl
	}
	return m
}

func (m *BootstrapManager) Ensure(ctx context.Context, req EnsureRequest) (*EnsureResult, error) {
	if m == nil {
		return nil, fmt.Errorf("bootstrap manager is nil")
	}
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		sessionID = "session-unknown"
	}
	template := strings.TrimSpace(req.Template)
	if template == "" {
		template = "team"
	}
	teamID := buildTeamID(template, req.Goal)
	rootDir := filepath.Join(m.baseDir, sessionID, "teams", teamID)
	logsDir := filepath.Join(rootDir, "logs")
	bootstrapPath := filepath.Join(rootDir, "bootstrap.yaml")
	capabilitiesPath := filepath.Join(rootDir, "capabilities.yaml")
	roleRegistryPath := filepath.Join(rootDir, "role_registry.yaml")
	runtimeStatePath := filepath.Join(rootDir, "runtime_state.yaml")
	eventLogPath := filepath.Join(rootDir, "events.jsonl")

	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create team runtime dir: %w", err)
	}
	recorder := NewEventRecorder(eventLogPath)
	_ = recorder.Record("bootstrap_started", map[string]any{
		"session_id": sessionID,
		"team_id":    teamID,
		"template":   template,
	})

	capabilities, err := m.loadOrProbeCapabilities(ctx, capabilitiesPath, req)
	if err != nil {
		return nil, err
	}

	roleBindings := make(map[string]RoleBinding, len(req.RoleIDs))
	for _, roleID := range req.RoleIDs {
		trimmed := strings.TrimSpace(roleID)
		if trimmed == "" {
			continue
		}
		logPath := filepath.Join(logsDir, trimmed+".log")
		binding := SelectRoleBinding(
			trimmed,
			req.Profiles[trimmed],
			req.Targets[trimmed],
			capabilities,
			logPath,
		)
		roleBindings[trimmed] = binding
	}

	tmuxSession := ""
	if m.tmuxManager != nil && m.tmuxManager.Available() {
		session, panes, tmuxErr := m.tmuxManager.EnsureRolePanes(ctx, teamID, roleBindings, recorder)
		if tmuxErr != nil {
			m.logger.Warn("teamruntime: tmux setup failed team=%s err=%v", teamID, tmuxErr)
			_ = recorder.Record("tmux_setup_failed", map[string]any{"error": tmuxErr.Error()})
		} else {
			tmuxSession = session
			for roleID, pane := range panes {
				b := roleBindings[roleID]
				b.TmuxPane = pane
				roleBindings[roleID] = b
			}
		}
	} else {
		_ = recorder.Record("tmux_unavailable", map[string]any{})
	}

	roleRegistry := RoleRegistry{
		Roles: make([]RoleBinding, 0, len(roleBindings)),
	}
	for _, roleID := range req.RoleIDs {
		if b, ok := roleBindings[roleID]; ok {
			roleRegistry.Roles = append(roleRegistry.Roles, b)
		}
	}
	if err := writeYAML(roleRegistryPath, roleRegistry); err != nil {
		return nil, fmt.Errorf("write role registry: %w", err)
	}

	runtimeState := RuntimeState{
		SessionID: sessionID,
		TeamID:    teamID,
		Status:    "initialized",
		UpdatedAt: time.Now().UTC(),
		Roles:     make(map[string]RoleRuntimeState, len(roleBindings)),
	}
	for _, role := range roleRegistry.Roles {
		runtimeState.Roles[role.RoleID] = RoleRuntimeState{
			RoleID:       role.RoleID,
			Status:       "initialized",
			UpdatedAt:    runtimeState.UpdatedAt,
			SelectedCLI:  role.SelectedCLI,
			FallbackCLIs: append([]string(nil), role.FallbackCLIs...),
		}
	}
	if err := writeYAML(runtimeStatePath, runtimeState); err != nil {
		return nil, fmt.Errorf("write runtime state: %w", err)
	}

	bootstrap := BootstrapState{
		SessionID:        sessionID,
		TeamID:           teamID,
		Template:         template,
		Goal:             strings.TrimSpace(req.Goal),
		InitializedAt:    time.Now().UTC(),
		CapabilitiesPath: capabilitiesPath,
		RoleRegistryPath: roleRegistryPath,
		RuntimeStatePath: runtimeStatePath,
		EventLogPath:     eventLogPath,
		TmuxSession:      tmuxSession,
	}
	if err := writeYAML(bootstrapPath, bootstrap); err != nil {
		return nil, fmt.Errorf("write bootstrap state: %w", err)
	}
	_ = recorder.Record("bootstrap_completed", map[string]any{
		"session_id":   sessionID,
		"team_id":      teamID,
		"roles":        len(roleBindings),
		"capabilities": len(capabilities),
		"tmux_session": tmuxSession,
	})

	roleLogPaths := make(map[string]string, len(roleBindings))
	for roleID, binding := range roleBindings {
		roleLogPaths[roleID] = binding.RoleLogPath
	}
	return &EnsureResult{
		BaseDir:      rootDir,
		Bootstrap:    bootstrap,
		Capabilities: capabilities,
		RoleBindings: roleBindings,
		RoleLogPaths: roleLogPaths,
		EventLogPath: eventLogPath,
	}, nil
}

func (m *BootstrapManager) loadOrProbeCapabilities(ctx context.Context, path string, req EnsureRequest) ([]coding.DiscoveredCLICapability, error) {
	var cached CapabilitySnapshot
	if err := readYAML(path, &cached); err == nil {
		ttl := time.Duration(cached.TTLSeconds) * time.Second
		if ttl <= 0 {
			ttl = m.capabilityTTL
		}
		if ttl > 0 && !cached.GeneratedAt.IsZero() && time.Since(cached.GeneratedAt) < ttl && len(cached.Capabilities) > 0 {
			return cached.Capabilities, nil
		}
	}

	candidates := make([]string, 0, len(req.Targets))
	for _, target := range req.Targets {
		if trimmed := strings.TrimSpace(target); trimmed != "" {
			candidates = append(candidates, trimmed)
		}
	}
	capabilities := coding.DiscoverCodingCLIs(ctx, coding.DiscoveryOptions{
		Candidates:      candidates,
		IncludePathScan: true,
		ProbeTimeout:    2 * time.Second,
	})
	snapshot := CapabilitySnapshot{
		GeneratedAt:  time.Now().UTC(),
		TTLSeconds:   int(m.capabilityTTL.Seconds()),
		Capabilities: capabilities,
	}
	if err := writeYAML(path, snapshot); err != nil {
		return nil, fmt.Errorf("write capabilities snapshot: %w", err)
	}
	return capabilities, nil
}

func buildTeamID(template, goal string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.TrimSpace(goal)))
	hash := h.Sum32()
	safeTemplate := strings.NewReplacer(" ", "-", "/", "-", ":", "-", ".", "-").Replace(strings.TrimSpace(template))
	if safeTemplate == "" {
		safeTemplate = "team"
	}
	return fmt.Sprintf("%s-%08x", safeTemplate, hash)
}

func writeYAML(path string, value any) error {
	data, err := yaml.Marshal(value)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readYAML(path string, dest any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, dest)
}
