package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	agentstorage "alex/internal/agent/ports/storage"
)

func TestFileCostStore_SaveAndGet(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	store, err := NewFileCostStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileCostStore failed: %v", err)
	}

	ctx := context.Background()

	// Create test record
	now := time.Now()
	record := agentstorage.UsageRecord{
		ID:           "test-1",
		SessionID:    "session-1",
		Model:        "gpt-4o",
		Provider:     "openrouter",
		InputTokens:  1000,
		OutputTokens: 500,
		TotalTokens:  1500,
		InputCost:    0.005,
		OutputCost:   0.0075,
		TotalCost:    0.0125,
		Timestamp:    now,
	}

	// Save record
	err = store.SaveUsage(ctx, record)
	if err != nil {
		t.Fatalf("SaveUsage failed: %v", err)
	}

	// Verify file was created in date directory
	dateStr := now.Format("2006-01-02")
	recordsFile := filepath.Join(tmpDir, dateStr, "records.jsonl")
	if _, err := os.Stat(recordsFile); os.IsNotExist(err) {
		t.Errorf("Records file not created: %s", recordsFile)
	}

	// Retrieve by session
	records, err := store.GetBySession(ctx, "session-1")
	if err != nil {
		t.Fatalf("GetBySession failed: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(records))
	}

	if records[0].ID != "test-1" {
		t.Errorf("Record ID = %s, want test-1", records[0].ID)
	}
}

func TestFileCostStore_GetByDateRange(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileCostStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileCostStore failed: %v", err)
	}

	ctx := context.Background()

	// Create records on different days
	today := time.Now()
	yesterday := today.Add(-24 * time.Hour)
	twoDaysAgo := today.Add(-48 * time.Hour)

	records := []agentstorage.UsageRecord{
		{
			ID:        "1",
			SessionID: "session-1",
			Model:     "gpt-4o",
			Provider:  "openrouter",
			TotalCost: 0.01,
			Timestamp: twoDaysAgo,
		},
		{
			ID:        "2",
			SessionID: "session-1",
			Model:     "gpt-4o",
			Provider:  "openrouter",
			TotalCost: 0.02,
			Timestamp: yesterday,
		},
		{
			ID:        "3",
			SessionID: "session-1",
			Model:     "gpt-4o",
			Provider:  "openrouter",
			TotalCost: 0.03,
			Timestamp: today,
		},
	}

	for _, record := range records {
		if err := store.SaveUsage(ctx, record); err != nil {
			t.Fatalf("SaveUsage failed: %v", err)
		}
	}

	// Get records from yesterday to today
	start := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, yesterday.Location())
	end := today.Add(24 * time.Hour)

	retrieved, err := store.GetByDateRange(ctx, start, end)
	if err != nil {
		t.Fatalf("GetByDateRange failed: %v", err)
	}

	// Should get yesterday's and today's records (2 total)
	if len(retrieved) < 2 {
		t.Errorf("Expected at least 2 records, got %d", len(retrieved))
	}
}

func TestFileCostStore_GetByModel(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileCostStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileCostStore failed: %v", err)
	}

	ctx := context.Background()

	records := []agentstorage.UsageRecord{
		{
			ID:        "1",
			SessionID: "session-1",
			Model:     "gpt-4o",
			Provider:  "openrouter",
			Timestamp: time.Now(),
		},
		{
			ID:        "2",
			SessionID: "session-1",
			Model:     "gpt-4o-mini",
			Provider:  "openrouter",
			Timestamp: time.Now(),
		},
		{
			ID:        "3",
			SessionID: "session-1",
			Model:     "gpt-4o",
			Provider:  "openrouter",
			Timestamp: time.Now(),
		},
	}

	for _, record := range records {
		if err := store.SaveUsage(ctx, record); err != nil {
			t.Fatalf("SaveUsage failed: %v", err)
		}
	}

	// Get gpt-4o records
	retrieved, err := store.GetByModel(ctx, "gpt-4o")
	if err != nil {
		t.Fatalf("GetByModel failed: %v", err)
	}

	if len(retrieved) != 2 {
		t.Errorf("Expected 2 gpt-4o records, got %d", len(retrieved))
	}

	// Verify all are gpt-4o
	for _, r := range retrieved {
		if r.Model != "gpt-4o" {
			t.Errorf("Expected model gpt-4o, got %s", r.Model)
		}
	}
}

func TestFileCostStore_ListAll(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileCostStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileCostStore failed: %v", err)
	}

	ctx := context.Background()

	// Create records on multiple days
	today := time.Now()
	yesterday := today.Add(-24 * time.Hour)

	records := []agentstorage.UsageRecord{
		{
			ID:        "1",
			SessionID: "session-1",
			Model:     "gpt-4o",
			Timestamp: yesterday,
		},
		{
			ID:        "2",
			SessionID: "session-1",
			Model:     "gpt-4o",
			Timestamp: today,
		},
	}

	for _, record := range records {
		if err := store.SaveUsage(ctx, record); err != nil {
			t.Fatalf("SaveUsage failed: %v", err)
		}
	}

	// List all
	all, err := store.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll failed: %v", err)
	}

	if len(all) != 2 {
		t.Errorf("Expected 2 records, got %d", len(all))
	}
}

func TestFileCostStore_SessionIndex(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileCostStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileCostStore failed: %v", err)
	}

	ctx := context.Background()

	// Create records for same session on different days
	today := time.Now()
	yesterday := today.Add(-24 * time.Hour)

	records := []agentstorage.UsageRecord{
		{
			ID:        "1",
			SessionID: "session-1",
			Model:     "gpt-4o",
			Timestamp: yesterday,
		},
		{
			ID:        "2",
			SessionID: "session-1",
			Model:     "gpt-4o",
			Timestamp: today,
		},
	}

	for _, record := range records {
		if err := store.SaveUsage(ctx, record); err != nil {
			t.Fatalf("SaveUsage failed: %v", err)
		}
	}

	// Check that session index was created
	indexFile := filepath.Join(tmpDir, "_index", "session-1.json")
	if _, err := os.Stat(indexFile); os.IsNotExist(err) {
		t.Errorf("Session index file not created: %s", indexFile)
	}

	// Retrieve by session should work efficiently
	retrieved, err := store.GetBySession(ctx, "session-1")
	if err != nil {
		t.Fatalf("GetBySession failed: %v", err)
	}

	if len(retrieved) != 2 {
		t.Errorf("Expected 2 records for session-1, got %d", len(retrieved))
	}
}

func TestFileCostStore_HomeDirectory(t *testing.T) {
	// Use a temporary home directory so the test can run in restricted environments.
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	store, err := NewFileCostStore("~/.test-alex-costs")
	if err != nil {
		t.Fatalf("NewFileCostStore with home dir failed: %v", err)
	}

	if store == nil {
		t.Fatal("Store should not be nil")
	}

	testDir := filepath.Join(tempHome, ".test-alex-costs")
	if _, err := os.Stat(testDir); err != nil {
		t.Fatalf("expected test directory to be created: %v", err)
	}
}
