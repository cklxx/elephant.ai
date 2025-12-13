package agent_eval

import (
	"fmt"
	"path/filepath"
	"strings"
)

func sanitizeOutputPath(path string, baseDir string) (string, error) {
	cleaned := filepath.Clean(path)
	if cleaned == "." || cleaned == ".." {
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

	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to evaluate base output directory: %w", err)
	}
	absTarget, err := filepath.Abs(cleaned)
	if err != nil {
		return "", fmt.Errorf("failed to evaluate absolute output path: %w", err)
	}
	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("output path must be within the base output directory")
	}

	return absTarget, nil
}
