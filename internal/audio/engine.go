package audio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"log/slog"

	"alex/internal/ffmpeg"
	"alex/internal/storage"
	"alex/internal/task"
)

// Engine wraps FFmpeg to mix audio tracks according to the job specification.
type Engine struct {
	Executor ffmpeg.Executor
	Storage  *storage.Manager
	Logger   *slog.Logger

	cacheMu       sync.RWMutex
	loudnormCache map[string]loudnormStats
}

// Mixdown renders the mix described by the audio spec. The tts map resolves
// TTS aliases to absolute file paths.
func (e *Engine) Mixdown(ctx context.Context, spec task.AudioSpec, workingAbs string, workingRel string, allowOverwrite bool, tts map[string]string) (string, error) {
	if e.Executor == nil {
		return "", fmt.Errorf("audio engine: executor is required")
	}
	if len(spec.Tracks) == 0 {
		return "", fmt.Errorf("audio engine: no tracks configured")
	}
	var inputs []string
	var preprocess []string
	var mixInputs []string
	for idx, track := range spec.Tracks {
		src, err := e.resolveSource(track.Source, workingAbs, workingRel, tts)
		if err != nil {
			return "", fmt.Errorf("track %q: %w", track.Name, err)
		}
		inputs = append(inputs, src)
		labelIn := fmt.Sprintf("[%d:a]", idx)
		labelOut := fmt.Sprintf("[a%d]", idx)
		filters, err := e.buildFilters(track)
		if err != nil {
			return "", fmt.Errorf("track %q: %w", track.Name, err)
		}
		if filters != "" {
			chain := fmt.Sprintf("%s%s%s", labelIn, filters, labelOut)
			preprocess = append(preprocess, chain)
			mixInputs = append(mixInputs, labelOut)
		} else {
			mixInputs = append(mixInputs, labelIn)
		}
	}
	if len(mixInputs) == 0 {
		return "", fmt.Errorf("audio engine: unable to build filter graph")
	}
	mixFilter := fmt.Sprintf("%samix=inputs=%d:normalize=0[mix]", strings.Join(mixInputs, ""), len(mixInputs))
	graphParts := append([]string{}, preprocess...)
	graphParts = append(graphParts, mixFilter)
	var filterGraph string
	if spec.LoudnessTarget != 0 {
		stats, fromCache, err := e.measureLoudnorm(ctx, inputs, graphParts, spec.LoudnessTarget)
		if err != nil {
			e.logger().Warn("audio engine loudnorm measurement failed, falling back to single pass", slog.String("error", err.Error()))
			graphParts = append(graphParts, fmt.Sprintf("[mix]loudnorm=I=%.1f:TP=-1.5:LRA=11[outa]", spec.LoudnessTarget))
		} else {
			e.logger().Info("audio loudnorm stats ready", slog.Bool("cache", fromCache))
			measured := fmt.Sprintf("[mix]loudnorm=I=%.1f:TP=-1.5:LRA=11:measured_I=%.2f:measured_LRA=%.2f:measured_TP=%.2f:measured_thresh=%.2f:offset=%.2f:linear=true[outa]",
				spec.LoudnessTarget,
				stats.InputI,
				stats.InputLRA,
				stats.InputTP,
				stats.InputThresh,
				stats.TargetOffset,
			)
			graphParts = append(graphParts, measured)
		}
	} else {
		graphParts = append(graphParts, "[mix]anull[outa]")
	}
	filterGraph = strings.Join(graphParts, ";")

	relOutput := spec.MixdownOutput
	if strings.TrimSpace(relOutput) == "" {
		return "", fmt.Errorf("audio engine: mixdown_output is empty")
	}
	if workingRel != "" {
		relOutput = filepath.Join(workingRel, relOutput)
	}
	output := filepath.Join(workingAbs, spec.MixdownOutput)
	if e.Storage != nil {
		if _, err := e.Storage.EnsureDir(relOutput); err != nil {
			return "", fmt.Errorf("ensure output dir: %w", err)
		}
		resolved, err := e.Storage.Resolve(relOutput)
		if err != nil {
			return "", err
		}
		output = resolved
	}

	args := []string{"-y"}
	if !allowOverwrite {
		args[0] = "-n"
	}
	for _, input := range inputs {
		args = append(args, "-i", input)
	}
	args = append(args, "-filter_complex", filterGraph, "-map", "[outa]", "-c:a", "pcm_s16le", output)

	if err := e.Executor.Run(ctx, args); err != nil {
		return "", err
	}
	e.logger().Info("audio mix complete", slog.String("output", output))
	return output, nil
}

