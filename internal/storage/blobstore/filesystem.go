package blobstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

// FilesystemStore implements BlobStore by writing files to the local disk. It is intended for development and testing.
type FilesystemStore struct {
	baseDir   string
	publicURL string
}

// NewFilesystemStore creates a new store rooted at the provided base directory. If publicURL is empty, signed URLs will be
// generated using the `file://` scheme pointing to the absolute file path.
func NewFilesystemStore(baseDir, publicURL string) (*FilesystemStore, error) {
	if baseDir == "" {
		baseDir = "data/blobs"
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("create blob dir: %w", err)
	}
	return &FilesystemStore{baseDir: baseDir, publicURL: publicURL}, nil
}

func (s *FilesystemStore) PutObject(ctx context.Context, key string, body io.Reader, opts PutOptions) (string, error) {
	if key == "" {
		sum := sha256.New()
		tee := io.TeeReader(body, sum)
		tmpName := fmt.Sprintf("tmp-%d", time.Now().UnixNano())
		tmpPath := filepath.Join(s.baseDir, tmpName)
		f, err := os.Create(tmpPath)
		if err != nil {
			return "", fmt.Errorf("create temp blob: %w", err)
		}
		if _, err := io.Copy(f, tee); err != nil {
			_ = f.Close()
			_ = os.Remove(tmpPath)
			return "", fmt.Errorf("write blob: %w", err)
		}
		if err := f.Close(); err != nil {
			_ = os.Remove(tmpPath)
			return "", fmt.Errorf("close blob: %w", err)
		}
		key = hex.EncodeToString(sum.Sum(nil))
		finalPath := filepath.Join(s.baseDir, key)
		if err := os.Rename(tmpPath, finalPath); err != nil {
			_ = os.Remove(tmpPath)
			return "", fmt.Errorf("rename blob: %w", err)
		}
		return key, nil
	}

	path := filepath.Join(s.baseDir, key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("ensure blob dir: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("create blob: %w", err)
	}
	if _, err := io.Copy(f, body); err != nil {
		_ = f.Close()
		return "", fmt.Errorf("write blob: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("close blob: %w", err)
	}
	return key, nil
}

func (s *FilesystemStore) GetSignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	if s.publicURL != "" {
		u, err := url.Parse(s.publicURL)
		if err != nil {
			return "", fmt.Errorf("parse public url: %w", err)
		}
		u.Path = filepath.ToSlash(filepath.Join(u.Path, key))
		q := u.Query()
		if expiry > 0 {
			q.Set("expires", fmt.Sprintf("%d", time.Now().Add(expiry).Unix()))
		}
		u.RawQuery = q.Encode()
		return u.String(), nil
	}
	abs := filepath.Join(s.baseDir, key)
	return (&url.URL{Scheme: "file", Path: abs}).String(), nil
}

func (s *FilesystemStore) DeleteObject(ctx context.Context, key string) error {
	if key == "" {
		return nil
	}
	path := filepath.Join(s.baseDir, key)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete blob: %w", err)
	}
	return nil
}
