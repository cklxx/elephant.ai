package attachments

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	ProviderLocal      = "local"
	ProviderCloudflare = "cloudflare"

	defaultPathPrefix   = "/api/attachments/"
	defaultCloudTimeout = 15 * time.Second
)

var attachmentFilePattern = regexp.MustCompile(`^[a-f0-9]{64}(\.[a-z0-9]{1,10})?$`)

// StoreConfig captures configuration for attachment storage.
//
// Provider defaults to "local". When Provider is "cloudflare", the Cloudflare fields
// must be provided.
type StoreConfig struct {
	Provider string
	Dir      string

	CloudflareAccountID       string
	CloudflareAccessKeyID     string
	CloudflareSecretAccessKey string
	CloudflareBucket          string
	CloudflarePublicBaseURL   string
	CloudflareKeyPrefix       string
	PresignTTL                time.Duration
}

// Store persists attachment payloads and returns stable URIs that clients can fetch.
type Store struct {
	provider        string
	pathPrefix      string
	localDir        string
	cloudClient     *minio.Client
	cloudBucket     string
	cloudKeyPrefix  string
	cloudPublicBase string
	presignTTL      time.Duration
	cloudTimeout    time.Duration
}

// NewStore constructs an attachment store from the supplied config.
func NewStore(cfg StoreConfig) (*Store, error) {
	provider := strings.TrimSpace(cfg.Provider)
	if provider == "" {
		provider = ProviderLocal
	}

	store := &Store{
		provider:   provider,
		pathPrefix: defaultPathPrefix,
		presignTTL: cfg.PresignTTL,
	}

	if store.presignTTL <= 0 {
		store.presignTTL = 4 * time.Hour
	}
	store.cloudTimeout = defaultCloudTimeout

	switch provider {
	case ProviderLocal:
		if strings.TrimSpace(cfg.Dir) == "" {
			return nil, fmt.Errorf("attachment store dir is required")
		}
		dir := cfg.Dir
		if strings.HasPrefix(dir, "~/") {
			if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
				dir = filepath.Join(home, strings.TrimPrefix(dir, "~/"))
			}
		}
		dir = filepath.Clean(dir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create attachment dir: %w", err)
		}
		store.localDir = dir

	case ProviderCloudflare:
		accountID := strings.TrimSpace(cfg.CloudflareAccountID)
		accessKey := strings.TrimSpace(cfg.CloudflareAccessKeyID)
		secretKey := strings.TrimSpace(cfg.CloudflareSecretAccessKey)
		bucket := strings.TrimSpace(cfg.CloudflareBucket)
		if accountID == "" || accessKey == "" || secretKey == "" || bucket == "" {
			return nil, fmt.Errorf("cloudflare configuration is incomplete")
		}

		endpoint := fmt.Sprintf("%s.r2.cloudflarestorage.com", accountID)
		client, err := minio.New(endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
			Secure: true,
			Region: "auto",
		})
		if err != nil {
			return nil, fmt.Errorf("init cloudflare client: %w", err)
		}

		store.cloudClient = client
		store.cloudBucket = bucket
		store.cloudPublicBase = strings.TrimRight(strings.TrimSpace(cfg.CloudflarePublicBaseURL), "/")
		store.cloudKeyPrefix = normalizePrefix(cfg.CloudflareKeyPrefix)

	default:
		return nil, fmt.Errorf("unsupported attachment provider %q", provider)
	}

	return store, nil
}

// Provider returns the configured provider name.
func (s *Store) Provider() string {
	return s.provider
}

// LocalDir returns the directory used for local storage, or an empty string for non-local providers.
func (s *Store) LocalDir() string {
	return s.localDir
}

// StoreBytes persists the payload and returns a fetchable URI.
func (s *Store) StoreBytes(name, mediaType string, data []byte) (string, error) {
	if s == nil {
		return "", fmt.Errorf("attachment store is nil")
	}
	if len(data) == 0 {
		return "", fmt.Errorf("attachment payload is empty")
	}

	filename := buildFilename(name, mediaType, data)
	switch s.provider {
	case ProviderLocal:
		return s.storeLocal(filename, data)
	case ProviderCloudflare:
		return s.storeCloudflare(filename, mediaType, data)
	default:
		return "", fmt.Errorf("unsupported attachment provider %q", s.provider)
	}
}

// Handler serves or redirects attachment fetches for relative URIs.
func (s *Store) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s == nil {
			http.NotFound(w, r)
			return
		}

		name := strings.TrimPrefix(r.URL.Path, s.pathPrefix)
		name = path.Clean(name)
		name = strings.TrimPrefix(name, "/")
		if name == "" || !attachmentFilePattern.MatchString(strings.ToLower(path.Base(name))) {
			http.NotFound(w, r)
			return
		}

		switch s.provider {
		case ProviderLocal:
			pathOnDisk := filepath.Join(s.localDir, filepath.FromSlash(name))
			if rel, err := filepath.Rel(s.localDir, pathOnDisk); err != nil || strings.HasPrefix(rel, "..") || rel == "." {
				http.NotFound(w, r)
				return
			}
			http.ServeFile(w, r, pathOnDisk)
		case ProviderCloudflare:
			uri := s.objectFetchURL(r.Context(), name)
			if uri == "" {
				http.NotFound(w, r)
				return
			}
			http.Redirect(w, r, uri, http.StatusTemporaryRedirect)
		default:
			http.NotFound(w, r)
		}
	})
}

