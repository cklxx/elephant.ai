package task

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// JobSpec represents the high level job description parsed from YAML.
type JobSpec struct {
	Name           string            `yaml:"name"`
	WorkingDir     string            `yaml:"working_dir"`
	AllowOverwrite bool              `yaml:"allow_overwrite"`
	Tags           map[string]string `yaml:"tags"`

	Video VideoSpec    `yaml:"video"`
	Audio AudioSpec    `yaml:"audio"`
	TTS   []TTSRequest `yaml:"tts"`

	RetryPolicy   RetryPolicy   `yaml:"retry_policy"`
	StageTimeouts StageTimeouts `yaml:"stage_timeouts"`
}

// VideoSpec contains instructions for the FFmpeg pipeline.
type VideoSpec struct {
	Segments    []VideoSegment `yaml:"segments"`
	Output      string         `yaml:"output"`
	FinalOutput string         `yaml:"final_output"`
	Preset      string         `yaml:"preset"`
	Filters     []string       `yaml:"filters"`
}

// VideoSegment describes an individual video fragment to be concatenated.
type VideoSegment struct {
	Path       string `yaml:"path"`
	TrimIn     string `yaml:"trim_in"`
	TrimOut    string `yaml:"trim_out"`
	Transition string `yaml:"transition"`
}

// AudioSpec controls the mixdown stage.
type AudioSpec struct {
	MixdownOutput  string       `yaml:"mixdown_output"`
	LoudnessTarget float64      `yaml:"loudness_target"`
	Tracks         []AudioTrack `yaml:"tracks"`
}

// AudioTrack configures a single audio source.
type AudioTrack struct {
	Name     string        `yaml:"name"`
	Source   string        `yaml:"source"`
	Gain     float64       `yaml:"gain"`
	Offset   string        `yaml:"offset"`
	Loop     bool          `yaml:"loop"`
	Effects  []string      `yaml:"effects"`
	Envelope *EnvelopeSpec `yaml:"envelope"`
}

// EnvelopeSpec describes automation applied to an audio track.
type EnvelopeSpec struct {
	FadeIn   string            `yaml:"fade_in"`
	FadeOut  *FadeOutSpec      `yaml:"fade_out"`
	Segments []EnvelopeSegment `yaml:"segments"`
}

// FadeOutSpec declares where a fade-out should begin and its duration.
type FadeOutSpec struct {
	Start    string `yaml:"start"`
	Duration string `yaml:"duration"`
}

// EnvelopeSegment applies a gain change over a time window.
type EnvelopeSegment struct {
	Start string  `yaml:"start"`
	End   string  `yaml:"end"`
	Gain  float64 `yaml:"gain"`
}

// TTSRequest describes a text-to-speech synthesis task.
type TTSRequest struct {
	Alias      string            `yaml:"alias"`
	Text       string            `yaml:"text"`
	Voice      string            `yaml:"voice"`
	Style      string            `yaml:"style"`
	Format     string            `yaml:"format"`
	Parameters map[string]string `yaml:"parameters"`
}

// RetryPolicy controls orchestrator retry behaviour.
type RetryPolicy struct {
	MaxAttempts int     `yaml:"max_attempts"`
	Backoff     string  `yaml:"backoff"`
	Jitter      float64 `yaml:"jitter"`
}

// StageTimeouts stores per-stage timeout strings.
type StageTimeouts map[string]string

// Duration resolves the timeout for a stage. If no timeout is set, ok=false.
func (s StageTimeouts) Duration(stage string) (time.Duration, bool, error) {
	raw, ok := s[stage]
	if !ok || strings.TrimSpace(raw) == "" {
		return 0, false, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, true, fmt.Errorf("invalid duration for stage %q: %w", stage, err)
	}
	return d, true, nil
}

// LoadSpec reads and parses a YAML job specification from disk.
func LoadSpec(path string) (*JobSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read spec: %w", err)
	}
	var spec JobSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parse spec: %w", err)
	}
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	return &spec, nil
}

