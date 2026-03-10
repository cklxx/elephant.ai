package panel

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// commandRecord captures a single command invocation.
type commandRecord struct {
	Binary string
	Args   []string
}

// commandRecorder is a test helper that records all run() calls.
type commandRecorder struct {
	mu      sync.Mutex
	calls   []commandRecord
	output  string // default output to return
	err     error  // default error to return
	outputs []string // per-call outputs (used in order, falls back to output)
}

func (r *commandRecorder) run(_ context.Context, binary string, args ...string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, commandRecord{Binary: binary, Args: args})
	idx := len(r.calls) - 1
	if idx < len(r.outputs) {
		return r.outputs[idx], r.err
	}
	return r.output, r.err
}

func (r *commandRecorder) lastCall() commandRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.calls) == 0 {
		return commandRecord{}
	}
	return r.calls[len(r.calls)-1]
}

func (r *commandRecorder) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

func (r *commandRecorder) call(i int) commandRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls[i]
}

// newTestTmuxPane creates a TmuxPane wired to the given recorder.
func newTestTmuxPane(id int, rec *commandRecorder) *TmuxPane {
	return &TmuxPane{ID: id, binary: "tmux", runner: rec.run}
}

// newTestTmuxManager creates a TmuxManager wired to the given recorder.
func newTestTmuxManager(rec *commandRecorder) *TmuxManager {
	return &TmuxManager{binary: "tmux", runner: rec.run}
}

// --- TmuxPane tests ---

func TestTmuxPane_PaneID(t *testing.T) {
	p := &TmuxPane{ID: 42}
	if p.PaneID() != 42 {
		t.Fatalf("PaneID() = %d, want 42", p.PaneID())
	}
}

func TestTmuxPane_InjectText(t *testing.T) {
	rec := &commandRecorder{}
	p := newTestTmuxPane(7, rec)

	if err := p.InjectText(context.Background(), "hello world"); err != nil {
		t.Fatal(err)
	}

	c := rec.lastCall()
	wantArgs := []string{"send-keys", "-l", "-t", "%7", "hello world"}
	assertArgs(t, c, "tmux", wantArgs)
}

func TestTmuxPane_Submit(t *testing.T) {
	rec := &commandRecorder{}
	p := newTestTmuxPane(3, rec)

	if err := p.Submit(context.Background()); err != nil {
		t.Fatal(err)
	}

	c := rec.lastCall()
	wantArgs := []string{"send-keys", "-t", "%3", "Enter"}
	assertArgs(t, c, "tmux", wantArgs)
}

func TestTmuxPane_Send(t *testing.T) {
	rec := &commandRecorder{}
	p := newTestTmuxPane(5, rec)

	if err := p.Send(context.Background(), "ls -la"); err != nil {
		t.Fatal(err)
	}

	if rec.callCount() != 2 {
		t.Fatalf("Send should produce 2 calls (InjectText + Submit), got %d", rec.callCount())
	}

	// First call: InjectText (send-keys -l)
	c0 := rec.call(0)
	assertArgs(t, c0, "tmux", []string{"send-keys", "-l", "-t", "%5", "ls -la"})

	// Second call: Submit (send-keys Enter)
	c1 := rec.call(1)
	assertArgs(t, c1, "tmux", []string{"send-keys", "-t", "%5", "Enter"})
}

func TestTmuxPane_SendKey(t *testing.T) {
	rec := &commandRecorder{}
	p := newTestTmuxPane(1, rec)

	if err := p.SendKey(context.Background(), "C-c"); err != nil {
		t.Fatal(err)
	}

	c := rec.lastCall()
	wantArgs := []string{"send-keys", "-t", "%1", "C-c"}
	assertArgs(t, c, "tmux", wantArgs)
}

