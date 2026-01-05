package attachments

import "testing"

func TestBuildCloudURIUsesR2PublicEndpoint(t *testing.T) {
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

	key := objectKey("prefix", "image.png")
	got := store.buildCloudURI(key)
	want := "https://acct.r2.cloudflarestorage.com/bucket/prefix/image.png"
	if got != want {
		t.Fatalf("buildCloudURI() = %q, want %q", got, want)
	}
}