func (s *Store) storeLocal(filename string, data []byte) (string, error) {
	pathOnDisk := filepath.Join(s.localDir, filepath.FromSlash(filename))
	if _, err := os.Stat(pathOnDisk); err == nil {
		return s.buildURI(filename), nil
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("stat attachment: %w", err)
	}

	tmp, err := os.CreateTemp(s.localDir, filename+".tmp-*")
	if err != nil {
		return "", fmt.Errorf("create temp attachment: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("write attachment: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("finalize attachment: %w", err)
	}

	if err := os.Rename(tmpPath, pathOnDisk); err != nil {
		_ = os.Remove(tmpPath)
		if _, statErr := os.Stat(pathOnDisk); statErr == nil {
			return s.buildURI(filename), nil
		}
		return "", fmt.Errorf("persist attachment: %w", err)
	}

	return s.buildURI(filename), nil
}

func (s *Store) storeCloudflare(filename, mediaType string, data []byte) (string, error) {
	key := objectKey(s.cloudKeyPrefix, filename)
	contentType := strings.TrimSpace(mediaType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	reader := bytes.NewReader(data)
	ctx, cancel := withTimeout(context.Background(), s.cloudTimeout)
	defer cancel()
	_, err := s.cloudClient.PutObject(ctx, s.cloudBucket, key, reader, int64(len(data)), minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return "", fmt.Errorf("store attachment in cloudflare: %w", err)
	}

	return s.buildCloudURI(key), nil
}

func (s *Store) buildCloudURI(key string) string {
	if s.cloudPublicBase != "" {
		return fmt.Sprintf("%s/%s", s.cloudPublicBase, objectKey("", key))
	}
	if s.cloudClient == nil {
		return ""
	}
	ctx, cancel := withTimeout(context.Background(), s.cloudTimeout)
	defer cancel()
	url, err := s.cloudClient.PresignedGetObject(ctx, s.cloudBucket, objectKey("", key), s.presignTTL, nil)
	if err != nil {
		return ""
	}
	return url.String()
}

func (s *Store) objectFetchURL(ctx context.Context, key string) string {
	clean := objectKey("", key)
	if s.cloudPublicBase != "" {
		return fmt.Sprintf("%s/%s", s.cloudPublicBase, clean)
	}
	if s.cloudClient == nil {
		return ""
	}
	ctx, cancel := withTimeout(ctx, s.cloudTimeout)
	defer cancel()
	url, err := s.cloudClient.PresignedGetObject(ctx, s.cloudBucket, clean, s.presignTTL, nil)
	if err != nil {
		return ""
	}
	return url.String()
}

func withTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout <= 0 {
		return ctx, func() {}
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

func (s *Store) buildURI(filename string) string {
	return s.pathPrefix + filename
}

func buildFilename(name, mediaType string, data []byte) string {
	hash := sha256.Sum256(data)
	id := hex.EncodeToString(hash[:])

	ext := sanitizeAttachmentExt(filepath.Ext(strings.TrimSpace(name)))
	if ext == "" {
		ext = extFromMediaType(mediaType)
	}

	return id + ext
}

func sanitizeAttachmentExt(ext string) string {
	trimmed := strings.ToLower(strings.TrimSpace(ext))
	if trimmed == "" {
		return ""
	}
	if !strings.HasPrefix(trimmed, ".") {
		return ""
	}
	trimmed = strings.TrimPrefix(trimmed, ".")
	if trimmed == "" || len(trimmed) > 10 {
		return ""
	}
	for _, r := range trimmed {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			continue
		}
		return ""
	}
	return "." + trimmed
}

func extFromMediaType(mediaType string) string {
	mt := strings.TrimSpace(mediaType)
	if mt == "" {
		return ""
	}
	exts, err := mime.ExtensionsByType(mt)
	if err != nil || len(exts) == 0 {
		return ""
	}
	return sanitizeAttachmentExt(exts[0])
}

func objectKey(prefix, filename string) string {
	clean := path.Clean(filename)
	clean = strings.TrimPrefix(clean, "/")
	if prefix == "" {
		return clean
	}
	return path.Clean(prefix + "/" + clean)
}

func normalizePrefix(prefix string) string {
	trimmed := strings.Trim(prefix, "/ ")
	if trimmed == "" {
		return ""
	}
	return path.Clean(trimmed)
}