func (e *Engine) resolveSource(source string, workingAbs string, workingRel string, tts map[string]string) (string, error) {
	if strings.HasPrefix(source, "tts:") {
		alias := strings.TrimPrefix(source, "tts:")
		path, ok := tts[alias]
		if !ok {
			return "", fmt.Errorf("missing TTS alias %q", alias)
		}
		return path, nil
	}
	if filepath.IsAbs(source) {
		return source, nil
	}
	joined := source
	if workingRel != "" {
		joined = filepath.Join(workingRel, source)
	}
	if e.Storage != nil {
		return e.Storage.Resolve(joined)
	}
	return filepath.Join(workingAbs, source), nil
}

func (e *Engine) buildFilters(track task.AudioTrack) (string, error) {
	var filters []string
	if track.Offset != "" {
		if d, err := time.ParseDuration(track.Offset); err == nil {
			ms := int(math.Round(d.Seconds() * 1000))
			if ms > 0 {
				filters = append(filters, fmt.Sprintf("adelay=%d|%d", ms, ms))
			}
		}
	}
	if track.Gain != 0 {
		filters = append(filters, fmt.Sprintf("volume=%fdB", track.Gain))
	}
	for _, effect := range track.Effects {
		if f := mapEffect(effect); f != "" {
			filters = append(filters, f)
		}
	}
	if track.Loop {
		filters = append(filters, "aloop=loop=-1")
	}
	if track.Envelope != nil {
		envFilters, err := buildEnvelopeFilters(track.Envelope)
		if err != nil {
			return "", err
		}
		filters = append(filters, envFilters...)
	}
	if len(filters) == 0 {
		return "", nil
	}
	return strings.Join(filters, ","), nil
}

func buildEnvelopeFilters(env *task.EnvelopeSpec) ([]string, error) {
	if env == nil {
		return nil, nil
	}
	var filters []string
	if strings.TrimSpace(env.FadeIn) != "" {
		dur, err := time.ParseDuration(env.FadeIn)
		if err != nil {
			return nil, fmt.Errorf("fade_in: %w", err)
		}
		if dur > 0 {
			filters = append(filters, fmt.Sprintf("afade=t=in:ss=0:d=%.3f", dur.Seconds()))
		}
	}
	if env.FadeOut != nil {
		start, err := time.ParseDuration(env.FadeOut.Start)
		if err != nil {
			return nil, fmt.Errorf("fade_out.start: %w", err)
		}
		dur, err := time.ParseDuration(env.FadeOut.Duration)
		if err != nil {
			return nil, fmt.Errorf("fade_out.duration: %w", err)
		}
		if dur > 0 {
			filters = append(filters, fmt.Sprintf("afade=t=out:st=%.3f:d=%.3f", start.Seconds(), dur.Seconds()))
		}
	}
	for _, seg := range env.Segments {
		start, err := time.ParseDuration(seg.Start)
		if err != nil {
			return nil, fmt.Errorf("segments start: %w", err)
		}
		var condition string
		if strings.TrimSpace(seg.End) != "" {
			end, err := time.ParseDuration(seg.End)
			if err != nil {
				return nil, fmt.Errorf("segments end: %w", err)
			}
			condition = fmt.Sprintf("between(t,%.3f,%.3f)", start.Seconds(), end.Seconds())
		} else {
			condition = fmt.Sprintf("gte(t,%.3f)", start.Seconds())
		}
		multiplier := math.Pow(10, seg.Gain/20)
		filters = append(filters, fmt.Sprintf("volume=volume=%.4f:enable='%s'", multiplier, condition))
	}
	return filters, nil
}

func (e *Engine) logger() *slog.Logger {
	if e.Logger != nil {
		return e.Logger
	}
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
}

func mapEffect(name string) string {
	switch name {
	case "anlmdn":
		return "anlmdn"
	case "eq_dialogue":
		return "equalizer=f=300:t=q:w=1.0:g=3,equalizer=f=3000:t=q:w=1.0:g=2"
	case "ducking":
		return "dynaudnorm"
	case "compressor":
		return "acompressor"
	default:
		return name
	}
}

type loudnormStats struct {
	InputI       float64
	InputLRA     float64
	InputTP      float64
	InputThresh  float64
	TargetOffset float64
}

