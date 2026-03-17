package ports

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestToolResultJSONRoundTripAndErrorDecoding(t *testing.T) {
	t.Run("marshal and decode string error", func(t *testing.T) {
		original := ToolResult{
			CallID:  "call-1",
			Content: "partial output",
			Error:   errors.New("boom"),
			Metadata: map[string]any{
				"attempt": float64(2),
			},
			Attachments: map[string]Attachment{
				"diagram.png": {Name: "diagram.png", URI: "https://example.com/diagram.png"},
			},
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}

		var decoded ToolResult
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		if decoded.Error == nil || decoded.Error.Error() != "boom" {
			t.Fatalf("expected decoded error boom, got %#v", decoded.Error)
		}
		if decoded.Attachments["diagram.png"].URI != "https://example.com/diagram.png" {
			t.Fatalf("expected attachment URI to round-trip, got %+v", decoded.Attachments)
		}
	})

	t.Run("decode object error payloads", func(t *testing.T) {
		cases := []struct {
			name string
			json string
			want string
		}{
			{
				name: "message field",
				json: `{"call_id":"call-2","content":"x","error":{"message":"denied"}}`,
				want: "denied",
			},
			{
				name: "error field",
				json: `{"call_id":"call-3","content":"x","error":{"error":"failed"}}`,
				want: "failed",
			},
			{
				name: "raw fallback",
				json: `{"call_id":"call-4","content":"x","error":42}`,
				want: "42",
			},
			{
				name: "null error",
				json: `{"call_id":"call-5","content":"x","error":null}`,
				want: "",
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				var decoded ToolResult
				if err := json.Unmarshal([]byte(tc.json), &decoded); err != nil {
					t.Fatalf("unmarshal failed: %v", err)
				}
				if tc.want == "" {
					if decoded.Error != nil {
						t.Fatalf("expected nil error, got %#v", decoded.Error)
					}
					return
				}
				if decoded.Error == nil || decoded.Error.Error() != tc.want {
					t.Fatalf("expected error %q, got %#v", tc.want, decoded.Error)
				}
			})
		}
	})
}

func TestToolMetadataSafetyAndMaterialCapabilities(t *testing.T) {
	if !((ToolMaterialCapabilities{}).IsZero()) {
		t.Fatal("expected empty material capabilities to be zero")
	}
	if (ToolMaterialCapabilities{Consumes: []string{"image/png"}}).IsZero() {
		t.Fatal("expected non-empty material capabilities to be non-zero")
	}

	if got := (ToolMetadata{SafetyLevel: SafetyLevelIrreversible}).EffectiveSafetyLevel(); got != SafetyLevelIrreversible {
		t.Fatalf("expected explicit safety level to win, got %d", got)
	}
	if got := (ToolMetadata{Dangerous: true}).EffectiveSafetyLevel(); got != SafetyLevelHighImpact {
		t.Fatalf("expected dangerous tool to default to high-impact, got %d", got)
	}
	if got := (ToolMetadata{}).EffectiveSafetyLevel(); got != SafetyLevelReadOnly {
		t.Fatalf("expected safe default to be read-only, got %d", got)
	}
}

