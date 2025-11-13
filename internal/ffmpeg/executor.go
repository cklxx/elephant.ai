package ffmpeg

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"log/slog"

	"alex/internal/storage"
)

// Executor defines the operations required from the FFmpeg pipeline.
type Executor interface {
	Concat(ctx context.Context, job ConcatJob) error
	Mux(ctx context.Context, job MuxJob) error
	Run(ctx context.Context, args []string) error
	RunWithOutput(ctx context.Context, args []string) (string, error)
}

// LocalExecutor executes FFmpeg commands on the host machine.
type LocalExecutor struct {
	Binary  string
	DryRun  bool
	Logger  *slog.Logger
	Storage *storage.Manager
}

// ConcatJob describes a concatenation task.
type ConcatJob struct {
	Inputs      []string
	Output      string
	FilterGraph string
	VideoCodec  string
	AudioCodec  string
	ExtraArgs   []string
	Overwrite   bool
}

// MuxJob merges separate audio and video assets into a single container.
type MuxJob struct {
	VideoPath string
	AudioPath string
	Output    string
	ExtraArgs []string
	Overwrite bool
}

// Run executes a raw FFmpeg command.
func (l *LocalExecutor) Run(ctx context.Context, args []string) error {
	_, err := l.runInternal(ctx, args)
	return err
}

// RunWithOutput executes an FFmpeg command and returns the combined stdout/stderr.
func (l *LocalExecutor) RunWithOutput(ctx context.Context, args []string) (string, error) {
	return l.runInternal(ctx, args)
}

func (l *LocalExecutor) binary() string {
	if strings.TrimSpace(l.Binary) == "" {
		return "ffmpeg"
	}
	return l.Binary
}

// Concat executes a concat pipeline for the provided inputs.
func (l *LocalExecutor) Concat(ctx context.Context, job ConcatJob) error {
	if len(job.Inputs) == 0 {
		return fmt.Errorf("concat: at least one input required")
	}
	if err := l.ensureParent(job.Output); err != nil {
		return err
	}
	args := []string{"-y"}
	if !job.Overwrite {
		args[0] = "-n"
	}
	for _, input := range job.Inputs {
		args = append(args, "-i", input)
	}
	filter := job.FilterGraph
	if strings.TrimSpace(filter) == "" {
		filter = fmt.Sprintf("concat=n=%d:v=1:a=1", len(job.Inputs))
	}
	args = append(args, "-filter_complex", filter)
	if job.VideoCodec != "" {
		args = append(args, "-c:v", job.VideoCodec)
	}
	if job.AudioCodec != "" {
		args = append(args, "-c:a", job.AudioCodec)
	}
	args = append(args, job.ExtraArgs...)
	args = append(args, job.Output)
	return l.Run(ctx, args)
}

// Mux combines a prepared video stream with an audio track.
func (l *LocalExecutor) Mux(ctx context.Context, job MuxJob) error {
	if err := l.ensureParent(job.Output); err != nil {
		return err
	}
	args := []string{"-y"}
	if !job.Overwrite {
		args[0] = "-n"
	}
	args = append(args, "-i", job.VideoPath, "-i", job.AudioPath)
	args = append(args, job.ExtraArgs...)
	args = append(args, "-c:v", "copy", "-c:a", "copy", job.Output)
	return l.Run(ctx, args)
}

func (l *LocalExecutor) ensureParent(output string) error {
	if l.Storage != nil {
		_, err := l.Storage.EnsureDir(output)
		return err
	}
	dir := filepath.Dir(output)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func (l *LocalExecutor) logger() *slog.Logger {
	if l.Logger != nil {
		return l.Logger
	}
	return defaultLogger
}

func (l *LocalExecutor) runInternal(ctx context.Context, args []string) (string, error) {
	logger := l.logger()
	logger.Debug("ffmpeg command", slog.String("binary", l.binary()), slog.String("args", strings.Join(args, " ")))
	if l.DryRun {
		return "", nil
	}
	cmd := exec.CommandContext(ctx, l.binary(), args...)
	out, err := cmd.CombinedOutput()
	output := string(out)
	logger.Debug("ffmpeg output", slog.String("output", output))
	if err != nil {
		return output, fmt.Errorf("ffmpeg: %w", err)
	}
	return output, nil
}

var defaultLogger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
