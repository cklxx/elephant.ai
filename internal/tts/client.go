package tts

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"alex/internal/storage"
)

// Request describes a synthesis call.
type Request struct {
	Alias      string
	Text       string
	Voice      string
	Style      string
	Format     string
	Parameters map[string]string
	Output     string
}

// Result captures the outcome of a synthesis call.
type Result struct {
	Path      string
	Provider  string
	FromCache bool
	Duration  time.Duration
	Metadata  map[string]string
}

// Client encapsulates TTS synthesis behaviour.
type Client interface {
	Synthesize(ctx context.Context, req Request) (Result, error)
}

// Provider is the low-level implementation for a specific vendor.
type Provider interface {
	Synthesize(ctx context.Context, req Request) (ProviderResult, error)
	Name() string
}

// ProviderResult returned by the vendor provider.
type ProviderResult struct {
	Audio       []byte
	ContentType string
	Duration    time.Duration
	Metadata    map[string]string
}

// FileCacheClient caches TTS responses on the filesystem.
type FileCacheClient struct {
	Provider Provider
	Storage  *storage.Manager
	CacheDir string
}

// Synthesize implements Client.
func (c *FileCacheClient) Synthesize(ctx context.Context, req Request) (Result, error) {
	if c.Provider == nil {
		return Result{}, errors.New("tts: provider is required")
	}
	if strings.TrimSpace(req.Text) == "" {
		return Result{}, errors.New("tts: text is required")
	}
	if strings.TrimSpace(req.Voice) == "" {
		return Result{}, errors.New("tts: voice is required")
	}
	format := req.Format
	if format == "" {
		format = "mp3"
	}
	cachePath := req.Output
	if cachePath == "" {
		cachePath = c.defaultCachePath(req, format)
	}
	resolvedPath, err := c.resolve(cachePath)
	if err != nil {
		return Result{}, err
	}
	exists, err := c.exists(cachePath)
	if err != nil {
		return Result{}, err
	}
	if exists {
		return Result{Path: resolvedPath, Provider: c.Provider.Name(), FromCache: true}, nil
	}
	providerResult, err := c.Provider.Synthesize(ctx, req)
	if err != nil {
		return Result{}, err
	}
	content := providerResult.Audio
	if len(content) == 0 {
		return Result{}, errors.New("tts: provider returned empty audio")
	}
	if _, err := c.write(cachePath, content); err != nil {
		return Result{}, err
	}
	return Result{
		Path:      resolvedPath,
		Provider:  c.Provider.Name(),
		FromCache: false,
		Duration:  providerResult.Duration,
		Metadata:  providerResult.Metadata,
	}, nil
}

func (c *FileCacheClient) defaultCachePath(req Request, format string) string {
	sum := sha1.Sum([]byte(strings.Join([]string{req.Voice, format, req.Text}, "|")))
	hash := hex.EncodeToString(sum[:])
	dir := strings.Trim(filepath.Clean(c.CacheDir), string(filepath.Separator))
	if dir == "" {
		dir = "tts"
	}
	file := fmt.Sprintf("%s.%s", hash, format)
	if req.Alias != "" {
		file = fmt.Sprintf("%s-%s.%s", sanitize(req.Alias), hash[:8], format)
	}
	return filepath.Join(dir, req.Voice, file)
}

func (c *FileCacheClient) resolve(rel string) (string, error) {
	if c.Storage != nil {
		return c.Storage.Resolve(rel)
	}
	return filepath.Abs(rel)
}

func (c *FileCacheClient) exists(rel string) (bool, error) {
	if c.Storage != nil {
		return c.Storage.Exists(rel)
	}
	path, err := filepath.Abs(rel)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func (c *FileCacheClient) write(rel string, data []byte) (string, error) {
	if c.Storage != nil {
		return c.Storage.WriteFile(rel, data, 0o644)
	}
	path, err := filepath.Abs(rel)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func sanitize(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "..", "")
	return name
}
