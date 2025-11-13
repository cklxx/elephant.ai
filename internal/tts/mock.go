package tts

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"time"
)

// MockProvider synthesizes silent audio for development and dry-run scenarios.
type MockProvider struct {
	SampleRate int
}

// Name returns the provider identifier.
func (m MockProvider) Name() string {
	return "mock"
}

// Synthesize generates a silent WAV file based on the input text length.
func (m MockProvider) Synthesize(_ context.Context, req Request) (ProviderResult, error) {
	rate := m.SampleRate
	if rate <= 0 {
		rate = 16000
	}
	duration := estimateDuration(req.Text)
	data, err := generateSilentWAV(duration, rate)
	if err != nil {
		return ProviderResult{}, fmt.Errorf("generate silent wav: %w", err)
	}
	return ProviderResult{
		Audio:       data,
		ContentType: "audio/wav",
		Duration:    duration,
		Metadata: map[string]string{
			"voice": req.Voice,
		},
	}, nil
}

func estimateDuration(text string) time.Duration {
	if len(text) == 0 {
		return 2 * time.Second
	}
	seconds := float64(len([]rune(text))) / 12.0
	seconds = math.Max(seconds, 2)
	return time.Duration(seconds * float64(time.Second))
}

func generateSilentWAV(duration time.Duration, sampleRate int) ([]byte, error) {
	totalSamples := int(math.Ceil(duration.Seconds() * float64(sampleRate)))
	if totalSamples < sampleRate {
		totalSamples = sampleRate
	}
	dataSize := totalSamples * 2
	buf := bytes.NewBuffer(make([]byte, 0, 44+dataSize))
	writeString(buf, "RIFF")
	if err := writeLE(buf, uint32(36+dataSize)); err != nil {
		return nil, err
	}
	writeString(buf, "WAVE")
	writeString(buf, "fmt ")
	if err := writeLE(buf, uint32(16)); err != nil {
		return nil, err
	}
	if err := writeLE(buf, uint16(1)); err != nil {
		return nil, err
	}
	if err := writeLE(buf, uint16(1)); err != nil {
		return nil, err
	}
	if err := writeLE(buf, uint32(sampleRate)); err != nil {
		return nil, err
	}
	if err := writeLE(buf, uint32(sampleRate*2)); err != nil {
		return nil, err
	}
	if err := writeLE(buf, uint16(2)); err != nil {
		return nil, err
	}
	if err := writeLE(buf, uint16(16)); err != nil {
		return nil, err
	}
	writeString(buf, "data")
	if err := writeLE(buf, uint32(dataSize)); err != nil {
		return nil, err
	}
	if _, err := buf.Write(make([]byte, dataSize)); err != nil {
		return nil, fmt.Errorf("write pcm data: %w", err)
	}
	return buf.Bytes(), nil
}

func writeString(buf *bytes.Buffer, s string) {
	buf.WriteString(s)
}

func writeLE[T any](buf *bytes.Buffer, value T) error {
	if err := binary.Write(buf, binary.LittleEndian, value); err != nil {
		return fmt.Errorf("write little endian value: %w", err)
	}
	return nil
}
