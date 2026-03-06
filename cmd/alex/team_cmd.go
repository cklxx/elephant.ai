package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	appcontext "alex/internal/app/agent/context"
	agentports "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/react"
	"alex/internal/domain/agent/taskfile"
	"alex/internal/infra/adapters"
	"alex/internal/infra/coding"
	"alex/internal/infra/external"
	"alex/internal/infra/process"
	"alex/internal/infra/runtime"
	"alex/internal/infra/teamruntime"
	"alex/internal/infra/tools/builtin/orchestration"
	toolshared "alex/internal/infra/tools/builtin/shared"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/logging"
	id "alex/internal/shared/utils/id"
	"gopkg.in/yaml.v3"
)

const (
	teamUsage         = "usage: alex team [status|run|inject|terminal]"
	teamRunUsage      = "usage: alex team run (--template name --goal text | --template list | --file path | --prompt text) [--mode team|swarm|auto] [--session-id id] [--timeout-seconds N] [--role-prompt role=prompt] [--task-id id]"
	teamInjectUsage   = "usage: alex team inject [--runtime-root path] [--session-id id] [--team-id id] [--role-id id|--task-id id] --message text"
	teamTerminalUsage = "usage: alex team terminal [--runtime-root path] [--session-id id] [--team-id id] [--role-id id|--task-id id] [--mode stream|attach|capture] [--lines N]"
)

func runTeamCommand(args []string) error {
	return runTeamCommandWithContainer(args, nil)
}

func runTeamCommandWithContainer(args []string, container *Container) error {
	if len(args) == 0 {
		return runTeamStatus(nil)
	}

	first := strings.TrimSpace(args[0])
	if first == "-h" || first == "--help" {
		fmt.Fprintln(os.Stdout, teamUsage)
		fmt.Fprintln(os.Stdout, teamStatusUsage)
		fmt.Fprintln(os.Stdout, teamRunUsage)
		fmt.Fprintln(os.Stdout, teamInjectUsage)
		fmt.Fprintln(os.Stdout, teamTerminalUsage)
		return nil
	}
	if strings.HasPrefix(first, "-") {
		return runTeamStatus(args)
	}

	switch strings.ToLower(first) {
	case "help":
		fmt.Fprintln(os.Stdout, teamUsage)
		fmt.Fprintln(os.Stdout, teamStatusUsage)
		fmt.Fprintln(os.Stdout, teamRunUsage)
		fmt.Fprintln(os.Stdout, teamInjectUsage)
		fmt.Fprintln(os.Stdout, teamTerminalUsage)
		return nil
	case "status":
		return runTeamStatus(args[1:])
	case "run":
		return runTeamRun(args[1:], container)
	case "inject":
		return runTeamInject(args[1:])
	case "terminal", "term", "attach":
		return runTeamTerminal(args[1:])
	default:
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("unknown team subcommand %q (expected: status|run|inject|terminal)", args[0])}
	}
}

type teamRunOptions struct {
	file          string
	template      string
	goal          string
	prompt        string
	agentType     string
	executionMode string
	autonomyLevel string
	workspaceMode string
	sessionID     string
	wait          bool
	timeoutSec    int
	mode          string
	taskIDs       []string
	rolePrompts   map[string]string
}

