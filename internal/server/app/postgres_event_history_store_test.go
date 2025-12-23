package app

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
)

func TestRecordFromEventStripsAttachmentData(t *testing.T) {
	payload := map[string]any{
		"result": map[string]any{
			"attachments": map[string]ports.Attachment{
				"video.mp4": {
					Name:      "video.mp4",
					MediaType: "video/mp4",
					Data:      base64.StdEncoding.EncodeToString([]byte{0x01, 0x02, 0x03}),
				},
			},
		},
	}

	envelope := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(ports.LevelCore, "sess", "task", "", time.Now()),
		Version:   1,
		Payload:   payload,
	}

	record, err := recordFromEvent(envelope)
	if err != nil {
		t.Fatalf("recordFromEvent returned error: %v", err)
	}

	var stored map[string]any
	if err := json.Unmarshal(record.payload, &stored); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}

	result, ok := stored["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result map, got %T", stored["result"])
	}

	attachments, ok := result["attachments"].(map[string]any)
	if !ok {
		t.Fatalf("expected attachments map, got %T", result["attachments"])
	}

	att, ok := attachments["video.mp4"].(map[string]any)
	if !ok {
		t.Fatalf("expected attachment entry, got %T", attachments["video.mp4"])
	}

	if data, ok := att["data"].(string); ok && data != "" {
		t.Fatalf("expected attachment data to be stripped, got %q", data)
	}
}

func TestRecordFromEventRetainsSmallTextAttachmentData(t *testing.T) {
	content := []byte("# Title\nBody")
	b64 := base64.StdEncoding.EncodeToString(content)

	payload := map[string]any{
		"attachments": map[string]ports.Attachment{
			"note.md": {
				Name:      "note.md",
				MediaType: "text/markdown",
				Data:      b64,
				URI:       "data:text/markdown;base64," + b64,
			},
		},
	}

	envelope := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(ports.LevelCore, "sess", "task", "", time.Now()),
		Version:   1,
		Payload:   payload,
	}

	record, err := recordFromEvent(envelope)
	if err != nil {
		t.Fatalf("recordFromEvent returned error: %v", err)
	}

	var stored map[string]any
	if err := json.Unmarshal(record.payload, &stored); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}

	attachments, ok := stored["attachments"].(map[string]any)
	if !ok {
		t.Fatalf("expected attachments map, got %T", stored["attachments"])
	}

	att, ok := attachments["note.md"].(map[string]any)
	if !ok {
		t.Fatalf("expected attachment entry, got %T", attachments["note.md"])
	}

	gotData, _ := att["data"].(string)
	if gotData != b64 {
		t.Fatalf("expected attachment data to be retained, got %q", gotData)
	}

	if uri, ok := att["uri"].(string); ok && uri != "" {
		t.Fatalf("expected data URI to be stripped when data retained, got %q", uri)
	}
}
