package aliases

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/pathutil"
	"alex/internal/infra/tools/builtin/shared"
)

const (
	// readFileLargeFileThreshold is the byte size above which we use
	// streaming preview when no line range is specified.
	readFileLargeFileThreshold = 50 * 1024 // 50 KB

	// readFileMaxPreviewLines is the number of lines returned as preview
	// for large files.
	readFileMaxPreviewLines = 200

	// readFileSingleLineMaxBytes is the threshold above which a single
	// line is considered degenerate (e.g. minified JS/JSON).
	readFileSingleLineMaxBytes = 50 * 1024 // 50 KB
)

type readFile struct {
	shared.BaseTool
}

func NewReadFile(cfg shared.FileToolConfig) tools.ToolExecutor {
	_ = cfg
	return &readFile{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "read_file",
				Description: "Open repository/workspace source/config files from an absolute path, including exact context windows around suspect code, interface contracts, or proof transitions. Use when the task is read-only inspection/extraction over known files. Do not use for in-place edits (use replace_in_file) and do not use for memory notes/chat history (use memory_search/memory_get).",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"path":       {Type: "string", Description: "Absolute file path"},
						"start_line": {Type: "integer", Description: "Optional start line (0-based)"},
						"end_line":   {Type: "integer", Description: "Optional end line (exclusive)"},
						"sudo":       {Type: "boolean", Description: "Use sudo privileges"},
					},
					Required: []string{"path"},
				},
			},
			ports.ToolMetadata{
				Name:     "read_file",
				Version:  "0.1.0",
				Category: "files",
				Tags:     []string{"file", "read", "inspect", "source", "code", "context", "window", "contract", "proof", "read_only", "inspect_first", "call_path"},
			},
		),
	}
}

func (t *readFile) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	path := strings.TrimSpace(shared.StringArg(call.Arguments, "path"))
	if path == "" {
		err := errors.New("path is required")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	resolved, err := pathutil.ResolveLocalPath(ctx, path)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	hasRange := false
	startLine, startOK := intArgOptional(call.Arguments, "start_line")
	_, endOK := intArgOptional(call.Arguments, "end_line")
	if startOK || endOK {
		hasRange = true
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	fileSize := info.Size()

	// Large file without explicit range: streaming preview.
	if fileSize > readFileLargeFileThreshold && !hasRange {
		return t.readLargeFilePreview(call.ID, resolved, fileSize)
	}

	// Normal path: read entire file.
	content, err := os.ReadFile(resolved)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	output := string(content)
	totalLines := strings.Count(output, "\n") + 1

	metadata := map[string]any{
		"path":            resolved,
		"tool_name":       "read_file",
		"total_lines":     totalLines,
		"file_size_bytes": int(fileSize),
	}

	// Apply line range slicing.
	if hasRange {
		endLine, _ := intArgOptional(call.Arguments, "end_line")
		if startLine < 0 {
			startLine = 0
		}
		lines := strings.Split(output, "\n")
		if startLine >= len(lines) {
			output = ""
		} else {
			if endLine <= 0 || endLine > len(lines) {
				endLine = len(lines)
			}
			if endLine < startLine {
				err := fmt.Errorf("end_line must be >= start_line")
				return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
			}
			output = strings.Join(lines[startLine:endLine], "\n")
			metadata["shown_range"] = [2]int{startLine, endLine}
		}
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  output,
		Metadata: metadata,
	}, nil
}

// readLargeFilePreview uses a scanner to read only the first N lines,
// avoiding loading the full file into memory. It also detects degenerate
// single-line files (e.g. minified JS/JSON).
func (t *readFile) readLargeFilePreview(callID, resolved string, fileSize int64) (*ports.ToolResult, error) {
	f, err := os.Open(resolved)
	if err != nil {
		return &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}, nil
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024) // 1 MB max line length

	var lines []string
	var totalLines int
	var firstLineLen int

	for scanner.Scan() {
		line := scanner.Text()
		totalLines++
		if totalLines == 1 {
			firstLineLen = len(line)
		}
		if totalLines <= readFileMaxPreviewLines {
			lines = append(lines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return &ports.ToolResult{CallID: callID, Content: fmt.Sprintf("error scanning file: %v", err), Error: err}, nil
	}

	metadata := map[string]any{
		"path":            resolved,
		"tool_name":       "read_file",
		"total_lines":     totalLines,
		"file_size_bytes": int(fileSize),
	}

	// Single-line degenerate file (e.g. minified JS/JSON).
	if totalLines <= 2 && firstLineLen > readFileSingleLineMaxBytes {
		metadata["single_line_file"] = true
		desc := fmt.Sprintf(
			"[Large single-line file: %s, %d bytes, %d line(s). "+
				"This appears to be a minified or generated file. "+
				"Use shell_exec with `head -c <bytes>` or `jq` to inspect specific parts, "+
				"or use start_line=0 end_line=1 to force-read the first line (will be truncated).]",
			resolved, fileSize, totalLines,
		)
		return &ports.ToolResult{
			CallID:   callID,
			Content:  desc,
			Metadata: metadata,
		}, nil
	}

	// Normal large file: return preview + hint.
	preview := strings.Join(lines, "\n")
	shownLines := len(lines)

	hint := fmt.Sprintf(
		"\n\n[File preview: showing first %d of %d lines (%d bytes total). "+
			"Use start_line/end_line to read specific sections, e.g. start_line=%d end_line=%d.]",
		shownLines, totalLines, fileSize, shownLines, min(shownLines+readFileMaxPreviewLines, totalLines),
	)
	metadata["shown_range"] = [2]int{0, shownLines}
	metadata["preview_mode"] = true

	return &ports.ToolResult{
		CallID:   callID,
		Content:  preview + hint,
		Metadata: metadata,
	}, nil
}
