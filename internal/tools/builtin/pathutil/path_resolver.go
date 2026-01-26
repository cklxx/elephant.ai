package pathutil

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
	rootDir    string
}

// NewPathResolver creates a new path resolver
func NewPathResolver(workingDir string) *PathResolver {
	root := defaultWorkingDir()
	normalized, ok := normalizeWorkingDir(workingDir)
	if !ok {
		normalized = root
	}
	if root != "" && normalized != "" && !pathWithinBase(root, normalized) {
		normalized = root
	}
	return &PathResolver{workingDir: normalized, rootDir: root}
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
	cleaned := filepath.Clean(path)
	if cleaned == "" || cleaned == "." {
		return pr.workingDir
	}

	// Project-relative paths are treated as relative to the working directory.
	if pr.isProjectRelativePath(path) {
		cleaned = strings.TrimPrefix(cleaned, string(filepath.Separator))
		if cleaned == "" || cleaned == "." {
			return pr.workingDir
		}
		resolved := filepath.Join(pr.workingDir, cleaned)
		return filepath.Clean(resolved)
	}

	if filepath.IsAbs(cleaned) {
		// Allow absolute paths only when they are already within the working directory.
		if pr.workingDir != "" && pathWithinBase(pr.workingDir, cleaned) {
			return cleaned
		}
		// Leave absolute paths outside the working directory intact so they can
		// be rejected by downstream containment checks.
		return cleaned
	}

	// Convert relative paths to absolute paths based on working directory.
	resolved := filepath.Join(pr.workingDir, cleaned)
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

func normalizeWorkingDir(workingDir string) (string, bool) {
	if workingDir == "" {
		return "", false
	}
	abs, err := filepath.Abs(filepath.Clean(workingDir))
	if err != nil || abs == "" {
		return "", false
	}
	return abs, true
}

func defaultWorkingDir() string {
	workingDir, err := os.Getwd()
	if err != nil {
		return ""
	}
	abs, err := filepath.Abs(filepath.Clean(workingDir))
	if err != nil {
		return ""
	}
	return abs
}

// DefaultWorkingDir exposes the current working directory used as root.
func DefaultWorkingDir() string {
	return defaultWorkingDir()
}