func (e *Engine) measureLoudnorm(ctx context.Context, inputs []string, baseGraph []string, target float64) (loudnormStats, bool, error) {
	if len(inputs) == 0 {
		return loudnormStats{}, false, errors.New("audio engine: no inputs for loudnorm measurement")
	}
	key := e.loudnormCacheKey(inputs, baseGraph, target)
	if stats, ok := e.getLoudnormCache(key); ok {
		return stats, true, nil
	}
	measurementGraph := append([]string{}, baseGraph...)
	measurementGraph = append(measurementGraph, fmt.Sprintf("[mix]loudnorm=I=%.1f:TP=-1.5:LRA=11:print_format=json[outa]", target))
	filterGraph := strings.Join(measurementGraph, ";")
	args := []string{"-hide_banner"}
	for _, input := range inputs {
		args = append(args, "-i", input)
	}
	args = append(args, "-filter_complex", filterGraph, "-map", "[outa]", "-f", "null", "-")
	output, err := e.Executor.RunWithOutput(ctx, args)
	if err != nil {
		return loudnormStats{}, false, err
	}
	stats, parseErr := parseLoudnormStats(output)
	if parseErr != nil {
		return loudnormStats{}, false, parseErr
	}
	e.setLoudnormCache(key, stats)
	return stats, false, nil
}

func (e *Engine) loudnormCacheKey(inputs []string, baseGraph []string, target float64) string {
	parts := make([]string, 0, len(inputs)+len(baseGraph)+1)
	parts = append(parts, inputs...)
	parts = append(parts, baseGraph...)
	parts = append(parts, fmt.Sprintf("%.1f", target))
	return strings.Join(parts, "|")
}

func (e *Engine) getLoudnormCache(key string) (loudnormStats, bool) {
	e.cacheMu.RLock()
	defer e.cacheMu.RUnlock()
	if e.loudnormCache == nil {
		return loudnormStats{}, false
	}
	stats, ok := e.loudnormCache[key]
	return stats, ok
}

func (e *Engine) setLoudnormCache(key string, stats loudnormStats) {
	e.cacheMu.Lock()
	defer e.cacheMu.Unlock()
	if e.loudnormCache == nil {
		e.loudnormCache = make(map[string]loudnormStats)
	}
	e.loudnormCache[key] = stats
}

func parseLoudnormStats(output string) (loudnormStats, error) {
	var block strings.Builder
	capturing := false
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if !capturing {
			if idx := strings.Index(trimmed, "{"); idx >= 0 {
				capturing = true
				block.WriteString(trimmed[idx:])
				if strings.Contains(trimmed[idx:], "}") {
					break
				}
			}
			continue
		}
		if idx := strings.Index(trimmed, "}"); idx >= 0 {
			block.WriteString(trimmed[:idx+1])
			break
		}
		block.WriteString(trimmed)
	}
	jsonText := block.String()
	if jsonText == "" || !strings.Contains(jsonText, "\"input_i\"") {
		return loudnormStats{}, fmt.Errorf("audio engine: loudnorm stats not found in output")
	}
	if !strings.HasPrefix(jsonText, "{") {
		jsonText = "{" + jsonText
	}
	if !strings.HasSuffix(jsonText, "}") {
		jsonText = jsonText + "}"
	}
	var raw struct {
		InputI       string `json:"input_i"`
		InputLRA     string `json:"input_lra"`
		InputTP      string `json:"input_tp"`
		InputThresh  string `json:"input_thresh"`
		TargetOffset string `json:"target_offset"`
	}
	if err := json.Unmarshal([]byte(jsonText), &raw); err != nil {
		return loudnormStats{}, fmt.Errorf("audio engine: parse loudnorm stats: %w", err)
	}
	toFloat := func(value string, field string) (float64, error) {
		f, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			return 0, fmt.Errorf("audio engine: parse %s: %w", field, err)
		}
		return f, nil
	}
	inputI, err := toFloat(raw.InputI, "input_i")
	if err != nil {
		return loudnormStats{}, err
	}
	inputLRA, err := toFloat(raw.InputLRA, "input_lra")
	if err != nil {
		return loudnormStats{}, err
	}
	inputTP, err := toFloat(raw.InputTP, "input_tp")
	if err != nil {
		return loudnormStats{}, err
	}
	inputThresh, err := toFloat(raw.InputThresh, "input_thresh")
	if err != nil {
		return loudnormStats{}, err
	}
	offset, err := toFloat(raw.TargetOffset, "target_offset")
	if err != nil {
		return loudnormStats{}, err
	}
	return loudnormStats{
		InputI:       inputI,
		InputLRA:     inputLRA,
		InputTP:      inputTP,
		InputThresh:  inputThresh,
		TargetOffset: offset,
	}, nil
}
