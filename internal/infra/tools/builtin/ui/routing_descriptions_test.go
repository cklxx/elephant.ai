package ui

import (
	"strings"
	"testing"
)

func TestUIDescriptionsExpressAskUserBoundaries(t *testing.T) {
	t.Parallel()

	askUserDesc := NewAskUser().Definition().Description
	if !strings.Contains(askUserDesc, "clarification questions") || !strings.Contains(askUserDesc, "request a user decision/action") {
		t.Fatalf("expected ask_user description to cover both clarify and request actions, got %q", askUserDesc)
	}
}
