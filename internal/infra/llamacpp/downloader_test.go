package llamacpp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestDownloadGGUFDownloadsToCacheLayout(t *testing.T) {
	t.Parallel()

	var hits atomic.Int64
	content := []byte("gguf-bytes")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if got := r.URL.Path; got != "/testorg/testrepo/resolve/main/model.Q4_K_M.gguf" {
			t.Fatalf("unexpected path: %s", got)
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(content)
	}))
	t.Cleanup(server.Close)

	baseDir := t.TempDir()
	gotPath, err := DownloadGGUF(context.Background(), GGUFRef{
		Repo: "testorg/testrepo",
		File: "model.Q4_K_M.gguf",
	}, DownloadOptions{
		BaseDir:   baseDir,
		HFBaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("DownloadGGUF error: %v", err)
	}
	wantPath := filepath.Join(baseDir, "hf", "testorg", "testrepo", "main", "model.Q4_K_M.gguf")
	if gotPath != wantPath {
		t.Fatalf("unexpected dest: got %s want %s", gotPath, wantPath)
	}
	data, err := os.ReadFile(gotPath)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(data) != string(content) {
		t.Fatalf("unexpected content: %q", string(data))
	}
	if hits.Load() != 1 {
		t.Fatalf("unexpected hits: %d", hits.Load())
	}
}

func TestDownloadGGUFSkipsExistingFileWithoutSHA(t *testing.T) {
	t.Parallel()

	var hits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Error(w, "unexpected", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	baseDir := t.TempDir()
	dest := filepath.Join(baseDir, "hf", "testorg", "testrepo", "main", "model.gguf")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(dest, []byte("existing"), 0o600); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	gotPath, err := DownloadGGUF(context.Background(), GGUFRef{
		Repo: "testorg/testrepo",
		File: "model.gguf",
	}, DownloadOptions{
		BaseDir:   baseDir,
		HFBaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("DownloadGGUF error: %v", err)
	}
	if gotPath != dest {
		t.Fatalf("unexpected dest: got %s want %s", gotPath, dest)
	}
	if hits.Load() != 0 {
		t.Fatalf("expected no download, got hits=%d", hits.Load())
	}
}

func TestDownloadGGUFRejectsSHA256Mismatch(t *testing.T) {
	t.Parallel()

	content := []byte("gguf-bytes")
	sum := sha256.Sum256(content)
	actual := hex.EncodeToString(sum[:])
	bad := strings.Repeat("0", 64)

	if actual == bad {
		t.Fatalf("bad test sha collision")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(content)
	}))
	t.Cleanup(server.Close)

	baseDir := t.TempDir()
	_, err := DownloadGGUF(context.Background(), GGUFRef{
		Repo:   "testorg/testrepo",
		File:   "model.gguf",
		SHA256: bad,
	}, DownloadOptions{
		BaseDir:   baseDir,
		HFBaseURL: server.URL,
	})
	if err == nil {
		t.Fatalf("expected error")
	}

	dest := filepath.Join(baseDir, "hf", "testorg", "testrepo", "main", "model.gguf")
	if _, statErr := os.Stat(dest); statErr == nil {
		t.Fatalf("expected dest not to exist on sha mismatch")
	}
}

