package audio

import (
	"context"
	"errors"
	"strings"
	"testing"

	"alex/internal/ffmpeg"
	"alex/internal/task"
)

type stubFFmpegExecutor struct {
	runCalls       [][]string
	captureCalls   [][]string
	captureOutputs []string
	runErr         error
	captureErr     error
}

func (s *stubFFmpegExecutor) Run(ctx context.Context, args []string) error {
	cloned := append([]string(nil), args...)
	s.runCalls = append(s.runCalls, cloned)
	if s.runErr != nil {
		return s.runErr
	}
	return nil
}

func (s *stubFFmpegExecutor) RunWithOutput(ctx context.Context, args []string) (string, error) {
	cloned := append([]string(nil), args...)
	s.captureCalls = append(s.captureCalls, cloned)
	if s.captureErr != nil {
		return "", s.captureErr
	}
	if len(s.captureOutputs) == 0 {
		return "", nil
	}
	out := s.captureOutputs[0]
	s.captureOutputs = s.captureOutputs[1:]
	return out, nil
}

func (s *stubFFmpegExecutor) Concat(ctx context.Context, job ffmpeg.ConcatJob) error {
	return nil
}

func (s *stubFFmpegExecutor) Mux(ctx context.Context, job ffmpeg.MuxJob) error {
	return nil
}

func TestMixdownUsesLoudnormMeasurements(t *testing.T) {
	exec := &stubFFmpegExecutor{
		captureOutputs: []string{`[Parsed_loudnorm_0 @ 0x7f] {
    "input_i" : "-20.3",
    "input_lra" : "4.5",
    "input_tp" : "-1.2",
    "input_thresh" : "-30.0",
    "target_offset" : "-3.2"
}`},
	}
	engine := &Engine{Executor: exec}
	spec := task.AudioSpec{
		MixdownOutput:  "mix.wav",
		LoudnessTarget: -16,
		Tracks:         []task.AudioTrack{{Name: "voice", Source: "voice.wav"}},
	}
	if _, err := engine.Mixdown(context.Background(), spec, "/abs", "rel", true, nil); err != nil {
		t.Fatalf("mixdown failed: %v", err)
	}
	if got := len(exec.captureCalls); got != 1 {
		t.Fatalf("expected 1 loudnorm measurement call, got %d", got)
	}
	if got := len(exec.runCalls); got != 1 {
		t.Fatalf("expected 1 render call, got %d", got)
	}
	finalArgs := strings.Join(exec.runCalls[0], " ")
	for _, token := range []string{"measured_I=-20.30", "measured_LRA=4.50", "measured_TP=-1.20", "measured_thresh=-30.00", "offset=-3.20"} {
		if !strings.Contains(finalArgs, token) {
			t.Fatalf("expected final args to contain %q, got %s", token, finalArgs)
		}
	}
}

func TestMixdownCachesLoudnormStats(t *testing.T) {
	exec := &stubFFmpegExecutor{
		captureOutputs: []string{`[Parsed_loudnorm_0 @ 0x7f] {
    "input_i" : "-22.1",
    "input_lra" : "3.1",
    "input_tp" : "-0.5",
    "input_thresh" : "-28.0",
    "target_offset" : "-1.0"
}`},
	}
	engine := &Engine{Executor: exec}
	spec := task.AudioSpec{
		MixdownOutput:  "mix.wav",
		LoudnessTarget: -16,
		Tracks:         []task.AudioTrack{{Name: "voice", Source: "voice.wav"}},
	}
	if _, err := engine.Mixdown(context.Background(), spec, "/abs", "rel", true, nil); err != nil {
		t.Fatalf("first mixdown failed: %v", err)
	}
	if _, err := engine.Mixdown(context.Background(), spec, "/abs", "rel", true, nil); err != nil {
		t.Fatalf("second mixdown failed: %v", err)
	}
	if got := len(exec.captureCalls); got != 1 {
		t.Fatalf("expected loudnorm measurement to execute once, got %d", got)
	}
	if got := len(exec.runCalls); got != 2 {
		t.Fatalf("expected two render calls, got %d", got)
	}
}