func runTeamRun(args []string, container *Container) error {
	fs, flagBuf := newBufferedFlagSet("alex team run")
	filePath := fs.String("file", "", "TaskFile YAML path.")
	templateName := fs.String("template", "", "Team template name.")
	goal := fs.String("goal", "", "Team goal (required with --template).")
	prompt := fs.String("prompt", "", "Single prompt task (alternative to --file/--template).")
	agentType := fs.String("agent-type", "codex", "Agent type for --prompt mode.")
	executionMode := fs.String("execution-mode", "execute", "Execution mode for --prompt mode (execute|plan).")
	autonomyLevel := fs.String("autonomy-level", "full", "Autonomy level for --prompt mode.")
	workspaceMode := fs.String("workspace-mode", "shared", "Workspace mode for --prompt mode.")
	sessionID := fs.String("session-id", "", "Session ID for runtime artifacts.")
	wait := fs.Bool("wait", true, "Wait for completion (must be true in CLI mode).")
	timeoutSec := fs.Int("timeout-seconds", 600, "Wait timeout in seconds.")
	mode := fs.String("mode", "auto", "Execution strategy: team|swarm|auto.")

	var taskIDs stringListFlag
	fs.Var(&taskIDs, "task-id", "Run only specific task IDs (repeatable or comma-separated).")
	var rolePrompts stringListFlag
	fs.Var(&rolePrompts, "role-prompt", "Role prompt override: role=prompt (repeatable).")

	if err := fs.Parse(args); err != nil {
		return &ExitCodeError{Code: 2, Err: formatBufferedFlagParseError(err, flagBuf)}
	}
	if !*wait {
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("--wait=false is not supported in CLI mode")}
	}
	if *timeoutSec <= 0 {
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("--timeout-seconds must be > 0")}
	}

	opts := teamRunOptions{
		file:          strings.TrimSpace(*filePath),
		template:      strings.TrimSpace(*templateName),
		goal:          strings.TrimSpace(*goal),
		prompt:        strings.TrimSpace(*prompt),
		agentType:     strings.TrimSpace(*agentType),
		executionMode: strings.TrimSpace(*executionMode),
		autonomyLevel: strings.TrimSpace(*autonomyLevel),
		workspaceMode: strings.TrimSpace(*workspaceMode),
		sessionID:     strings.TrimSpace(*sessionID),
		wait:          *wait,
		timeoutSec:    *timeoutSec,
		mode:          strings.TrimSpace(*mode),
		taskIDs:       []string(taskIDs),
	}
	var err error
	opts.rolePrompts, err = parseRolePrompts([]string(rolePrompts))
	if err != nil {
		return &ExitCodeError{Code: 2, Err: err}
	}

	if err := validateTeamRunOptions(opts, fs.Args()); err != nil {
		return &ExitCodeError{Code: 2, Err: err}
	}

	if opts.sessionID == "" && teamRunNeedsRuntimeArtifacts(opts) {
		opts.sessionID = id.NewSessionID()
	}

	effectiveContainer, cleanup, err := ensureTeamContainer(container)
	if err != nil {
		return &ExitCodeError{Code: 1, Err: err}
	}
	defer cleanup()

	logger := logging.NewComponentLogger("TeamCLI")
	baseCtx := applyPinnedCLILLMSelection(cliBaseContext(), runtimeEnvLookup(), logger)

	dispatcher, shutdown := newTeamCLIDispatcher(effectiveContainer, baseCtx, opts.sessionID)
	defer shutdown()

	ctx := buildTeamRunContext(baseCtx, effectiveContainer, dispatcher, opts.sessionID)

	actualFilePath := opts.file
	cleanupTaskFile := func() {}
	if opts.prompt != "" {
		generated, genErr := writeSinglePromptTaskFile(opts)
		if genErr != nil {
			return &ExitCodeError{Code: 1, Err: genErr}
		}
		actualFilePath = generated
		cleanupTaskFile = func() { _ = os.Remove(generated) }
	}
	defer cleanupTaskFile()

	executionStrategy := taskfile.ExecutionMode(opts.mode)
	runner := orchestration.NewTeamRunner()
	result, execErr := runner.Run(ctx, orchestration.RunRequest{
		Dispatcher:      dispatcher,
		FilePath:        actualFilePath,
		TemplateName:    opts.template,
		Goal:            opts.goal,
		PromptOverrides: opts.rolePrompts,
		Wait:            opts.wait,
		Timeout:         time.Duration(opts.timeoutSec) * time.Second,
		Mode:            executionStrategy,
		TaskIDs:         opts.taskIDs,
		CausationID:     "team-cli-run-" + id.NewKSUID(),
		SessionID:       opts.sessionID,
		TeamDefinitions: convertTeamConfigsForCLI(effectiveContainer.Runtime.ExternalAgents.Teams),
	})
	if execErr != nil {
		return &ExitCodeError{Code: 1, Err: execErr}
	}
	if result == nil {
		return &ExitCodeError{Code: 1, Err: fmt.Errorf("team run returned empty result")}
	}
	if rendered := renderTeamRunCLIOutput(result.Content, teamRunOutputSessionID(opts)); rendered != "" {
		fmt.Fprintln(os.Stdout, rendered)
	}
	return nil
}

