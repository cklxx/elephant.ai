package preparation

import "testing"

func TestCloneHeaders(t *testing.T) {
	headers := map[string]string{
		"Authorization": "Bearer token",
		"":              "drop",
		" X-Test ":      "ok",
	}
	cloned := cloneHeaders(headers)
	if len(cloned) != 2 {
		t.Fatalf("expected 2 headers, got %d", len(cloned))
	}
	if cloned["Authorization"] != "Bearer token" {
		t.Fatalf("missing Authorization header: %#v", cloned)
	}
	if cloned["X-Test"] != "ok" {
		t.Fatalf("expected trimmed key X-Test, got %#v", cloned)
	}
	headers["Authorization"] = "changed"
	if cloned["Authorization"] != "Bearer token" {
		t.Fatalf("expected cloned map to be isolated: %#v", cloned)
	}
}
