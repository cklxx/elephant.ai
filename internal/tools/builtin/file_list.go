package builtin

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// FileListTool implements directory listing functionality
type FileListTool struct{}

func CreateFileListTool() *FileListTool {
	return &FileListTool{}
}

func (t *FileListTool) Name() string {
	return "file_list"
}

func (t *FileListTool) Description() string {
	return "List files and directories in a specified path. Supports recursive listing."
}

func (t *FileListTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to list (directory)",
				"default":     ".",
			},
			"recursive": map[string]interface{}{
				"type":        "boolean",
				"description": "List files recursively",
				"default":     false,
			},
			"show_hidden": map[string]interface{}{
				"type":        "boolean",
				"description": "Show hidden files (starting with .)",
				"default":     false,
			},
			"max_depth": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum depth for recursive listing",
				"default":     3,
			},
		},
	}
}

func (t *FileListTool) Validate(args map[string]interface{}) error {
	validator := NewValidationFramework().
		AddOptionalStringField("path", "Path to list").
		AddOptionalBooleanField("recursive", "List recursively").
		AddOptionalBooleanField("show_hidden", "Show hidden files").
		AddOptionalIntField("max_depth", "Maximum depth", 1, 10)

	return validator.Validate(args)
}

func (t *FileListTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	path := "."
	if p, ok := args["path"]; ok {
		path = p.(string)
	}

	// 解析路径（处理相对路径）
	resolver := GetPathResolverFromContext(ctx)
	resolvedPath := resolver.ResolvePath(path)

	recursive := false
	if r, ok := args["recursive"].(bool); ok {
		recursive = r
	}

	showHidden := false
	if sh, ok := args["show_hidden"].(bool); ok {
		showHidden = sh
	}

	maxDepth := 3
	if md, ok := args["max_depth"]; ok {
		if mdFloat, ok := md.(float64); ok {
			maxDepth = int(mdFloat)
		}
	}

	var files []map[string]interface{}
	var totalSize int64

	if recursive {
		err := filepath.WalkDir(resolvedPath, func(currentPath string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// Calculate depth
			relPath, _ := filepath.Rel(resolvedPath, currentPath)
			depth := strings.Count(relPath, string(filepath.Separator)) + 1
			if relPath == "." {
				depth = 1
			}

			// Skip if beyond max depth
			if depth > maxDepth {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			// Skip hidden files if not requested
			if !showHidden && strings.HasPrefix(d.Name(), ".") && currentPath != resolvedPath {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			info, err := d.Info()
			if err != nil {
				return err
			}

			fileInfo := map[string]interface{}{
				"name":     d.Name(),
				"path":     currentPath,
				"is_dir":   d.IsDir(),
				"size":     info.Size(),
				"modified": info.ModTime().Unix(),
				"depth":    depth,
			}

			files = append(files, fileInfo)
			if !d.IsDir() {
				totalSize += info.Size()
			}

			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("failed to walk directory: %w", err)
		}
	} else {
		entries, err := os.ReadDir(resolvedPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read directory: %w", err)
		}

		for _, entry := range entries {
			// Skip hidden files if not requested
			if !showHidden && strings.HasPrefix(entry.Name(), ".") {
				continue
			}

			info, err := entry.Info()
			if err != nil {
				continue
			}

			fileInfo := map[string]interface{}{
				"name":     entry.Name(),
				"path":     filepath.Join(resolvedPath, entry.Name()),
				"is_dir":   entry.IsDir(),
				"size":     info.Size(),
				"modified": info.ModTime().Unix(),
				"depth":    1,
			}

			files = append(files, fileInfo)
			if !entry.IsDir() {
				totalSize += info.Size()
			}
		}
	}

	// Count files and directories
	fileCount := 0
	dirCount := 0
	for _, file := range files {
		if file["is_dir"].(bool) {
			dirCount++
		} else {
			fileCount++
		}
	}

	// Build content
	var contentBuilder strings.Builder
	contentBuilder.WriteString(fmt.Sprintf("Directory listing for: %s\n", path))
	contentBuilder.WriteString(fmt.Sprintf("Total: %d files, %d directories\n", fileCount, dirCount))

	if totalSize > 0 {
		contentBuilder.WriteString(fmt.Sprintf("Total size: %d bytes\n", totalSize))
	}

	contentBuilder.WriteString("\nFiles and directories:\n")

	for _, file := range files {
		indent := strings.Repeat("  ", file["depth"].(int)-1)
		if file["is_dir"].(bool) {
			contentBuilder.WriteString(fmt.Sprintf("%s📁 %s/\n", indent, file["name"]))
		} else {
			contentBuilder.WriteString(fmt.Sprintf("%s📄 %s (%d bytes)\n", indent, file["name"], file["size"]))
		}
	}

	return &ToolResult{
		Content: contentBuilder.String(),
		Data: map[string]interface{}{
			"path":          path,
			"resolved_path": resolvedPath,
			"files":         files,
			"file_count":    fileCount,
			"dir_count":     dirCount,
			"total_size":    totalSize,
			"recursive":     recursive,
			"show_hidden":   showHidden,
			"content":       contentBuilder.String(),
		},
	}, nil
}
