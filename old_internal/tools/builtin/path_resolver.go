package builtin

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"alex/internal/utils"
)

// 常见的项目目录名称
var projectDirs = []string{
	"src", "lib", "components", "pages", "app", "docs", "test", "tests",
	"assets", "static", "public", "build", "dist", "config", "scripts",
	"styles", "css", "scss", "less", "images", "fonts", "locales",
	"utils", "helpers", "services", "models", "views", "controllers",
	"middleware", "types", "interfaces", "hooks", "store", "redux",
	"api", "data", "database", "migrations", "seeds", "fixtures",
}

// PathResolver 路径解析器
type PathResolver struct {
	workingDir string
}

// NewPathResolver 创建新的路径解析器
func NewPathResolver(workingDir string) *PathResolver {
	if workingDir == "" {
		workingDir, _ = os.Getwd()
	}
	return &PathResolver{workingDir: workingDir}
}

// isProjectRelativePath 判断是否为项目相对路径
func (pr *PathResolver) isProjectRelativePath(path string) bool {
	if len(path) == 0 || path[0] != '/' {
		return false
	}

	// 移除开头的 /
	cleanPath := path[1:]
	if cleanPath == "" {
		return false
	}

	// 获取第一个路径段
	parts := strings.Split(cleanPath, "/")
	if len(parts) == 0 {
		return false
	}

	firstDir := parts[0]

	// 检查是否是常见的项目目录
	for _, projectDir := range projectDirs {
		if firstDir == projectDir {
			return true
		}
	}

	// 检查是否是配置文件（在根目录）
	if strings.Contains(firstDir, ".") && (strings.HasSuffix(firstDir, ".json") ||
		strings.HasSuffix(firstDir, ".js") || strings.HasSuffix(firstDir, ".ts") ||
		strings.HasSuffix(firstDir, ".yaml") || strings.HasSuffix(firstDir, ".yml") ||
		strings.HasSuffix(firstDir, ".md") || strings.HasSuffix(firstDir, ".txt")) {
		return true
	}

	return false
}

// ResolvePath 解析路径，将相对路径转换为绝对路径
func (pr *PathResolver) ResolvePath(path string) string {
	// 特殊处理：检查是否为项目相对路径
	if pr.isProjectRelativePath(path) {
		// 去掉开头的 / 并基于工作目录解析
		relativePath := path[1:]
		resolved := filepath.Join(pr.workingDir, relativePath)
		return filepath.Clean(resolved)
	}

	// 系统绝对路径直接返回（例如 /usr/local/bin/node 或 C:\）
	if filepath.IsAbs(path) {
		return path
	}

	// 将相对路径基于工作目录解析为绝对路径
	resolved := filepath.Join(pr.workingDir, path)
	return filepath.Clean(resolved)
}

// GetPathResolverFromContext 从context获取路径解析器
func GetPathResolverFromContext(ctx context.Context) *PathResolver {
	if ctx == nil {
		return NewPathResolver("")
	}

	if workingDir, ok := ctx.Value(utils.WorkingDirKey).(string); ok {
		return NewPathResolver(workingDir)
	}

	return NewPathResolver("")
}

// WithWorkingDir 在context中设置工作目录
func WithWorkingDir(ctx context.Context, workingDir string) context.Context {
	return context.WithValue(ctx, utils.WorkingDirKey, workingDir)
}
