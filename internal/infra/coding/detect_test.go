package coding

import (
	"fmt"
	"reflect"
	"testing"
)

func TestDetectLocalCLIs_IncludesSupportedAndUnsupported(t *testing.T) {
	old := detectLookPath
	defer func() { detectLookPath = old }()

	detectLookPath = func(name string) (string, error) {
		switch name {
		case "codex":
			return "/fake/codex", nil
		case "claude":
			return "/fake/claude", nil
		case "kimi":
			return "/fake/kimi", nil
		default:
			return "", fmt.Errorf("%s not found", name)
		}
	}

	got := DetectLocalCLIs()
	if len(got) != 3 {
		t.Fatalf("expected 3 detected CLIs, got %d: %+v", len(got), got)
	}
	if got[0].ID != "codex" || !got[0].AdapterSupport || got[0].AgentType != "codex" {
		t.Fatalf("unexpected codex detection: %+v", got[0])
	}
	if got[1].ID != "claude" || !got[1].AdapterSupport || got[1].AgentType != "claude_code" {
		t.Fatalf("unexpected claude detection: %+v", got[1])
	}
	if got[2].ID != "kimi" || got[2].AdapterSupport || got[2].AgentType != "" {
		t.Fatalf("unexpected kimi detection: %+v", got[2])
	}
}

func TestDetectLocalAdapters_ReturnsOnlyIntegratedAdapters(t *testing.T) {
	old := detectLookPath
	defer func() { detectLookPath = old }()

	detectLookPath = func(name string) (string, error) {
		switch name {
		case "codex":
			return "/fake/codex", nil
		case "k2":
			return "/fake/k2", nil
		default:
			return "", fmt.Errorf("%s not found", name)
		}
	}

	got := DetectLocalAdapters()
	want := []string{"codex"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected adapters: got=%v want=%v", got, want)
	}
}

func TestDetectLocalCLIs_UsesFallbackBinaryNames(t *testing.T) {
	old := detectLookPath
	defer func() { detectLookPath = old }()

	detectLookPath = func(name string) (string, error) {
		switch name {
		case "claude-code":
			return "/fake/claude-code", nil
		default:
			return "", fmt.Errorf("%s not found", name)
		}
	}

	got := DetectLocalCLIs()
	if len(got) != 1 {
		t.Fatalf("expected single detected CLI, got %d: %+v", len(got), got)
	}
	if got[0].ID != "claude" || got[0].Binary != "claude-code" {
		t.Fatalf("unexpected detection result: %+v", got[0])
	}
}
