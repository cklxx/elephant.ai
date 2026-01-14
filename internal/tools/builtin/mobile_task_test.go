package builtin

import (
	"errors"
	"testing"
)

func TestParseMobileTaskDecisionWithObservation(t *testing.T) {
	raw := `{"observation":"Home screen","action":{"action":"tap","x":10,"y":20,"reason":"open app"}}`
	decision, err := parseMobileTaskDecision(raw)
	if err != nil {
		t.Fatalf("parseMobileTaskDecision returned error: %v", err)
	}
	if decision.Observation != "Home screen" {
		t.Fatalf("expected observation %q, got %q", "Home screen", decision.Observation)
	}
	if decision.Action.Action != "tap" || decision.Action.X != 10 || decision.Action.Y != 20 {
		t.Fatalf("unexpected action parsed: %+v", decision.Action)
	}
}

func TestParseMobileTaskDecisionLegacyAction(t *testing.T) {
	raw := `{"action":"done","summary":"Finished"}`
	decision, err := parseMobileTaskDecision(raw)
	if err != nil {
		t.Fatalf("parseMobileTaskDecision returned error: %v", err)
	}
	if decision.Observation != "" {
		t.Fatalf("expected empty observation, got %q", decision.Observation)
	}
	if decision.Action.Action != "done" || decision.Action.Summary != "Finished" {
		t.Fatalf("unexpected action parsed: %+v", decision.Action)
	}
}

func TestParseMobileTaskDecisionNonJSONResponse(t *testing.T) {
	raw := "sorry I cannot process screenshots"
	_, err := parseMobileTaskDecision(raw)
	if err == nil {
		t.Fatalf("expected error for non-json response")
	}
	var parseErr *mobileTaskParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("expected mobileTaskParseError, got %T", err)
	}
	if parseErr.Kind != "non_json_response" {
		t.Fatalf("expected kind non_json_response, got %s", parseErr.Kind)
	}
	if parseErr.Raw != raw {
		t.Fatalf("expected raw response preserved")
	}
}

func TestParseMobileTaskDecisionInvalidJSON(t *testing.T) {
	raw := `{"action": }`
	_, err := parseMobileTaskDecision(raw)
	if err == nil {
		t.Fatalf("expected error for invalid json response")
	}
	var parseErr *mobileTaskParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("expected mobileTaskParseError, got %T", err)
	}
	if parseErr.Kind != "invalid_json_response" {
		t.Fatalf("expected kind invalid_json_response, got %s", parseErr.Kind)
	}
	if parseErr.Raw == "" {
		t.Fatalf("expected raw response preserved")
	}
}

func TestParseMobileTaskDecisionSupportsCodeBlock(t *testing.T) {
	raw := "```json\n{\"action\":\"text\",\"text\":\"hello\"}\n```"
	decision, err := parseMobileTaskDecision(raw)
	if err != nil {
		t.Fatalf("parseMobileTaskDecision returned error: %v", err)
	}
	if decision.Action.Action != "text" || decision.Action.Text != "hello" {
		t.Fatalf("unexpected action parsed: %+v", decision.Action)
	}
}

func TestNormalizeKeyCode(t *testing.T) {
	if got := normalizeKeyCode("home"); got != "KEYCODE_HOME" {
		t.Fatalf("expected KEYCODE_HOME, got %s", got)
	}
	if got := normalizeKeyCode("KEYCODE_BACK"); got != "KEYCODE_BACK" {
		t.Fatalf("expected KEYCODE_BACK, got %s", got)
	}
	if got := normalizeKeyCode("42"); got != "42" {
		t.Fatalf("expected numeric passthrough, got %s", got)
	}
}

func TestEscapeADBText(t *testing.T) {
	got := escapeADBText("hello world\nline")
	if got != "hello%sworld%sline" {
		t.Fatalf("unexpected escaped text: %s", got)
	}
}

func TestParseScreenSize(t *testing.T) {
	width, height, err := parseScreenSize("Physical size: 1080x2400")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if width != 1080 || height != 2400 {
		t.Fatalf("unexpected size %dx%d", width, height)
	}
}

func TestParseADBDevices(t *testing.T) {
	devices := parseADBDevices("List of devices attached\nabc123\tdevice usb:1-1\nlocalhost:5555\toffline\n")
	if len(devices) != 1 || devices[0] != "abc123" {
		t.Fatalf("unexpected devices: %#v", devices)
	}
}

func TestParseADBDeviceStates(t *testing.T) {
	devices := parseADBDeviceStates("List of devices attached\nlocalhost:5555\toffline\nemulator-5554\tdevice\n")
	if len(devices) != 2 {
		t.Fatalf("unexpected device count: %#v", devices)
	}
	if devices[0].Serial != "localhost:5555" || devices[0].State != "offline" {
		t.Fatalf("unexpected device: %#v", devices[0])
	}
	if devices[1].Serial != "emulator-5554" || devices[1].State != "device" {
		t.Fatalf("unexpected device: %#v", devices[1])
	}
}

func TestBuildFinalSummary(t *testing.T) {
	t.Run("prefers action summary", func(t *testing.T) {
		steps := []mobileTaskStep{
			{Index: 1, Observation: "Home screen", Action: mobileTaskAction{Action: "done", Summary: "All set"}},
		}
		if got := buildFinalSummary(steps); got != "All set" {
			t.Fatalf("expected summary %q, got %q", "All set", got)
		}
	})

	t.Run("falls back to observation", func(t *testing.T) {
		steps := []mobileTaskStep{
			{Index: 1, Observation: "Settings page", Action: mobileTaskAction{Action: "tap", X: 10, Y: 20}},
		}
		if got := buildFinalSummary(steps); got != "Settings page" {
			t.Fatalf("expected summary %q, got %q", "Settings page", got)
		}
	})
}
