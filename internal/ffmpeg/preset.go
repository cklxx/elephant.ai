package ffmpeg

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Preset describes reusable FFmpeg output settings.
type Preset struct {
	Name         string
	VideoCodec   string
	AudioCodec   string
	VideoBitrate string
	AudioBitrate string
	PixelFormat  string
	FrameRate    string
	Filters      []string
	ExtraArgs    []string
}

// Args returns additional ffmpeg arguments encoded by the preset.
func (p Preset) Args() []string {
	args := make([]string, 0, 8+len(p.ExtraArgs))
	if p.VideoBitrate != "" {
		args = append(args, "-b:v", p.VideoBitrate)
	}
	if p.AudioBitrate != "" {
		args = append(args, "-b:a", p.AudioBitrate)
	}
	if p.PixelFormat != "" {
		args = append(args, "-pix_fmt", p.PixelFormat)
	}
	if p.FrameRate != "" {
		args = append(args, "-r", p.FrameRate)
	}
	args = append(args, p.ExtraArgs...)
	return args
}

// PresetLibrary stores named presets loaded from disk.
type PresetLibrary struct {
	presets map[string]Preset
}

// NewPresetLibrary constructs a library from a map of presets.
func NewPresetLibrary(m map[string]Preset) *PresetLibrary {
	cp := make(map[string]Preset, len(m))
	for k, v := range m {
		v.Name = k
		cp[k] = v
	}
	return &PresetLibrary{presets: cp}
}

// EmptyPresetLibrary creates an empty library.
func EmptyPresetLibrary() *PresetLibrary {
	return &PresetLibrary{presets: map[string]Preset{}}
}

// Get retrieves a preset by name.
func (l *PresetLibrary) Get(name string) (Preset, bool) {
	if l == nil {
		return Preset{}, false
	}
	preset, ok := l.presets[name]
	return preset, ok
}

// LoadPresetFile reads presets from a YAML file on disk.
func LoadPresetFile(path string) (*PresetLibrary, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("load preset file: %w", err)
	}
	type rawPreset struct {
		VideoCodec   string   `yaml:"video_codec"`
		AudioCodec   string   `yaml:"audio_codec"`
		VideoBitrate string   `yaml:"video_bitrate"`
		AudioBitrate string   `yaml:"audio_bitrate"`
		PixelFormat  string   `yaml:"pixel_format"`
		FrameRate    string   `yaml:"frame_rate"`
		Filters      []string `yaml:"filters"`
		ExtraArgs    []string `yaml:"extra_args"`
	}
	var payload struct {
		Presets map[string]rawPreset `yaml:"presets"`
	}
	if err := yaml.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("parse preset file: %w", err)
	}
	presets := make(map[string]Preset, len(payload.Presets))
	for name, rp := range payload.Presets {
		presets[name] = Preset{
			Name:         name,
			VideoCodec:   rp.VideoCodec,
			AudioCodec:   rp.AudioCodec,
			VideoBitrate: rp.VideoBitrate,
			AudioBitrate: rp.AudioBitrate,
			PixelFormat:  rp.PixelFormat,
			FrameRate:    rp.FrameRate,
			Filters:      append([]string(nil), rp.Filters...),
			ExtraArgs:    append([]string(nil), rp.ExtraArgs...),
		}
	}
	return NewPresetLibrary(presets), nil
}
