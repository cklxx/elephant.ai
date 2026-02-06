package sandbox

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"alex/internal/infra/sandbox"
)

func TestParseSandboxAttachmentSpecs(t *testing.T) {
	args := map[string]any{
		"attachments": []any{
			"/tmp/output.txt",
			map[string]any{
				"path":                  "/tmp/report.png",
				"name":                  "report.png",
				"media_type":            "image/png",
				"retention_ttl_seconds": float64(3600),
			},
		},
	}

	specs, err := parseSandboxAttachmentSpecs(args)
	if err != nil {
		t.Fatalf("parseSandboxAttachmentSpecs error: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("expected 2 specs, got %d", len(specs))
	}
	if specs[0].Path != "/tmp/output.txt" {
		t.Fatalf("unexpected first path: %s", specs[0].Path)
	}
	if specs[1].Name != "report.png" || specs[1].MediaType != "image/png" {
		t.Fatalf("unexpected second spec: %+v", specs[1])
	}
	if specs[1].RetentionTTLSeconds != 3600 {
		t.Fatalf("unexpected retention: %d", specs[1].RetentionTTLSeconds)
	}
}

func TestDownloadSandboxAttachmentsBase64(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/file/read" {
			http.NotFound(w, r)
			return
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		response := sandbox.Response[sandbox.FileReadResult]{
			Success: true,
			Data: &sandbox.FileReadResult{
				File:    "/tmp/hello.txt",
				Content: "aGVsbG8=",
			},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	t.Cleanup(server.Close)

	client := sandbox.NewClient(sandbox.Config{BaseURL: server.URL})
	specs := []sandboxAttachmentSpec{{Path: "/tmp/hello.txt"}}
	attachments, errs := downloadSandboxAttachments(context.Background(), client, "session-1", specs, "sandbox_shell_exec")

	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(attachments))
	}
	att, ok := attachments["hello.txt"]
	if !ok {
		t.Fatalf("expected attachment hello.txt, got keys: %+v", attachments)
	}
	if strings.TrimSpace(att.Data) != "aGVsbG8=" {
		t.Fatalf("unexpected data payload: %s", att.Data)
	}
}
