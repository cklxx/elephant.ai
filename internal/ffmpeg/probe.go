package ffmpeg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"log/slog"
)

// ErrNoVideoStreams indicates ffprobe did not detect any video tracks.
var ErrNoVideoStreams = errors.New("ffprobe: no video streams found")

// Prober gathers metadata for media files.
type Prober interface {
	Probe(ctx context.Context, path string) (ProbeResult, error)
}

// VideoStream captures relevant properties of a video track.
type VideoStream struct {
	CodecName   string
	Width       int
	Height      int
	PixelFormat string
	FrameRate   float64
}

// ProbeResult holds aggregated ffprobe information.
type ProbeResult struct {
	VideoStreams []VideoStream
	Duration     time.Duration
}

// FirstVideo returns the first detected video stream.
func (r ProbeResult) FirstVideo() (VideoStream, bool) {
	if len(r.VideoStreams) == 0 {
		return VideoStream{}, false
	}
	return r.VideoStreams[0], true
}

// LocalProber executes ffprobe on the host.
type LocalProber struct {
	Binary string
	Logger *slog.Logger
}

// Probe runs ffprobe for the given path.
func (p *LocalProber) Probe(ctx context.Context, path string) (ProbeResult, error) {
	if strings.TrimSpace(path) == "" {
		return ProbeResult{}, errors.New("ffprobe: path is required")
	}
	args := []string{"-v", "error", "-print_format", "json", "-show_streams", "-show_format", path}
	cmd := exec.CommandContext(ctx, p.binary(), args...)
	output, err := cmd.Output()
	if err != nil {
		return ProbeResult{}, fmt.Errorf("ffprobe: %w", err)
	}
	if logger := p.logger(); logger != nil {
		logger.Debug("ffprobe output", slog.String("path", path), slog.String("payload", string(output)))
	}
	return parseProbeOutput(output)
}

func (p *LocalProber) binary() string {
	if strings.TrimSpace(p.Binary) == "" {
		return "ffprobe"
	}
	return p.Binary
}

func (p *LocalProber) logger() *slog.Logger {
	if p.Logger != nil {
		return p.Logger
	}
	return slog.Default()
}

type ffprobeOutput struct {
	Streams []struct {
		CodecType    string `json:"codec_type"`
		CodecName    string `json:"codec_name"`
		Width        int    `json:"width"`
		Height       int    `json:"height"`
		PixelFormat  string `json:"pix_fmt"`
		AvgFrameRate string `json:"avg_frame_rate"`
		RFrameRate   string `json:"r_frame_rate"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

func parseProbeOutput(data []byte) (ProbeResult, error) {
	var payload ffprobeOutput
	if err := json.Unmarshal(data, &payload); err != nil {
		return ProbeResult{}, fmt.Errorf("parse ffprobe output: %w", err)
	}
	result := ProbeResult{}
	for _, stream := range payload.Streams {
		if stream.CodecType != "video" {
			continue
		}
		frameRate := parseFrameRate(stream.AvgFrameRate)
		if frameRate == 0 {
			frameRate = parseFrameRate(stream.RFrameRate)
		}
		result.VideoStreams = append(result.VideoStreams, VideoStream{
			CodecName:   stream.CodecName,
			Width:       stream.Width,
			Height:      stream.Height,
			PixelFormat: stream.PixelFormat,
			FrameRate:   frameRate,
		})
	}
	if payload.Format.Duration != "" {
		if seconds, err := strconv.ParseFloat(payload.Format.Duration, 64); err == nil {
			result.Duration = time.Duration(seconds * float64(time.Second))
		}
	}
	if len(result.VideoStreams) == 0 {
		return result, ErrNoVideoStreams
	}
	return result, nil
}

func parseFrameRate(raw string) float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "0/0" {
		return 0
	}
	if strings.Contains(raw, "/") {
		parts := strings.Split(raw, "/")
		if len(parts) != 2 {
			return 0
		}
		num, err1 := strconv.ParseFloat(parts[0], 64)
		den, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 != nil || err2 != nil || den == 0 {
			return 0
		}
		return num / den
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return value
}
