package environment

import (
	"strings"
	"testing"
)

func TestFormatSummaryIncludesAllSections(t *testing.T) {
	summary := Summary{
		WorkingDirectory: "/workspace/project",
		FileEntries:      []string{"README.md", "cmd/", "internal/"},
		HasMoreFiles:     true,
		OperatingSystem:  "Ubuntu 22.04",
		Kernel:           "Linux 5.15.0-100-generic",
		Capabilities:     []string{"git version 2.42.0", "go version go1.21.0 linux/amd64"},
	}

	formatted := FormatSummary(summary)
	expectedFragments := []string{
		"Environment context:",
		"Working directory: /workspace/project",
		"Project files: README.md, cmd/, internal/ …",
		"Operating system: Ubuntu 22.04",
		"Kernel: Linux 5.15.0-100-generic",
		"Capabilities: git version 2.42.0, go version go1.21.0 linux/amd64",
	}

	for _, fragment := range expectedFragments {
		if !strings.Contains(formatted, fragment) {
			t.Fatalf("expected formatted summary to contain %q, got %q", fragment, formatted)
		}
	}
}

func TestFormatSummaryEmpty(t *testing.T) {
	if formatted := FormatSummary(Summary{}); formatted != "" {
		t.Fatalf("expected empty summary to render empty string, got %q", formatted)
	}
}

func TestParseOSRelease(t *testing.T) {
	content := "NAME=\"Ubuntu\"\nVERSION=\"22.04.4 LTS (Jammy Jellyfish)\"\n"
	result := parseOSRelease(content)
	expected := "Ubuntu 22.04.4 LTS (Jammy Jellyfish)"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}
