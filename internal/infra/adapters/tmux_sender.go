package adapters

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	agent "alex/internal/domain/agent/ports/agent"
)

// ExecTmuxSender implements agent.TmuxSender using exec.CommandContext.
type ExecTmuxSender struct{}

var _ agent.TmuxSender = (*ExecTmuxSender)(nil)

// NewExecTmuxSender creates a new ExecTmuxSender.
func NewExecTmuxSender() *ExecTmuxSender {
	return &ExecTmuxSender{}
}

// SendKeys sends keystrokes to a tmux pane.
func (s *ExecTmuxSender) SendKeys(ctx context.Context, pane string, data string) error {
	cmd := exec.CommandContext(ctx, "tmux", "-L", "elephant", "send-keys", "-t", pane, data, "C-m")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("inject input to pane %s: %s: %w", pane, strings.TrimSpace(string(out)), err)
	}
	return nil
}
