package formatter

import (
	"fmt"
	"strings"
	"testing"
)

func TestToolFormatterFormatToolCall(t *testing.T) {
	tf := NewToolFormatter()

	cases := []struct {
		name     string
		toolName string
		args     map[string]any
		wants    []string
	}{
		{
			name:     "code execute with code",
			toolName: "code_execute",
			args: map[string]any{
				"language": "python",
				"code":     "print('hello')\nprint('world')",
			},
			wants: []string{"language=python", "lines=2", "chars="},
		},
		{
			name:     "execute_code with code",
			toolName: "execute_code",
			args: map[string]any{
				"language": "python",
				"code":     "print('hello')\nprint('world')",
			},
			wants: []string{"language=python", "lines=2", "chars="},
		},
		{
			name:     "code execute without code",
			toolName: "code_execute",
			args:     map[string]any{"language": "go"},
			wants:    []string{"code_execute(language=go"},
		},
		{
			name:     "bash",
			toolName: "bash",
			args:     map[string]any{"command": "go test ./..."},
			wants:    []string{"bash(command=go test ./...)"},
		},
		{
			name:     "file_read with window",
			toolName: "file_read",
			args: map[string]any{
				"file_path": "README.md",
				"offset":    float64(10),
				"limit":     5,
			},
			wants: []string{"file_read(limit=5", "offset=10", "path=README.md"},
		},
		{
			name:     "read_file canonical with range",
			toolName: "read_file",
			args: map[string]any{
				"path":       "/workspace/README.md",
				"start_line": 10,
				"end_line":   15,
			},
			wants: []string{"read_file(limit=5", "offset=10", "path=/workspace/README.md"},
		},
		{
			name:     "file_edit",
			toolName: "file_edit",
			args: map[string]any{
				"file_path":  "main.go",
				"old_string": "fmt.Println('old')",
				"new_string": "fmt.Println('new')",
			},
			wants: []string{"file_edit(", "path=main.go", "old_lines=1", "new_lines=1"},
		},
		{
			name:     "replace_in_file canonical",
			toolName: "replace_in_file",
			args: map[string]any{
				"path":    "main.go",
				"old_str": "fmt.Println('old')",
				"new_str": "fmt.Println('new')",
			},
			wants: []string{"replace_in_file(", "path=main.go", "old_lines=1", "new_lines=1"},
		},
		{
			name:     "file_write",
			toolName: "file_write",
			args: map[string]any{
				"file_path": "main.go",
				"content":   "line1\nline2",
			},
			wants: []string{"file_write(", "path=main.go", "lines=2"},
		},
		{
			name:     "write_file canonical",
			toolName: "write_file",
			args: map[string]any{
				"path":    "/workspace/main.go",
				"content": "line1\nline2",
			},
			wants: []string{"write_file(", "path=/workspace/main.go", "lines=2"},
		},
		{
			name:     "grep",
			toolName: "grep",
			args: map[string]any{
				"pattern": strings.Repeat("a", 50),
				"path":    "./internal",
			},
			wants: []string{"grep(", "pattern="},
		},
		{
			name:     "find",
			toolName: "find",
			args: map[string]any{
				"pattern": "*.go",
				"path":    "./cmd",
			},
			wants: []string{"find(", "pattern=*.go", "path=./cmd"},
		},
		{
			name:     "web_search",
			toolName: "web_search",
			args: map[string]any{
				"query":       strings.Repeat("search", 15),
				"max_results": 3,
			},
			wants: []string{"web_search(", "max_results=3", "query="},
		},
		{
			name:     "web_fetch",
			toolName: "web_fetch",
			args: map[string]any{
				"url": "https://example.com/" + strings.Repeat("path", 20),
			},
			wants: []string{"web_fetch(url=https://example.com/"},
		},
		{
			name:     "todo_update",
			toolName: "todo_update",
			args:     map[string]any{},
			wants:    []string{"todo_update"},
		},
		{
			name:     "todo_read",
			toolName: "todo_read",
			args:     map[string]any{"path": "TODO.md"},
			wants:    []string{"todo_read(path=TODO.md)"},
		},
		{
			name:     "subagent with tasks",
			toolName: "subagent",
			args: map[string]any{
				"prompt": "investigate auth module",
			},
			wants: []string{"subagent(prompt=investigate auth module"},
		},
		{
			name:     "subagent without tasks",
			toolName: "subagent",
			args:     map[string]any{},
			wants:    []string{"subagent"},
		},
		{
			name:     "final summary",
			toolName: "final",
			args: map[string]any{
				"answer":     "全部交付完成",
				"highlights": []any{"3 subagents", "rollout ready"},
			},
			wants: []string{"final(", "answer=", "highlights="},
		},
		{
			name:     "unknown",
			toolName: "custom_tool",
			args: map[string]any{
				"flag":  true,
				"count": 10,
			},
			wants: []string{"custom_tool(", "count=10", "flag=true"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tf.FormatToolCall(tc.toolName, tc.args)
			for _, want := range tc.wants {
				if !strings.Contains(got, want) {
					t.Fatalf("expected %q to contain %q", got, want)
				}
			}
		})
	}
}

