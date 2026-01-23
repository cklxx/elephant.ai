package builtin

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

func resolveLocalPath(ctx context.Context, raw string) (string, error) {
	return sanitizePathWithinBase(ctx, raw)
}

func sanitizePathWithinBase(ctx context.Context, raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	resolver := GetPathResolverFromContext(ctx)
	base := resolver.ResolvePath(".")
	baseAbs, err := filepath.Abs(filepath.Clean(base))
	if err != nil {
		return "", fmt.Errorf("failed to resolve base path: %w", err)
	}
	if baseAbs == "" {
		return "", fmt.Errorf("failed to resolve base path")
	}
	if root := defaultWorkingDir(); root != "" && !pathWithinBase(root, baseAbs) {
		baseAbs = root
	}

	candidate := resolver.ResolvePath(trimmed)
	candidateAbs, err := filepath.Abs(filepath.Clean(candidate))
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	rel, err := filepath.Rel(baseAbs, candidateAbs)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path within base: %w", err)
	}
	if rel == "." {
		return baseAbs, nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path must stay within the working directory")
	}

	safe := filepath.Join(baseAbs, rel)
	if !pathWithinBase(baseAbs, safe) {
		return "", fmt.Errorf("path must stay within the working directory")
	}

	return safe, nil
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

	baseResolved, err := filepath.EvalSymlinks(baseClean)
	if err != nil {
		return false
	}
	targetResolved, err := filepath.EvalSymlinks(targetClean)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return false
		}
		parent := filepath.Dir(targetClean)
		parentResolved, err := filepath.EvalSymlinks(parent)
		if err != nil {
			return false
		}
		targetResolved = filepath.Join(parentResolved, filepath.Base(targetClean))
	}

	rel, err := filepath.Rel(baseResolved, targetResolved)
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
