package builtin

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func TestRunCommand_DisablesPromptsAndPager(t *testing.T) {
	if _, err := exec.LookPath("env"); err != nil {
		t.Skip("env command not available")
	}

	output, err := runCommand(context.Background(), true, "env")
	if err != nil {
		t.Fatalf("runCommand() error = %v", err)
	}

	env := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}

	expectations := map[string]string{
		"GIT_PAGER":           "cat",
		"GIT_TERMINAL_PROMPT": "0",
		"GIT_SSH_COMMAND":     "ssh -oBatchMode=yes",
		"GH_PROMPT_DISABLED":  "1",
		"GH_PAGER":            "cat",
		"NO_COLOR":            "1",
	}

	for key, want := range expectations {
		if got, ok := env[key]; !ok {
			t.Errorf("expected %s to be set", key)
		} else if got != want {
			t.Errorf("expected %s=%s, got %s", key, want, got)
		}
	}
}
