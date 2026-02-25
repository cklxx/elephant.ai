package pathutil

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// ResolveLocalPath resolves a local path to an absolute local path.
func ResolveLocalPath(ctx context.Context, raw string) (string, error) {
	return resolveAbsolutePath(ctx, raw)
}

// ResolveLocalPathOrTemp resolves a local path to an absolute local path.
// It is kept for compatibility with existing callers that use temp files.
func ResolveLocalPathOrTemp(ctx context.Context, raw string) (string, error) {
	return resolveAbsolutePath(ctx, raw)
}

// SanitizePathWithinBase resolves a path to an absolute local path.
func SanitizePathWithinBase(ctx context.Context, raw string) (string, error) {
	return resolveAbsolutePath(ctx, raw)
}

func resolveAbsolutePath(ctx context.Context, raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	resolver := GetPathResolverFromContext(ctx)

	candidate := resolver.ResolvePath(trimmed)
	candidateAbs, err := filepath.Abs(filepath.Clean(candidate))
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}
	return candidateAbs, nil
}

// PathWithinBase reports whether target is contained within base after resolving symlinks.
func PathWithinBase(base, target string) bool {
	return pathWithinBase(base, target)
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
		// Walk up from targetClean until we find an existing ancestor,
		// then reconstruct the resolved path with the remaining suffix.
		cur := targetClean
		var suffix []string
		for {
			parent := filepath.Dir(cur)
			suffix = append([]string{filepath.Base(cur)}, suffix...)
			resolved, resolveErr := filepath.EvalSymlinks(parent)
			if resolveErr == nil {
				targetResolved = filepath.Join(append([]string{resolved}, suffix...)...)
				break
			}
			if !errors.Is(resolveErr, fs.ErrNotExist) {
				return false
			}
			if parent == cur {
				// Reached filesystem root without finding existing ancestor.
				return false
			}
			cur = parent
		}
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
