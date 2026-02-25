package main

import "testing"

func TestParseApprovalDecision(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		input          string
		ok             bool
		approved       bool
		action         string
		autoApproveAll bool
	}{
		{name: "default yes", input: "", ok: true, approved: true, action: "approve"},
		{name: "yes short", input: "y", ok: true, approved: true, action: "approve"},
		{name: "yes word", input: "yes", ok: true, approved: true, action: "approve"},
		{name: "yes allow", input: "allow", ok: true, approved: true, action: "approve"},
		{name: "yes trimmed", input: "  YES  ", ok: true, approved: true, action: "approve"},
		{name: "approve all short", input: "a", ok: true, approved: true, action: "approve_all", autoApproveAll: true},
		{name: "approve all word", input: "all", ok: true, approved: true, action: "approve_all", autoApproveAll: true},
		{name: "approve all always", input: "always", ok: true, approved: true, action: "approve_all", autoApproveAll: true},
		{name: "reject short", input: "n", ok: true, approved: false, action: "reject"},
		{name: "reject word", input: "no", ok: true, approved: false, action: "reject"},
		{name: "reject keyword", input: "reject", ok: true, approved: false, action: "reject"},
		{name: "quit short", input: "q", ok: true, approved: false, action: "quit"},
		{name: "quit word", input: "quit", ok: true, approved: false, action: "quit"},
		{name: "exit word", input: "exit", ok: true, approved: false, action: "quit"},
		{name: "unknown", input: "maybe", ok: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			decision, ok := parseApprovalDecision(tc.input)
			if ok != tc.ok {
				t.Fatalf("expected ok=%v, got %v", tc.ok, ok)
			}
			if !ok {
				return
			}
			if decision.Approved != tc.approved {
				t.Fatalf("expected approved=%v, got %v", tc.approved, decision.Approved)
			}
			if decision.Action != tc.action {
				t.Fatalf("expected action=%s, got %s", tc.action, decision.Action)
			}
			if decision.AutoApproveAll != tc.autoApproveAll {
				t.Fatalf("expected autoApproveAll=%v, got %v", tc.autoApproveAll, decision.AutoApproveAll)
			}
		})
	}
}