func validateTeamRunOptions(opts teamRunOptions, extraArgs []string) error {
	if len(extraArgs) > 0 {
		return fmt.Errorf("unexpected positional arguments: %s", strings.Join(extraArgs, " "))
	}
	modeCount := 0
	if opts.file != "" {
		modeCount++
	}
	if opts.template != "" {
		modeCount++
	}
	if opts.prompt != "" {
		modeCount++
	}
	if modeCount != 1 {
		return errors.New(teamRunUsage)
	}
	if opts.template == "" && opts.goal != "" {
		return fmt.Errorf("--goal requires --template")
	}
	if opts.template == "" && len(opts.rolePrompts) > 0 {
		return fmt.Errorf("--role-prompt requires --template")
	}
	if opts.template != "" && opts.goal == "" && !strings.EqualFold(opts.template, "list") {
		return fmt.Errorf("--goal is required when --template is provided (except --template list)")
	}
	if strings.EqualFold(opts.template, "list") {
		if opts.goal != "" {
			return fmt.Errorf("--goal is not supported with --template list")
		}
		if len(opts.rolePrompts) > 0 {
			return fmt.Errorf("--role-prompt is not supported with --template list")
		}
	}
	return nil
}

func ensureTeamContainer(existing *Container) (*Container, func(), error) {
	if existing != nil && existing.Container != nil {
		return existing, func() {}, nil
	}
	container, err := buildContainer()
	if err != nil {
		return nil, nil, fmt.Errorf("build container: %w", err)
	}
	if err := container.Container.Start(); err != nil {
		return nil, nil, fmt.Errorf("start container: %w", err)
	}
	cleanup := func() {
		drainCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = container.Container.Drain(drainCtx)
	}
	return container, cleanup, nil
}

func buildTeamRunContext(
	baseCtx context.Context,
	container *Container,
	dispatcher agentports.BackgroundTaskDispatcher,
	sessionID string,
) context.Context {
	ctx := toolshared.WithSessionID(baseCtx, sessionID)
	ctx = id.WithSessionID(ctx, sessionID)
	ctx = agentports.WithOrchestrationContext(ctx, agentports.OrchestrationContext{
		TeamDefinitions: convertTeamConfigsForCLI(container.Runtime.ExternalAgents.Teams),
		TeamRunRecorder: container.Container.AgentCoordinator.TeamRunRecorder(),
		Dispatcher:      dispatcher,
	})
	return ctx
}

func newTeamCLIDispatcher(
	container *Container,
	runCtx context.Context,
	sessionID string,
) (*react.BackgroundTaskManager, func()) {
	logger := logging.NewComponentLogger("TeamCLI")
	var externalExecutor agentports.ExternalAgentExecutor
	externalRegistry := external.NewRegistry(container.Runtime.ExternalAgents, process.NewController(), logger)
	if len(externalRegistry.SupportedTypes()) > 0 {
		externalExecutor = coding.NewManagedExternalExecutor(externalRegistry, logger)
	}

	idAdapter := runtime.IDsAdapter{}
	manager := react.NewBackgroundTaskManager(react.BackgroundManagerConfig{
		RunContext:          runCtx,
		Logger:              logger,
		Clock:               agentports.SystemClock{},
		IDGenerator:         idAdapter,
		IDContextReader:     idAdapter,
		GoRunner:            runtime.GoRunner,
		WorkingDirResolver:  runtime.WorkingDirResolver,
		WorkspaceMgrFactory: runtime.WorkspaceManagerFactory,
		ExecuteTask: func(ctx context.Context, prompt, subSessionID string, listener agentports.EventListener) (*agentports.TaskResult, error) {
			ctx = appcontext.MarkSubagentContext(ctx)
			return container.Container.AgentCoordinator.ExecuteTask(ctx, prompt, subSessionID, listener)
		},
		ExternalExecutor:   externalExecutor,
		SessionID:          sessionID,
		MaxConcurrentTasks: container.Runtime.ExternalAgents.MaxParallelAgents,
		ContextPropagators: []agentports.ContextPropagatorFunc{
			appcontext.PropagateLLMSelection,
		},
		TmuxSender:    adapters.NewExecTmuxSender(),
		EventAppender: adapters.NewFileEventAppender(),
	})
	return manager, manager.Shutdown
}

