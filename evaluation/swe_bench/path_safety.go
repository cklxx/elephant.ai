package swe_bench

import (
	"fmt"
	"path/filepath"
	"strings"
)

const safeOutputBaseDir = "results"

func sanitizeOutputPath(baseDir, userPath string) (string, error) {
	if baseDir == "" {
		return "", fmt.Errorf("output base directory must not be empty")
	}

	cleaned := filepath.Clean(userPath)
	if cleaned == "" || cleaned == "." || cleaned == ".." {
		return "", fmt.Errorf("output path cannot be empty or traverse parent directories")
	}

	// Absolute paths are allowed but still need basic validation
	if filepath.IsAbs(cleaned) {
		return cleaned, nil
	}

	joined := filepath.Join(baseDir, cleaned)
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("could not resolve absolute base path: %w", err)
	}

	absJoined, err := filepath.Abs(joined)
	if err != nil {
		return "", fmt.Errorf("could not resolve absolute output path: %w", err)
	}

	rel, err := filepath.Rel(absBase, absJoined)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("output path escapes allowed base directory")
	}

	return absJoined, nil
}

func sanitizeDatasetKey(key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("dataset key cannot be empty")
	}
	if key == "." || key == ".." {
		return "", fmt.Errorf("dataset key cannot be a current or parent directory reference")
	}
	if strings.Contains(key, string(filepath.Separator)) {
		return "", fmt.Errorf("dataset key must not contain path separators")
	}

	return key, nil
}
