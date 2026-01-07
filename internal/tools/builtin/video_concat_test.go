package builtin

import (
	"context"
	"strings"
	"testing"

	"alex/internal/agent/ports"
)

func TestVideoConcatFailsWithoutFFmpeg(t *testing.T) {
	tool := NewVideoConcat()
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:   "concat-1",
		Name: "video_concat",
		Arguments: map[string]any{
			"videos": []any{
				"data:video/mp4;base64,AA==",
				"data:video/mp4;base64,AA==",
			},
			"ffmpeg_path": "ffmpeg-missing",
		},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result == nil || result.Error == nil {
		t.Fatalf("expected error result when ffmpeg is missing")
	}
	if !strings.Contains(result.Content, "ffmpeg binary") {
		t.Fatalf("expected missing ffmpeg message, got %q", result.Content)
	}
}
