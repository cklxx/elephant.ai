package kernel

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// kernelStateFallbackPath returns an absolute path to the fallback state file
// under the current working directory's artifacts/ folder. The result is
// computed once and cached so that subsequent calls are allocation-free.
var kernelStateFallbackPath = sync.OnceValue(func() string {
	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}
	return filepath.Join(wd, "artifacts", "kernel_state.md")
})

// WriteKernelStateFallback persists the provided content to the fallback path.
func WriteKernelStateFallback(content string) (string, error) {
	fallbackPath := filepath.Clean(kernelStateFallbackPath())
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
	fallbackPath := filepath.Clean(kernelStateFallbackPath())
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