func convertTeamConfigsForCLI(configs []runtimeconfig.TeamConfig) []agentports.TeamDefinition {
	if len(configs) == 0 {
		return nil
	}
	teams := make([]agentports.TeamDefinition, 0, len(configs))
	for _, cfg := range configs {
		roles := make([]agentports.TeamRoleDefinition, 0, len(cfg.Roles))
		for _, role := range cfg.Roles {
			roles = append(roles, agentports.TeamRoleDefinition{
				Name:              role.Name,
				AgentType:         role.AgentType,
				CapabilityProfile: role.CapabilityProfile,
				TargetCLI:         role.TargetCLI,
				PromptTemplate:    role.PromptTemplate,
				ExecutionMode:     role.ExecutionMode,
				AutonomyLevel:     role.AutonomyLevel,
				WorkspaceMode:     role.WorkspaceMode,
				Config:            role.Config,
				InheritContext:    role.InheritContext,
			})
		}
		stages := make([]agentports.TeamStageDefinition, 0, len(cfg.Stages))
		for _, stage := range cfg.Stages {
			stages = append(stages, agentports.TeamStageDefinition{
				Name:       stage.Name,
				Roles:      stage.Roles,
				DebateMode: stage.DebateMode,
			})
		}
		teams = append(teams, agentports.TeamDefinition{
			Name:        cfg.Name,
			Description: cfg.Description,
			Roles:       roles,
			Stages:      stages,
		})
	}
	return teams
}

