package builtin

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

func resolveLocalPath(ctx context.Context, raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	resolver := GetPathResolverFromContext(ctx)
	resolved := resolver.ResolvePath(trimmed)
	base := resolver.ResolvePath(".")

	if !pathWithinBase(base, resolved) {
		return "", fmt.Errorf("path must stay within the working directory")
	}

	return resolved, nil
}

func pathWithinBase(base, target string) bool {
	baseClean, err := filepath.Abs(filepath.Clean(base))
	if err != nil {
		return false
	}
	targetClean, err := filepath.Abs(filepath.Clean(target))
	if err != nil {
		return false
	}

	rel, err := filepath.Rel(baseClean, targetClean)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return false
	}
	return true
}
