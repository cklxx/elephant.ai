package builtin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"alex/internal/agent/ports"
	"alex/internal/llm"
)

const mobileTaskSystemPrompt = `You are a mobile automation planner operating an Android device via adb.
Analyze the screenshot and the task, then return the next action as JSON only.

Rules:
- Use absolute pixel coordinates with origin at top-left.
- Keep actions minimal and deterministic.
- If the task is complete, return {"action":"done","summary":"..."}.
- Allowed actions: tap, swipe, text, key, wait, done.
- JSON schema:
  {"action":"tap","x":123,"y":456,"reason":"..."}
  {"action":"swipe","x1":123,"y1":456,"x2":123,"y2":456,"duration_ms":300,"reason":"..."}
  {"action":"text","text":"...","reason":"..."}
  {"action":"key","key":"HOME|BACK|ENTER|APP_SWITCH|KEYCODE_...","reason":"..."}
  {"action":"wait","duration_ms":1000,"reason":"..."}
  {"action":"done","summary":"..."}
`

const defaultMobileMaxSteps = 100

type MobileTaskConfig struct {
	LLM         ports.LLMClient
	ADBAddress  string
	ADBSerial   string
	MaxSteps    int
}

type mobileTaskTool struct {
	llm        ports.LLMClient
	adbAddress string
	adbSerial  string
	maxSteps   int
}

