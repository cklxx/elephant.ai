package swe_bench

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DatasetLoaderImpl implements the DatasetLoader interface
type DatasetLoaderImpl struct {
	client       *http.Client
	cacheDir     string
	downloadURLs map[string]string
}

// NewDatasetLoader creates a new dataset loader
func NewDatasetLoader() *DatasetLoaderImpl {
	// Setup cache directory
	homeDir, _ := os.UserHomeDir()
	cacheDir := filepath.Join(homeDir, ".alex", "datasets", "swe_bench")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		log.Printf("Warning: Failed to create cache directory: %v", err)
	}

	return &DatasetLoaderImpl{
		client:   &http.Client{Timeout: 30 * time.Minute},
		cacheDir: cacheDir,
		downloadURLs: map[string]string{
			// SWE-bench Lite (300 instances) - Hugging Face API
			"swe_bench_lite_dev":  "https://datasets-server.huggingface.co/rows?dataset=princeton-nlp/SWE-bench_Lite&config=default&split=test&offset=0&length=300",
			"swe_bench_lite_test": "https://datasets-server.huggingface.co/rows?dataset=princeton-nlp/SWE-bench_Lite&config=default&split=test&offset=0&length=300",

			// SWE-bench Full (2,294 instances) - Hugging Face API
			"swe_bench_full_dev":   "https://datasets-server.huggingface.co/rows?dataset=princeton-nlp/SWE-bench&config=default&split=test&offset=0&length=2294",
			"swe_bench_full_test":  "https://datasets-server.huggingface.co/rows?dataset=princeton-nlp/SWE-bench&config=default&split=test&offset=0&length=2294",
			"swe_bench_full_train": "https://datasets-server.huggingface.co/rows?dataset=princeton-nlp/SWE-bench&config=default&split=train&offset=0&length=23000",

			// SWE-bench Verified (500 instances) - Hugging Face API
			"swe_bench_verified_dev":  "https://datasets-server.huggingface.co/rows?dataset=princeton-nlp/SWE-bench_Verified&config=default&split=test&offset=0&length=500",
			"swe_bench_verified_test": "https://datasets-server.huggingface.co/rows?dataset=princeton-nlp/SWE-bench_Verified&config=default&split=test&offset=0&length=500",
		},
	}
}

// LoadInstances loads instances based on the dataset configuration
func (dl *DatasetLoaderImpl) LoadInstances(ctx context.Context, config DatasetConfig) ([]Instance, error) {
	switch config.Type {
	case "swe_bench":
		return dl.loadSWEBenchInstances(ctx, config)
	case "file":
		return dl.loadFileInstances(ctx, config)
	case "huggingface":
		return dl.loadHuggingFaceInstances(ctx, config)
	default:
		return nil, fmt.Errorf("unsupported dataset type: %s", config.Type)
	}
}

// GetInstanceCount returns the total number of instances in the dataset
func (dl *DatasetLoaderImpl) GetInstanceCount(ctx context.Context, config DatasetConfig) (int, error) {
	instances, err := dl.LoadInstances(ctx, config)
	if err != nil {
		return 0, err
	}
	return len(instances), nil
}

// ValidateConfig validates the dataset configuration
func (dl *DatasetLoaderImpl) ValidateConfig(config DatasetConfig) error {
	switch config.Type {
	case "swe_bench":
		return dl.validateSWEBenchConfig(config)
	case "file":
		return dl.validateFileConfig(config)
	case "huggingface":
		return dl.validateHuggingFaceConfig(config)
	default:
		return fmt.Errorf("unsupported dataset type: %s", config.Type)
	}
}

// loadSWEBenchInstances loads SWE-bench instances
func (dl *DatasetLoaderImpl) loadSWEBenchInstances(ctx context.Context, config DatasetConfig) ([]Instance, error) {
	// Determine dataset key
	datasetKey := fmt.Sprintf("swe_bench_%s_%s", config.Subset, config.Split)

	// Check if URL exists
	url, exists := dl.downloadURLs[datasetKey]
	if !exists {
		return nil, fmt.Errorf("unsupported SWE-bench configuration: subset=%s, split=%s", config.Subset, config.Split)
	}

	// Download or load from cache
	filePath, err := dl.downloadDataset(ctx, datasetKey, url)
	if err != nil {
		return nil, fmt.Errorf("failed to download dataset: %w", err)
	}

	// Parse instances
	instances, err := dl.parseInstancesFromFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse instances: %w", err)
	}

	// Apply filtering
	instances = dl.applyFiltering(instances, config)

	return instances, nil
}

