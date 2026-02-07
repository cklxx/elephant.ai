package config

import "testing"

func TestResolveStewardMode(t *testing.T) {
	tests := []struct {
		name      string
		enabled   bool
		persona   string
		sessionID string
		channel   string
		want      bool
	}{
		{
			name:    "explicit enabled",
			enabled: true,
			want:    true,
		},
		{
			name:    "steward persona auto enables",
			persona: "steward",
			want:    true,
		},
		{
			name:      "lark session auto enables",
			sessionID: "lark-session-1",
			want:      true,
		},
		{
			name:    "lark channel auto enables",
			channel: "lark",
			want:    true,
		},
		{
			name:    "feishu channel auto enables",
			channel: "Feishu",
			want:    true,
		},
		{
			name:      "disabled for non steward non lark contexts",
			persona:   "default",
			sessionID: "web-session-1",
			channel:   "web",
			want:      false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveStewardMode(tc.enabled, tc.persona, tc.sessionID, tc.channel)
			if got != tc.want {
				t.Fatalf("ResolveStewardMode()=%v, want %v", got, tc.want)
			}
		})
	}
}