func writeSinglePromptTaskFile(opts teamRunOptions) (string, error) {
	agentType := strings.TrimSpace(opts.agentType)
	if agentType == "" {
		agentType = "codex"
	}
	planID := "team-single-" + id.NewKSUID()
	tf := taskfile.TaskFile{
		Version: "1",
		PlanID:  planID,
		Tasks: []taskfile.TaskSpec{
			{
				ID:             "single",
				Description:    "single prompt team task",
				Prompt:         opts.prompt,
				AgentType:      agentType,
				ExecutionMode:  strings.TrimSpace(opts.executionMode),
				AutonomyLevel:  strings.TrimSpace(opts.autonomyLevel),
				WorkspaceMode:  strings.TrimSpace(opts.workspaceMode),
				InheritContext: false,
			},
		},
	}
	data, err := yaml.Marshal(tf)
	if err != nil {
		return "", fmt.Errorf("marshal single prompt taskfile: %w", err)
	}
	f, err := os.CreateTemp("", "alex-team-single-*.yaml")
	if err != nil {
		return "", fmt.Errorf("create temp taskfile: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return "", fmt.Errorf("write temp taskfile: %w", err)
	}
	return f.Name(), nil
}

func parseRolePrompts(entries []string) (map[string]string, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(entries))
	for _, raw := range entries {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid --role-prompt %q, expected role=prompt", raw)
		}
		role := strings.TrimSpace(parts[0])
		prompt := strings.TrimSpace(parts[1])
		if role == "" || prompt == "" {
			return nil, fmt.Errorf("invalid --role-prompt %q, expected role=prompt", raw)
		}
		out[role] = prompt
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func parseTeamTerminalMode(raw string) string {
	mode := strings.ToLower(strings.TrimSpace(raw))
	switch mode {
	case "", "stream":
		return "stream"
	case "attach", "capture":
		return mode
	default:
		return ""
	}
}

func renderTeamRunCLIOutput(content string, sessionID string) string {
	parts := make([]string, 0, 2)
	if trimmed := strings.TrimSpace(content); trimmed != "" {
		parts = append(parts, trimmed)
	}
	if trimmed := strings.TrimSpace(sessionID); trimmed != "" {
		parts = append(parts, "Session ID: "+trimmed)
	}
	return strings.Join(parts, "\n")
}

func teamRunNeedsRuntimeArtifacts(opts teamRunOptions) bool {
	return !strings.EqualFold(strings.TrimSpace(opts.template), "list")
}

func teamRunOutputSessionID(opts teamRunOptions) string {
	if !teamRunNeedsRuntimeArtifacts(opts) {
		return ""
	}
	return strings.TrimSpace(opts.sessionID)
}

func runTeamTerminal(args []string) error {
	fs, flagBuf := newBufferedFlagSet("alex team terminal")
	runtimeRoot := fs.String("runtime-root", "", "Team runtime root (_team_runtime). Default: auto-discover.")
	sessionID := fs.String("session-id", "", "Filter by session_id.")
	teamID := fs.String("team-id", "", "Filter by team_id.")
	roleID := fs.String("role-id", "", "Target role_id.")
	taskID := fs.String("task-id", "", "Target task_id (auto-resolves role_id from team-*).")
	mode := fs.String("mode", "stream", "View mode: stream|attach|capture.")
	lines := fs.Int("lines", 120, "Capture/stream line window.")
	if err := fs.Parse(args); err != nil {
		return &ExitCodeError{Code: 2, Err: formatBufferedFlagParseError(err, flagBuf)}
	}
	if *lines <= 0 {
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("--lines must be > 0")}
	}
	resolvedMode := parseTeamTerminalMode(*mode)
	if resolvedMode == "" {
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("invalid --mode %q (expected stream|attach|capture)", strings.TrimSpace(*mode))}
	}

	opts := teamInjectOptions{
		runtimeRoot: strings.TrimSpace(*runtimeRoot),
		sessionID:   strings.TrimSpace(*sessionID),
		teamID:      strings.TrimSpace(*teamID),
		roleID:      strings.TrimSpace(*roleID),
		taskID:      strings.TrimSpace(*taskID),
	}
	resolvedRoleID, err := resolveRequestedRoleID(opts)
	if err != nil {
		return &ExitCodeError{Code: 2, Err: err}
	}

	statuses, err := loadTeamRuntimeStatus(teamStatusOptions{
		runtimeRoot: opts.runtimeRoot,
		sessionID:   opts.sessionID,
		teamID:      opts.teamID,
		includeAll:  true,
		eventsTail:  0,
	})
	if err != nil {
		return &ExitCodeError{Code: 1, Err: err}
	}
	if len(statuses) == 0 {
		return &ExitCodeError{Code: 1, Err: fmt.Errorf("no team runtime artifacts found")}
	}
	entry, err := selectTeamRuntimeStatus(statuses, opts, resolvedRoleID)
	if err != nil {
		return &ExitCodeError{Code: 2, Err: err}
	}
	if resolvedMode == "attach" {
		session := strings.TrimSpace(entry.TmuxSession)
		if session == "" {
			return &ExitCodeError{Code: 1, Err: fmt.Errorf("team %q has no tmux session", entry.TeamID)}
		}
		cmd := exec.Command("tmux", "-L", "elephant", "attach", "-t", session)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return &ExitCodeError{Code: 1, Err: fmt.Errorf("attach tmux session %q: %w", session, err)}
		}
		return nil
	}

	if resolvedRoleID == "" && len(entry.Roles) != 1 {
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("--role-id is required when team has multiple roles")}
	}
	binding, err := resolveInjectRole(entry, resolvedRoleID)
	if err != nil {
		return &ExitCodeError{Code: 1, Err: err}
	}
	pane := strings.TrimSpace(binding.TmuxPane)
	if pane == "" {
		return &ExitCodeError{Code: 1, Err: fmt.Errorf("role %q has no tmux pane binding", binding.RoleID)}
	}

	var cmd *exec.Cmd
	target := "-L elephant"
	switch resolvedMode {
	case "capture":
		start := fmt.Sprintf("-%d", *lines)
		cmd = exec.Command("tmux", "-L", "elephant", "capture-pane", "-pt", pane, "-S", start)
		fmt.Fprintf(os.Stdout, "Capturing pane %s (%s role=%s)\n", pane, target, nonEmpty(binding.RoleID, "(unknown)"))
	case "stream":
		start := fmt.Sprintf("-%d", *lines)
		cmd = exec.Command("tmux", "-L", "elephant", "capture-pane", "-pet", pane, "-S", start)
		fmt.Fprintf(os.Stdout, "Streaming pane snapshot %s (%s role=%s)\n", pane, target, nonEmpty(binding.RoleID, "(unknown)"))
	default:
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("invalid --mode %q", resolvedMode)}
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return &ExitCodeError{Code: 1, Err: fmt.Errorf("tmux %s pane %s: %s", resolvedMode, pane, strings.TrimSpace(string(out)))}
	}
	if len(out) > 0 {
		fmt.Fprintln(os.Stdout, string(out))
	}
	if resolvedMode == "stream" {
		fmt.Fprintln(os.Stdout, "Tip: run again to refresh, or use --mode attach for interactive view.")
	}
	return nil
}

