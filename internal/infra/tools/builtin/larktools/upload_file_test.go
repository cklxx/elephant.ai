package larktools

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
	toolports "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/pathutil"
	"alex/internal/infra/tools/builtin/shared"

	lark "github.com/larksuite/oapi-sdk-go/v3"
)

func TestUploadFile_NoLarkClient(t *testing.T) {
	tool := NewLarkUploadFile()
	ctx := context.Background()
	call := ports.ToolCall{
		ID:        "call-1",
		Name:      "lark_upload_file",
		Arguments: map[string]any{"path": "out/report.txt"},
	}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error in result when lark client is missing")
	}
	if result.Content != "lark_upload_file is only available inside a Lark chat context." {
		t.Fatalf("unexpected content: %s", result.Content)
	}
}

func TestUploadFile_InvalidClientType(t *testing.T) {
	tool := NewLarkUploadFile()
	ctx := shared.WithLarkClient(context.Background(), "not-a-lark-client")
	call := ports.ToolCall{
		ID:        "call-2",
		Name:      "lark_upload_file",
		Arguments: map[string]any{"path": "out/report.txt"},
	}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error in result for invalid client type")
	}
}

func TestUploadFile_NoChatID(t *testing.T) {
	tool := NewLarkUploadFile()
	larkClient := lark.NewClient("test_app_id", "test_app_secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)
	call := ports.ToolCall{
		ID:        "call-3",
		Name:      "lark_upload_file",
		Arguments: map[string]any{"path": "out/report.txt"},
	}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error in result when chat_id is missing")
	}
	if result.Content != "lark_upload_file: no chat_id available in context." {
		t.Fatalf("unexpected content: %s", result.Content)
	}
}

func TestUploadFile_Metadata(t *testing.T) {
	tool := NewLarkUploadFile()
	meta := tool.Metadata()
	if meta.Name != "lark_upload_file" {
		t.Fatalf("unexpected name: %s", meta.Name)
	}
	if meta.Category != "lark" {
		t.Fatalf("unexpected category: %s", meta.Category)
	}
}

func TestUploadFile_Definition(t *testing.T) {
	tool := NewLarkUploadFile()
	def := tool.Definition()
	if def.Name != "lark_upload_file" {
		t.Fatalf("unexpected name: %s", def.Name)
	}
	if _, ok := def.Parameters.Properties["path"]; !ok {
		t.Fatalf("missing path parameter")
	}
	if _, ok := def.Parameters.Properties["attachment_name"]; !ok {
		t.Fatalf("missing attachment_name parameter")
	}
	if _, ok := def.Parameters.Properties["file_name"]; !ok {
		t.Fatalf("missing file_name parameter")
	}
	if _, ok := def.Parameters.Properties["max_bytes"]; !ok {
		t.Fatalf("missing max_bytes parameter")
	}
	if _, ok := def.Parameters.Properties["reply_to_message_id"]; ok {
		t.Fatalf("unexpected reply_to_message_id parameter")
	}
}

func TestPrepareUploadCandidate_SourceValidation(t *testing.T) {
	_, errResult := prepareUploadCandidate(context.Background(), "call-1", map[string]any{}, defaultMaxBytes)
	if errResult == nil || errResult.Error == nil {
		t.Fatal("expected error when neither path nor attachment_name is provided")
	}
	if !strings.Contains(errResult.Content, "either 'path' or 'attachment_name'") {
		t.Fatalf("unexpected error content: %s", errResult.Content)
	}

	_, errResult = prepareUploadCandidate(context.Background(), "call-2", map[string]any{
		"path":            "a.txt",
		"attachment_name": "b.txt",
	}, defaultMaxBytes)
	if errResult == nil || errResult.Error == nil {
		t.Fatal("expected error when both path and attachment_name are provided")
	}
	if !strings.Contains(errResult.Content, "provide exactly one") {
		t.Fatalf("unexpected error content: %s", errResult.Content)
	}
}

