package process

import (
	"os"
	"testing"
)

func TestIsAlive_Self(t *testing.T) {
	if !IsAlive(os.Getpid()) {
		t.Fatal("expected own process to be alive")
	}
}

func TestIsAlive_Zero(t *testing.T) {
	if IsAlive(0) {
		t.Fatal("pid 0 should not be alive")
	}
}

func TestIsAlive_Negative(t *testing.T) {
	if IsAlive(-1) {
		t.Fatal("negative pid should not be alive")
	}
}

func TestIsAlive_HighPID(t *testing.T) {
	// Very high PID that almost certainly doesn't exist.
	if IsAlive(1<<22 - 1) {
		t.Skip("unexpectedly found process at high PID")
	}
}
