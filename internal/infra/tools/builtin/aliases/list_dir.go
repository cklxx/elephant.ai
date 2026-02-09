package aliases

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/sandbox"
	"alex/internal/infra/tools/builtin/pathutil"
	"alex/internal/infra/tools/builtin/shared"
)

type listDir struct {
	shared.BaseTool
}

func NewListDir(cfg shared.FileToolConfig) tools.ToolExecutor {
	_ = cfg
	return &listDir{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "list_dir",
				Description: "List files/folders under a directory for repository topology/module ownership discovery (absolute paths). Use this before selecting candidate files; do not use for inside-file content search or artifact handoff inventory.",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"path":                {Type: "string", Description: "Absolute directory path"},
						"recursive":           {Type: "boolean", Description: "List recursively"},
						"show_hidden":         {Type: "boolean", Description: "Include hidden files"},
						"file_types":          {Type: "array", Description: "Filter by file extensions", Items: &ports.Property{Type: "string"}},
						"max_depth":           {Type: "integer", Description: "Maximum depth for recursive listing"},
						"include_size":        {Type: "boolean", Description: "Include file size"},
						"include_permissions": {Type: "boolean", Description: "Include permissions"},
						"sort_by":             {Type: "string", Description: "Sort by: name, size, modified, type"},
						"sort_desc":           {Type: "boolean", Description: "Sort in descending order"},
					},
					Required: []string{"path"},
				},
			},
			ports.ToolMetadata{
				Name:     "list_dir",
				Version:  "0.1.0",
				Category: "files",
				Tags:     []string{"file", "list", "directory", "folder", "workspace", "topology", "discovery"},
			},
		),
	}
}

func (t *listDir) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	path := strings.TrimSpace(shared.StringArg(call.Arguments, "path"))
	if path == "" {
		err := errors.New("path is required")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	resolved, err := pathutil.ResolveLocalPath(ctx, path)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	recursive, _ := boolArgOptional(call.Arguments, "recursive")
	showHidden, _ := boolArgOptional(call.Arguments, "show_hidden")
	fileTypes := normalizeFileTypes(shared.StringSliceArg(call.Arguments, "file_types"))
	maxDepth, _ := intArgOptional(call.Arguments, "max_depth")
	includeSize, _ := boolArgOptional(call.Arguments, "include_size")
	includePerms, _ := boolArgOptional(call.Arguments, "include_permissions")
	sortBy := strings.TrimSpace(shared.StringArg(call.Arguments, "sort_by"))
	sortDesc, _ := boolArgOptional(call.Arguments, "sort_desc")

	entries, err := listDirectoryEntries(resolved, recursive, maxDepth, showHidden)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	files := make([]sandbox.FileInfo, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		name := entry.Name()
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}
		isDir := entry.IsDir()
		ext := strings.ToLower(filepath.Ext(name))
		if !isDir && len(fileTypes) > 0 && !fileTypes[ext] {
			continue
		}

		fullPath := filepath.Join(resolved, name)
		if withPath, ok := entry.(interface{ Path() string }); ok {
			fullPath = withPath.Path()
		}

		fileInfo := sandbox.FileInfo{
			Name:        name,
			Path:        fullPath,
			IsDirectory: isDir,
		}
		if includeSize && !isDir {
			size := info.Size()
			fileInfo.Size = &size
		}
		if includePerms {
			perms := info.Mode().Perm().String()
			fileInfo.Permissions = perms
		}
		if mod := info.ModTime(); !mod.IsZero() {
			fileInfo.ModifiedTime = mod.Format(time.RFC3339)
		}
		if ext != "" {
			fileInfo.Extension = ext
		}

		files = append(files, fileInfo)
	}

	sortFileInfos(files, sortBy, sortDesc)

	result := sandbox.FileListResult{
		Path:           resolved,
		Files:          files,
		TotalCount:     len(files),
		DirectoryCount: countDirectories(files),
		FileCount:      countFiles(files),
	}

	payloadJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  string(payloadJSON),
		Metadata: map[string]any{"path": resolved},
	}, nil
}

func listDirectoryEntries(root string, recursive bool, maxDepth int, showHidden bool) ([]fs.DirEntry, error) {
	if !recursive {
		entries, err := os.ReadDir(root)
		return entries, err
	}
	if maxDepth <= 0 {
		maxDepth = 32
	}
	var entries []fs.DirEntry
	rootDepth := depthOf(root)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		if !showHidden && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if depthOf(path)-rootDepth > maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		entries = append(entries, &dirEntryWithPath{DirEntry: d, path: path})
		return nil
	})
	return entries, err
}

func depthOf(path string) int {
	clean := filepath.Clean(path)
	return len(strings.Split(clean, string(filepath.Separator)))
}

func normalizeFileTypes(types []string) map[string]bool {
	if len(types) == 0 {
		return nil
	}
	out := make(map[string]bool, len(types))
	for _, item := range types {
		trimmed := strings.TrimSpace(strings.ToLower(item))
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, ".") {
			trimmed = "." + trimmed
		}
		out[trimmed] = true
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func sortFileInfos(files []sandbox.FileInfo, sortBy string, desc bool) {
	if len(files) < 2 {
		return
	}
	sortBy = strings.ToLower(strings.TrimSpace(sortBy))
	sort.Slice(files, func(i, j int) bool {
		a, b := files[i], files[j]
		var less bool
		switch sortBy {
		case "size":
			less = safeSize(a) < safeSize(b)
		case "modified":
			less = a.ModifiedTime < b.ModifiedTime
		case "type":
			less = a.Extension < b.Extension
		default:
			less = strings.ToLower(a.Name) < strings.ToLower(b.Name)
		}
		if desc {
			return !less
		}
		return less
	})
}

func safeSize(info sandbox.FileInfo) int64 {
	if info.Size == nil {
		return 0
	}
	return *info.Size
}

func countDirectories(files []sandbox.FileInfo) int {
	count := 0
	for _, info := range files {
		if info.IsDirectory {
			count++
		}
	}
	return count
}

func countFiles(files []sandbox.FileInfo) int {
	count := 0
	for _, info := range files {
		if !info.IsDirectory {
			count++
		}
	}
	return count
}

type dirEntryWithPath struct {
	fs.DirEntry
	path string
}

func (d *dirEntryWithPath) Path() string {
	return d.path
}