func TestToolFormatterFormatToolResult(t *testing.T) {
	tf := NewToolFormatter()

	todoContent := strings.Join([]string{
		"Todo List:",
		"## Pending",
		"- [ ] write tests",
		"## Completed",
		"- [x] refactor",
	}, "\n")

	longFetch := strings.Repeat("data", 260) // > 1000 chars

	cases := []struct {
		name     string
		toolName string
		content  string
		success  bool
		wants    []string
	}{
		{
			name:     "failure",
			toolName: "bash",
			success:  false,
			wants:    []string{"✗ failed"},
		},
		{
			name:     "code_execute",
			toolName: "code_execute",
			success:  true,
			content: strings.Join([]string{
				"Execution time: 10ms",
				"stdout:",
				"result line",
			}, "\n"),
			wants: []string{"Success in 10ms", "result line"},
		},
		{
			name:     "execute_code canonical",
			toolName: "execute_code",
			success:  true,
			content: strings.Join([]string{
				"Execution time: 10ms",
				"stdout:",
				"result line",
			}, "\n"),
			wants: []string{"Success in 10ms", "result line"},
		},
		{
			name:     "bash single line",
			toolName: "bash",
			success:  true,
			content:  bashContent("echo", "ok", "", 0),
			wants:    []string{"→ ok"},
		},
		{
			name:     "shell_exec canonical single line",
			toolName: "shell_exec",
			success:  true,
			content:  bashContent("echo", "ok", "", 0),
			wants:    []string{"→ ok"},
		},
		{
			name:     "bash multi line",
			toolName: "bash",
			success:  true,
			content:  bashContent("echo", "line1\nline2", "", 0),
			wants:    []string{"stdout 2 lines"},
		},
		{
			name:     "bash stderr",
			toolName: "bash",
			success:  true,
			content:  bashContent("cmd", "", "permission denied", 1),
			wants:    []string{"exit 1", "stderr: permission denied"},
		},
		{
			name:     "file_read",
			toolName: "file_read",
			success:  true,
			content:  "package main\n\nfunc main() {}\n",
			wants:    []string{"file preview"},
		},
		{
			name:     "read_file canonical",
			toolName: "read_file",
			success:  true,
			content:  "package main\n\nfunc main() {}\n",
			wants:    []string{"file preview"},
		},
		{
			name:     "file_write",
			toolName: "file_write",
			success:  true,
			content:  "file created",
			wants:    []string{"File created"},
		},
		{
			name:     "write_file canonical",
			toolName: "write_file",
			success:  true,
			content:  "Wrote 20 bytes to /workspace/demo.md",
			wants:    []string{"File written"},
		},
		{
			name:     "file_edit",
			toolName: "file_edit",
			success:  true,
			content:  "1 replacement",
			wants:    []string{"1 replacement"},
		},
		{
			name:     "replace_in_file canonical",
			toolName: "replace_in_file",
			success:  true,
			content:  "Replaced 1 occurrence(s) in /workspace/main.go",
			wants:    []string{"Replacements made"},
		},
		{
			name:     "grep matches",
			toolName: "grep",
			success:  true,
			content:  "match1\nmatch2",
			wants:    []string{"2 matches"},
		},
		{
			name:     "find files",
			toolName: "find",
			success:  true,
			content:  "found/file.go",
			wants:    []string{"1 file"},
		},
		{
			name:     "list files",
			toolName: "list_files",
			success:  true,
			content: strings.Join([]string{
				"[DIR] pkg",
				"[DIR] cmd",
				"[FILE] main.go 1KB",
				"[FILE] go.mod 2KB",
			}, "\n"),
			wants: []string{"dirs", "files"},
		},
		{
			name:     "list_dir canonical",
			toolName: "list_dir",
			success:  true,
			content:  `{"directory_count":1,"file_count":2,"files":[{"name":"pkg","is_directory":true},{"name":"main.go","is_directory":false},{"name":"go.mod","is_directory":false}]}`,
			wants:    []string{"1 dirs", "2 files", "sample"},
		},
		{
			name:     "web_search",
			toolName: "web_search",
			success:  true,
			content: strings.Join([]string{
				"Title: Go 1.22",
				"http://golang.org",
				"Title: Release",
				"http://example.com",
			}, "\n"),
			wants: []string{"search results"},
		},
		{
			name:     "web_fetch",
			toolName: "web_fetch",
			success:  true,
			content:  longFetch,
			wants:    []string{"Fetched"},
		},
		{
			name:     "todo",
			toolName: "todo_update",
			success:  true,
			content:  todoContent,
			wants:    []string{"Todo List"},
		},
		{
			name:     "subagent",
			toolName: "subagent",
			success:  true,
			content:  "Summary\nSuccess: 2 tasks\nFailed: 1 tasks",
			wants:    []string{"Success: 2 tasks"},
		},
		{
			name:     "final",
			toolName: "final",
			success:  true,
			content:  "All done.",
			wants:    []string{"All done."},
		},
		{
			name:     "default",
			toolName: "custom",
			success:  true,
			content:  "some generic output that should be trimmed because it is quite long indeed",
			wants:    []string{"generic output"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tf.FormatToolResult(tc.toolName, tc.content, tc.success)
			for _, want := range tc.wants {
				if !strings.Contains(got, want) {
					t.Fatalf("expected %q to contain %q", got, want)
				}
			}
		})
	}
}

func bashContent(command, stdout, stderr string, exitCode int) string {
	return fmt.Sprintf(`{"command":%q,"stdout":%q,"stderr":%q,"exit_code":%d}`, command, stdout, stderr, exitCode)
}
