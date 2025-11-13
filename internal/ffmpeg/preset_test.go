package ffmpeg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPresetArgs(t *testing.T) {
	preset := Preset{
		VideoBitrate: "5M",
		AudioBitrate: "192k",
		PixelFormat:  "yuv420p",
		FrameRate:    "30",
		ExtraArgs:    []string{"-movflags", "+faststart"},
	}
	got := preset.Args()
	want := []string{"-b:v", "5M", "-b:a", "192k", "-pix_fmt", "yuv420p", "-r", "30", "-movflags", "+faststart"}
	if len(got) != len(want) {
		t.Fatalf("args length mismatch: got %d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("args[%d]=%q want %q", i, got[i], want[i])
		}
	}
}

func TestLoadPresetFile(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "presets.yaml")
	contents := []byte(`presets:
  default_1080p:
    video_codec: h264
    audio_codec: aac
    video_bitrate: 6M
    audio_bitrate: 192k
    pixel_format: yuv420p
    frame_rate: "30"
    filters:
      - scale=1920:1080
    extra_args:
      - -movflags
      - +faststart
`)
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		t.Fatalf("write presets: %v", err)
	}
	lib, err := LoadPresetFile(path)
	if err != nil {
		t.Fatalf("load presets: %v", err)
	}
	preset, ok := lib.Get("default_1080p")
	if !ok {
		t.Fatal("expected preset to be loaded")
	}
	if preset.VideoCodec != "h264" || preset.AudioCodec != "aac" {
		t.Fatalf("unexpected codec mapping: %+v", preset)
	}
	if len(preset.Filters) != 1 || preset.Filters[0] != "scale=1920:1080" {
		t.Fatalf("unexpected filters: %+v", preset.Filters)
	}
	if preset.Name != "default_1080p" {
		t.Fatalf("preset name not set: %s", preset.Name)
	}
}
