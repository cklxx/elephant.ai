package llm

import (
	"testing"

	"alex/internal/domain/agent/ports"
)

func TestShouldEmbedAttachmentsInContent(t *testing.T) {
	tests := []struct {
		name string
		msg  ports.Message
		want bool
	}{
		{
			name: "no attachments",
			msg: ports.Message{
				Role: "user",
			},
			want: false,
		},
		{
			name: "empty attachments map",
			msg: ports.Message{
				Role:        "user",
				Attachments: map[string]ports.Attachment{},
			},
			want: false,
		},
		{
			name: "assistant role with attachments",
			msg: ports.Message{
				Role:        "assistant",
				Attachments: map[string]ports.Attachment{"file.png": {Name: "file.png", MediaType: "image/png", Data: "abc"}},
			},
			want: false,
		},
		{
			name: "system role with attachments",
			msg: ports.Message{
				Role:        "system",
				Attachments: map[string]ports.Attachment{"file.png": {Name: "file.png", MediaType: "image/png", Data: "abc"}},
			},
			want: false,
		},
		{
			name: "user role with tool result source",
			msg: ports.Message{
				Role:        "user",
				Source:      ports.MessageSourceToolResult,
				Attachments: map[string]ports.Attachment{"file.png": {Name: "file.png", MediaType: "image/png", Data: "abc"}},
			},
			want: false,
		},
		{
			name: "user role with attachments and non tool source",
			msg: ports.Message{
				Role:        "user",
				Source:      ports.MessageSourceUserInput,
				Attachments: map[string]ports.Attachment{"file.png": {Name: "file.png", MediaType: "image/png", Data: "abc"}},
			},
			want: true,
		},
		{
			name: "capitalized user role",
			msg: ports.Message{
				Role:        "User",
				Source:      ports.MessageSourceUserInput,
				Attachments: map[string]ports.Attachment{"file.png": {Name: "file.png", MediaType: "image/png", Data: "abc"}},
			},
			want: true,
		},
		{
			name: "trimmed user role",
			msg: ports.Message{
				Role:        " user ",
				Source:      ports.MessageSourceUserInput,
				Attachments: map[string]ports.Attachment{"file.png": {Name: "file.png", MediaType: "image/png", Data: "abc"}},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldEmbedAttachmentsInContent(tt.msg)
			if got != tt.want {
				t.Fatalf("shouldEmbedAttachmentsInContent(%+v) = %v, want %v", tt.msg, got, tt.want)
			}
		})
	}
}
