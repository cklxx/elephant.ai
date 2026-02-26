package larktools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
		Arguments: map[string]any{"source_kind": "path", "source": "out/report.txt"},
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
		Arguments: map[string]any{"source_kind": "path", "source": "out/report.txt"},
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
		Arguments: map[string]any{"source_kind": "path", "source": "out/report.txt"},
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
	if _, ok := def.Parameters.Properties["source"]; !ok {
		t.Fatalf("missing source parameter")
	}
	if _, ok := def.Parameters.Properties["source_kind"]; !ok {
		t.Fatalf("missing source_kind parameter")
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
		t.Fatal("expected error when source/source_kind are missing")
	}
	if !strings.Contains(errResult.Content, "source is required") {
		t.Fatalf("unexpected error content: %s", errResult.Content)
	}

	_, errResult = prepareUploadCandidate(context.Background(), "call-2", map[string]any{
		"source":      "a.txt",
		"source_kind": "invalid",
	}, defaultMaxBytes)
	if errResult == nil || errResult.Error == nil {
		t.Fatal("expected error for invalid source_kind")
	}
	if !strings.Contains(errResult.Content, "source_kind must be one of") {
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

	cand, errResult := prepareUploadCandidate(ctx, "call-1", map[string]any{"source_kind": "path", "source": "a.txt"}, defaultMaxBytes)
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
		"source_kind": "path",
		"source":      "a.txt",
		"file_name":   "b.pdf",
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

	_, errResult = prepareUploadCandidate(ctx, "call-3", map[string]any{"source_kind": "path", "source": "a.txt"}, 4)
	if errResult == nil || errResult.Error == nil {
		t.Fatal("expected error when maxBytes is too small")
	}

	if err := os.Mkdir(filepath.Join(tempDir, "dir"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	_, errResult = prepareUploadCandidate(ctx, "call-4", map[string]any{"source_kind": "path", "source": "dir"}, defaultMaxBytes)
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

	cand, errResult := prepareUploadCandidate(context.Background(), "call-1", map[string]any{"source_kind": "path", "source": path}, defaultMaxBytes)
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

	cand, errResult := prepareUploadCandidate(ctx, "call-1", map[string]any{"source_kind": "attachment", "source": "a.txt"}, defaultMaxBytes)
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
	if cand.mimeType != "text/plain" {
		t.Fatalf("unexpected mimeType: %s", cand.mimeType)
	}

	cand, errResult = prepareUploadCandidate(ctx, "call-2", map[string]any{
		"source_kind": "attachment",
		"source":      "A.TXT",
	}, defaultMaxBytes)
	if errResult != nil {
		t.Fatalf("unexpected error: %v", errResult.Error)
	}
	if cand.fileName != "a.txt" {
		t.Fatalf("expected case-insensitive attachment name to resolve to a.txt, got %s", cand.fileName)
	}

	cand, errResult = prepareUploadCandidate(ctx, "call-3", map[string]any{
		"source_kind": "attachment",
		"source":      "a.txt",
		"file_name":   "b.pdf",
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

	_, errResult = prepareUploadCandidate(ctx, "call-4", map[string]any{"source_kind": "attachment", "source": "a.txt"}, 4)
	if errResult == nil || errResult.Error == nil {
		t.Fatal("expected error when maxBytes is too small")
	}
}

func TestPrepareUploadCandidate_AttachmentMode_AudioMimeType(t *testing.T) {
	payload := []byte("ID3\x04\x00\x00\x00\x00\x00\x21")
	encoded := base64.StdEncoding.EncodeToString(payload)
	att := ports.Attachment{Name: "voice.bin", MediaType: "audio/mpeg", Data: encoded}

	ctx := toolports.WithAttachmentContext(context.Background(), map[string]ports.Attachment{"voice.bin": att}, nil)

	cand, errResult := prepareUploadCandidate(ctx, "call-1", map[string]any{"source_kind": "attachment", "source": "voice.bin"}, defaultMaxBytes)
	if errResult != nil {
		t.Fatalf("unexpected error: %v", errResult.Error)
	}
	if !isAudioFile(cand.fileName, cand.mimeType) {
		t.Fatalf("expected audio detection by mime type, got name=%s mime=%s", cand.fileName, cand.mimeType)
	}
}

func TestUploadFile_Execute_ImageAttachment_UsesImageAPI(t *testing.T) {
	var mu sync.Mutex
	var imageUploadCalls int
	var fileUploadCalls int
	var sentMsgType string
	var sentContent string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/auth/v3/tenant_access_token/internal"):
			_, _ = w.Write(tokenResponse("tenant-token", 7200))
			return
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/im/v1/images"):
			mu.Lock()
			imageUploadCalls++
			mu.Unlock()
			_, _ = w.Write(jsonResponse(0, "ok", map[string]any{
				"image_key": "img_123",
			}))
			return
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/im/v1/files"):
			mu.Lock()
			fileUploadCalls++
			mu.Unlock()
			_, _ = w.Write(jsonResponse(0, "ok", map[string]any{
				"file_key": "file_123",
			}))
			return
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/im/v1/messages"):
			var payload struct {
				MsgType string `json:"msg_type"`
				Content string `json:"content"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode message payload: %v", err)
			}
			mu.Lock()
			sentMsgType = payload.MsgType
			sentContent = payload.Content
			mu.Unlock()
			_, _ = w.Write(jsonResponse(0, "ok", map[string]any{
				"message_id": "om_123",
			}))
			return
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	tool := NewLarkUploadFile()
	larkClient := lark.NewClient("test_app_id", "test_app_secret", lark.WithOpenBaseUrl(srv.URL))
	ctx := shared.WithLarkClient(context.Background(), larkClient)
	ctx = shared.WithLarkChatID(ctx, "oc_chat123")

	payload := []byte{0x89, 0x50, 0x4e, 0x47}
	encoded := base64.StdEncoding.EncodeToString(payload)
	att := ports.Attachment{Name: "photo.png", MediaType: "image/png", Data: encoded}
	ctx = toolports.WithAttachmentContext(ctx, map[string]ports.Attachment{"photo.png": att}, nil)

	call := ports.ToolCall{
		ID:        "call-image",
		Name:      "lark_upload_file",
		Arguments: map[string]any{"source_kind": "attachment", "source": "photo.png"},
	}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("expected success, got error: %v", result.Error)
	}

	mu.Lock()
	defer mu.Unlock()
	if imageUploadCalls != 1 {
		t.Fatalf("expected 1 image upload call, got %d", imageUploadCalls)
	}
	if fileUploadCalls != 0 {
		t.Fatalf("expected 0 file upload calls for image, got %d", fileUploadCalls)
	}
	if sentMsgType != "image" {
		t.Fatalf("expected sent msg_type=image, got %q", sentMsgType)
	}
	if sentContent != `{"image_key":"img_123"}` {
		t.Fatalf("unexpected message content: %s", sentContent)
	}
	if got := result.Metadata["msg_type"]; got != "image" {
		t.Fatalf("expected metadata msg_type=image, got %v", got)
	}
	if got := result.Metadata["image_key"]; got != "img_123" {
		t.Fatalf("expected metadata image_key=img_123, got %v", got)
	}
}

func TestUploadFile_Execute_Attachment_UsesFileAPIForNonImage(t *testing.T) {
	var mu sync.Mutex
	var imageUploadCalls int
	var fileUploadCalls int
	var sentMsgType string
	var sentContent string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/auth/v3/tenant_access_token/internal"):
			_, _ = w.Write(tokenResponse("tenant-token", 7200))
			return
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/im/v1/images"):
			mu.Lock()
			imageUploadCalls++
			mu.Unlock()
			_, _ = w.Write(jsonResponse(0, "ok", map[string]any{
				"image_key": "img_123",
			}))
			return
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/im/v1/files"):
			mu.Lock()
			fileUploadCalls++
			mu.Unlock()
			_, _ = w.Write(jsonResponse(0, "ok", map[string]any{
				"file_key": "file_123",
			}))
			return
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/im/v1/messages"):
			var payload struct {
				MsgType string `json:"msg_type"`
				Content string `json:"content"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode message payload: %v", err)
			}
			mu.Lock()
			sentMsgType = payload.MsgType
			sentContent = payload.Content
			mu.Unlock()
			_, _ = w.Write(jsonResponse(0, "ok", map[string]any{
				"message_id": "om_456",
			}))
			return
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	tool := NewLarkUploadFile()
	larkClient := lark.NewClient("test_app_id", "test_app_secret", lark.WithOpenBaseUrl(srv.URL))
	ctx := shared.WithLarkClient(context.Background(), larkClient)
	ctx = shared.WithLarkChatID(ctx, "oc_chat123")

	payload := []byte("hello")
	encoded := base64.StdEncoding.EncodeToString(payload)
	att := ports.Attachment{Name: "report.txt", MediaType: "text/plain", Data: encoded}
	ctx = toolports.WithAttachmentContext(ctx, map[string]ports.Attachment{"report.txt": att}, nil)

	call := ports.ToolCall{
		ID:        "call-file",
		Name:      "lark_upload_file",
		Arguments: map[string]any{"source_kind": "attachment", "source": "report.txt"},
	}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("expected success, got error: %v", result.Error)
	}

	mu.Lock()
	defer mu.Unlock()
	if imageUploadCalls != 0 {
		t.Fatalf("expected 0 image upload calls for non-image, got %d", imageUploadCalls)
	}
	if fileUploadCalls != 1 {
		t.Fatalf("expected 1 file upload call, got %d", fileUploadCalls)
	}
	if sentMsgType != "file" {
		t.Fatalf("expected sent msg_type=file, got %q", sentMsgType)
	}
	if sentContent != `{"file_key":"file_123"}` {
		t.Fatalf("unexpected message content: %s", sentContent)
	}
	if got := result.Metadata["msg_type"]; got != "file" {
		t.Fatalf("expected metadata msg_type=file, got %v", got)
	}
	if got := result.Metadata["file_key"]; got != "file_123" {
		t.Fatalf("expected metadata file_key=file_123, got %v", got)
	}
}

func TestFileHelpers(t *testing.T) {
	if got := fileContent("file_123"); got != `{"file_key":"file_123"}` {
		t.Fatalf("unexpected fileContent: %s", got)
	}
	if got := imageContent("img_123"); got != `{"image_key":"img_123"}` {
		t.Fatalf("unexpected imageContent: %s", got)
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

func TestIsImageFile(t *testing.T) {
	if !isImageFile("photo.jpg", "") {
		t.Fatal("expected jpg extension to be image")
	}
	if !isImageFile("photo.JPEG", "") {
		t.Fatal("expected jpeg extension to be image")
	}
	if !isImageFile("photo.png", "") {
		t.Fatal("expected png extension to be image")
	}
	if !isImageFile("photo.bin", "image/png") {
		t.Fatal("expected image/png mime to be image")
	}
	if isImageFile("report.txt", "text/plain") {
		t.Fatal("did not expect txt to be image")
	}
}

func TestIsAudioFile(t *testing.T) {
	if !isAudioFile("track.mp3", "") {
		t.Fatal("expected mp3 extension to be audio")
	}
	if !isAudioFile("track.M4A", "") {
		t.Fatal("expected m4a extension to be audio")
	}
	if !isAudioFile("track.bin", "audio/mpeg") {
		t.Fatal("expected audio/mpeg mime to be audio")
	}
	if !isAudioFile("track.bin", "audio/x-wav; charset=binary") {
		t.Fatal("expected audio/x-wav mime with params to be audio")
	}
	if isAudioFile("report.pdf", "application/pdf") {
		t.Fatal("did not expect pdf to be audio")
	}
}
