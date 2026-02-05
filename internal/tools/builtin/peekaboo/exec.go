package peekaboo

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/tools/builtin/shared"
)

const (
	defaultTimeoutSeconds  = 120
	defaultMaxAttachments  = 8
	tempDirPrefix          = "alex-peekaboo-"
	peekabooInstallHintBrew = "brew install steipete/tap/peekaboo"
)

type runner interface {
	Run(ctx context.Context, binary string, args []string, env []string, dir string) (stdout, stderr []byte, exitCode int, err error)
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, binary string, args []string, env []string, dir string) ([]byte, []byte, int, error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = env
	cmd.Dir = dir

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	runErr := cmd.Run()
	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	} else if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	return stdoutBuf.Bytes(), stderrBuf.Bytes(), exitCode, runErr
}

type peekabooExec struct {
	shared.BaseTool
	binary string
	mu     sync.Mutex
	runner runner
}

func NewPeekabooExec() tools.ToolExecutor {
	return newPeekabooExec("peekaboo", execRunner{})
}

func newPeekabooExec(binary string, runner runner) *peekabooExec {
	return &peekabooExec{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name: "peekaboo_exec",
				Description: `Run the Peekaboo macOS automation CLI (no MCP).

Notes:
- macOS 15+ only.
- The tool runs Peekaboo in a working directory (temp dir by default) and embeds any images generated in that directory as tool attachments.
- Prefer relative paths (or omit --path) so outputs land in the working directory and can be attached.`,
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"args": {
							Type:        "array",
							Description: "Args passed to peekaboo (binary not included)",
							Items:       &ports.Property{Type: "string"},
						},
						"cwd": {
							Type:        "string",
							Description: "Working directory for the command (defaults to a temp dir)",
						},
						"timeout_seconds": {
							Type:        "number",
							Description: "Process timeout in seconds (default: 120)",
						},
						"max_attachments": {
							Type:        "number",
							Description: "Maximum number of image files to attach (default: 8)",
						},
					},
					Required: []string{"args"},
				},
				MaterialCapabilities: ports.ToolMaterialCapabilities{
					Produces:          []string{"text/plain"},
					ProducesArtifacts: []string{"image/png", "image/jpeg", "image/gif", "image/webp"},
				},
			},
			ports.ToolMetadata{
				Name:      "peekaboo_exec",
				Version:   "0.1.0",
				Category:  "automation",
				Tags:      []string{"peekaboo", "macos", "gui", "automation"},
				Dangerous: true,
				MaterialCapabilities: ports.ToolMaterialCapabilities{
					Produces:          []string{"text/plain"},
					ProducesArtifacts: []string{"image/png", "image/jpeg", "image/gif", "image/webp"},
				},
			},
		),
		binary: strings.TrimSpace(binary),
		runner: runner,
	}
}

