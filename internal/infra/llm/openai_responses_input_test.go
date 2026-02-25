package llm

import "testing"

func TestPruneOrphanFunctionCallOutputsDropsUnknownCallIDs(t *testing.T) {
	t.Parallel()

	items := []map[string]any{
		{"role": "user", "content": "hi"},
		{"type": "function_call_output", "call_id": "orphan-1", "output": "stale"},
		{"type": "function_call_output", "call_id": "", "output": "missing id"},
	}

	filtered, dropped := pruneOrphanFunctionCallOutputs(items)

	if len(filtered) != 1 {
		t.Fatalf("expected only non-tool item to remain, got %d items", len(filtered))
	}
	if len(dropped) != 2 {
		t.Fatalf("expected 2 dropped markers, got %d (%v)", len(dropped), dropped)
	}
	if dropped[0] != "orphan-1" {
		t.Fatalf("expected orphan call id first, got %q", dropped[0])
	}
	if dropped[1] != "<empty_call_id>" {
		t.Fatalf("expected empty call id marker, got %q", dropped[1])
	}
}

func TestPruneOrphanFunctionCallOutputsKeepsOnlyOutputsAfterCall(t *testing.T) {
	t.Parallel()

	items := []map[string]any{
		{"type": "function_call_output", "call_id": "call-1", "output": "before"},
		{"type": "function_call", "call_id": "call-1", "name": "shell_exec", "arguments": "{}"},
		{"type": "function_call_output", "call_id": "call-1", "output": "after"},
	}

	filtered, dropped := pruneOrphanFunctionCallOutputs(items)

	if len(dropped) != 1 || dropped[0] != "call-1" {
		t.Fatalf("expected early output to be dropped for call-1, got %v", dropped)
	}
	if len(filtered) != 2 {
		t.Fatalf("expected function_call + trailing output to remain, got %d items", len(filtered))
	}
	if filtered[0]["type"] != "function_call" {
		t.Fatalf("expected first item to be function_call, got %#v", filtered[0])
	}
	if filtered[1]["type"] != "function_call_output" || filtered[1]["output"] != "after" {
		t.Fatalf("expected trailing function_call_output to be preserved, got %#v", filtered[1])
	}
}
