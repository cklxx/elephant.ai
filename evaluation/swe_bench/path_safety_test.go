package swe_bench

import "testing"

func TestSanitizeOutputPath(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "relative", input: "results/run", wantErr: false},
		{name: "absolute", input: "/tmp/results/run", wantErr: false},
		{name: "traversal", input: "../escape", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := sanitizeOutputPath(safeOutputBaseDir, tt.input)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error for %q", tt.input)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.input, err)
			}
		})
	}
}

func TestSanitizeDatasetKey(t *testing.T) {
	if _, err := sanitizeDatasetKey(".."); err == nil {
		t.Fatalf("expected rejection for dot traversal")
	}
	if _, err := sanitizeDatasetKey("foo/bar"); err == nil {
		t.Fatalf("expected rejection for path separator")
	}
	if _, err := sanitizeDatasetKey("swe_bench_lite_dev"); err != nil {
		t.Fatalf("expected valid key, got error: %v", err)
	}
}