func (t *peekabooExec) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	args := shared.StringSliceArg(call.Arguments, "args")
	if len(args) == 0 {
		err := fmt.Errorf("args is required and must be a non-empty string array")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	timeoutSeconds := defaultTimeoutSeconds
	if v, ok := shared.IntArg(call.Arguments, "timeout_seconds"); ok && v > 0 {
		timeoutSeconds = v
	}

	maxAttachments := defaultMaxAttachments
	if v, ok := shared.IntArg(call.Arguments, "max_attachments"); ok && v >= 0 {
		maxAttachments = v
	}

	cwd := strings.TrimSpace(shared.StringArg(call.Arguments, "cwd"))
	tempDir := ""
	if cwd == "" {
		dir, err := os.MkdirTemp("", tempDirPrefix+"*")
		if err != nil {
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		tempDir = dir
		cwd = dir
	}

	resolvedBinary, err := exec.LookPath(t.binary)
	if err != nil {
		msg := fmt.Sprintf("peekaboo not found in PATH. Install with:\n\n%s\n\nThen grant permissions:\n- peekaboo permissions check\n- peekaboo permissions request screen-recording\n- peekaboo permissions request accessibility", peekabooInstallHintBrew)
		return &ports.ToolResult{CallID: call.ID, Content: msg, Error: fmt.Errorf("peekaboo not found: %w", err)}, nil
	}

	runCtx := ctx
	cancel := func() {}
	if timeoutSeconds > 0 {
		runCtx, cancel = context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	}
	defer cancel()

	env := os.Environ()

	t.mu.Lock()
	defer t.mu.Unlock()

	var cleanupErr error
	if tempDir != "" {
		defer func() {
			if err := os.RemoveAll(tempDir); err != nil && cleanupErr == nil {
				cleanupErr = err
			}
		}()
	}

	stdout, stderr, exitCode, runErr := t.runner.Run(runCtx, resolvedBinary, args, env, cwd)

	metadata := map[string]any{
		"binary":          resolvedBinary,
		"args":            append([]string(nil), args...),
		"cwd":             cwd,
		"timeout_seconds": timeoutSeconds,
		"exit_code":       exitCode,
		"stdout":          string(stdout),
		"stderr":          string(stderr),
	}
	if tempDir != "" {
		metadata["temp_dir"] = true
	}

	parsedJSON, jsonParseErr := tryParseJSON(stdout)
	if parsedJSON != nil {
		metadata["json"] = parsedJSON
	} else if jsonParseErr != "" {
		metadata["json_parse_error"] = jsonParseErr
	}

	generatedFiles, attachments, attachedFiles, truncated, attachErrs := scanAndAttachImages(cwd, maxAttachments)
	if len(generatedFiles) > 0 {
		metadata["generated_files"] = generatedFiles
		metadata["attached_files"] = attachedFiles
		metadata["truncated"] = truncated
	}
	if len(attachErrs) > 0 {
		metadata["attachment_errors"] = attachErrs
	}
	if cleanupErr != nil {
		metadata["cleanup_error"] = cleanupErr.Error()
	}

	content := buildContent(exitCode, stdout, stderr, attachedFiles, truncated)
	if len(attachErrs) > 0 {
		content = content + "\n\nWarning: some attachments could not be read. See metadata.attachment_errors."
	}

	return &ports.ToolResult{
		CallID:      call.ID,
		Content:     content,
		Error:       runErr,
		Metadata:    metadata,
		Attachments: attachments,
	}, nil
}

func tryParseJSON(stdout []byte) (any, string) {
	trimmed := bytes.TrimSpace(stdout)
	if len(trimmed) == 0 {
		return nil, ""
	}
	if trimmed[0] != '{' && trimmed[0] != '[' {
		return nil, ""
	}
	var parsed any
	if err := json.Unmarshal(trimmed, &parsed); err != nil {
		return nil, err.Error()
	}
	return parsed, ""
}

func scanAndAttachImages(dir string, maxAttachments int) (generated []string, attachments map[string]ports.Attachment, attached []string, truncated bool, attachErrs map[string]string) {
	files, err := scanImageFiles(dir)
	if err != nil {
		return nil, nil, nil, false, map[string]string{"scan": err.Error()}
	}
	generated = append([]string(nil), files...)
	if maxAttachments == 0 || len(files) == 0 {
		return generated, map[string]ports.Attachment{}, nil, len(files) > 0, nil
	}

	limit := maxAttachments
	if limit < 0 {
		limit = defaultMaxAttachments
	}
	if limit > len(files) {
		limit = len(files)
	}
	truncated = len(files) > limit

	attachments = make(map[string]ports.Attachment, limit)
	attached = make([]string, 0, limit)
	for _, name := range files[:limit] {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			if attachErrs == nil {
				attachErrs = make(map[string]string)
			}
			attachErrs[name] = err.Error()
			continue
		}
		mimeType, ok := mimeTypeForFile(name)
		if !ok {
			continue
		}
		attachments[name] = ports.Attachment{
			Name:      name,
			MediaType: mimeType,
			Data:      base64.StdEncoding.EncodeToString(data),
			Source:    "peekaboo_exec",
		}
		attached = append(attached, name)
	}

	return generated, attachments, attached, truncated, attachErrs
}

func scanImageFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if _, ok := mimeTypeForFile(name); !ok {
			continue
		}
		files = append(files, name)
	}
	sort.Strings(files)
	return files, nil
}

func mimeTypeForFile(name string) (string, bool) {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png":
		return "image/png", true
	case ".jpg", ".jpeg":
		return "image/jpeg", true
	case ".gif":
		return "image/gif", true
	case ".webp":
		return "image/webp", true
	default:
		return "", false
	}
}

func buildContent(exitCode int, stdout, stderr []byte, attached []string, truncated bool) string {
	if len(attached) == 0 {
		out := strings.TrimSpace(string(stdout))
		if out == "" {
			out = strings.TrimSpace(string(stderr))
		}
		if out == "" {
			return fmt.Sprintf("Peekaboo executed (exit_code=%d) with no output.", exitCode)
		}
		return fmt.Sprintf("Peekaboo executed (exit_code=%d). Output:\n%s", exitCode, shared.ContentSnippet(out, 2000))
	}

	placeholders := make([]string, 0, len(attached))
	for _, name := range attached {
		placeholders = append(placeholders, fmt.Sprintf("[%s]", name))
	}
	content := fmt.Sprintf("Peekaboo executed (exit_code=%d). Attached: %s.", exitCode, strings.Join(placeholders, ", "))
	if truncated {
		content = content + " (truncated)"
	}
	return content
}