func TestAttachmentCoercionAndCloneIsolation(t *testing.T) {
	raw := map[string]any{
		"diagram.png": map[string]any{
			"media_type": "image/png",
			"preview_assets": []any{
				map[string]any{"label": "page-1"},
			},
		},
		"skip.txt": "not-an-attachment",
	}

	attachments := CoerceAttachmentMap(raw)
	if len(attachments) != 1 {
		t.Fatalf("expected only valid attachment payloads to survive, got %+v", attachments)
	}
	if attachments["diagram.png"].Name != "diagram.png" {
		t.Fatalf("expected key fallback to populate attachment name, got %+v", attachments["diagram.png"])
	}

	cloned := CloneAttachmentMap(attachments)
	clonedAttachment := cloned["diagram.png"]
	clonedAttachment.PreviewAssets[0].Label = "mutated"
	cloned["diagram.png"] = clonedAttachment

	if attachments["diagram.png"].PreviewAssets[0].Label != "page-1" {
		t.Fatalf("expected preview asset clone isolation, got %+v", attachments["diagram.png"].PreviewAssets)
	}

	typed := map[string]Attachment{
		"photo.jpg": {Name: "photo.jpg"},
	}
	if got := CoerceAttachmentMap(typed); got["photo.jpg"].Name != "photo.jpg" {
		t.Fatalf("expected typed attachment map to pass through, got %+v", got)
	}
	if got := CoerceAttachmentMap(map[string]Attachment{}); got != nil {
		t.Fatalf("expected empty typed attachment map to normalize to nil, got %+v", got)
	}
	if _, ok := AttachmentFromAny("bad"); ok {
		t.Fatal("expected invalid payload to be rejected")
	}
}

func TestCloneUserPersonaProfileDeepCopiesNestedFields(t *testing.T) {
	now := time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC)
	profile := &UserPersonaProfile{
		Version:           "v1",
		UpdatedAt:         now,
		InitiativeSources: []string{"calendar"},
		CoreDrives:        []UserPersonaDrive{{ID: "ownership", Label: "Ownership", Score: 9}},
		TopDrives:         []string{"ownership"},
		Values:            []string{"clarity"},
		Goals:             UserPersonaGoals{CurrentFocus: "ship"},
		Traits:            map[string]int{"decisiveness": 8},
		KeyChoices:        []string{"delete dead code"},
		ConstructionRules: []string{"prefer clean redesigns"},
		RawAnswers: map[string]interface{}{
			"nested": map[string]interface{}{
				"list": []interface{}{"a", map[string]interface{}{"k": "v"}},
			},
		},
	}

	cloned := CloneUserPersonaProfile(profile)
	if cloned == nil {
		t.Fatal("expected cloned profile")
	}

	profile.InitiativeSources[0] = "email"
	profile.CoreDrives[0].Label = "Changed"
	profile.Traits["decisiveness"] = 1
	profile.RawAnswers["nested"].(map[string]interface{})["list"].([]interface{})[1].(map[string]interface{})["k"] = "changed"

	if cloned.InitiativeSources[0] != "calendar" {
		t.Fatalf("expected initiative sources to be cloned, got %+v", cloned.InitiativeSources)
	}
	if cloned.CoreDrives[0].Label != "Ownership" {
		t.Fatalf("expected core drives to be cloned, got %+v", cloned.CoreDrives)
	}
	if cloned.Traits["decisiveness"] != 8 {
		t.Fatalf("expected traits to be cloned, got %+v", cloned.Traits)
	}
	gotNested := cloned.RawAnswers["nested"].(map[string]interface{})["list"].([]interface{})[1].(map[string]interface{})["k"]
	if gotNested != "v" {
		t.Fatalf("expected nested raw answers to be deeply cloned, got %#v", cloned.RawAnswers)
	}
}

func TestCloneImportantNotesAndTruncateRuneSnippet(t *testing.T) {
	now := time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC)
	notes := map[string]ImportantNote{
		"n1": {
			ID:        "n1",
			Content:   "keep this",
			Tags:      []string{"persona"},
			CreatedAt: now,
		},
	}

	cloned := CloneImportantNotes(notes)
	if cloned["n1"].Content != "keep this" {
		t.Fatalf("expected important note clone, got %+v", cloned)
	}
	if got := CloneImportantNotes(nil); got != nil {
		t.Fatalf("expected nil notes clone, got %+v", got)
	}

	if got := TruncateRuneSnippet("  你好世界  ", 3); got != "你好…" {
		t.Fatalf("expected rune-aware truncation, got %q", got)
	}
	if got := TruncateRuneSnippet("  keep full text  ", 0); got != "keep full text" {
		t.Fatalf("expected non-positive limit to return trimmed content, got %q", got)
	}
}
