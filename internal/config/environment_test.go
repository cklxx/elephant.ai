package config

import "testing"

func TestSnapshotProcessEnvCopiesVariables(t *testing.T) {
	t.Setenv("ALEX_TEST_ENV", "value")
	snapshot := SnapshotProcessEnv()
	if snapshot["ALEX_TEST_ENV"] != "value" {
		t.Fatalf("expected env value 'value', got %q", snapshot["ALEX_TEST_ENV"])
	}

	snapshot["ALEX_TEST_ENV"] = "mutated"
	refreshed := SnapshotProcessEnv()
	if refreshed["ALEX_TEST_ENV"] != "value" {
		t.Fatalf("mutation should not impact process env")
	}
}
