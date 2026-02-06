package attachments

import (
	"strings"
	"testing"
)

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
