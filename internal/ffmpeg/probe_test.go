package ffmpeg

import (
	"context"
	"errors"
	"testing"
)

func TestParseProbeOutput(t *testing.T) {
	payload := []byte(`{
  "streams": [
    {
      "codec_type": "video",
      "codec_name": "h264",
      "width": 1920,
      "height": 1080,
      "pix_fmt": "yuv420p",
      "avg_frame_rate": "30000/1001"
    }
  ],
  "format": {
    "duration": "5.500000"
  }
}`)
	result, err := parseProbeOutput(payload)
	if err != nil {
		t.Fatalf("parseProbeOutput: %v", err)
	}
	if len(result.VideoStreams) != 1 {
		t.Fatalf("expected 1 video stream, got %d", len(result.VideoStreams))
	}
	stream := result.VideoStreams[0]
	if stream.FrameRate < 29.9 || stream.FrameRate > 30.1 {
		t.Fatalf("unexpected frame rate: %f", stream.FrameRate)
	}
	if result.Duration.Seconds() != 5.5 {
		t.Fatalf("unexpected duration: %v", result.Duration)
	}
}

func TestParseProbeOutput_NoVideo(t *testing.T) {
	payload := []byte(`{"streams": [], "format": {}}`)
	_, err := parseProbeOutput(payload)
	if !errors.Is(err, ErrNoVideoStreams) {
		t.Fatalf("expected no video stream error, got %v", err)
	}
}

func TestLocalProberRejectsEmptyPath(t *testing.T) {
	prober := &LocalProber{}
	_, err := prober.Probe(context.Background(), " ")
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}
