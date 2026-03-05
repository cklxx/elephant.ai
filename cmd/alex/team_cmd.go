package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"alex/internal/domain/agent/ports"
	agentports "alex/internal/domain/agent/ports/agent"
	"alex/internal/infra/tools/builtin/orchestration"
	id "alex/internal/shared/utils/id"
)

const (
	teamCommandUsage            = "usage: alex team <run|templates|reply|inject> [...]"
	teamRunUsage                = "usage: alex team run [--file path | --template name --goal text] [--session-id id] [--wait] [--wait-timeout-seconds N] [--mode auto|team|swarm] [--only-task id] [--role-prompt role=text]"
	teamTemplatesUsage          = "usage: alex team templates"
	teamReplyUsage              = "usage: alex team reply --task-id id --request-id id [--decision approve|reject] [--option-id id] [--message text]"
	teamInjectUsage             = "usage: alex team inject --task-id id --message text"
	defaultTeamWaitTimeoutInSec = 120
)

type rolePromptFlag map[string]string

func (f *rolePromptFlag) String() string {
	if f == nil || len(*f) == 0 {
		return ""
	}
	parts := make([]string, 0, len(*f))
	for role, prompt := range *f {
		parts = append(parts, role+"="+prompt)
	}
	return strings.Join(parts, ",")
}

func (f *rolePromptFlag) Set(v string) error {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	idx := strings.Index(v, "=")
	if idx <= 0 || idx == len(v)-1 {
		return fmt.Errorf("expected role=prompt, got %q", v)
	}
	role := strings.TrimSpace(v[:idx])
	prompt := strings.TrimSpace(v[idx+1:])
	if role == "" || prompt == "" {
		return fmt.Errorf("expected role=prompt, got %q", v)
	}
	if *f == nil {
		*f = map[string]string{}
	}
	(*f)[role] = prompt
	return nil
}

func (c *CLI) handleTeam(args []string) error {
	if len(args) == 0 {
		return &ExitCodeError{Code: 2, Err: errors.New(teamCommandUsage)}
	}
	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "run":
		return c.runTeamCLI(args[1:])
	case "templates":
		return c.teamTemplatesCLI(args[1:])
	case "reply":
		return c.replyTeamCLI(args[1:])
	case "inject":
		return c.injectTeamCLI(args[1:])
	default:
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("unknown team subcommand %q (expected: run, templates, reply, inject)", args[0])}
	}
}

type teamRunOptions struct {
	filePath       string
	templateName   string
	goalText       string
	sessionID      string
	wait           bool
	waitTimeoutSec int
	mode           string
	onlyTaskIDs    []string
	rolePromptByID map[string]string
}

func parseTeamRunOptions(args []string) (teamRunOptions, error) {
	opts := teamRunOptions{mode: "auto", waitTimeoutSec: defaultTeamWaitTimeoutInSec}
	fs, flagBuf := newBufferedFlagSet("alex team run")
	file := fs.String("file", "", "Path to task YAML file")
	template := fs.String("template", "", "Team template name")
	goal := fs.String("goal", "", "Goal text for template mode")
	sessionID := fs.String("session-id", "", "Session ID to bind background orchestration state")
	wait := fs.Bool("wait", false, "Wait for task completion")
	timeoutSeconds := fs.Int("wait-timeout-seconds", defaultTeamWaitTimeoutInSec, "Wait timeout in seconds (used with --wait)")
	mode := fs.String("mode", "auto", "Execution mode: auto|team|swarm")

	var onlyTaskIDs stringListFlag
	fs.Var(&onlyTaskIDs, "only-task", "Execute only selected task IDs (repeatable or comma-separated)")
	var prompts rolePromptFlag
	fs.Var(&prompts, "role-prompt", "Template prompt override in role=prompt form (repeatable)")

	if err := fs.Parse(args); err != nil {
		return opts, formatBufferedFlagParseError(err, flagBuf)
	}
	if len(fs.Args()) > 0 {
		return opts, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}

	filePath := strings.TrimSpace(*file)
	templateName := strings.TrimSpace(*template)
	goalText := strings.TrimSpace(*goal)
	if filePath == "" && templateName == "" {
		return opts, errors.New(teamRunUsage)
	}
	if filePath != "" && templateName != "" {
		return opts, fmt.Errorf("--file and --template are mutually exclusive")
	}
	if templateName != "" && goalText == "" {
		return opts, fmt.Errorf("--goal is required when --template is provided")
	}
	if filePath != "" && goalText != "" {
		return opts, fmt.Errorf("--goal can only be used with --template")
	}
	if filePath != "" && len(prompts) > 0 {
		return opts, fmt.Errorf("--role-prompt can only be used with --template")
	}
	if *timeoutSeconds <= 0 {
		return opts, fmt.Errorf("--wait-timeout-seconds must be > 0")
	}
	if !*wait && flagProvided(fs, "wait-timeout-seconds") {
		return opts, fmt.Errorf("--wait-timeout-seconds requires --wait")
	}

	normalizedMode := strings.ToLower(strings.TrimSpace(*mode))
	switch normalizedMode {
	case "", "auto", "team", "swarm":
		if normalizedMode == "" {
			normalizedMode = "auto"
		}
	default:
		return opts, fmt.Errorf("invalid --mode %q (expected: auto|team|swarm)", *mode)
	}

	opts.filePath = filePath
	opts.templateName = templateName
	opts.goalText = goalText
	opts.sessionID = strings.TrimSpace(*sessionID)
	opts.wait = *wait
	opts.waitTimeoutSec = *timeoutSeconds
	opts.mode = normalizedMode
	if len(onlyTaskIDs) > 0 {
		opts.onlyTaskIDs = []string(onlyTaskIDs)
	}
	if len(prompts) > 0 {
		opts.rolePromptByID = map[string]string(prompts)
	}
	return opts, nil
}

