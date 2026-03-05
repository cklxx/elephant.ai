package main

import (
	"errors"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	agentports "alex/internal/domain/agent/ports/agent"
	"alex/internal/infra/tools/builtin/orchestration"
	id "alex/internal/shared/utils/id"
)

const (
	teamCommandUsage = "usage: alex team <run|reply> [...]"
	teamRunUsage     = "usage: alex team run [--file path | --template name --goal text] [--session-id id] [--wait] [--timeout-seconds N] [--mode auto|team|swarm] [--task-id id] [--prompt role=text]"
	teamReplyUsage   = "usage: alex team reply --task-id id [--request-id id --approved=true|false --option-id id --message text]"
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
	case "reply":
		return c.replyTeamCLI(args[1:])
	default:
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("unknown team subcommand %q (expected: run, reply)", args[0])}
	}
}

func (c *CLI) runTeamCLI(args []string) error {
	if c == nil || c.container == nil || c.container.Container == nil || c.container.Container.AgentCoordinator == nil {
		return &ExitCodeError{Code: 1, Err: fmt.Errorf("container not initialized")}
	}

	fs, flagBuf := newBufferedFlagSet("alex team run")
	file := fs.String("file", "", "Path to task YAML file")
	template := fs.String("template", "", "Team template name (or list)")
	goal := fs.String("goal", "", "Goal text for template mode")
	sessionID := fs.String("session-id", "", "Session ID to bind background orchestration state")
	wait := fs.Bool("wait", false, "Wait for task completion")
	timeoutSeconds := fs.Int("timeout-seconds", 120, "Wait timeout in seconds (used with --wait)")
	mode := fs.String("mode", "auto", "Execution mode: auto|team|swarm")

	var taskIDs stringListFlag
	fs.Var(&taskIDs, "task-id", "Execute only the selected task IDs (repeatable or comma-separated)")
	var prompts rolePromptFlag
	fs.Var(&prompts, "prompt", "Template prompt override in role=prompt form (repeatable)")

	if err := fs.Parse(args); err != nil {
		return &ExitCodeError{Code: 2, Err: formatBufferedFlagParseError(err, flagBuf)}
	}

	filePath := strings.TrimSpace(*file)
	templateName := strings.TrimSpace(*template)
	goalText := strings.TrimSpace(*goal)
	if filePath == "" && templateName == "" {
		return &ExitCodeError{Code: 2, Err: errors.New(teamRunUsage)}
	}
	if filePath != "" && templateName != "" {
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("--file and --template are mutually exclusive")}
	}
	if templateName != "" && !strings.EqualFold(templateName, "list") && goalText == "" {
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("--goal is required when --template is provided")}
	}
	if *timeoutSeconds <= 0 {
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("--timeout-seconds must be > 0")}
	}

	normalizedMode := strings.ToLower(strings.TrimSpace(*mode))
	switch normalizedMode {
	case "", "auto", "team", "swarm":
		if normalizedMode == "" {
			normalizedMode = "auto"
		}
	default:
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("invalid --mode %q (expected: auto|team|swarm)", *mode)}
	}

	runSessionID := strings.TrimSpace(*sessionID)
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
		"wait":            *wait,
		"timeout_seconds": *timeoutSeconds,
		"mode":            normalizedMode,
	}
	if filePath != "" {
		callArgs["file"] = filePath
	}
	if templateName != "" {
		callArgs["template"] = templateName
	}
	if goalText != "" {
		callArgs["goal"] = goalText
	}
	if len(taskIDs) > 0 {
		callArgs["task_ids"] = []string(taskIDs)
	}
	if len(prompts) > 0 {
		callArgs["prompts"] = map[string]string(prompts)
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

func (c *CLI) replyTeamCLI(args []string) error {
	if c == nil || c.container == nil || c.container.Container == nil || c.container.Container.AgentCoordinator == nil {
		return &ExitCodeError{Code: 1, Err: fmt.Errorf("container not initialized")}
	}

	fs, flagBuf := newBufferedFlagSet("alex team reply")
	taskID := fs.String("task-id", "", "Background task ID")
	requestID := fs.String("request-id", "", "Input request ID from progress notification")
	approved := fs.Bool("approved", false, "Approval decision for permission requests")
	optionID := fs.String("option-id", "", "Selected option ID")
	message := fs.String("message", "", "Free-form response text or direct pane input")
	if err := fs.Parse(args); err != nil {
		return &ExitCodeError{Code: 2, Err: formatBufferedFlagParseError(err, flagBuf)}
	}

	tid := strings.TrimSpace(*taskID)
	rid := strings.TrimSpace(*requestID)
	msg := strings.TrimSpace(*message)
	opt := strings.TrimSpace(*optionID)
	if tid == "" {
		return &ExitCodeError{Code: 2, Err: errors.New(teamReplyUsage)}
	}

	ctx := cliBaseContext()
	if rid == "" {
		if msg == "" {
			return &ExitCodeError{Code: 2, Err: fmt.Errorf("--message is required when --request-id is omitted")}
		}
		if err := c.container.Container.AgentCoordinator.InjectBackgroundInput(ctx, tid, msg); err != nil {
			return &ExitCodeError{Code: 1, Err: err}
		}
		fmt.Printf("Injected input into task %q.\n", tid)
		return nil
	}

	if err := c.container.Container.AgentCoordinator.ReplyBackgroundInput(ctx, agentports.InputResponse{
		TaskID:    tid,
		RequestID: rid,
		Approved:  *approved,
		OptionID:  opt,
		Text:      msg,
	}); err != nil {
		return &ExitCodeError{Code: 1, Err: err}
	}

	fmt.Printf("Reply sent for task %q request %q.\n", tid, rid)
	return nil
}
