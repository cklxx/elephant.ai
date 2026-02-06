package pathutil

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ResolveLocalPath resolves a local path and ensures it stays within the working directory.
func ResolveLocalPath(ctx context.Context, raw string) (string, error) {
	return SanitizePathWithinBase(ctx, raw)
}

// ResolveLocalPathOrTemp resolves a local path and ensures it stays within the
// working directory or a temp directory (os.TempDir, /tmp, /var/tmp, ...).
//
// This is intended for cases where callers need to read artifacts generated in
// system temp directories while keeping the default local-path guard strict.
func ResolveLocalPathOrTemp(ctx context.Context, raw string) (string, error) {
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

	if safe, ok, err := sanitizeWithinBase(baseAbs, candidateAbs); err != nil {
		return "", err
	} else if ok {
		return safe, nil
	}

	for _, root := range allowedTempRoots() {
		if root == "" {
			continue
		}
		if pathWithinBase(root, candidateAbs) {
			return candidateAbs, nil
		}
	}

	return "", fmt.Errorf("path must stay within the working directory or a temp directory")
}

// SanitizePathWithinBase validates that a path stays within the working directory.
func SanitizePathWithinBase(ctx context.Context, raw string) (string, error) {
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

	safe, ok, err := sanitizeWithinBase(baseAbs, candidateAbs)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("path must stay within the working directory")
	}

	return safe, nil
}

func sanitizeWithinBase(baseAbs, candidateAbs string) (string, bool, error) {
	rel, err := filepath.Rel(baseAbs, candidateAbs)
	if err != nil {
		return "", false, fmt.Errorf("failed to resolve path within base: %w", err)
	}
	if rel == "." {
		return baseAbs, true, nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false, nil
	}

	safe := filepath.Join(baseAbs, rel)
	if !pathWithinBase(baseAbs, safe) {
		return "", false, nil
	}

	return safe, true, nil
}

func allowedTempRoots() []string {
	seen := make(map[string]struct{}, 8)
	var roots []string

	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		clean := filepath.Clean(value)
		if clean == "" || clean == "." {
			return
		}
		if _, ok := seen[clean]; ok {
			return
		}
		seen[clean] = struct{}{}
		roots = append(roots, clean)
	}

	add(os.TempDir())
	if filepath.Separator == '/' {
		add("/tmp")
		add("/var/tmp")
		add("/private/tmp")
	}

	return roots
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