func TestTmuxPane_CaptureOutput(t *testing.T) {
	rec := &commandRecorder{output: "line1\nline2"}
	p := newTestTmuxPane(9, rec)

	out, err := p.CaptureOutput(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if out != "line1\nline2" {
		t.Fatalf("CaptureOutput() = %q, want %q", out, "line1\nline2")
	}

	c := rec.lastCall()
	wantArgs := []string{"capture-pane", "-t", "%9", "-p"}
	assertArgs(t, c, "tmux", wantArgs)
}

func TestTmuxPane_Activate(t *testing.T) {
	rec := &commandRecorder{}
	p := newTestTmuxPane(4, rec)

	if err := p.Activate(context.Background()); err != nil {
		t.Fatal(err)
	}

	c := rec.lastCall()
	wantArgs := []string{"select-pane", "-t", "%4"}
	assertArgs(t, c, "tmux", wantArgs)
}

func TestTmuxPane_Kill(t *testing.T) {
	rec := &commandRecorder{}
	p := newTestTmuxPane(6, rec)

	if err := p.Kill(context.Background()); err != nil {
		t.Fatal(err)
	}

	c := rec.lastCall()
	wantArgs := []string{"kill-pane", "-t", "%6"}
	assertArgs(t, c, "tmux", wantArgs)
}

func TestTmuxPane_ErrorPropagation(t *testing.T) {
	rec := &commandRecorder{err: fmt.Errorf("connection refused")}
	p := newTestTmuxPane(1, rec)
	ctx := context.Background()

	if err := p.InjectText(ctx, "x"); err == nil {
		t.Fatal("expected error from InjectText")
	}
	if err := p.Submit(ctx); err == nil {
		t.Fatal("expected error from Submit")
	}
	if err := p.SendKey(ctx, "C-c"); err == nil {
		t.Fatal("expected error from SendKey")
	}
	if _, err := p.CaptureOutput(ctx); err == nil {
		t.Fatal("expected error from CaptureOutput")
	}
	if err := p.Activate(ctx); err == nil {
		t.Fatal("expected error from Activate")
	}
	if err := p.Kill(ctx); err == nil {
		t.Fatal("expected error from Kill")
	}
}

// --- TmuxManager tests ---

func TestTmuxManager_Split_Bottom(t *testing.T) {
	rec := &commandRecorder{output: "%12"}
	mgr := newTestTmuxManager(rec)

	pane, err := mgr.Split(context.Background(), SplitOpts{
		Direction: "bottom",
		Percent:   50,
		WorkDir:   "/tmp",
	})
	if err != nil {
		t.Fatal(err)
	}
	if pane.PaneID() != 12 {
		t.Fatalf("Split returned pane ID %d, want 12", pane.PaneID())
	}

	c := rec.lastCall()
	// Direction "bottom" → -v
	assertContains(t, c.Args, "-v")
	assertContains(t, c.Args, "-p")
	assertContains(t, c.Args, "50")
	assertContains(t, c.Args, "-c")
	assertContains(t, c.Args, "/tmp")
	assertContains(t, c.Args, "-P")
	assertContains(t, c.Args, "-F")
	assertContains(t, c.Args, "#{pane_id}")
}

func TestTmuxManager_Split_Right(t *testing.T) {
	rec := &commandRecorder{output: "%0"}
	mgr := newTestTmuxManager(rec)

	pane, err := mgr.Split(context.Background(), SplitOpts{
		Direction: "right",
		Percent:   30,
		WorkDir:   "/home",
	})
	if err != nil {
		t.Fatal(err)
	}
	if pane.PaneID() != 0 {
		t.Fatalf("Split returned pane ID %d, want 0", pane.PaneID())
	}

	c := rec.lastCall()
	assertContains(t, c.Args, "-h")
}

func TestTmuxManager_Split_DefaultPercent(t *testing.T) {
	rec := &commandRecorder{output: "%1"}
	mgr := newTestTmuxManager(rec)

	_, err := mgr.Split(context.Background(), SplitOpts{
		WorkDir: "/tmp",
	})
	if err != nil {
		t.Fatal(err)
	}

	c := rec.lastCall()
	// Default percent should be 65
	assertContains(t, c.Args, "65")
}

func TestTmuxManager_Split_WithParentPane(t *testing.T) {
	rec := &commandRecorder{output: "%5"}
	mgr := newTestTmuxManager(rec)

	_, err := mgr.Split(context.Background(), SplitOpts{
		ParentPaneID: 3,
		WorkDir:      "/tmp",
	})
	if err != nil {
		t.Fatal(err)
	}

	c := rec.lastCall()
	assertContains(t, c.Args, "-t")
	assertContains(t, c.Args, "%3")
}

func TestTmuxManager_Split_ParseError(t *testing.T) {
	rec := &commandRecorder{output: "garbage"}
	mgr := newTestTmuxManager(rec)

	_, err := mgr.Split(context.Background(), SplitOpts{WorkDir: "/tmp"})
	if err == nil {
		t.Fatal("expected error for unparseable output")
	}
	if !strings.Contains(err.Error(), "unexpected output") {
		t.Fatalf("error should mention unexpected output, got: %v", err)
	}
}

func TestTmuxManager_List(t *testing.T) {
	rec := &commandRecorder{output: "%0 @1 zsh\n%1 @1 vim"}
	mgr := newTestTmuxManager(rec)

	out, err := mgr.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if out != "%0 @1 zsh\n%1 @1 vim" {
		t.Fatalf("List() = %q, want %q", out, "%0 @1 zsh\n%1 @1 vim")
	}

	c := rec.lastCall()
	assertContains(t, c.Args, "list-panes")
	assertContains(t, c.Args, "-a")
}

// --- parseTmuxPaneID tests ---

func TestParseTmuxPaneID(t *testing.T) {
	tests := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{"%0", 0, false},
		{"%12", 12, false},
		{"%99\n", 99, false},
		{"  %5  ", 5, false},
		{"garbage", 0, true},
		{"12", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		got, err := parseTmuxPaneID(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseTmuxPaneID(%q): err=%v, wantErr=%v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("parseTmuxPaneID(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

// --- NewAutoManager tests ---

func TestNewAutoManager_NoBackend(t *testing.T) {
	// If this test runs in CI without kaku or tmux, NewAutoManager should return an error.
	// We can't reliably mock binary resolution without more infrastructure,
	// so we just verify the function signature works and returns a sensible type.
	mgr, err := NewAutoManager()
	if err != nil {
		// Expected in most test environments.
		if !strings.Contains(err.Error(), "no backend available") {
			t.Fatalf("unexpected error: %v", err)
		}
		return
	}
	// If it succeeded (tmux or kaku is installed), verify it returns a valid manager.
	if mgr == nil {
		t.Fatal("NewAutoManager returned nil manager without error")
	}
}

// --- Compile-time interface checks ---

var (
	_ PaneIface    = (*TmuxPane)(nil)
	_ ManagerIface = (*TmuxManager)(nil)
)

// --- helpers ---

func assertArgs(t *testing.T, c commandRecord, wantBin string, wantArgs []string) {
	t.Helper()
	if c.Binary != wantBin {
		t.Fatalf("binary = %q, want %q", c.Binary, wantBin)
	}
	if len(c.Args) != len(wantArgs) {
		t.Fatalf("args = %v (len %d), want %v (len %d)", c.Args, len(c.Args), wantArgs, len(wantArgs))
	}
	for i, a := range c.Args {
		if a != wantArgs[i] {
			t.Fatalf("args[%d] = %q, want %q (full: %v)", i, a, wantArgs[i], c.Args)
		}
	}
}

func assertContains(t *testing.T, args []string, want string) {
	t.Helper()
	for _, a := range args {
		if a == want {
			return
		}
	}
	t.Fatalf("args %v does not contain %q", args, want)
}
