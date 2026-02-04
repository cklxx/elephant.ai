package aliases

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"alex/internal/tools/builtin/shared"
)

func TestBuildAttachmentsFromSpecs_AllowsTempDir(t *testing.T) {
	tmpDir := os.TempDir()
	if strings.TrimSpace(tmpDir) == "" {
		t.Skip("os.TempDir is empty")
	}
	file, err := os.CreateTemp(tmpDir, "aliases-attachments-*.txt")
	if err != nil {
		t.Skipf("failed to create temp file: %v", err)
	}
	path := file.Name()
	if _, err := file.WriteString("hello"); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		t.Skipf("failed to write temp file: %v", err)
	}
	_ = file.Close()
	t.Cleanup(func() {
		_ = os.Remove(path)
	})

	specs := []attachmentSpec{{Path: path}}
	cfg := shared.AutoUploadConfig{MaxBytes: 1024 * 1024}
	attachments, errs := buildAttachmentsFromSpecs(context.Background(), specs, cfg)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(attachments))
	}

	name := filepath.Base(path)
	att, ok := attachments[name]
	if !ok {
		t.Fatalf("expected attachment %q to exist", name)
	}
	if att.Name != name {
		t.Fatalf("expected attachment name %q, got %q", name, att.Name)
	}
	if att.Source != "lark_local" {
		t.Fatalf("expected attachment source lark_local, got %q", att.Source)
	}
	if strings.TrimSpace(att.MediaType) == "" {
		t.Fatalf("expected media type to be populated")
	}
	if att.Data != base64.StdEncoding.EncodeToString([]byte("hello")) {
		t.Fatalf("unexpected attachment data")
	}
}

func TestBuildAttachmentsFromSpecs_RespectsMaxBytes(t *testing.T) {
	tmpDir := os.TempDir()
	if strings.TrimSpace(tmpDir) == "" {
		t.Skip("os.TempDir is empty")
	}
	file, err := os.CreateTemp(tmpDir, "aliases-attachments-*.txt")
	if err != nil {
		t.Skipf("failed to create temp file: %v", err)
	}
	path := file.Name()
	if _, err := file.WriteString("hello"); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		t.Skipf("failed to write temp file: %v", err)
	}
	_ = file.Close()
	t.Cleanup(func() {
		_ = os.Remove(path)
	})

	specs := []attachmentSpec{{Path: path}}
	cfg := shared.AutoUploadConfig{MaxBytes: 4}
	attachments, errs := buildAttachmentsFromSpecs(context.Background(), specs, cfg)
	if len(attachments) != 0 {
		t.Fatalf("expected no attachments, got %d", len(attachments))
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d (%v)", len(errs), errs)
	}
	if !strings.Contains(errs[0], "exceeds max size") {
		t.Fatalf("unexpected error: %s", errs[0])
	}
}