type teamInjectOptions struct {
	runtimeRoot string
	sessionID   string
	teamID      string
	roleID      string
	taskID      string
	message     string
}

func runTeamInject(args []string) error {
	fs, flagBuf := newBufferedFlagSet("alex team inject")
	runtimeRoot := fs.String("runtime-root", "", "Team runtime root (_team_runtime). Default: auto-discover.")
	sessionID := fs.String("session-id", "", "Filter by session_id.")
	teamID := fs.String("team-id", "", "Filter by team_id.")
	roleID := fs.String("role-id", "", "Target role_id.")
	taskID := fs.String("task-id", "", "Target task_id (auto-resolves role_id from team-*).")
	message := fs.String("message", "", "Input text to inject into the role pane.")
	if err := fs.Parse(args); err != nil {
		return &ExitCodeError{Code: 2, Err: formatBufferedFlagParseError(err, flagBuf)}
	}
	msg := strings.TrimSpace(*message)
	if msg == "" && len(fs.Args()) > 0 {
		msg = strings.TrimSpace(strings.Join(fs.Args(), " "))
	}
	if msg == "" {
		return &ExitCodeError{Code: 2, Err: errors.New(teamInjectUsage)}
	}

	opts := teamInjectOptions{
		runtimeRoot: strings.TrimSpace(*runtimeRoot),
		sessionID:   strings.TrimSpace(*sessionID),
		teamID:      strings.TrimSpace(*teamID),
		roleID:      strings.TrimSpace(*roleID),
		taskID:      strings.TrimSpace(*taskID),
		message:     msg,
	}

	resolvedRoleID, err := resolveRequestedRoleID(opts)
	if err != nil {
		return &ExitCodeError{Code: 2, Err: err}
	}

	statuses, err := loadTeamRuntimeStatus(teamStatusOptions{
		runtimeRoot: opts.runtimeRoot,
		sessionID:   opts.sessionID,
		teamID:      opts.teamID,
		includeAll:  true,
		eventsTail:  0,
	})
	if err != nil {
		return &ExitCodeError{Code: 1, Err: err}
	}
	if len(statuses) == 0 {
		return &ExitCodeError{Code: 1, Err: fmt.Errorf("no team runtime artifacts found")}
	}
	entry, err := selectTeamRuntimeStatus(statuses, opts, resolvedRoleID)
	if err != nil {
		return &ExitCodeError{Code: 2, Err: err}
	}
	if resolvedRoleID == "" && len(entry.Roles) != 1 {
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("--role-id is required when team has multiple roles")}
	}

	binding, err := resolveInjectRole(entry, resolvedRoleID)
	if err != nil {
		return &ExitCodeError{Code: 1, Err: err}
	}
	pane := strings.TrimSpace(binding.TmuxPane)
	if pane == "" {
		return &ExitCodeError{Code: 1, Err: fmt.Errorf("role %q has no tmux pane binding", binding.RoleID)}
	}

	tmux := teamruntime.NewTmuxManager("")
	if !tmux.Available() {
		return &ExitCodeError{Code: 1, Err: fmt.Errorf("tmux not available")}
	}
	if err := tmux.Inject(context.Background(), pane, opts.message); err != nil {
		appendInjectEvent(entry, binding, opts.message, err)
		return &ExitCodeError{Code: 1, Err: err}
	}
	appendInjectEvent(entry, binding, opts.message, nil)

	fmt.Fprintf(
		os.Stdout,
		"Injected input into team=%s role=%s pane=%s\n",
		nonEmpty(entry.TeamID, "(unknown)"),
		nonEmpty(binding.RoleID, "(unknown)"),
		pane,
	)
	return nil
}