// loadFileInstances loads instances from a local file
func (dl *DatasetLoaderImpl) loadFileInstances(_ context.Context, config DatasetConfig) ([]Instance, error) {
	instances, err := dl.parseInstancesFromFile(config.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse instances from file %s: %w", config.FilePath, err)
	}

	// Apply filtering
	instances = dl.applyFiltering(instances, config)

	return instances, nil
}

// loadHuggingFaceInstances loads instances from Hugging Face dataset
func (dl *DatasetLoaderImpl) loadHuggingFaceInstances(_ context.Context, _ DatasetConfig) ([]Instance, error) {
	// This is a simplified implementation
	// In a real implementation, you would use the Hugging Face datasets library
	return nil, fmt.Errorf("hugging Face dataset loading not implemented yet")
}

// downloadDataset downloads a dataset if not already cached
func (dl *DatasetLoaderImpl) downloadDataset(ctx context.Context, datasetKey, url string) (string, error) {
	filePath := filepath.Join(dl.cacheDir, datasetKey+".json")

	// Check if file already exists and is recent (less than 7 days old)
	if stat, err := os.Stat(filePath); err == nil {
		if time.Since(stat.ModTime()) < 7*24*time.Hour {
			return filePath, nil
		}
	}

	// Download file
	fmt.Printf("Downloading dataset %s from %s...\n", datasetKey, url)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := dl.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download dataset: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Warning: Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download dataset: HTTP %d", resp.StatusCode)
	}

	// Create temporary file
	tempFile := filePath + ".tmp"
	file, err := os.Create(tempFile)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Warning: Failed to close temp file: %v", err)
		}
	}()

	// Copy data
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		if removeErr := os.Remove(tempFile); removeErr != nil {
			log.Printf("Warning: Failed to remove temp file after copy error: %v", removeErr)
		}
		return "", fmt.Errorf("failed to write dataset file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempFile, filePath); err != nil {
		if removeErr := os.Remove(tempFile); removeErr != nil {
			log.Printf("Warning: Failed to remove temp file after rename error: %v", removeErr)
		}
		return "", fmt.Errorf("failed to rename dataset file: %w", err)
	}

	fmt.Printf("Dataset %s downloaded successfully\n", datasetKey)
	return filePath, nil
}

// parseInstancesFromFile parses instances from a JSON file
func (dl *DatasetLoaderImpl) parseInstancesFromFile(filePath string) ([]Instance, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Warning: Failed to close file: %v", err)
		}
	}()

	// Try to parse as Hugging Face API response first
	instances, err := dl.parseHuggingFaceResponse(file)
	if err != nil {
		// Reset file position and try JSONL
		if _, err := file.Seek(0, 0); err != nil {
			return nil, fmt.Errorf("failed to reset file position: %w", err)
		}
		instances, err = dl.parseJSONL(file)
		if err != nil {
			// Reset file position and try JSON array
			if _, err := file.Seek(0, 0); err != nil {
				return nil, fmt.Errorf("failed to reset file position: %w", err)
			}
			instances, err = dl.parseJSONArray(file)
			if err != nil {
				return nil, fmt.Errorf("failed to parse as Hugging Face response, JSONL, or JSON array: %w", err)
			}
		}
	}

	return instances, nil
}

// parseJSONL parses instances from JSONL format
func (dl *DatasetLoaderImpl) parseJSONL(reader io.Reader) ([]Instance, error) {
	decoder := json.NewDecoder(reader)
	var instances []Instance

	for {
		var instance Instance
		if err := decoder.Decode(&instance); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		instances = append(instances, instance)
	}

	return instances, nil
}

// parseJSONArray parses instances from JSON array format
func (dl *DatasetLoaderImpl) parseJSONArray(reader io.Reader) ([]Instance, error) {
	var instances []Instance
	decoder := json.NewDecoder(reader)

	if err := decoder.Decode(&instances); err != nil {
		return nil, err
	}

	return instances, nil
}

// HuggingFaceResponse represents the response format from Hugging Face API
type HuggingFaceResponse struct {
	Rows []struct {
		RowIdx int      `json:"row_idx"`
		Row    Instance `json:"row"`
	} `json:"rows"`
}

// parseHuggingFaceResponse parses instances from Hugging Face API response format
func (dl *DatasetLoaderImpl) parseHuggingFaceResponse(reader io.Reader) ([]Instance, error) {
	var response HuggingFaceResponse
	decoder := json.NewDecoder(reader)

	if err := decoder.Decode(&response); err != nil {
		return nil, err
	}

	instances := make([]Instance, len(response.Rows))
	for i, row := range response.Rows {
		instances[i] = row.Row
	}

	return instances, nil
}

