package coordinator

import "testing"

func TestObfuscateSessionID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: ""},
		{name: "short", input: "12345678", want: "****"},
		{name: "long", input: "session-1234567890", want: "sess...7890"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := obfuscateSessionID(tt.input); got != tt.want {
				t.Fatalf("obfuscateSessionID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
