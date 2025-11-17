package main

import "testing"

func TestParseGlobalCLIOptions(t *testing.T) {
	opts, remaining, err := parseGlobalCLIOptions([]string{"--auto-review=false", "--auto-review-min-score", "0.6", "task"})
	if err != nil {
		t.Fatalf("parseGlobalCLIOptions returned error: %v", err)
	}
	if len(remaining) != 1 || remaining[0] != "task" {
		t.Fatalf("expected remaining task argument, got %v", remaining)
	}
	if opts.autoReview.enabled == nil || *opts.autoReview.enabled != false {
		t.Fatalf("expected auto review disabled override")
	}
	if opts.autoReview.minScore == nil || *opts.autoReview.minScore != 0.6 {
		t.Fatalf("expected min score override, got %+v", opts.autoReview.minScore)
	}
}

func TestGlobalCLIOptionsLoaderOptions(t *testing.T) {
	opts := globalCLIOptions{}
	if got := opts.loaderOptions(); got != nil {
		t.Fatalf("expected nil when no overrides, got %v", got)
	}
	opts.autoReview.maxRework = intPtr(2)
	loader := opts.loaderOptions()
	if len(loader) != 1 {
		t.Fatalf("expected single loader option, got %d", len(loader))
	}
}

func intPtr(v int) *int {
	return &v
}