func (c *CLI) runTeamCLI(args []string) error {
	opts, err := parseTeamRunOptions(args)
	if err != nil {
		return &ExitCodeError{Code: 2, Err: err}
	}
	if c == nil || c.container == nil || c.container.Container == nil || c.container.Container.AgentCoordinator == nil {
		return &ExitCodeError{Code: 1, Err: fmt.Errorf("container not initialized")}
	}

	runSessionID := opts.sessionID
	if runSessionID == "" {
		runSessionID = id.NewSessionID()
	}

	ctx := cliBaseContext()
	ctx = id.WithSessionID(ctx, runSessionID)

	dispatcher, err := c.container.Container.AgentCoordinator.EnsureBackgroundDispatcher(ctx, runSessionID)
	if err != nil {
		return &ExitCodeError{Code: 1, Err: err}
	}
	ctx = agentports.WithOrchestrationContext(ctx, agentports.OrchestrationContext{
		TeamDefinitions: c.container.Container.AgentCoordinator.TeamDefinitionsSnapshot(),
		TeamRunRecorder: c.container.Container.AgentCoordinator.TeamRunRecorder(),
		Dispatcher:      dispatcher,
	})

	callArgs := map[string]any{
		"wait": opts.wait,
		"mode": opts.mode,
	}
	if opts.wait {
		callArgs["timeout_seconds"] = opts.waitTimeoutSec
	}
	if opts.filePath != "" {
		callArgs["file"] = opts.filePath
	}
	if opts.templateName != "" {
		callArgs["template"] = opts.templateName
	}
	if opts.goalText != "" {
		callArgs["goal"] = opts.goalText
	}
	if len(opts.onlyTaskIDs) > 0 {
		callArgs["task_ids"] = opts.onlyTaskIDs
	}
	if len(opts.rolePromptByID) > 0 {
		callArgs["prompts"] = opts.rolePromptByID
	}

	result, execErr := orchestration.NewRunTasks().Execute(ctx, ports.ToolCall{
		ID:        id.NewEventID(),
		Name:      "run_tasks",
		Arguments: callArgs,
		SessionID: runSessionID,
	})
	if execErr != nil {
		return &ExitCodeError{Code: 1, Err: execErr}
	}
	if result == nil {
		return &ExitCodeError{Code: 1, Err: fmt.Errorf("run returned empty result")}
	}
	if result.Error != nil {
		msg := strings.TrimSpace(result.Content)
		if msg != "" {
			return &ExitCodeError{Code: 1, Err: fmt.Errorf("%s", msg)}
		}
		return &ExitCodeError{Code: 1, Err: result.Error}
	}

	fmt.Println(strings.TrimSpace(result.Content))
	fmt.Printf("\nSession ID: %s\n", runSessionID)
	return nil
}

func (c *CLI) teamTemplatesCLI(args []string) error {
	if len(args) > 0 {
		return &ExitCodeError{Code: 2, Err: errors.New(teamTemplatesUsage)}
	}
	if c == nil || c.container == nil || c.container.Container == nil || c.container.Container.AgentCoordinator == nil {
		return &ExitCodeError{Code: 1, Err: fmt.Errorf("container not initialized")}
	}

	teams := c.container.Container.AgentCoordinator.TeamDefinitionsSnapshot()
	if len(teams) == 0 {
		fmt.Println("No team templates configured.")
		return nil
	}
	sort.Slice(teams, func(i, j int) bool {
		return teams[i].Name < teams[j].Name
	})
	fmt.Println("Available team templates:")
	for _, team := range teams {
		line := fmt.Sprintf("- %s (roles=%d stages=%d)", team.Name, len(team.Roles), len(team.Stages))
		if desc := strings.TrimSpace(team.Description); desc != "" {
			line = fmt.Sprintf("%s: %s", line, desc)
		}
		fmt.Println(line)
	}
	return nil
}

