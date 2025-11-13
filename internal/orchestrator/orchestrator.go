package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"log/slog"

	"alex/internal/audio"
	"alex/internal/ffmpeg"
	"alex/internal/storage"
	"alex/internal/task"
	"alex/internal/tts"
)

// Orchestrator coordinates TTS, audio mixing and video processing stages.
type Orchestrator struct {
	ffmpeg  ffmpeg.Executor
	audio   *audio.Engine
	tts     tts.Client
	storage *storage.Manager
	logger  *slog.Logger
	metrics *Metrics
	prober  ffmpeg.Prober
	presets *ffmpeg.PresetLibrary
}

// Dependencies lists the collaborators required to build an Orchestrator.
type Dependencies struct {
	FFmpeg  ffmpeg.Executor
	Audio   *audio.Engine
	TTS     tts.Client
	Storage *storage.Manager
	Logger  *slog.Logger
	Metrics *Metrics
	Prober  ffmpeg.Prober
	Presets *ffmpeg.PresetLibrary
}

// New creates an orchestrator with the provided dependencies.
func New(deps Dependencies) (*Orchestrator, error) {
	if deps.FFmpeg == nil {
		return nil, errors.New("orchestrator: ffmpeg executor is required")
	}
	if deps.Audio == nil {
		return nil, errors.New("orchestrator: audio engine is required")
	}
	metrics := deps.Metrics
	if metrics == nil {
		metrics = defaultMetrics()
	}

	return &Orchestrator{
		ffmpeg:  deps.FFmpeg,
		audio:   deps.Audio,
		tts:     deps.TTS,
		storage: deps.Storage,
		logger:  deps.Logger,
		metrics: metrics,
		prober:  deps.Prober,
		presets: deps.Presets,
	}, nil
}