// Validate performs static validation on the specification.
func (j *JobSpec) Validate() error {
	if strings.TrimSpace(j.Name) == "" {
		return errors.New("job name is required")
	}
	if strings.TrimSpace(j.WorkingDir) == "" {
		return errors.New("working_dir is required")
	}
	if err := validateSafePath(j.WorkingDir); err != nil {
		return fmt.Errorf("working_dir: %w", err)
	}
	if err := j.Video.Validate(); err != nil {
		return fmt.Errorf("video: %w", err)
	}
	if err := j.Audio.Validate(); err != nil {
		return fmt.Errorf("audio: %w", err)
	}
	aliases := map[string]struct{}{}
	for i, req := range j.TTS {
		if strings.TrimSpace(req.Alias) == "" {
			return fmt.Errorf("tts[%d]: alias is required", i)
		}
		if _, exists := aliases[req.Alias]; exists {
			return fmt.Errorf("tts[%d]: duplicate alias %q", i, req.Alias)
		}
		aliases[req.Alias] = struct{}{}
		if strings.TrimSpace(req.Text) == "" {
			return fmt.Errorf("tts[%d]: text is required", i)
		}
		if strings.TrimSpace(req.Voice) == "" {
			return fmt.Errorf("tts[%d]: voice is required", i)
		}
		if req.Format == "" {
			req.Format = "mp3"
		}
	}
	for i, track := range j.Audio.Tracks {
		if strings.HasPrefix(track.Source, "tts:") {
			alias := strings.TrimPrefix(track.Source, "tts:")
			if _, ok := aliases[alias]; !ok {
				return fmt.Errorf("audio.tracks[%d]: unknown TTS alias %q", i, alias)
			}
		}
		if track.Offset != "" {
			if _, err := time.ParseDuration(track.Offset); err != nil {
				return fmt.Errorf("audio.tracks[%d]: invalid offset: %w", i, err)
			}
		}
		if track.Envelope != nil {
			if err := track.Envelope.Validate(); err != nil {
				return fmt.Errorf("audio.tracks[%d]: envelope: %w", i, err)
			}
		}
	}
	if j.RetryPolicy.MaxAttempts < 0 {
		return errors.New("retry_policy.max_attempts must be >= 0")
	}
	if j.RetryPolicy.Backoff != "" {
		if _, err := time.ParseDuration(j.RetryPolicy.Backoff); err != nil {
			return fmt.Errorf("retry_policy.backoff: %w", err)
		}
	}
	for stage, raw := range j.StageTimeouts {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		if _, err := time.ParseDuration(raw); err != nil {
			return fmt.Errorf("stage_timeouts[%s]: %w", stage, err)
		}
	}
	return nil
}

// Validate ensures the video spec is well-formed.
func (v *VideoSpec) Validate() error {
	if len(v.Segments) == 0 {
		return errors.New("at least one segment is required")
	}
	for i, seg := range v.Segments {
		if strings.TrimSpace(seg.Path) == "" {
			return fmt.Errorf("segments[%d]: path is required", i)
		}
	}
	if strings.TrimSpace(v.Output) == "" {
		return errors.New("output path is required")
	}
	if v.FinalOutput == "" {
		v.FinalOutput = v.Output
	}
	return nil
}

// Validate ensures audio spec is well-formed.
func (a *AudioSpec) Validate() error {
	if strings.TrimSpace(a.MixdownOutput) == "" {
		return errors.New("mixdown_output is required")
	}
	if len(a.Tracks) == 0 {
		return errors.New("at least one audio track is required")
	}
	return nil
}

// Validate ensures the automation envelope is well-formed.
func (e *EnvelopeSpec) Validate() error {
	if e == nil {
		return nil
	}
	if strings.TrimSpace(e.FadeIn) != "" {
		if _, err := time.ParseDuration(e.FadeIn); err != nil {
			return fmt.Errorf("fade_in: %w", err)
		}
	}
	if e.FadeOut != nil {
		if err := e.FadeOut.Validate(); err != nil {
			return err
		}
	}
	for idx, seg := range e.Segments {
		if err := seg.Validate(); err != nil {
			return fmt.Errorf("segments[%d]: %w", idx, err)
		}
	}
	return nil
}

// Validate ensures the fade-out definition is sound.
func (f *FadeOutSpec) Validate() error {
	if f == nil {
		return nil
	}
	if strings.TrimSpace(f.Start) == "" {
		return errors.New("fade_out.start is required")
	}
	if _, err := time.ParseDuration(f.Start); err != nil {
		return fmt.Errorf("fade_out.start: %w", err)
	}
	if strings.TrimSpace(f.Duration) == "" {
		return errors.New("fade_out.duration is required")
	}
	if _, err := time.ParseDuration(f.Duration); err != nil {
		return fmt.Errorf("fade_out.duration: %w", err)
	}
	return nil
}

// Validate ensures the envelope segment definition is sound.
func (s *EnvelopeSegment) Validate() error {
	if strings.TrimSpace(s.Start) == "" {
		return errors.New("start is required")
	}
	start, err := time.ParseDuration(s.Start)
	if err != nil {
		return fmt.Errorf("start: %w", err)
	}
	if strings.TrimSpace(s.End) != "" {
		end, err := time.ParseDuration(s.End)
		if err != nil {
			return fmt.Errorf("end: %w", err)
		}
		if end <= start {
			return errors.New("end must be greater than start")
		}
	}
	return nil
}

// validateSafePath ensures a path does not break out of the working tree.
func validateSafePath(path string) error {
	cleaned := filepath.Clean(path)
	if strings.Contains(cleaned, "..") {
		return fmt.Errorf("path %q must not contain parent directory references", path)
	}
	if filepath.IsAbs(path) {
		return nil
	}
	if strings.HasPrefix(cleaned, string(filepath.Separator)) {
		return fmt.Errorf("path %q escapes root", path)
	}
	return nil
}
