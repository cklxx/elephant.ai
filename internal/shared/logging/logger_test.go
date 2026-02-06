package logging

import (
	"testing"

	"alex/internal/utils"
)

func TestOrNopHandlesTypedNilPointers(t *testing.T) {
	var legacy *utils.Logger
	var logger Logger = legacy
	if !IsNil(logger) {
		t.Fatalf("expected typed nil pointer to be detected")
	}
	safe := OrNop(logger)
	if IsNil(safe) {
		t.Fatalf("expected OrNop to return a usable logger")
	}
	safe.Info("hello %s", "world") // should not panic
}
