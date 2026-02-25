package attachments

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewStoreLocal_ExpandsHomePath(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	store, err := NewStore(StoreConfig{
		Provider: ProviderLocal,
		Dir:      "~/.alex/attachments",
	})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	want := filepath.Join(tempHome, ".alex", "attachments")
	if got := store.LocalDir(); got != want {
		t.Fatalf("LocalDir() = %q, want %q", got, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("expected local dir to be created: %v", err)
	}
}

func TestNewStoreLocal_ExpandsEnvPath(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ALEX_ATTACH_ROOT", filepath.Join(root, "attachments-root"))

	store, err := NewStore(StoreConfig{
		Provider: ProviderLocal,
		Dir:      "$ALEX_ATTACH_ROOT/store",
	})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	want := filepath.Join(root, "attachments-root", "store")
	if got := store.LocalDir(); got != want {
		t.Fatalf("LocalDir() = %q, want %q", got, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("expected env-resolved local dir to be created: %v", err)
	}
}

func TestBuildCloudURIUsesR2PublicEndpoint(t *testing.T) {
	store, err := NewStore(StoreConfig{
		Provider:                  ProviderCloudflare,
		CloudflareAccountID:       "acct",
		CloudflareAccessKeyID:     "access",
		CloudflareSecretAccessKey: "secret",
		CloudflareBucket:          "bucket",
		CloudflarePublicBaseURL:   "https://acct.r2.cloudflarestorage.com/bucket",
	})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	key := objectKey("prefix", "image.png")
	got := store.buildCloudURI(key)
	want := "https://acct.r2.cloudflarestorage.com/bucket/prefix/image.png"
	if got != want {
		t.Fatalf("buildCloudURI() = %q, want %q", got, want)
	}
}

func TestBuildCloudURIPresignsWhenNoPublicBase(t *testing.T) {
	store, err := NewStore(StoreConfig{
		Provider:                  ProviderCloudflare,
		CloudflareAccountID:       "acct",
		CloudflareAccessKeyID:     "access",
		CloudflareSecretAccessKey: "secret",
		CloudflareBucket:          "bucket",
	})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if store.cloudPublicBase != "" {
		t.Fatalf("cloudPublicBase = %q, want empty", store.cloudPublicBase)
	}

	key := objectKey("prefix", "image.png")
	got := store.buildCloudURI(key)
	if got == "" {
		t.Fatalf("buildCloudURI() returned empty URL")
	}
	if !strings.Contains(got, "X-Amz-Signature") {
		t.Fatalf("expected presigned URL, got %q", got)
	}
}
