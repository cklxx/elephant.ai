package swe_bench

import (
	"fmt"
	"path/filepath"
	"strings"
)

func sanitizeOutputPath(path string) (string, error) {
	cleaned := filepath.Clean(path)
	if cleaned == "" || cleaned == "." || cleaned == ".." {
		return "", fmt.Errorf("output path cannot be empty or traverse parent directories")
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

	return cleaned, nil
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
