package process

import (
	"context"
	"testing"
	"time"
)

func TestController_StartExec_RegistersAndDeregisters(t *testing.T) {
	ctrl := NewController()

	h, err := ctrl.StartExec(context.Background(), ProcessConfig{
		Name:    "ctrl-test",
		Command: "echo",
		Args:    []string{"hello"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Should be registered.
	got, ok := ctrl.Get("ctrl-test")
	if !ok {
		t.Fatal("expected process to be registered")
	}
	if got.PID() != h.PID() {
		t.Fatalf("PID mismatch: %d vs %d", got.PID(), h.PID())
	}

	// Wait for process to exit.
	_ = h.Wait()

	// Give auto-deregister goroutine time.
	time.Sleep(100 * time.Millisecond)

	_, ok = ctrl.Get("ctrl-test")
	if ok {
		t.Fatal("expected process to be deregistered after exit")
	}
}

func TestController_List(t *testing.T) {
	ctrl := NewController()

	_, err := ctrl.StartExec(context.Background(), ProcessConfig{
		Name:    "list-a",
		Command: "sleep",
		Args:    []string{"60"},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = ctrl.StartExec(context.Background(), ProcessConfig{
		Name:    "list-b",
		Command: "sleep",
		Args:    []string{"60"},
	})
	if err != nil {
		t.Fatal(err)
	}

	infos := ctrl.List()
	if len(infos) != 2 {
		t.Fatalf("expected 2, got %d", len(infos))
	}

	names := map[string]bool{}
	for _, info := range infos {
		names[info.Name] = true
		if !info.Alive {
			t.Fatalf("expected %s to be alive", info.Name)
		}
		if info.Backend != "exec" {
			t.Fatalf("expected exec backend, got %s", info.Backend)
		}
	}
	if !names["list-a"] || !names["list-b"] {
		t.Fatal("expected both list-a and list-b")
	}

	_ = ctrl.StopAll()
}

func TestController_StopAll(t *testing.T) {
	ctrl := NewController()

	for _, name := range []string{"stop-a", "stop-b", "stop-c"} {
		_, err := ctrl.StartExec(context.Background(), ProcessConfig{
			Name:    name,
			Command: "sleep",
			Args:    []string{"60"},
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	if err := ctrl.StopAll(); err != nil {
		t.Fatal(err)
	}

	// Give deregister goroutines time.
	time.Sleep(200 * time.Millisecond)

	infos := ctrl.List()
	if len(infos) != 0 {
		t.Fatalf("expected 0 after StopAll, got %d", len(infos))
	}
}

func TestController_Shutdown(t *testing.T) {
	ctrl := NewController()

	_, err := ctrl.StartExec(context.Background(), ProcessConfig{
		Name:    "shutdown-test",
		Command: "sleep",
		Args:    []string{"60"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := ctrl.Shutdown(5 * time.Second); err != nil {
		t.Fatal(err)
	}
}
