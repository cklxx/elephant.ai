package builtin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// Context key for working directory
type contextKey string

const WorkingDirKey contextKey = "working_dir"

// Common project directory names
var projectDirs = []string{
	"src", "lib", "components", "pages", "app", "docs", "test", "tests",
	"assets", "static", "public", "build", "dist", "config", "scripts",
	"styles", "css", "scss", "less", "images", "fonts", "locales",
	"utils", "helpers", "services", "models", "views", "controllers",
	"middleware", "types", "interfaces", "hooks", "store", "redux",
	"api", "data", "database", "migrations", "seeds", "fixtures",
	"internal", "pkg", "cmd",
}

// PathResolver resolves file paths
type PathResolver struct {
	workingDir string
}

// NewPathResolver creates a new path resolver
func NewPathResolver(workingDir string) *PathResolver {
	if workingDir == "" {
		workingDir, _ = os.Getwd()
	}
	return &PathResolver{workingDir: workingDir}
}

// isProjectRelativePath determines if a path is project-relative
func (pr *PathResolver) isProjectRelativePath(path string) bool {
	if len(path) == 0 || path[0] != '/' {
		return false
	}

	// Remove leading /
	cleanPath := path[1:]
	if cleanPath == "" {
		return false
	}

	// Get first path segment
	parts := strings.Split(cleanPath, "/")
	if len(parts) == 0 {
		return false
	}

	firstDir := parts[0]

	// Check if it's a common project directory
	for _, projectDir := range projectDirs {
		if firstDir == projectDir {
			return true
		}
	}

	// Check if it's a config file in root directory
	if strings.Contains(firstDir, ".") && (strings.HasSuffix(firstDir, ".json") ||
		strings.HasSuffix(firstDir, ".js") || strings.HasSuffix(firstDir, ".ts") ||
		strings.HasSuffix(firstDir, ".yaml") || strings.HasSuffix(firstDir, ".yml") ||
		strings.HasSuffix(firstDir, ".md") || strings.HasSuffix(firstDir, ".txt") ||
		strings.HasSuffix(firstDir, ".go")) {
		return true
	}

	return false
}

// ResolvePath resolves a path, converting relative paths to absolute paths
func (pr *PathResolver) ResolvePath(path string) string {
	// Special handling: check if it's a project-relative path
	if pr.isProjectRelativePath(path) {
		// Remove leading / and resolve based on working directory
		relativePath := path[1:]
		resolved := filepath.Join(pr.workingDir, relativePath)
		return filepath.Clean(resolved)
	}

	// System absolute paths are returned directly (e.g., /usr/local/bin/node or C:\)
	if filepath.IsAbs(path) {
		return path
	}

	// Convert relative paths to absolute paths based on working directory
	resolved := filepath.Join(pr.workingDir, path)
	return filepath.Clean(resolved)
}

// GetPathResolverFromContext retrieves the path resolver from context
func GetPathResolverFromContext(ctx context.Context) *PathResolver {
	if ctx == nil {
		return NewPathResolver("")
	}

	if workingDir, ok := ctx.Value(WorkingDirKey).(string); ok {
		return NewPathResolver(workingDir)
	}

	return NewPathResolver("")
}

// WithWorkingDir sets the working directory in context
func WithWorkingDir(ctx context.Context, workingDir string) context.Context {
	return context.WithValue(ctx, WorkingDirKey, workingDir)
}
