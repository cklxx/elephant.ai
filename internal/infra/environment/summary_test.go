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
		EnvironmentHints: []string{"SHELL=/bin/zsh", "PATH entries=3 [/usr/local/bin, /usr/bin, /bin]"},
	}

	formatted := FormatSummary(summary)
	expectedFragments := []string{
		"Environment context:",
		"Working directory: /workspace/project",
		"Project files: README.md, cmd/, internal/ …",
		"Operating system: Ubuntu 22.04",
		"Kernel: Linux 5.15.0-100-generic",
		"Capabilities: git version 2.42.0, go version go1.21.0 linux/amd64",
		"Runtime environment: PATH entries=3 [/usr/local/bin, /usr/bin, /bin], SHELL=/bin/zsh",
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

func TestCollectEnvironmentHintsFromMapRedactsSecrets(t *testing.T) {
	input := map[string]string{
		"SHELL":             "/bin/zsh",
		"LANG":              "en_US.UTF-8",
		"OPENAI_API_KEY":    "sk-live-secret",
		"AWS_SECRET_ACCESS": "top-secret",
		"ALEX_DEBUG_DUMP":   "internal-only",
		"PATH":              "/usr/local/bin:/usr/bin:/bin",
	}

	hints := collectEnvironmentHintsFromMap(input, 8)
	rendered := strings.Join(hints, " | ")

	if strings.Contains(rendered, "OPENAI_API_KEY") || strings.Contains(rendered, "AWS_SECRET_ACCESS") {
		t.Fatalf("expected secret-like keys to be filtered, got %q", rendered)
	}
	if !strings.Contains(rendered, "SHELL=/bin/zsh") {
		t.Fatalf("expected shell hint, got %q", rendered)
	}
	if !strings.Contains(rendered, "PATH entries=3 [/usr/local/bin, /usr/bin, /bin]") {
		t.Fatalf("expected path summary hint, got %q", rendered)
	}
	if strings.Contains(rendered, "ALEX_DEBUG_DUMP") {
		t.Fatalf("expected unknown ALEX_* env keys to be omitted from hints, got %q", rendered)
	}
}