// Run executes the orchestration pipeline.
func (o *Orchestrator) Run(ctx context.Context, spec *task.JobSpec) error {
	if spec == nil {
		return errors.New("nil job spec")
	}
	if err := spec.Validate(); err != nil {
		return err
	}
	workingRel := filepath.Clean(spec.WorkingDir)
	workingAbs := workingRel
	if o.storage != nil {
		abs, err := o.storage.EnsureDirectory(workingRel)
		if err != nil {
			return err
		}
		workingAbs = abs
	} else if !filepath.IsAbs(workingRel) {
		abs, err := filepath.Abs(workingRel)
		if err != nil {
			return err
		}
		workingAbs = abs
		if err := ensureDir(abs); err != nil {
			return err
		}
	}

	attempts := spec.RetryPolicy.MaxAttempts
	if attempts <= 0 {
		attempts = 1
	}
	backoff, err := parseBackoff(spec.RetryPolicy.Backoff)
	if err != nil {
		return err
	}
	jitter := spec.RetryPolicy.Jitter

	// Stage 1: TTS synthesis
	ttsOutputs := map[string]string{}
	o.metrics.IncActiveJobs()
	defer o.metrics.DecActiveJobs()

	if err := o.runStage(ctx, "tts", attempts, backoff, jitter, spec.StageTimeouts, func(stageCtx context.Context) error {
		if len(spec.TTS) == 0 {
			o.log().Info("stage skipped", slog.String("stage", "tts"))
			return nil
		}
		if o.tts == nil {
			return errors.New("tts requests configured but no TTS client available")
		}
		for _, request := range spec.TTS {
			select {
			case <-stageCtx.Done():
				return stageCtx.Err()
			default:
			}
			req := tts.Request{
				Alias:      request.Alias,
				Text:       request.Text,
				Voice:      request.Voice,
				Style:      request.Style,
				Format:     request.Format,
				Parameters: request.Parameters,
			}
			result, synthErr := o.tts.Synthesize(stageCtx, req)
			if synthErr != nil {
				return fmt.Errorf("tts alias %s: %w", request.Alias, synthErr)
			}
			ttsOutputs[request.Alias] = result.Path
			o.log().Info("tts complete", slog.String("alias", request.Alias), slog.Bool("cache", result.FromCache))
		}
		return nil
	}); err != nil {
		return err
	}

	// Stage 2: audio mix
	var mixPath string
	if err := o.runStage(ctx, "audio_mix", attempts, backoff, jitter, spec.StageTimeouts, func(stageCtx context.Context) error {
		path, mixErr := o.audio.Mixdown(stageCtx, spec.Audio, workingAbs, workingRel, spec.AllowOverwrite, ttsOutputs)
		if mixErr != nil {
			return mixErr
		}
		mixPath = path
		return nil
	}); err != nil {
		return err
	}

	// Determine video intermediate and final paths
	videoOutputRel := spec.Video.Output
	finalOutputRel := spec.Video.FinalOutput
	if strings.TrimSpace(finalOutputRel) == "" {
		finalOutputRel = videoOutputRel
	}
	if finalOutputRel == videoOutputRel {
		ext := filepath.Ext(videoOutputRel)
		base := strings.TrimSuffix(videoOutputRel, ext)
		videoOutputRel = fmt.Sprintf("%s-video%s", base, ext)
	}

	videoInputs := make([]string, 0, len(spec.Video.Segments))
	for _, segment := range spec.Video.Segments {
		path, resolveErr := o.resolvePath(segment.Path, workingAbs, workingRel)
		if resolveErr != nil {
			return fmt.Errorf("segment %s: %w", segment.Path, resolveErr)
		}
		videoInputs = append(videoInputs, path)
	}
	if len(spec.Video.Filters) == 0 {
		if err := o.validateVideoInputs(ctx, videoInputs); err != nil {
			return err
		}
	}
	var preset *ffmpeg.Preset
	if strings.TrimSpace(spec.Video.Preset) != "" {
		if o.presets == nil {
			return fmt.Errorf("video.preset %q requested but no preset library configured", spec.Video.Preset)
		}
		p, ok := o.presets.Get(spec.Video.Preset)
		if !ok {
			return fmt.Errorf("video.preset %q not found", spec.Video.Preset)
		}
		preset = &p
	}
	videoOutput, err := o.resolveOutput(videoOutputRel, workingAbs, workingRel)
	if err != nil {
		return err
	}
	if err := o.runStage(ctx, "video_concat", attempts, backoff, jitter, spec.StageTimeouts, func(stageCtx context.Context) error {
		videoCodec := "h264"
		audioCodec := "aac"
		extraArgs := []string(nil)
		if preset != nil {
			if preset.VideoCodec != "" {
				videoCodec = preset.VideoCodec
			}
			if preset.AudioCodec != "" {
				audioCodec = preset.AudioCodec
			}
			extraArgs = append(extraArgs, preset.Args()...)
		}
		concatJob := ffmpeg.ConcatJob{
			Inputs:      videoInputs,
			Output:      videoOutput,
			FilterGraph: buildVideoFilter(spec.Video, preset),
			VideoCodec:  videoCodec,
			AudioCodec:  audioCodec,
			ExtraArgs:   extraArgs,
			Overwrite:   spec.AllowOverwrite,
		}
		if concatErr := o.ffmpeg.Concat(stageCtx, concatJob); concatErr != nil {
			return concatErr
		}
		return nil
	}); err != nil {
		return err
	}

	muxOutput, err := o.resolveOutput(finalOutputRel, workingAbs, workingRel)
	if err != nil {
		return err
	}
	if err := o.runStage(ctx, "mux", attempts, backoff, jitter, spec.StageTimeouts, func(stageCtx context.Context) error {
		muxJob := ffmpeg.MuxJob{
			VideoPath: videoOutput,
			AudioPath: mixPath,
			Output:    muxOutput,
			Overwrite: spec.AllowOverwrite,
		}
		return o.ffmpeg.Mux(stageCtx, muxJob)
	}); err != nil {
		return err
	}

	o.log().Info("job complete", slog.String("job", spec.Name), slog.String("output", muxOutput))
	return nil
}

func (o *Orchestrator) resolvePath(path string, workingAbs string, workingRel string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	if o.storage != nil {
		return o.storage.Resolve(filepath.Join(workingRel, path))
	}
	return filepath.Join(workingAbs, path), nil
}

func (o *Orchestrator) resolveOutput(path string, workingAbs string, workingRel string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("output path is empty")
	}
	if o.storage != nil {
		rel := filepath.Join(workingRel, path)
		if _, err := o.storage.EnsureDir(rel); err != nil {
			return "", err
		}
		return o.storage.Resolve(rel)
	}
	full := filepath.Join(workingAbs, path)
	if err := ensureDir(filepath.Dir(full)); err != nil {
		return "", err
	}
	return full, nil
}

func (o *Orchestrator) log() *slog.Logger {
	if o.logger != nil {
		return o.logger
	}
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
}

func (o *Orchestrator) validateVideoInputs(ctx context.Context, inputs []string) error {
	if o.prober == nil || len(inputs) <= 1 {
		return nil
	}
	var baseline *ffmpeg.VideoStream
	for _, input := range inputs {
		result, err := o.prober.Probe(ctx, input)
		if err != nil {
			return fmt.Errorf("ffprobe %s: %w", input, err)
		}
		stream, ok := result.FirstVideo()
		if !ok {
			return fmt.Errorf("ffprobe %s: no video stream detected", input)
		}
		if baseline == nil {
			s := stream
			baseline = &s
			continue
		}
		if !compatibleVideoStream(*baseline, stream) {
			return fmt.Errorf("video input %s parameters differ from baseline (%dx%d %s %.3ffps)", input, baseline.Width, baseline.Height, baseline.PixelFormat, baseline.FrameRate)
		}
	}
	return nil
}