// applyFiltering applies filtering options to instances
func (dl *DatasetLoaderImpl) applyFiltering(instances []Instance, config DatasetConfig) []Instance {
	// Apply instance ID filtering
	if len(config.InstanceIDs) > 0 {
		idSet := make(map[string]bool)
		for _, id := range config.InstanceIDs {
			idSet[id] = true
		}

		filtered := make([]Instance, 0)
		for _, instance := range instances {
			if idSet[instance.ID] {
				filtered = append(filtered, instance)
			}
		}
		instances = filtered
	}

	// Apply instance slice filtering
	if len(config.InstanceSlice) == 2 {
		start := config.InstanceSlice[0]
		end := config.InstanceSlice[1]

		if start < 0 {
			start = 0
		}
		if end > len(instances) {
			end = len(instances)
		}
		if start < end {
			instances = instances[start:end]
		}
	}

	// Apply shuffling
	if config.Shuffle {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		r.Shuffle(len(instances), func(i, j int) {
			instances[i], instances[j] = instances[j], instances[i]
		})
	}

	// Apply instance limit
	if config.InstanceLimit > 0 && config.InstanceLimit < len(instances) {
		instances = instances[:config.InstanceLimit]
	}

	return instances
}

// validateSWEBenchConfig validates SWE-bench specific configuration
func (dl *DatasetLoaderImpl) validateSWEBenchConfig(config DatasetConfig) error {
	if config.Subset == "" {
		return fmt.Errorf("subset is required for swe_bench dataset")
	}

	validSubsets := []string{"lite", "full", "verified"}
	validSubset := false
	for _, subset := range validSubsets {
		if config.Subset == subset {
			validSubset = true
			break
		}
	}
	if !validSubset {
		return fmt.Errorf("invalid subset %s, must be one of: %s", config.Subset, strings.Join(validSubsets, ", "))
	}

	if config.Split == "" {
		return fmt.Errorf("split is required for swe_bench dataset")
	}

	validSplits := []string{"dev", "test", "train"}
	validSplit := false
	for _, split := range validSplits {
		if config.Split == split {
			validSplit = true
			break
		}
	}
	if !validSplit {
		return fmt.Errorf("invalid split %s, must be one of: %s", config.Split, strings.Join(validSplits, ", "))
	}

	// Check if combination exists
	datasetKey := fmt.Sprintf("swe_bench_%s_%s", config.Subset, config.Split)
	if _, exists := dl.downloadURLs[datasetKey]; !exists {
		return fmt.Errorf("dataset combination not available: subset=%s, split=%s", config.Subset, config.Split)
	}

	return nil
}

// validateFileConfig validates file-based dataset configuration
func (dl *DatasetLoaderImpl) validateFileConfig(config DatasetConfig) error {
	if config.FilePath == "" {
		return fmt.Errorf("file_path is required for file dataset")
	}

	if _, err := os.Stat(config.FilePath); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", config.FilePath)
	}

	return nil
}

// validateHuggingFaceConfig validates Hugging Face dataset configuration
func (dl *DatasetLoaderImpl) validateHuggingFaceConfig(config DatasetConfig) error {
	if config.HFDataset == "" {
		return fmt.Errorf("hf_dataset is required for huggingface dataset")
	}

	return nil
}

// GetAvailableDatasets returns information about available datasets
func (dl *DatasetLoaderImpl) GetAvailableDatasets() map[string]interface{} {
	return map[string]interface{}{
		"swe_bench": map[string]interface{}{
			"subsets":     []string{"lite", "full", "verified"},
			"splits":      []string{"dev", "test", "train"},
			"description": "SWE-bench: A benchmark for evaluating large language models on software engineering tasks",
		},
		"file": map[string]interface{}{
			"description": "Load instances from a local JSON or JSONL file",
			"parameters":  []string{"file_path"},
		},
		"huggingface": map[string]interface{}{
			"description": "Load instances from a Hugging Face dataset (not implemented)",
			"parameters":  []string{"hf_dataset"},
		},
	}
}

// ClearCache clears the dataset cache
func (dl *DatasetLoaderImpl) ClearCache() error {
	return os.RemoveAll(dl.cacheDir)
}

// GetCacheInfo returns information about the cache
func (dl *DatasetLoaderImpl) GetCacheInfo() (map[string]interface{}, error) {
	info := make(map[string]interface{})

	// Get cache directory size
	var totalSize int64
	err := filepath.Walk(dl.cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// List cached files
	files, err := os.ReadDir(dl.cacheDir)
	if err != nil {
		return nil, err
	}

	cachedDatasets := make([]string, 0)
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
			datasetName := strings.TrimSuffix(file.Name(), ".json")
			cachedDatasets = append(cachedDatasets, datasetName)
		}
	}

	info["cache_dir"] = dl.cacheDir
	info["total_size_bytes"] = totalSize
	info["total_size_mb"] = float64(totalSize) / (1024 * 1024)
	info["cached_datasets"] = cachedDatasets

	return info, nil
}
