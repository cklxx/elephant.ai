package swe_bench

import "testing"

func TestConfigManagerEnvOverrides(t *testing.T) {
	t.Setenv("NUM_WORKERS", "7")
	t.Setenv("OUTPUT_PATH", "/tmp/results")
	t.Setenv("DATASET_TYPE", "swe_bench")
	t.Setenv("DATASET_SUBSET", "verified")
	t.Setenv("DATASET_SPLIT", "test")

	cm := NewConfigManager()
	cfg, err := cm.LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if cfg.NumWorkers != 7 {
		t.Fatalf("expected num workers 7, got %d", cfg.NumWorkers)
	}
	if cfg.OutputPath != "/tmp/results" {
		t.Fatalf("expected output path override, got %s", cfg.OutputPath)
	}
	if cfg.Instances.Type != "swe_bench" {
		t.Fatalf("expected dataset type override, got %s", cfg.Instances.Type)
	}
	if cfg.Instances.Subset != "verified" {
		t.Fatalf("expected dataset subset override, got %s", cfg.Instances.Subset)
	}
	if cfg.Instances.Split != "test" {
		t.Fatalf("expected dataset split override, got %s", cfg.Instances.Split)
	}
}

func TestConfigManagerEnvOverrideParsingErrors(t *testing.T) {
	t.Setenv("NUM_WORKERS", "not-a-number")

	cm := NewConfigManager()
	if _, err := cm.LoadConfig(""); err == nil {
		t.Fatal("LoadConfig expected to error for invalid worker count")
	}
}