func compatibleVideoStream(a, b ffmpeg.VideoStream) bool {
	if a.Width != b.Width || a.Height != b.Height {
		return false
	}
	if !strings.EqualFold(a.PixelFormat, b.PixelFormat) {
		return false
	}
	if !strings.EqualFold(a.CodecName, b.CodecName) {
		return false
	}
	const frameRateTolerance = 0.01
	diff := math.Abs(a.FrameRate - b.FrameRate)
	return diff <= frameRateTolerance
}

func buildVideoFilter(spec task.VideoSpec, preset *ffmpeg.Preset) string {
	filters := make([]string, 0, len(spec.Filters))
	if preset != nil {
		filters = append(filters, preset.Filters...)
	}
	filters = append(filters, spec.Filters...)
	if len(filters) > 0 {
		return strings.Join(filters, ";")
	}
	return ""
}

func ensureDir(dir string) error {
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func (o *Orchestrator) runStage(ctx context.Context, stage string, attempts int, backoff time.Duration, jitter float64, timeouts task.StageTimeouts, fn func(context.Context) error) error {
	if attempts <= 0 {
		attempts = 1
	}
	timeout, hasTimeout, err := timeouts.Duration(stage)
	if err != nil {
		return fmt.Errorf("stage_timeouts[%s]: %w", stage, err)
	}
	for attempt := 1; attempt <= attempts; attempt++ {
		stageCtx := ctx
		var cancel context.CancelFunc
		if hasTimeout && timeout > 0 {
			stageCtx, cancel = context.WithTimeout(ctx, timeout)
		}
		start := time.Now()
		o.log().Info("stage start", slog.String("stage", stage), slog.Int("attempt", attempt))
		runErr := fn(stageCtx)
		stageErr := stageCtx.Err()
		if cancel != nil {
			cancel()
		}
		duration := time.Since(start)
		if runErr == nil {
			o.metrics.ObserveStageDuration(stage, "success", duration)
			o.log().Info("stage complete", slog.String("stage", stage), slog.Duration("duration", duration), slog.Int("attempt", attempt))
			return nil
		}
		if errors.Is(stageErr, context.DeadlineExceeded) {
			o.metrics.ObserveStageDuration(stage, "timeout", duration)
			o.metrics.IncStageFailure(stage, "timeout")
			o.log().Error("stage timeout", slog.String("stage", stage), slog.Duration("duration", duration))
			return fmt.Errorf("%s: %w", stage, stageErr)
		}
		if errors.Is(stageErr, context.Canceled) {
			o.metrics.ObserveStageDuration(stage, "canceled", duration)
			o.metrics.IncStageFailure(stage, "canceled")
			o.log().Error("stage cancelled", slog.String("stage", stage), slog.Duration("duration", duration))
			return fmt.Errorf("%s: %w", stage, stageErr)
		}
		if errors.Is(runErr, context.DeadlineExceeded) || errors.Is(runErr, context.Canceled) {
			o.metrics.ObserveStageDuration(stage, "canceled", duration)
			o.metrics.IncStageFailure(stage, "canceled")
			o.log().Error("stage cancelled", slog.String("stage", stage), slog.Duration("duration", duration), slog.String("error", runErr.Error()))
			return fmt.Errorf("%s: %w", stage, runErr)
		}
		if attempt == attempts {
			o.metrics.ObserveStageDuration(stage, "failed", duration)
			o.metrics.IncStageFailure(stage, "error")
			o.log().Error("stage failed", slog.String("stage", stage), slog.Int("attempt", attempt), slog.Duration("duration", duration), slog.String("error", runErr.Error()))
			return fmt.Errorf("%s: %w", stage, runErr)
		}
		o.metrics.ObserveStageDuration(stage, "retry", duration)
		o.metrics.IncStageRetry(stage)
		o.log().Warn("stage retry", slog.String("stage", stage), slog.Int("attempt", attempt), slog.String("error", runErr.Error()))
		wait := applyBackoff(backoff, jitter)
		if wait > 0 {
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return fmt.Errorf("%s: %w", stage, ctx.Err())
			case <-timer.C:
			}
		}
	}
	return nil
}

func applyBackoff(base time.Duration, jitter float64) time.Duration {
	if base <= 0 {
		return 0
	}
	if jitter <= 0 {
		return base
	}
	jitter = math.Min(math.Max(jitter, 0), 1)
	factor := 1 + (rand.Float64()*2-1)*jitter
	if factor < 0 {
		factor = 0
	}
	return time.Duration(float64(base) * factor)
}

func parseBackoff(raw string) (time.Duration, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("retry_policy.backoff: %w", err)
	}
	return d, nil
}