func TestPrepareUploadCandidate_PathMode(t *testing.T) {
	baseDir := pathutil.DefaultWorkingDir()
	if baseDir == "" {
		t.Fatalf("default working dir is empty")
	}
	tempDir, err := os.MkdirTemp(baseDir, "lark-upload-test-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(tempDir)
	})
	ctx := pathutil.WithWorkingDir(context.Background(), tempDir)

	path := filepath.Join(tempDir, "a.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	cand, errResult := prepareUploadCandidate(ctx, "call-1", map[string]any{"path": "a.txt"}, defaultMaxBytes)
	if errResult != nil {
		t.Fatalf("unexpected error: %v", errResult.Error)
	}
	if cand.cleanup == nil {
		t.Fatal("expected cleanup for path mode")
	}
	cand.cleanup()

	if cand.fileName != "a.txt" {
		t.Fatalf("unexpected fileName: %s", cand.fileName)
	}
	if cand.size != int64(len("hello world")) {
		t.Fatalf("unexpected size: %d", cand.size)
	}
	if cand.fileType != "stream" {
		t.Fatalf("unexpected fileType: %s", cand.fileType)
	}
	if got := cand.meta["resolved_path"]; got != filepath.Join(tempDir, "a.txt") {
		t.Fatalf("unexpected resolved_path: %v", got)
	}

	cand, errResult = prepareUploadCandidate(ctx, "call-2", map[string]any{
		"path":      "a.txt",
		"file_name": "b.pdf",
	}, defaultMaxBytes)
	if errResult != nil {
		t.Fatalf("unexpected error: %v", errResult.Error)
	}
	cand.cleanup()
	if cand.fileName != "b.pdf" {
		t.Fatalf("unexpected fileName override: %s", cand.fileName)
	}
	if cand.fileType != "pdf" {
		t.Fatalf("unexpected fileType override: %s", cand.fileType)
	}

	_, errResult = prepareUploadCandidate(ctx, "call-3", map[string]any{"path": "a.txt"}, 4)
	if errResult == nil || errResult.Error == nil {
		t.Fatal("expected error when maxBytes is too small")
	}

	if err := os.Mkdir(filepath.Join(tempDir, "dir"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	_, errResult = prepareUploadCandidate(ctx, "call-4", map[string]any{"path": "dir"}, defaultMaxBytes)
	if errResult == nil || errResult.Error == nil {
		t.Fatal("expected error when path is a directory")
	}
}

func TestPrepareUploadCandidate_PathMode_AllowsTempDir(t *testing.T) {
	tmpDir := os.TempDir()
	if strings.TrimSpace(tmpDir) == "" {
		t.Skip("os.TempDir is empty")
	}
	file, err := os.CreateTemp(tmpDir, "lark-upload-temp-*.txt")
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

	cand, errResult := prepareUploadCandidate(context.Background(), "call-1", map[string]any{"path": path}, defaultMaxBytes)
	if errResult != nil {
		t.Fatalf("unexpected error: %v", errResult.Error)
	}
	if cand.cleanup == nil {
		t.Fatal("expected cleanup for path mode")
	}
	cand.cleanup()

	if cand.fileName != filepath.Base(path) {
		t.Fatalf("unexpected fileName: %s", cand.fileName)
	}
	if cand.size != int64(len("hello")) {
		t.Fatalf("unexpected size: %d", cand.size)
	}
	if cand.fileType != "stream" {
		t.Fatalf("unexpected fileType: %s", cand.fileType)
	}
	expected, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		t.Fatalf("failed to normalize expected path: %v", err)
	}
	if got := cand.meta["resolved_path"]; got != expected {
		t.Fatalf("unexpected resolved_path: %v", got)
	}
}

func TestPrepareUploadCandidate_AttachmentMode(t *testing.T) {
	payload := []byte("hello")
	encoded := base64.StdEncoding.EncodeToString(payload)
	att := ports.Attachment{Name: "a.txt", MediaType: "text/plain", Data: encoded}

	ctx := toolports.WithAttachmentContext(context.Background(), map[string]ports.Attachment{"a.txt": att}, nil)

	cand, errResult := prepareUploadCandidate(ctx, "call-1", map[string]any{"attachment_name": "a.txt"}, defaultMaxBytes)
	if errResult != nil {
		t.Fatalf("unexpected error: %v", errResult.Error)
	}
	if cand.cleanup != nil {
		t.Fatal("did not expect cleanup for attachment mode")
	}
	if cand.fileName != "a.txt" {
		t.Fatalf("unexpected fileName: %s", cand.fileName)
	}
	if cand.size != int64(len(payload)) {
		t.Fatalf("unexpected size: %d", cand.size)
	}
	if cand.fileType != "stream" {
		t.Fatalf("unexpected fileType: %s", cand.fileType)
	}

	cand, errResult = prepareUploadCandidate(ctx, "call-2", map[string]any{
		"attachment_name": "A.TXT",
	}, defaultMaxBytes)
	if errResult != nil {
		t.Fatalf("unexpected error: %v", errResult.Error)
	}
	if cand.fileName != "a.txt" {
		t.Fatalf("expected case-insensitive attachment name to resolve to a.txt, got %s", cand.fileName)
	}

	cand, errResult = prepareUploadCandidate(ctx, "call-3", map[string]any{
		"attachment_name": "a.txt",
		"file_name":       "b.pdf",
	}, defaultMaxBytes)
	if errResult != nil {
		t.Fatalf("unexpected error: %v", errResult.Error)
	}
	if cand.fileName != "b.pdf" {
		t.Fatalf("unexpected fileName override: %s", cand.fileName)
	}
	if cand.fileType != "pdf" {
		t.Fatalf("unexpected fileType override: %s", cand.fileType)
	}

	_, errResult = prepareUploadCandidate(ctx, "call-4", map[string]any{"attachment_name": "a.txt"}, 4)
	if errResult == nil || errResult.Error == nil {
		t.Fatal("expected error when maxBytes is too small")
	}
}

func TestFileHelpers(t *testing.T) {
	if got := fileContent("file_123"); got != `{"file_key":"file_123"}` {
		t.Fatalf("unexpected fileContent: %s", got)
	}

	if got := fileTypeForName("a.PDF"); got != "pdf" {
		t.Fatalf("unexpected fileTypeForName: %s", got)
	}
	if got := fileTypeForName("noext"); got != "" {
		t.Fatalf("expected empty ext, got %q", got)
	}

	if got := larkFileType("pdf"); got != "pdf" {
		t.Fatalf("unexpected larkFileType(pdf): %s", got)
	}
	if got := larkFileType(".pdf"); got != "pdf" {
		t.Fatalf("unexpected larkFileType(.pdf): %s", got)
	}
	if got := larkFileType("txt"); got != "stream" {
		t.Fatalf("unexpected larkFileType(txt): %s", got)
	}
	if got := larkFileType(""); got != "stream" {
		t.Fatalf("unexpected larkFileType(empty): %s", got)
	}
}
