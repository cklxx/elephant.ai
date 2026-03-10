package domain

import "testing"

func TestToolResultPreviewRunes(t *testing.T) {
	if ToolResultPreviewRunes != 280 {
		t.Errorf("ToolResultPreviewRunes = %d, want 280", ToolResultPreviewRunes)
	}
}
