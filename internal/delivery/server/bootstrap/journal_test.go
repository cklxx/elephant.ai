package bootstrap

import (
	"testing"
)

func TestBuildJournalReader_EmptyDirReturnsNil(t *testing.T) {
	reader := BuildJournalReader("", nil)
	if reader != nil {
		t.Fatalf("expected nil reader, got %T", reader)
	}
}
