package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"alex/internal/audio"
	"alex/internal/ffmpeg"
	"alex/internal/orchestrator"
	"alex/internal/storage"
	"alex/internal/task"
	"alex/internal/tts"
)

func main() {
	var (
		specPath  = flag.String("spec", "", "Path to the job specification YAML")
		rootDir   = flag.String("root", ".", "Root directory for storage operations")
		ffmpegBin = flag.String("ffmpeg-bin", "", "Path to the ffmpeg binary")
		dryRun    = flag.Bool("dry-run", false, "Print commands without executing them")
		mockTTS   = flag.Bool("mock-tts", false, "Use a silent mock TTS provider")
		logLevel  = flag.String("log-level", "info", "Log level (debug|info|warn|error)")
		timeout   = flag.Duration("timeout", 0, "Optional timeout for the entire job")
	)
	flag.Parse()

	if strings.TrimSpace(*specPath) == "" {
		fmt.Fprintln(os.Stderr, "--spec is required")
		os.Exit(2)
	}

	spec, err := task.LoadSpec(*specPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load spec: %v\n", err)
		os.Exit(1)
	}

	level := parseLevel(*logLevel)
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level, AddSource: false})
	logger := slog.New(handler)

	storageManager, err := storage.NewManager(storage.Config{Root: *rootDir, AllowOverwrite: spec.AllowOverwrite})
	if err != nil {
		fmt.Fprintf(os.Stderr, "init storage: %v\n", err)
		os.Exit(1)
	}

	ffmpegExec := &ffmpeg.LocalExecutor{
		Binary:  *ffmpegBin,
		DryRun:  *dryRun,
		Logger:  logger,
		Storage: storageManager,
	}
	audioEngine := &audio.Engine{
		Executor: ffmpegExec,
		Storage:  storageManager,
		Logger:   logger,
	}

	var ttsClient tts.Client
	if len(spec.TTS) > 0 {
		if *mockTTS {
			cacheDir := filepath.Join(spec.WorkingDir, "cache", "tts")
			ttsClient = &tts.FileCacheClient{
				Provider: tts.MockProvider{SampleRate: 16000},
				Storage:  storageManager,
				CacheDir: cacheDir,
			}
			// ensure mixdown expects wav format
			for i := range spec.TTS {
				if spec.TTS[i].Format == "" {
					spec.TTS[i].Format = "wav"
				}
			}
		} else {
			fmt.Fprintln(os.Stderr, "TTS requests present but no provider configured. Use --mock-tts for development.")
			os.Exit(1)
		}
	}

	orch, err := orchestrator.New(orchestrator.Dependencies{
		FFmpeg:  ffmpegExec,
		Audio:   audioEngine,
		TTS:     ttsClient,
		Storage: storageManager,
		Logger:  logger,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "init orchestrator: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	if timeout != nil && *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}

	if err := orch.Run(ctx, spec); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			fmt.Fprintln(os.Stderr, "job timed out")
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "job failed: %v\n", err)
		os.Exit(1)
	}
}

func parseLevel(value string) slog.Level {
	switch strings.ToLower(value) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
