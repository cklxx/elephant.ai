package task

import (
	"strings"
	"testing"
)

func TestJobSpecValidateNormalizesTTSFormat(t *testing.T) {
	spec := &JobSpec{
		Name:           "tts-default",
		WorkingDir:     "workdir",
		AllowOverwrite: true,
		Video: VideoSpec{
			Segments: []VideoSegment{{Path: "video/a.mp4"}},
			Output:   "video/out.mp4",
		},
		Audio: AudioSpec{
			MixdownOutput: "audio/mix.wav",
			Tracks:        []AudioTrack{{Name: "narration", Source: "tts:narr"}},
		},
		TTS: []TTSRequest{
			{
				Alias: "narr",
				Text:  "hello world",
				Voice: "en-US-test",
			},
			{
				Alias:  "intro",
				Text:   "intro line",
				Voice:  "en-US-test",
				Format: " wav ",
			},
		},
	}

	if err := spec.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if got, want := spec.TTS[0].Format, "mp3"; got != want {
		t.Fatalf("expected default format %q, got %q", want, got)
	}
	if got, want := spec.TTS[1].Format, "wav"; got != want {
		t.Fatalf("expected trimmed format %q, got %q", want, got)
	}
}

func TestJobSpecValidateTrimsTTSAliasAndAudioReferences(t *testing.T) {
	spec := &JobSpec{
		Name:           "tts-alias",
		WorkingDir:     "workdir",
		AllowOverwrite: true,
		Video: VideoSpec{
			Segments: []VideoSegment{{Path: "video/a.mp4"}},
			Output:   "video/out.mp4",
		},
		Audio: AudioSpec{
			MixdownOutput: "audio/mix.wav",
			Tracks:        []AudioTrack{{Name: "narration", Source: "tts: narr "}},
		},
		TTS: []TTSRequest{{
			Alias: " narr ",
			Text:  "hello world",
			Voice: "en-US-test",
		}},
	}

	if err := spec.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if got, want := spec.TTS[0].Alias, "narr"; got != want {
		t.Fatalf("expected trimmed alias %q, got %q", want, got)
	}
}

func TestJobSpecValidateDetectsDuplicateTTSAliasAfterTrim(t *testing.T) {
	spec := &JobSpec{
		Name:           "tts-dup",
		WorkingDir:     "workdir",
		AllowOverwrite: true,
		Video: VideoSpec{
			Segments: []VideoSegment{{Path: "video/a.mp4"}},
			Output:   "video/out.mp4",
		},
		Audio: AudioSpec{
			MixdownOutput: "audio/mix.wav",
			Tracks:        []AudioTrack{{Name: "narration", Source: "tts:narr"}},
		},
		TTS: []TTSRequest{
			{Alias: " narr ", Text: "hello world", Voice: "en-US-test"},
			{Alias: "narr", Text: "second", Voice: "en-US-test"},
		},
	}

	err := spec.Validate()
	if err == nil {
		t.Fatalf("expected duplicate alias error, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate alias \"narr\"") {
		t.Fatalf("unexpected error: %v", err)
	}
}
