package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"alex/internal/audio"
	"alex/internal/ffmpeg"
	"alex/internal/task"
)

func TestOrchestratorRetriesAudioStage(t *testing.T) {
	t.Helper()
	tempDir := t.TempDir()
	exec := &stubExecutor{audioFailures: 1}
	eng := &audio.Engine{Executor: exec}
	orch, err := New(Dependencies{FFmpeg: exec, Audio: eng})
	if err != nil {
		t.Fatalf("new orchestrator: %v", err)
	}
	spec := &task.JobSpec{
		Name:           "retry-audio",
		WorkingDir:     tempDir,
		AllowOverwrite: true,
		Video: task.VideoSpec{
			Segments: []task.VideoSegment{{Path: "video/seg1.mp4"}},
			Output:   "video/out.mp4",
		},
		Audio: task.AudioSpec{
			MixdownOutput: "audio/mix.wav",
			Tracks:        []task.AudioTrack{{Name: "bgm", Source: "audio/bgm.wav"}},
		},
		RetryPolicy: task.RetryPolicy{MaxAttempts: 2, Backoff: "0s"},
	}
	if err := orch.Run(context.Background(), spec); err != nil {
		t.Fatalf("run: %v", err)
	}
	if exec.audioRuns != 2 {
		t.Fatalf("expected 2 audio runs, got %d", exec.audioRuns)
	}
}

func TestOrchestratorStageTimeout(t *testing.T) {
	t.Helper()
	tempDir := t.TempDir()
	exec := &stubExecutor{concatDelay: 50 * time.Millisecond}
	eng := &audio.Engine{Executor: exec}
	orch, err := New(Dependencies{FFmpeg: exec, Audio: eng})
	if err != nil {
		t.Fatalf("new orchestrator: %v", err)
	}
	spec := &task.JobSpec{
		Name:           "timeout-video",
		WorkingDir:     tempDir,
		AllowOverwrite: true,
		Video: task.VideoSpec{
			Segments: []task.VideoSegment{{Path: "video/seg1.mp4"}},
			Output:   "video/out.mp4",
		},
		Audio: task.AudioSpec{
			MixdownOutput: "audio/mix.wav",
			Tracks:        []task.AudioTrack{{Name: "bgm", Source: "audio/bgm.wav"}},
		},
		StageTimeouts: task.StageTimeouts{"video_concat": "10ms"},
		RetryPolicy:   task.RetryPolicy{MaxAttempts: 1},
	}
	err = orch.Run(context.Background(), spec)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
	if exec.concatCalls == 0 {
		t.Fatal("expected concat to be called")
	}
}

func TestOrchestratorMetricsRecorded(t *testing.T) {
	t.Helper()
	tempDir := t.TempDir()
	registry := prometheus.NewRegistry()
	metrics := MustNewMetrics(registry)
	exec := &stubExecutor{audioFailures: 2}
	eng := &audio.Engine{Executor: exec}
	orch, err := New(Dependencies{FFmpeg: exec, Audio: eng, Metrics: metrics})
	if err != nil {
		t.Fatalf("new orchestrator: %v", err)
	}
	spec := &task.JobSpec{
		Name:           "metrics-audio-failure",
		WorkingDir:     tempDir,
		AllowOverwrite: true,
		Video: task.VideoSpec{
			Segments: []task.VideoSegment{{Path: "video/seg1.mp4"}},
			Output:   "video/out.mp4",
		},
		Audio: task.AudioSpec{
			MixdownOutput: "audio/mix.wav",
			Tracks:        []task.AudioTrack{{Name: "bgm", Source: "audio/bgm.wav"}},
		},
		RetryPolicy: task.RetryPolicy{MaxAttempts: 2, Backoff: "0s"},
	}
	err = orch.Run(context.Background(), spec)
	if err == nil {
		t.Fatal("expected run to fail after exhausting retries")
	}

	if got := testutil.ToFloat64(metrics.stageRetries.WithLabelValues("audio_mix")); got != 1 {
		t.Fatalf("expected 1 retry metric, got %v", got)
	}
	if got := testutil.ToFloat64(metrics.stageFailures.WithLabelValues("audio_mix", "error")); got != 1 {
		t.Fatalf("expected 1 failure metric, got %v", got)
	}
	if got := testutil.ToFloat64(metrics.jobsActive); got != 0 {
		t.Fatalf("expected jobs_active gauge to return to 0, got %v", got)
	}

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	var retrySamples, failureSamples uint64
	for _, mf := range families {
		if mf.GetName() != "local_av_orchestrator_job_stage_duration_seconds" {
			continue
		}
		for _, metric := range mf.GetMetric() {
			if metric.GetHistogram() == nil {
				continue
			}
			labels := map[string]string{}
			for _, lp := range metric.GetLabel() {
				labels[lp.GetName()] = lp.GetValue()
			}
			if labels["stage"] != "audio_mix" {
				continue
			}
			switch labels["status"] {
			case "retry":
				retrySamples += metric.GetHistogram().GetSampleCount()
			case "failed":
				failureSamples += metric.GetHistogram().GetSampleCount()
			}
		}
	}
	if retrySamples == 0 {
		t.Fatal("expected retry histogram samples to be recorded")
	}
	if failureSamples == 0 {
		t.Fatal("expected failure histogram samples to be recorded")
	}

}

type stubExecutor struct {
	audioFailures int
	audioRuns     int
	concatDelay   time.Duration
	concatErr     error
	concatCalls   int
	muxCalls      int
}

func (s *stubExecutor) Run(ctx context.Context, args []string) error {
	s.audioRuns++
	if s.audioRuns <= s.audioFailures {
		return errors.New("audio failure")
	}
	return nil
}

func (s *stubExecutor) RunWithOutput(ctx context.Context, args []string) (string, error) {
	return "", nil
}

func (s *stubExecutor) Concat(ctx context.Context, job ffmpeg.ConcatJob) error {
	s.concatCalls++
	if s.concatDelay > 0 {
		timer := time.NewTimer(s.concatDelay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	if s.concatErr != nil {
		return s.concatErr
	}
	return nil
}

func (s *stubExecutor) Mux(ctx context.Context, job ffmpeg.MuxJob) error {
	s.muxCalls++
	return nil
}