func TestMixdownFallsBackWhenMeasurementFails(t *testing.T) {
	exec := &stubFFmpegExecutor{
		captureErr: errors.New("ffmpeg failure"),
	}
	engine := &Engine{Executor: exec}
	spec := task.AudioSpec{
		MixdownOutput:  "mix.wav",
		LoudnessTarget: -16,
		Tracks:         []task.AudioTrack{{Name: "voice", Source: "voice.wav"}},
	}
	if _, err := engine.Mixdown(context.Background(), spec, "/abs", "rel", true, nil); err != nil {
		t.Fatalf("mixdown failed despite fallback: %v", err)
	}
	if got := len(exec.captureCalls); got != 1 {
		t.Fatalf("expected one measurement attempt, got %d", got)
	}
	if got := len(exec.runCalls); got != 1 {
		t.Fatalf("expected one render call, got %d", got)
	}
	finalArgs := strings.Join(exec.runCalls[0], " ")
	if strings.Contains(finalArgs, "measured_") {
		t.Fatalf("expected fallback render to omit measured values, got %s", finalArgs)
	}
	if !strings.Contains(finalArgs, "loudnorm=I=-16.0:TP=-1.5:LRA=11") {
		t.Fatalf("expected fallback loudnorm filter, got %s", finalArgs)
	}
}

func TestMixdownAppliesEnvelopeFilters(t *testing.T) {
	exec := &stubFFmpegExecutor{captureOutputs: []string{`{"input_i":"-20.0","input_lra":"3.0","input_tp":"-1.0","input_thresh":"-30.0","target_offset":"-3.0"}`}}
	engine := &Engine{Executor: exec}
	spec := task.AudioSpec{
		MixdownOutput:  "mix.wav",
		LoudnessTarget: -16,
		Tracks: []task.AudioTrack{{
			Name:   "bgm",
			Source: "bgm.wav",
			Loop:   true,
			Envelope: &task.EnvelopeSpec{
				FadeIn: "1.5s",
				FadeOut: &task.FadeOutSpec{
					Start:    "55s",
					Duration: "5s",
				},
				Segments: []task.EnvelopeSegment{
					{Start: "0s", End: "10s", Gain: -12},
					{Start: "10s", Gain: -6},
				},
			},
		}},
	}
	if _, err := engine.Mixdown(context.Background(), spec, "/abs", "rel", true, nil); err != nil {
		t.Fatalf("mixdown failed: %v", err)
	}
	if len(exec.runCalls) != 1 {
		t.Fatalf("expected one run call, got %d", len(exec.runCalls))
	}
	final := strings.Join(exec.runCalls[0], " ")
	for _, expected := range []string{
		"aloop=loop=-1",
		"afade=t=in:ss=0:d=1.500",
		"afade=t=out:st=55.000:d=5.000",
		"volume=volume=0.2512:enable='between(t,0.000,10.000)'",
		"volume=volume=0.5012:enable='gte(t,10.000)'",
	} {
		if !strings.Contains(final, expected) {
			t.Fatalf("expected final args to contain %q, got %s", expected, final)
		}
	}
}

func TestMixdownFailsForInvalidEnvelope(t *testing.T) {
	engine := &Engine{Executor: &stubFFmpegExecutor{}}
	spec := task.AudioSpec{
		MixdownOutput:  "mix.wav",
		LoudnessTarget: -16,
		Tracks: []task.AudioTrack{{
			Name:   "bgm",
			Source: "bgm.wav",
			Envelope: &task.EnvelopeSpec{
				FadeIn: "not-a-duration",
			},
		}},
	}
	if _, err := engine.Mixdown(context.Background(), spec, "/abs", "rel", true, nil); err == nil {
		t.Fatalf("expected error for invalid envelope duration")
	}
}
