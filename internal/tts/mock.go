package tts

import (
	"bytes"
	"context"
	"encoding/binary"
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
	data := generateSilentWAV(duration, rate)
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

func generateSilentWAV(duration time.Duration, sampleRate int) []byte {
	totalSamples := int(math.Ceil(duration.Seconds() * float64(sampleRate)))
	if totalSamples < sampleRate {
		totalSamples = sampleRate
	}
	dataSize := totalSamples * 2
	buf := bytes.NewBuffer(make([]byte, 0, 44+dataSize))
	writeString(buf, "RIFF")
	binary.Write(buf, binary.LittleEndian, uint32(36+dataSize))
	writeString(buf, "WAVE")
	writeString(buf, "fmt ")
	binary.Write(buf, binary.LittleEndian, uint32(16))
	binary.Write(buf, binary.LittleEndian, uint16(1))
	binary.Write(buf, binary.LittleEndian, uint16(1))
	binary.Write(buf, binary.LittleEndian, uint32(sampleRate))
	binary.Write(buf, binary.LittleEndian, uint32(sampleRate*2))
	binary.Write(buf, binary.LittleEndian, uint16(2))
	binary.Write(buf, binary.LittleEndian, uint16(16))
	writeString(buf, "data")
	binary.Write(buf, binary.LittleEndian, uint32(dataSize))
	buf.Write(make([]byte, dataSize))
	return buf.Bytes()
}

func writeString(buf *bytes.Buffer, s string) {
	buf.WriteString(s)
}
