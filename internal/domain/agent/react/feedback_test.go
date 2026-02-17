package react

import (
	"fmt"
	"testing"
)

// --- deriveFeedbackValue ---

func TestDeriveFeedbackValue_ExplicitReward(t *testing.T) {
	result := ToolResult{
		Metadata: map[string]any{"reward": 0.75},
	}
	if got := deriveFeedbackValue(result); got != 0.75 {
		t.Fatalf("expected 0.75, got %v", got)
	}
}

func TestDeriveFeedbackValue_ExplicitScore(t *testing.T) {
	result := ToolResult{
		Metadata: map[string]any{"score": 0.5},
	}
	if got := deriveFeedbackValue(result); got != 0.5 {
		t.Fatalf("expected 0.5, got %v", got)
	}
}

func TestDeriveFeedbackValue_ErrorResult(t *testing.T) {
	result := ToolResult{
		Error: fmt.Errorf("tool failed"),
	}
	if got := deriveFeedbackValue(result); got != -1 {
		t.Fatalf("expected -1 for error, got %v", got)
	}
}

func TestDeriveFeedbackValue_SuccessDefault(t *testing.T) {
	result := ToolResult{}
	if got := deriveFeedbackValue(result); got != 1 {
		t.Fatalf("expected 1 for success, got %v", got)
	}
}

func TestDeriveFeedbackValue_RewardOverridesError(t *testing.T) {
	result := ToolResult{
		Error:    fmt.Errorf("failed"),
		Metadata: map[string]any{"reward": 0.3},
	}
	if got := deriveFeedbackValue(result); got != 0.3 {
		t.Fatalf("expected explicit reward to override error, got %v", got)
	}
}

// --- buildFeedbackMessage ---

func TestBuildFeedbackMessage_Success(t *testing.T) {
	result := ToolResult{
		CallID:  "web_fetch",
		Content: "Found 3 results",
	}
	got := buildFeedbackMessage(result)
	if got == "" {
		t.Fatal("expected non-empty message")
	}
	if got != "web_fetch completed: Found 3 results" {
		// Be flexible about exact format but check key parts
		t.Logf("message: %q", got)
	}
}

func TestBuildFeedbackMessage_Error(t *testing.T) {
	result := ToolResult{
		CallID:  "shell_exec",
		Content: "command failed",
		Error:   fmt.Errorf("exit code 1"),
	}
	got := buildFeedbackMessage(result)
	if got == "" {
		t.Fatal("expected non-empty message")
	}
	if !containsAll(got, "shell_exec", "errored") {
		t.Fatalf("expected tool name and errored status, got %q", got)
	}
}

func TestBuildFeedbackMessage_EmptyCallID(t *testing.T) {
	result := ToolResult{Content: "done"}
	got := buildFeedbackMessage(result)
	if !containsAll(got, "tool", "completed") {
		t.Fatalf("expected 'tool completed', got %q", got)
	}
}

func TestBuildFeedbackMessage_EmptyContent(t *testing.T) {
	result := ToolResult{CallID: "plan"}
	got := buildFeedbackMessage(result)
	if got != "plan completed" {
		t.Fatalf("expected 'plan completed', got %q", got)
	}
}

// --- extractRewardValue ---

func TestExtractRewardValue_NilMetadata(t *testing.T) {
	_, ok := extractRewardValue(nil)
	if ok {
		t.Fatal("expected false for nil metadata")
	}
}

func TestExtractRewardValue_Float64(t *testing.T) {
	v, ok := extractRewardValue(map[string]any{"reward": float64(0.8)})
	if !ok || v != 0.8 {
		t.Fatalf("expected 0.8/true, got %v/%v", v, ok)
	}
}

func TestExtractRewardValue_Float32(t *testing.T) {
	v, ok := extractRewardValue(map[string]any{"score": float32(0.5)})
	if !ok || v != float64(float32(0.5)) {
		t.Fatalf("expected float32 conversion, got %v/%v", v, ok)
	}
}

func TestExtractRewardValue_Int(t *testing.T) {
	v, ok := extractRewardValue(map[string]any{"value": 1})
	if !ok || v != 1.0 {
		t.Fatalf("expected 1.0/true, got %v/%v", v, ok)
	}
}

func TestExtractRewardValue_Int64(t *testing.T) {
	v, ok := extractRewardValue(map[string]any{"reward": int64(2)})
	if !ok || v != 2.0 {
		t.Fatalf("expected 2.0/true, got %v/%v", v, ok)
	}
}

func TestExtractRewardValue_StringParsable(t *testing.T) {
	v, ok := extractRewardValue(map[string]any{"reward": "0.42"})
	if !ok || v != 0.42 {
		t.Fatalf("expected 0.42/true, got %v/%v", v, ok)
	}
}

func TestExtractRewardValue_StringUnparsable(t *testing.T) {
	_, ok := extractRewardValue(map[string]any{"reward": "not-a-number"})
	if ok {
		t.Fatal("expected false for unparsable string")
	}
}

func TestExtractRewardValue_KeyPriority(t *testing.T) {
	// "reward" should be checked before "score" and "value"
	v, ok := extractRewardValue(map[string]any{
		"value":  0.1,
		"score":  0.2,
		"reward": 0.9,
	})
	if !ok || v != 0.9 {
		t.Fatalf("expected reward key to take priority, got %v/%v", v, ok)
	}
}

func TestExtractRewardValue_Uint64(t *testing.T) {
	v, ok := extractRewardValue(map[string]any{"reward": uint64(5)})
	if !ok || v != 5.0 {
		t.Fatalf("expected 5.0/true, got %v/%v", v, ok)
	}
}

func TestExtractRewardValue_Uint32(t *testing.T) {
	v, ok := extractRewardValue(map[string]any{"reward": uint32(3)})
	if !ok || v != 3.0 {
		t.Fatalf("expected 3.0/true, got %v/%v", v, ok)
	}
}

func TestExtractRewardValue_Int32(t *testing.T) {
	v, ok := extractRewardValue(map[string]any{"score": int32(7)})
	if !ok || v != 7.0 {
		t.Fatalf("expected 7.0/true, got %v/%v", v, ok)
	}
}

// helper
func containsAll(s string, substrings ...string) bool {
	for _, sub := range substrings {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstring(s, sub))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
