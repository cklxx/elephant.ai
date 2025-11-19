package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatFloat(t *testing.T) {
	cases := map[float64]string{
		2:      "2",
		2.5:    "2.5",
		0.25:   "0.25",
		1.2345: "1.2345",
	}
	for input, expected := range cases {
		if got := formatFloat(input); got != expected {
			t.Fatalf("formatFloat(%v)=%q want %q", input, got, expected)
		}
	}
}

func TestBuildDemoEnv(t *testing.T) {
        opts := demoOptions{
		outputPath:        "/tmp/out.mp4",
		resolution:        "1920x1080",
		segmentDuration:   1.5,
		sourceManifest:    "/tmp/manifest.txt",
		primaryAudio:      "bgm.wav",
		secondaryAudio:    "voice.wav",
		fontFile:          "/tmp/font.ttf",
		audioVolume:       0.6,
		simulateMissing:   true,
		ffmpegBin:         "/usr/bin/ffmpeg",
		ffprobeBin:        "/usr/bin/ffprobe",
		enableGPU:         true,
		gpuBackend:        "cuda",
		videoCodec:        "libx265",
		videoPreset:       "slow",
		runStatusFile:     "/tmp/run_status.txt",
		metricsLog:        "/tmp/pseudo_metrics.prom",
		watermarkText:     "AGENT_READY",
		watermarkFont:     64,
		watermarkOpacity:  0.55,
		watermarkMargin:   24,
                watermarkPosition: "top-left",
                watermarkImage:    "/tmp/logo.png",
                watermarkScale:    0.8,
                watermarkImgAlpha: 0.65,
                subtitleFile:      "/tmp/demo.srt",
                subtitleCharset:   "UTF-8",
                subtitleStyle:     "FontName=Roboto,Fontsize=42",
        }

	env := buildDemoEnv(opts)
	assertions := map[string]string{
		"OUTPUT_PATH":             "/tmp/out.mp4",
		"VIDEO_RESOLUTION":        "1920x1080",
		"SEGMENT_DURATION":        "1.5",
		"SOURCE_MANIFEST":         "/tmp/manifest.txt",
		"PRIMARY_AUDIO_PATH":      "bgm.wav",
		"SECONDARY_AUDIO_PATH":    "voice.wav",
		"FONT_FILE":               "/tmp/font.ttf",
		"AUDIO_VOLUME":            "0.6",
		"SIMULATE_MISSING_INPUT":  "1",
		"FFMPEG_BIN":              "/usr/bin/ffmpeg",
		"FFPROBE_BIN":             "/usr/bin/ffprobe",
		"ENABLE_GPU":              "1",
		"PREFERRED_GPU_BACKEND":   "cuda",
		"VIDEO_CODEC":             "libx265",
		"VIDEO_PRESET":            "slow",
		"RUN_STATUS_FILE":         "/tmp/run_status.txt",
		"METRICS_LOG_PATH":        "/tmp/pseudo_metrics.prom",
		"WATERMARK_TEXT":          "AGENT_READY",
		"WATERMARK_FONT_SIZE":     "64",
		"WATERMARK_OPACITY":       "0.55",
		"WATERMARK_MARGIN":        "24",
		"WATERMARK_POSITION":      "top-left",
                "WATERMARK_IMAGE_PATH":    "/tmp/logo.png",
                "WATERMARK_IMAGE_SCALE":   "0.8",
                "WATERMARK_IMAGE_OPACITY": "0.65",
                "SUBTITLE_FILE":           "/tmp/demo.srt",
                "SUBTITLE_CHARSET":        "UTF-8",
                "SUBTITLE_FORCE_STYLE":    "FontName=Roboto,Fontsize=42",
        }

	for key, expected := range assertions {
		if got := findEnv(env, key); got != expected {
			t.Fatalf("env %s=%q want %q", key, got, expected)
		}
	}
}

func TestResolveScriptCandidates(t *testing.T) {
	tmpDir := t.TempDir()
	script := filepath.Join(tmpDir, "demo.sh")
	if err := os.WriteFile(script, []byte("echo demo"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	resolved, err := resolveScript(script)
	if err != nil {
		t.Fatalf("resolveScript returned error: %v", err)
	}

	if resolved != script {
		t.Fatalf("resolveScript=%q want %q", resolved, script)
	}
}

func findEnv(env []string, key string) string {
	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return strings.TrimPrefix(entry, prefix)
		}
	}
	return ""
}