type teamReplyOptions struct {
	taskID    string
	requestID string
	decision  string
	optionID  string
	message   string
}

func parseTeamReplyOptions(args []string) (teamReplyOptions, error) {
	opts := teamReplyOptions{}
	fs, flagBuf := newBufferedFlagSet("alex team reply")
	taskID := fs.String("task-id", "", "Background task ID")
	requestID := fs.String("request-id", "", "Input request ID from progress notification")
	decision := fs.String("decision", "", "Approval decision: approve|reject")
	optionID := fs.String("option-id", "", "Selected option ID")
	message := fs.String("message", "", "Free-form response text")
	if err := fs.Parse(args); err != nil {
		return opts, formatBufferedFlagParseError(err, flagBuf)
	}
	if len(fs.Args()) > 0 {
		return opts, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}

	opts.taskID = strings.TrimSpace(*taskID)
	opts.requestID = strings.TrimSpace(*requestID)
	opts.optionID = strings.TrimSpace(*optionID)
	opts.message = strings.TrimSpace(*message)
	opts.decision = strings.ToLower(strings.TrimSpace(*decision))

	if opts.taskID == "" || opts.requestID == "" {
		return opts, errors.New(teamReplyUsage)
	}
	switch opts.decision {
	case "", "approve", "reject":
	default:
		return opts, fmt.Errorf("invalid --decision %q (expected: approve|reject)", opts.decision)
	}
	if opts.decision == "" && opts.optionID == "" && opts.message == "" {
		return opts, fmt.Errorf("at least one of --decision, --option-id, or --message is required")
	}
	return opts, nil
}

func (c *CLI) replyTeamCLI(args []string) error {
	opts, err := parseTeamReplyOptions(args)
	if err != nil {
		return &ExitCodeError{Code: 2, Err: err}
	}
	if c == nil || c.container == nil || c.container.Container == nil || c.container.Container.AgentCoordinator == nil {
		return &ExitCodeError{Code: 1, Err: fmt.Errorf("container not initialized")}
	}
	ctx := cliBaseContext()

	if err := c.container.Container.AgentCoordinator.ReplyBackgroundInput(ctx, agentports.InputResponse{
		TaskID:    opts.taskID,
		RequestID: opts.requestID,
		Approved:  opts.decision == "approve",
		OptionID:  opts.optionID,
		Text:      opts.message,
	}); err != nil {
		return &ExitCodeError{Code: 1, Err: err}
	}

	fmt.Printf("Reply sent for task %q request %q.\n", opts.taskID, opts.requestID)
	return nil
}

type teamInjectOptions struct {
	taskID  string
	message string
}

func parseTeamInjectOptions(args []string) (teamInjectOptions, error) {
	opts := teamInjectOptions{}
	fs, flagBuf := newBufferedFlagSet("alex team inject")
	taskID := fs.String("task-id", "", "Background task ID")
	message := fs.String("message", "", "Injected free-form text")
	if err := fs.Parse(args); err != nil {
		return opts, formatBufferedFlagParseError(err, flagBuf)
	}
	if len(fs.Args()) > 0 {
		return opts, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}
	opts.taskID = strings.TrimSpace(*taskID)
	opts.message = strings.TrimSpace(*message)
	if opts.taskID == "" || opts.message == "" {
		return opts, errors.New(teamInjectUsage)
	}
	return opts, nil
}

func (c *CLI) injectTeamCLI(args []string) error {
	opts, err := parseTeamInjectOptions(args)
	if err != nil {
		return &ExitCodeError{Code: 2, Err: err}
	}
	if c == nil || c.container == nil || c.container.Container == nil || c.container.Container.AgentCoordinator == nil {
		return &ExitCodeError{Code: 1, Err: fmt.Errorf("container not initialized")}
	}
	ctx := cliBaseContext()
	if err := c.container.Container.AgentCoordinator.InjectBackgroundInput(ctx, opts.taskID, opts.message); err != nil {
		return &ExitCodeError{Code: 1, Err: err}
	}

	fmt.Printf("Injected input into task %q.\n", opts.taskID)
	return nil
}
