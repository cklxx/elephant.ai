package process

import (
	"os"
	"strings"
	"testing"
)

func TestMergeEnv_Override(t *testing.T) {
	// Set a known variable, then override it.
	const key = "PROCESS_TEST_MERGE_OVERRIDE"
	os.Setenv(key, "old")
	defer os.Unsetenv(key)

	env := MergeEnv(map[string]string{key: "new"})

	found := false
	for _, e := range env {
		if strings.HasPrefix(e, key+"=") {
			if e != key+"=new" {
				t.Fatalf("expected override, got %q", e)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("override key not found in result")
	}
}

func TestMergeEnv_Unset(t *testing.T) {
	const key = "PROCESS_TEST_MERGE_UNSET"
	os.Setenv(key, "value")
	defer os.Unsetenv(key)

	env := MergeEnv(map[string]string{key: ""})

	for _, e := range env {
		if strings.HasPrefix(e, key+"=") {
			t.Fatalf("expected %s to be unset, found %q", key, e)
		}
	}
}

func TestMergeEnv_Add(t *testing.T) {
	const key = "PROCESS_TEST_MERGE_ADD_UNIQUE"
	os.Unsetenv(key) // ensure not inherited

	env := MergeEnv(map[string]string{key: "added"})

	found := false
	for _, e := range env {
		if e == key+"=added" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected %s=added in env", key)
	}
}

func TestMergeEnv_Nil(t *testing.T) {
	env := MergeEnv(nil)
	if len(env) == 0 {
		t.Fatal("expected inherited env, got empty")
	}
}
