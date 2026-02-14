package kernel

import (
	"os"
	"path/filepath"
	"strings"
)

const kernelStateFallbackPath = "/Users/bytedance/code/elephant.ai/artifacts/kernel_state.md"

// WriteKernelStateFallback persists the provided content to the fallback path.
func WriteKernelStateFallback(content string) (string, error) {
	fallbackPath := filepath.Clean(kernelStateFallbackPath)
	artifactsDir := filepath.Dir(fallbackPath)
	if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
		return fallbackPath, err
	}
	if err := os.WriteFile(fallbackPath, []byte(content), 0o644); err != nil {
		return fallbackPath, err
	}
	return fallbackPath, nil
}

// AppendKernelStateFallback appends a section to the fallback path.
func AppendKernelStateFallback(sectionTitle, content string) (string, error) {
	fallbackPath := filepath.Clean(kernelStateFallbackPath)
	artifactsDir := filepath.Dir(fallbackPath)
	if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
		return fallbackPath, err
	}
	separator := ""
	if info, err := os.Stat(fallbackPath); err == nil && info.Size() > 0 {
		separator = "\n\n"
	}
	file, err := os.OpenFile(fallbackPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fallbackPath, err
	}
	defer file.Close()
	if separator != "" {
		if _, err := file.WriteString(separator); err != nil {
			return fallbackPath, err
		}
	}
	if sectionTitle != "" {
		if _, err := file.WriteString("## " + sectionTitle + "\n"); err != nil {
			return fallbackPath, err
		}
	}
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	if _, err := file.WriteString(content); err != nil {
		return fallbackPath, err
	}
	return fallbackPath, nil
}
