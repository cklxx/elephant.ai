package panel_test

import (
	"context"
	"sync"
	"testing"

	"alex/internal/runtime/panel"
)

// MockPane is a test double that implements PaneIface without any terminal dependency.
type MockPane struct {
	mu          sync.Mutex
	id          int
	Injected    []string // text passed to InjectText
	Submitted   int      // number of Submit calls
	Sent        []string // text passed to Send
	SentKeys    []string // keys passed to SendKey
	Activated   int      // number of Activate calls
	Killed      bool
	CaptureText string // value returned by CaptureOutput
	CaptureErr  error  // error returned by CaptureOutput
}

func NewMockPane(id int) *MockPane {
	return &MockPane{id: id}
}

func (m *MockPane) PaneID() int { return m.id }

func (m *MockPane) InjectText(_ context.Context, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Injected = append(m.Injected, text)
	return nil
}

func (m *MockPane) Submit(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Submitted++
	return nil
}

func (m *MockPane) Send(_ context.Context, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Sent = append(m.Sent, text)
	return nil
}

func (m *MockPane) SendKey(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SentKeys = append(m.SentKeys, key)
	return nil
}

func (m *MockPane) CaptureOutput(_ context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.CaptureText, m.CaptureErr
}

func (m *MockPane) Activate(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Activated++
	return nil
}

func (m *MockPane) Kill(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Killed = true
	return nil
}

// MockManager is a test double that implements ManagerIface.
type MockManager struct {
	mu        sync.Mutex
	NextPane  *MockPane
	SplitOpts []panel.SplitOpts
	SplitErr  error
	ListText  string
	ListErr   error
}

func (m *MockManager) Split(_ context.Context, opts panel.SplitOpts) (panel.PaneIface, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SplitOpts = append(m.SplitOpts, opts)
	if m.SplitErr != nil {
		return nil, m.SplitErr
	}
	return m.NextPane, nil
}

func (m *MockManager) List(_ context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ListText, m.ListErr
}

// Compile-time interface checks.
var (
	_ panel.PaneIface    = (*MockPane)(nil)
	_ panel.ManagerIface = (*MockManager)(nil)
)

func TestMockPane_ImplementsPaneIface(t *testing.T) {
	mp := NewMockPane(42)
	ctx := context.Background()

	if mp.PaneID() != 42 {
		t.Fatalf("PaneID() = %d, want 42", mp.PaneID())
	}

	if err := mp.InjectText(ctx, "hello"); err != nil {
		t.Fatal(err)
	}
	if err := mp.Submit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := mp.Send(ctx, "ls"); err != nil {
		t.Fatal(err)
	}
	if err := mp.SendKey(ctx, "C-c"); err != nil {
		t.Fatal(err)
	}
	if _, err := mp.CaptureOutput(ctx); err != nil {
		t.Fatal(err)
	}
	if err := mp.Activate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := mp.Kill(ctx); err != nil {
		t.Fatal(err)
	}

	if len(mp.Injected) != 1 || mp.Injected[0] != "hello" {
		t.Fatalf("Injected = %v, want [hello]", mp.Injected)
	}
	if mp.Submitted != 1 {
		t.Fatalf("Submitted = %d, want 1", mp.Submitted)
	}
	if len(mp.Sent) != 1 || mp.Sent[0] != "ls" {
		t.Fatalf("Sent = %v, want [ls]", mp.Sent)
	}
	if len(mp.SentKeys) != 1 || mp.SentKeys[0] != "C-c" {
		t.Fatalf("SentKeys = %v, want [C-c]", mp.SentKeys)
	}
	if mp.Activated != 1 {
		t.Fatalf("Activated = %d, want 1", mp.Activated)
	}
	if !mp.Killed {
		t.Fatal("Killed = false, want true")
	}
}

func TestMockManager_ImplementsManagerIface(t *testing.T) {
	mp := NewMockPane(10)
	mm := &MockManager{NextPane: mp}

	pane, err := mm.Split(context.Background(), panel.SplitOpts{
		ParentPaneID: 1,
		Direction:    "bottom",
		Percent:      50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if pane.PaneID() != 10 {
		t.Fatalf("Split returned pane with ID %d, want 10", pane.PaneID())
	}
	if len(mm.SplitOpts) != 1 {
		t.Fatalf("SplitOpts recorded %d calls, want 1", len(mm.SplitOpts))
	}
}
