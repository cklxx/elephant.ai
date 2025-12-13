package agent_eval

import (
	"fmt"
	"path/filepath"
	"strings"
)

const defaultOutputBaseDir = "."

func sanitizeOutputPath(baseDir, path string) (string, error) {
	if baseDir == "" {
		return "", fmt.Errorf("output base directory must not be empty")
	}

	cleaned := filepath.Clean(path)
	if cleaned == "" || cleaned == "." || cleaned == ".." {
		return "", fmt.Errorf("output path cannot be current or parent directory")
	}

	segments := strings.Split(cleaned, string(filepath.Separator))
	for idx, segment := range segments {
		if idx == 0 && segment == "" && filepath.IsAbs(cleaned) {
			continue
		}
		if segment == ".." || segment == "" {
			return "", fmt.Errorf("output path contains invalid traversal segments")
		}
	}

	if filepath.IsAbs(cleaned) {
		return cleaned, nil
	}

	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to evaluate base output directory: %w", err)
	}

	absTarget, err := filepath.Abs(filepath.Join(baseDir, cleaned))
	if err != nil {
		return "", fmt.Errorf("failed to evaluate absolute output path: %w", err)
	}

	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("output path must be within the base output directory")
	}

	return absTarget, nil
}
