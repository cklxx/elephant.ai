package llamacpp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultHFBaseURL   = "https://huggingface.co"
	defaultRevision    = "main"
	defaultModelsDir   = "~/.alex/models/llama.cpp"
	maxErrorBodyBytes  = 8 * 1024
	defaultHTTPTimeout = 30 * time.Minute
)

// GGUFRef identifies a single GGUF weight file hosted on Hugging Face.
type GGUFRef struct {
	Repo     string // e.g. "TheBloke/Meta-Llama-3-8B-Instruct-GGUF"
	File     string // e.g. "meta-llama-3-8b-instruct-q4_k_m.gguf"
	Revision string // optional; defaults to "main"
	SHA256   string // optional hex digest; when set, download is verified
}

// DownloadOptions controls how GGUF weights are downloaded and stored.
type DownloadOptions struct {
	// BaseDir is the root directory for the model cache. When empty, defaults
	// to "~/.alex/models/llama.cpp".
	BaseDir string

	// HFBaseURL overrides the Hugging Face base URL (used by tests).
	HFBaseURL string

	// HFToken is an optional token for private repos.
	HFToken string

	// HTTPClient overrides the HTTP client used for download.
	HTTPClient *http.Client

	// Timeout overrides the HTTP client timeout when HTTPClient is nil.
	Timeout time.Duration
}

func (r GGUFRef) normalized() GGUFRef {
	out := r
	out.Repo = strings.TrimSpace(out.Repo)
	out.File = strings.TrimSpace(out.File)
	out.Revision = strings.TrimSpace(out.Revision)
	out.SHA256 = strings.TrimSpace(out.SHA256)
	if out.Revision == "" {
		out.Revision = defaultRevision
	}
	return out
}

func (r GGUFRef) validate() error {
	if r.Repo == "" {
		return fmt.Errorf("repo is required")
	}
	if r.File == "" {
		return fmt.Errorf("file is required")
	}
	if r.SHA256 != "" {
		if _, err := hex.DecodeString(r.SHA256); err != nil {
			return fmt.Errorf("sha256 must be hex: %w", err)
		}
		if len(r.SHA256) != 64 {
			return fmt.Errorf("sha256 must be 64 hex chars, got %d", len(r.SHA256))
		}
	}
	return nil
}

// DownloadGGUF downloads a GGUF file from Hugging Face into a local cache
// directory and returns the resolved local filesystem path.
//
// The download is atomic: bytes are written to a temporary file in the target
// directory and then renamed into place.
func DownloadGGUF(ctx context.Context, ref GGUFRef, opts DownloadOptions) (string, error) {
	ref = ref.normalized()
	if err := ref.validate(); err != nil {
		return "", err
	}

	baseDir := strings.TrimSpace(opts.BaseDir)
	if baseDir == "" {
		baseDir = defaultModelsDir
	}
	expanded, err := expandHomeDir(baseDir)
	if err != nil {
		return "", err
	}

	dest := filepath.Join(expanded, "hf", filepath.FromSlash(ref.Repo), ref.Revision, filepath.FromSlash(ref.File))

	if info, err := os.Stat(dest); err == nil && info.Mode().IsRegular() && info.Size() > 0 {
		if ref.SHA256 == "" {
			return dest, nil
		}
		matches, err := fileSHA256Matches(dest, ref.SHA256)
		if err != nil {
			return "", err
		}
		if matches {
			return dest, nil
		}
		return "", fmt.Errorf("existing file sha256 mismatch: %s", dest)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("stat dest: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", fmt.Errorf("mkdir dest dir: %w", err)
	}

	resolveURL, err := buildHFResolveURL(opts.HFBaseURL, ref)
	if err != nil {
		return "", err
	}

	client := opts.HTTPClient
	if client == nil {
		timeout := opts.Timeout
		if timeout <= 0 {
			timeout = defaultHTTPTimeout
		}
		client = &http.Client{Timeout: timeout}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resolveURL, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/octet-stream")
	if strings.TrimSpace(opts.HFToken) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(opts.HFToken))
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		msg := strings.TrimSpace(string(body))
		if msg != "" {
			return "", fmt.Errorf("download failed: %s: %s", resp.Status, msg)
		}
		return "", fmt.Errorf("download failed: %s", resp.Status)
	}

	tmp, err := os.CreateTemp(filepath.Dir(dest), filepath.Base(dest)+".partial-*")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}()

	var writer io.Writer = tmp
	var hasher hash.Hash
	if ref.SHA256 != "" {
		hasher = sha256.New()
		writer = io.MultiWriter(tmp, hasher)
	}

	if _, err := io.Copy(writer, resp.Body); err != nil {
		return "", fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close temp: %w", err)
	}

	if ref.SHA256 != "" {
		actual := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(actual, ref.SHA256) {
			return "", fmt.Errorf("sha256 mismatch: want %s got %s", ref.SHA256, actual)
		}
	}

	if err := os.Rename(tmpPath, dest); err != nil {
		return "", fmt.Errorf("rename into place: %w", err)
	}

	return dest, nil
}

func buildHFResolveURL(base string, ref GGUFRef) (string, error) {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		base = defaultHFBaseURL
	}
	u, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("parse hf base url: %w", err)
	}

	repo := strings.Trim(strings.TrimSpace(ref.Repo), "/")
	rev := strings.TrimSpace(ref.Revision)
	if rev == "" {
		rev = defaultRevision
	}
	file := strings.Trim(strings.TrimSpace(ref.File), "/")
	if repo == "" || file == "" {
		return "", fmt.Errorf("repo and file are required")
	}

	repoEscaped := escapeURLPath(repo)
	fileEscaped := escapeURLPath(file)

	pathPrefix := strings.TrimRight(u.Path, "/")
	u.Path = pathPrefix + "/" + repoEscaped + "/resolve/" + url.PathEscape(rev) + "/" + fileEscaped
	return u.String(), nil
}

func escapeURLPath(raw string) string {
	if raw == "" {
		return ""
	}
	parts := strings.Split(raw, "/")
	escaped := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		escaped = append(escaped, url.PathEscape(part))
	}
	return strings.Join(escaped, "/")
}

func expandHomeDir(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path is empty")
	}
	if path[0] != '~' {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	if path == "~" {
		return home, nil
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:]), nil
	}
	return filepath.Join(home, path[1:]), nil
}

func fileSHA256Matches(path string, wantHex string) (bool, error) {
	wantHex = strings.TrimSpace(wantHex)
	if wantHex == "" {
		return false, fmt.Errorf("want sha256 is empty")
	}
	f, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("open file: %w", err)
	}
	defer func() { _ = f.Close() }()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return false, fmt.Errorf("hash file: %w", err)
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	return strings.EqualFold(actual, wantHex), nil
}

