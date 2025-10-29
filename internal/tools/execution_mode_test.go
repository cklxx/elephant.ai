package tools

import "testing"

func TestExecutionModeValidate(t *testing.T) {
	cases := []struct {
		name    string
		mode    ExecutionMode
		wantErr bool
	}{
		{name: "local", mode: ExecutionModeLocal},
		{name: "sandbox", mode: ExecutionModeSandbox},
		{name: "unknown", mode: ExecutionModeUnknown, wantErr: true},
		{name: "invalid", mode: ExecutionMode(42), wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.mode.Validate()
			if tc.wantErr && err == nil {
				t.Fatalf("Validate() error = nil, want error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
		})
	}
}

func TestExecutionModeString(t *testing.T) {
	cases := []struct {
		mode ExecutionMode
		want string
	}{
		{ExecutionModeLocal, "local"},
		{ExecutionModeSandbox, "sandbox"},
		{ExecutionModeUnknown, "unknown"},
		{ExecutionMode(99), "unknown"},
	}

	for _, tc := range cases {
		if got := tc.mode.String(); got != tc.want {
			t.Fatalf("String() = %q, want %q", got, tc.want)
		}
	}
}