func resolveRequestedRoleID(opts teamInjectOptions) (string, error) {
	trimmedRoleID := strings.TrimSpace(opts.roleID)
	taskID := strings.TrimSpace(opts.taskID)
	if trimmedRoleID != "" && taskID != "" {
		return "", fmt.Errorf("--role-id and --task-id are mutually exclusive")
	}
	if trimmedRoleID != "" {
		return trimmedRoleID, nil
	}
	if taskID == "" {
		return "", nil
	}
	roleID := taskfile.ExtractRoleID(taskID)
	if roleID == "" {
		return "", fmt.Errorf("task %q is not a team task id", taskID)
	}
	return roleID, nil
}

func selectTeamRuntimeStatus(
	statuses []teamRuntimeStatus,
	opts teamInjectOptions,
	roleID string,
) (teamRuntimeStatus, error) {
	if len(statuses) == 0 {
		return teamRuntimeStatus{}, fmt.Errorf("no team runtime artifacts found")
	}
	candidates := statuses
	if roleID != "" {
		filtered := make([]teamRuntimeStatus, 0, len(statuses))
		for _, status := range statuses {
			if _, err := resolveInjectRole(status, roleID); err == nil {
				filtered = append(filtered, status)
			}
		}
		candidates = filtered
		if len(candidates) == 0 {
			return teamRuntimeStatus{}, fmt.Errorf("role %q not found in matched team runtimes", roleID)
		}
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	hint := "use --session-id or --team-id to choose one runtime"
	if roleID != "" {
		return teamRuntimeStatus{}, fmt.Errorf("role %q matched multiple team runtimes; %s", roleID, hint)
	}
	if strings.TrimSpace(opts.sessionID) != "" || strings.TrimSpace(opts.teamID) != "" {
		return teamRuntimeStatus{}, fmt.Errorf("multiple team runtimes matched the provided filters; %s", hint)
	}
	return teamRuntimeStatus{}, fmt.Errorf("multiple team runtimes found; %s", hint)
}

func resolveInjectRole(entry teamRuntimeStatus, roleID string) (teamruntime.RoleBinding, error) {
	if len(entry.Roles) == 0 {
		return teamruntime.RoleBinding{}, fmt.Errorf("no role bindings found in runtime")
	}
	if strings.TrimSpace(roleID) == "" {
		return entry.Roles[0], nil
	}
	for _, role := range entry.Roles {
		if strings.EqualFold(strings.TrimSpace(role.RoleID), strings.TrimSpace(roleID)) {
			return role, nil
		}
	}
	return teamruntime.RoleBinding{}, fmt.Errorf("role %q not found in team %q", roleID, entry.TeamID)
}

func appendInjectEvent(entry teamRuntimeStatus, role teamruntime.RoleBinding, message string, injectErr error) {
	bootstrapPath := filepath.Join(strings.TrimSpace(entry.BaseDir), "bootstrap.yaml")
	var bootstrap teamruntime.BootstrapState
	_ = readYAMLFile(bootstrapPath, &bootstrap)

	payload := map[string]any{
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"type":      "tmux_input_injected",
		"team_id":   strings.TrimSpace(entry.TeamID),
		"role_id":   strings.TrimSpace(role.RoleID),
		"pane":      strings.TrimSpace(role.TmuxPane),
		"input_len": len([]rune(strings.TrimSpace(message))),
	}
	if injectErr != nil {
		payload["type"] = "tmux_input_inject_failed"
		payload["error"] = injectErr.Error()
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return
	}
	line := strings.TrimSpace(string(encoded))
	appendLine(strings.TrimSpace(bootstrap.EventLogPath), line)
	appendLine(strings.TrimSpace(role.RoleLogPath), line)
}

func appendLine(path string, line string) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(trimmedPath), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(trimmedPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(strings.TrimSpace(line) + "\n")
}
