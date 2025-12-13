package logging

import (
	"bytes"
	"testing"

	"alex/internal/observability"
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

func TestFromObservabilityFormatsMessages(t *testing.T) {
	buf := &bytes.Buffer{}
	base := observability.NewLogger(observability.LogConfig{
		Level:  "info",
		Format: "text",
		Output: buf,
	})

	logger := FromObservabilityWithComponent(base, "test")
	logger.Info("hello %s", "world")

	if got := buf.String(); got == "" {
		t.Fatalf("expected log output")
	}
	if want := "hello world"; !bytes.Contains(buf.Bytes(), []byte(want)) {
		t.Fatalf("expected %q in output, got %q", want, buf.String())
	}
}