type mobileTaskAction struct {
	Action     string `json:"action"`
	X          int    `json:"x,omitempty"`
	Y          int    `json:"y,omitempty"`
	X1         int    `json:"x1,omitempty"`
	Y1         int    `json:"y1,omitempty"`
	X2         int    `json:"x2,omitempty"`
	Y2         int    `json:"y2,omitempty"`
	DurationMS int    `json:"duration_ms,omitempty"`
	Text       string `json:"text,omitempty"`
	Key        string `json:"key,omitempty"`
	Summary    string `json:"summary,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

type mobileTaskStep struct {
	Index  int
	Action mobileTaskAction
}

type adbDevice struct {
	Serial string
	State  string
}

func NewMobileTask(cfg MobileTaskConfig) ports.ToolExecutor {
	client := cfg.LLM
	if client == nil {
		client = llm.NewMockClient()
	}
	return &mobileTaskTool{
		llm:        client,
		adbAddress: strings.TrimSpace(cfg.ADBAddress),
		adbSerial:  strings.TrimSpace(cfg.ADBSerial),
		maxSteps:   cfg.MaxSteps,
	}
}

func (t *mobileTaskTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "mobile_task",
		Version:  "0.1.0",
		Category: "mobile",
		Tags:     []string{"android", "adb", "automation"},
		MaterialCapabilities: ports.ToolMaterialCapabilities{
			Produces:          []string{"text/plain"},
			ProducesArtifacts: []string{"image/png"},
		},
	}
}

func (t *mobileTaskTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name: "mobile_task",
		Description: `Execute a mobile task on a connected Android device via adb.

Provide a task to run on the configured device. If an adb TCP address is configured, the tool will run "adb connect" and operate on that device. Otherwise it uses the configured adb serial or the first local device.

The tool plans and executes the actions internally and streams progress events; the result only includes the execution summary.`,
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"task": {
					Type:        "string",
					Description: "Task instruction for the phone.",
				},
			},
			Required: []string{"task"},
		},
		MaterialCapabilities: ports.ToolMaterialCapabilities{
			Produces:          []string{"text/plain"},
			ProducesArtifacts: []string{"image/png"},
		},
	}
}

func (t *mobileTaskTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	task := strings.TrimSpace(stringArg(call.Arguments, "task"))
	if task == "" {
		err := errors.New("task is required")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	maxSteps := t.maxSteps
	if maxSteps <= 0 {
		maxSteps = defaultMobileMaxSteps
	}

	if _, err := runADBOutput(ctx, "", "start-server"); err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	serial, err := resolveADBSerial(ctx, t.adbAddress, t.adbSerial)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	width, height, err := fetchScreenSize(ctx, serial)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	steps := make([]mobileTaskStep, 0, maxSteps)
	history := make([]string, 0, 5)
	var lastAttachment ports.Attachment
	var lastAttachmentName string

	for i := 0; i < maxSteps; i++ {
		ports.EmitToolProgress(ctx, fmt.Sprintf("mobile_task step %d/%d: capturing screen", i+1, maxSteps), false)
		screenshot, err := captureScreenshot(ctx, serial)
		if err != nil {
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		name := fmt.Sprintf("mobile-step-%d.png", i+1)
		attachment := buildPNGAttachment(name, screenshot, "mobile_task")
		lastAttachment = attachment
		lastAttachmentName = name

		prompt := buildMobileTaskPrompt(task, width, height, i+1, maxSteps, history)
		resp, err := t.llm.Complete(ctx, ports.CompletionRequest{
			Messages: []ports.Message{
				{Role: "system", Content: mobileTaskSystemPrompt},
				{Role: "user", Content: prompt, Attachments: map[string]ports.Attachment{name: attachment}},
			},
			Temperature: 0.2,
			MaxTokens:   600,
		})
		if err != nil {
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}

		action, err := parseMobileTaskAction(resp.Content)
		if err != nil {
			wrapped := fmt.Errorf("parse action: %w", err)
			return &ports.ToolResult{CallID: call.ID, Content: wrapped.Error(), Error: wrapped}, nil
		}

		steps = append(steps, mobileTaskStep{Index: i + 1, Action: action})

		if action.Action == "done" {
			return t.buildResult(ctx, call, task, serial, width, height, steps, lastAttachmentName, lastAttachment, true)
		}

		ports.EmitToolProgress(ctx, fmt.Sprintf("mobile_task step %d/%d: executing action", i+1, maxSteps), false)
		if err := executeMobileAction(ctx, serial, action); err != nil {
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		history = appendHistory(history, summarizeAction(action))
	}

	return t.buildResult(ctx, call, task, serial, width, height, steps, lastAttachmentName, lastAttachment, false)
}

func (t *mobileTaskTool) buildResult(
	ctx context.Context,
	call ports.ToolCall,
	task string,
	serial string,
	width int,
	height int,
	steps []mobileTaskStep,
	attachmentName string,
	attachment ports.Attachment,
	completed bool,
) (*ports.ToolResult, error) {
	status := "incomplete"
	if completed {
		status = "completed"
	}

	content := fmt.Sprintf("Mobile task %s after %d step(s).", status, len(steps))
	if len(steps) > 0 {
		last := steps[len(steps)-1].Action
		if summary := strings.TrimSpace(last.Summary); summary != "" {
			content = fmt.Sprintf("%s Summary: %s", content, summary)
		}
	}

	attachments := map[string]ports.Attachment{}
	if attachmentName != "" {
		attachments[attachmentName] = attachment
	}

	metadata := map[string]any{
		"task":         task,
		"status":       status,
		"adb_serial":   serial,
		"screen_width": width,
		"screen_height": height,
	}

	ports.EmitToolProgress(ctx, content, true)
	return &ports.ToolResult{
		CallID:      call.ID,
		Content:     content,
		Metadata:    metadata,
		Attachments: attachments,
	}, nil
}

func buildMobileTaskPrompt(task string, width, height, step, maxSteps int, history []string) string {
	var builder strings.Builder
	builder.WriteString("Task:\n")
	builder.WriteString(task)
	builder.WriteString("\n\n")
	builder.WriteString(fmt.Sprintf("Screen size: %dx%d\n", width, height))
	builder.WriteString(fmt.Sprintf("Step: %d/%d\n", step, maxSteps))
	builder.WriteString("Recent actions:\n")
	if len(history) == 0 {
		builder.WriteString("- None\n")
	} else {
		for _, item := range history {
			builder.WriteString("- ")
			builder.WriteString(item)
			builder.WriteString("\n")
		}
	}
	builder.WriteString("\nReturn JSON only.")
	return builder.String()
}

func appendHistory(history []string, entry string) []string {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return history
	}
	if len(history) >= 5 {
		history = history[1:]
	}
	return append(history, entry)
}

func summarizeAction(action mobileTaskAction) string {
	switch action.Action {
	case "tap":
		return fmt.Sprintf("tap (%d,%d)", action.X, action.Y)
	case "swipe":
		return fmt.Sprintf("swipe (%d,%d)->(%d,%d)", action.X1, action.Y1, action.X2, action.Y2)
	case "text":
		return fmt.Sprintf("text \"%s\"", action.Text)
	case "key":
		return fmt.Sprintf("key %s", action.Key)
	case "wait":
		return fmt.Sprintf("wait %dms", action.DurationMS)
	case "done":
		return "done"
	default:
		return action.Action
	}
}

func parseMobileTaskAction(raw string) (mobileTaskAction, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return mobileTaskAction{}, errors.New("empty action output")
	}
	payload := extractJSONObject(trimmed)
	var action mobileTaskAction
	if err := json.Unmarshal([]byte(payload), &action); err != nil {
		return mobileTaskAction{}, err
	}
	action.Action = strings.ToLower(strings.TrimSpace(action.Action))
	action.Key = strings.TrimSpace(action.Key)
	action.Text = strings.TrimSpace(action.Text)
	action.Summary = strings.TrimSpace(action.Summary)
	action.Reason = strings.TrimSpace(action.Reason)
	if action.Action == "" {
		return mobileTaskAction{}, errors.New("action is required")
	}
	switch action.Action {
	case "tap":
		return action, nil
	case "swipe":
		return action, nil
	case "text":
		if action.Text == "" {
			return mobileTaskAction{}, errors.New("text is required for text action")
		}
		return action, nil
	case "key":
		if action.Key == "" {
			return mobileTaskAction{}, errors.New("key is required for key action")
		}
		return action, nil
	case "wait":
		return action, nil
	case "done":
		return action, nil
	default:
		return mobileTaskAction{}, fmt.Errorf("unsupported action: %s", action.Action)
	}
}

func extractJSONObject(input string) string {
	start := strings.Index(input, "{")
	end := strings.LastIndex(input, "}")
	if start >= 0 && end > start {
		return input[start : end+1]
	}
	return input
}

func executeMobileAction(ctx context.Context, serial string, action mobileTaskAction) error {
	switch action.Action {
	case "tap":
		_, err := runADBOutput(ctx, serial, "shell", "input", "tap", strconv.Itoa(action.X), strconv.Itoa(action.Y))
		return err
	case "swipe":
		duration := action.DurationMS
		if duration <= 0 {
			duration = 300
		}
		_, err := runADBOutput(ctx, serial, "shell", "input", "swipe", strconv.Itoa(action.X1), strconv.Itoa(action.Y1), strconv.Itoa(action.X2), strconv.Itoa(action.Y2), strconv.Itoa(duration))
		return err
	case "text":
		text := escapeADBText(action.Text)
		_, err := runADBOutput(ctx, serial, "shell", "input", "text", text)
		return err
	case "key":
		keycode := normalizeKeyCode(action.Key)
		if keycode == "" {
			return errors.New("invalid key code")
		}
		_, err := runADBOutput(ctx, serial, "shell", "input", "keyevent", keycode)
		return err
	case "wait":
		duration := action.DurationMS
		if duration <= 0 {
			duration = 800
		}
		time.Sleep(time.Duration(duration) * time.Millisecond)
		return nil
	default:
		return fmt.Errorf("unsupported action: %s", action.Action)
	}
}

func escapeADBText(text string) string {
	replacer := strings.NewReplacer(" ", "%s", "\n", "%s", "\t", "%s")
	return replacer.Replace(text)
}

func normalizeKeyCode(key string) string {
	trimmed := strings.TrimSpace(strings.ToUpper(key))
	if trimmed == "" {
		return ""
	}
	trimmed = strings.ReplaceAll(trimmed, " ", "_")
	if strings.HasPrefix(trimmed, "KEYCODE_") {
		return trimmed
	}
	switch trimmed {
	case "HOME":
		return "KEYCODE_HOME"
	case "BACK":
		return "KEYCODE_BACK"
	case "ENTER":
		return "KEYCODE_ENTER"
	case "APP_SWITCH":
		return "KEYCODE_APP_SWITCH"
	case "POWER":
		return "KEYCODE_POWER"
	case "VOLUME_UP":
		return "KEYCODE_VOLUME_UP"
	case "VOLUME_DOWN":
		return "KEYCODE_VOLUME_DOWN"
	case "VOLUME_MUTE":
		return "KEYCODE_VOLUME_MUTE"
	}
	if _, err := strconv.Atoi(trimmed); err == nil {
		return trimmed
	}
	return "KEYCODE_" + trimmed
}

func resolveADBSerial(ctx context.Context, address, serial string) (string, error) {
	if address != "" {
		if err := connectADB(ctx, address); err != nil {
			return "", err
		}
		state, ok, err := lookupADBDeviceState(ctx, address)
		if err != nil {
			return "", err
		}
		if ok && state == "offline" {
			if _, err := runADBOutput(ctx, "", "disconnect", address); err != nil {
				return "", fmt.Errorf("adb disconnect %s failed: %w", address, err)
			}
			if err := connectADB(ctx, address); err != nil {
				return "", err
			}
			state, ok, err = lookupADBDeviceState(ctx, address)
			if err != nil {
				return "", err
			}
		}
		if !ok {
			return "", fmt.Errorf("adb device %s not found after connect", address)
		}
		if state != "device" {
			return "", fmt.Errorf("adb device %s is %s", address, state)
		}
		return address, nil
	}
	if serial != "" {
		return serial, nil
	}
	output, err := runADBOutput(ctx, "", "devices", "-l")
	if err != nil {
		return "", err
	}
	devices := parseADBDevices(output)
	if len(devices) == 0 {
		return "", errors.New("no adb devices detected")
	}
	return devices[0], nil
}

func parseADBDevices(output string) []string {
	states := parseADBDeviceStates(output)
	devices := make([]string, 0, len(states))
	for _, device := range states {
		if device.State == "device" {
			devices = append(devices, device.Serial)
		}
	}
	return devices
}

func parseADBDeviceStates(output string) []adbDevice {
	var devices []adbDevice
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "List of devices attached") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			continue
		}
		devices = append(devices, adbDevice{Serial: fields[0], State: fields[1]})
	}
	return devices
}

func findADBDeviceState(devices []adbDevice, serial string) (string, bool) {
	for _, device := range devices {
		if device.Serial == serial {
			return device.State, true
		}
	}
	return "", false
}

func lookupADBDeviceState(ctx context.Context, serial string) (string, bool, error) {
	output, err := runADBOutput(ctx, "", "devices", "-l")
	if err != nil {
		return "", false, err
	}
	state, ok := findADBDeviceState(parseADBDeviceStates(output), serial)
	return state, ok, nil
}

func connectADB(ctx context.Context, address string) error {
	if _, err := runADBOutput(ctx, "", "connect", address); err != nil {
		return fmt.Errorf("adb connect %s failed: %w", address, err)
	}
	return nil
}

func fetchScreenSize(ctx context.Context, serial string) (int, int, error) {
	output, err := runADBOutput(ctx, serial, "shell", "wm", "size")
	if err != nil {
		return 0, 0, err
	}
	width, height, err := parseScreenSize(output)
	if err != nil {
		return 0, 0, err
	}
	return width, height, nil
}

func parseScreenSize(output string) (int, int, error) {
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(line, "size") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 2 {
			continue
		}
		size := strings.TrimSpace(parts[len(parts)-1])
		dims := strings.Split(size, "x")
		if len(dims) != 2 {
			continue
		}
		width, err := strconv.Atoi(strings.TrimSpace(dims[0]))
		if err != nil {
			return 0, 0, err
		}
		height, err := strconv.Atoi(strings.TrimSpace(dims[1]))
		if err != nil {
			return 0, 0, err
		}
		return width, height, nil
	}
	return 0, 0, errors.New("unable to parse screen size")
}

func captureScreenshot(ctx context.Context, serial string) ([]byte, error) {
	return runADBBytes(ctx, serial, "exec-out", "screencap", "-p")
}

func buildPNGAttachment(name string, data []byte, source string) ports.Attachment {
	encoded := base64.StdEncoding.EncodeToString(data)
	return ports.Attachment{
		Name:           name,
		MediaType:      "image/png",
		Data:           encoded,
		URI:            fmt.Sprintf("data:image/png;base64,%s", encoded),
		Source:         source,
		Description:    "Mobile screenshot",
		Kind:           "artifact",
		Format:         "png",
		PreviewProfile: "document.image",
	}
}

func runADBOutput(ctx context.Context, serial string, args ...string) (string, error) {
	output, err := runADBBytes(ctx, serial, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func runADBBytes(ctx context.Context, serial string, args ...string) ([]byte, error) {
	cmdArgs := append([]string(nil), args...)
	if serial != "" {
		cmdArgs = append([]string{"-s", serial}, cmdArgs...)
	}
	cmd := exec.CommandContext(ctx, "adb", cmdArgs...)
	output, err := cmd.Output()
	if err == nil {
		return output, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		stderr := strings.TrimSpace(string(exitErr.Stderr))
		if stderr != "" {
			return output, fmt.Errorf("adb %s: %s", strings.Join(cmdArgs, " "), stderr)
		}
	}
	return output, fmt.Errorf("adb %s failed: %w", strings.Join(cmdArgs, " "), err)
}
