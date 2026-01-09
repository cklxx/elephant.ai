package builtin

import "testing"

func TestParseMobileTaskAction(t *testing.T) {
	action, err := parseMobileTaskAction(`{"action":"tap","x":12,"y":34}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Action != "tap" || action.X != 12 || action.Y != 34 {
		t.Fatalf("unexpected action: %+v", action)
	}
}

func TestParseMobileTaskActionSupportsCodeBlock(t *testing.T) {
	action, err := parseMobileTaskAction("```json\n{\"action\":\"text\",\"text\":\"hello\"}\n```")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Action != "text" || action.Text != "hello" {
		t.Fatalf("unexpected action: %+v", action)
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
	devices := parseADBDevices("List of devices attached\nabc123\tdevice usb:1-1\n")
	if len(devices) != 1 || devices[0] != "abc123" {
		t.Fatalf("unexpected devices: %#v", devices)
	}
}
