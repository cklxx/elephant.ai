package panel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakePane struct {
	injectErr error
	submitErr error
}

func (p *fakePane) PaneID() int { return 1 }

func (p *fakePane) InjectText(context.Context, string) error { return p.injectErr }

func (p *fakePane) Submit(context.Context) error { return p.submitErr }

func (p *fakePane) Send(context.Context, string) error { return nil }

func (p *fakePane) SendKey(context.Context, string) error { return nil }

func (p *fakePane) CaptureOutput(context.Context) (string, error) { return "", nil }

func (p *fakePane) Activate(context.Context) error { return nil }

func (p *fakePane) Kill(context.Context) error { return nil }

func writeFakeKakuScript(t *testing.T, body string) (string, string) {
	t.Helper()

	dir := t.TempDir()
	argsPath := filepath.Join(dir, "args.log")
	scriptPath := filepath.Join(dir, "kaku")
	script := "#!/bin/sh\n" +
		"ARGSFILE=\"" + argsPath + "\"\n" +
		"echo \"---\" >> \"$ARGSFILE\"\n" +
		"for arg in \"$@\"; do printf '%s\\n' \"$arg\" >> \"$ARGSFILE\"; done\n" +
		body + "\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return scriptPath, argsPath
}

func readArgsLog(t *testing.T, path string) [][]string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var invocations [][]string
	var current []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "---" {
			if current != nil {
				invocations = append(invocations, current)
			}
			current = []string{}
			continue
		}
		current = append(current, line)
	}
	if current != nil {
		invocations = append(invocations, current)
	}
	return invocations
}

func TestNewPane_UsesEnvOverride(t *testing.T) {
	t.Setenv("KAKU_BIN", "/tmp/custom-kaku")

	pane := NewPane(7)
	kakuPane, ok := pane.(*Pane)
	if !ok {
		t.Fatalf("NewPane() returned %T, want *Pane", pane)
	}
	if kakuPane.binary != "/tmp/custom-kaku" {
		t.Fatalf("binary = %q, want env override", kakuPane.binary)
	}
}

func TestManagerSplit_DefaultsAndList(t *testing.T) {
	binPath, argsPath := writeFakeKakuScript(t, `
if [ "$2" = "split-pane" ]; then
  printf '42\n'
  exit 0
fi
if [ "$2" = "list" ]; then
  printf 'pane-1\n'
  exit 0
fi
`)
	mgr := &Manager{binary: binPath}
	wd := t.TempDir()
	t.Chdir(wd)

	pane, err := mgr.Split(context.Background(), SplitOpts{ParentPaneID: 9, Percent: 100})
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	if pane.PaneID() != 42 {
		t.Fatalf("PaneID = %d, want 42", pane.PaneID())
	}

	list, err := mgr.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if list != "pane-1" {
		t.Fatalf("List() = %q, want pane-1", list)
	}

	invocations := readArgsLog(t, argsPath)
	if len(invocations) != 2 {
		t.Fatalf("expected 2 invocations, got %d", len(invocations))
	}
	splitArgs := invocations[0]
	if !contains(splitArgs, "--bottom") {
		t.Fatalf("split args missing default direction: %v", splitArgs)
	}
	if !contains(splitArgs, "65") {
		t.Fatalf("split args missing default percent: %v", splitArgs)
	}
	if !contains(splitArgs, wd) {
		t.Fatalf("split args missing cwd %q: %v", wd, splitArgs)
	}
}

func TestManagerSplit_UnexpectedOutput(t *testing.T) {
	binPath, _ := writeFakeKakuScript(t, "printf 'not-a-pane\\n'\n")
	mgr := &Manager{binary: binPath}

	_, err := mgr.Split(context.Background(), SplitOpts{ParentPaneID: 1})
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "unexpected output") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestManagerRun_PropagatesStderr(t *testing.T) {
	binPath, _ := writeFakeKakuScript(t, "echo 'boom' 1>&2\nexit 1\n")
	mgr := &Manager{binary: binPath}

	_, err := mgr.List(context.Background())
	if err == nil {
		t.Fatal("expected command error")
	}
	if !strings.Contains(err.Error(), "stderr: boom") {
		t.Fatalf("stderr not preserved: %v", err)
	}
}

func TestPane_MethodsShellOut(t *testing.T) {
	binPath, argsPath := writeFakeKakuScript(t, `
if [ "$2" = "get-text" ]; then
  printf 'screen output\n'
fi
`)
	pane := &Pane{ID: 12, binary: binPath}
	ctx := context.Background()

	if err := pane.InjectText(ctx, "hello"); err != nil {
		t.Fatalf("InjectText: %v", err)
	}
	if err := pane.Submit(ctx); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if err := pane.Send(ctx, "pwd"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if err := pane.SendKey(ctx, "C-c"); err != nil {
		t.Fatalf("SendKey: %v", err)
	}
	out, err := pane.CaptureOutput(ctx)
	if err != nil {
		t.Fatalf("CaptureOutput: %v", err)
	}
	if out != "screen output" {
		t.Fatalf("CaptureOutput() = %q, want %q", out, "screen output")
	}
	if err := pane.Activate(ctx); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if err := pane.Kill(ctx); err != nil {
		t.Fatalf("Kill: %v", err)
	}

	invocations := readArgsLog(t, argsPath)
	if len(invocations) != 8 {
		t.Fatalf("expected 8 invocations, got %d", len(invocations))
	}
	if invocations[0][1] != "send-text" {
		t.Fatalf("first command = %v, want send-text", invocations[0])
	}
	if !contains(invocations[1], "--no-paste") {
		t.Fatalf("submit args missing --no-paste: %v", invocations[1])
	}
	if invocations[5][1] != "get-text" {
		t.Fatalf("capture command = %v, want get-text", invocations[5])
	}
	if invocations[6][1] != "activate-pane" {
		t.Fatalf("activate command = %v, want activate-pane", invocations[6])
	}
	if invocations[7][1] != "kill-pane" {
		t.Fatalf("kill command = %v, want kill-pane", invocations[7])
	}
}

func TestSendViaPane_PropagatesErrors(t *testing.T) {
	injectErr := errors.New("inject failed")
	if err := sendViaPane(context.Background(), &fakePane{injectErr: injectErr}, "ls"); !errors.Is(err, injectErr) {
		t.Fatalf("sendViaPane() error = %v, want %v", err, injectErr)
	}

	submitErr := errors.New("submit failed")
	if err := sendViaPane(context.Background(), &fakePane{submitErr: submitErr}, "ls"); !errors.Is(err, submitErr) {
		t.Fatalf("sendViaPane() error = %v, want %v", err, submitErr)
	}
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
