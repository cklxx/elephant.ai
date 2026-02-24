package executioncontrol

import "testing"

func TestNormalizeExecutionMode(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "plan lowercase", in: "plan", want: "plan"},
		{name: "plan mixed case with spaces", in: "  PlAn  ", want: "plan"},
		{name: "empty defaults execute", in: "", want: "execute"},
		{name: "unknown defaults execute", in: "dry-run", want: "execute"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeExecutionMode(tc.in); got != tc.want {
				t.Fatalf("NormalizeExecutionMode(%q)=%q want=%q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeAutonomyLevel(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "full lowercase", in: "full", want: "full"},
		{name: "semi mixed case with spaces", in: "  SeMi ", want: "semi"},
		{name: "empty defaults controlled", in: "", want: "controlled"},
		{name: "unknown defaults controlled", in: "auto", want: "controlled"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeAutonomyLevel(tc.in); got != tc.want {
				t.Fatalf("NormalizeAutonomyLevel(%q)=%q want=%q", tc.in, got, tc.want)
			}
		})
	}
}
