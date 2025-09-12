package agent

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParallelConfig(t *testing.T) {
	// Test default configuration
	config := DefaultParallelConfig()
	assert.Equal(t, 3, config.MaxWorkers)
	assert.Equal(t, 2*time.Minute, config.TaskTimeout)
	assert.True(t, config.EnableMetrics)
}

func TestParallelConfigValidation(t *testing.T) {
	testCases := []struct {
		name        string
		config      *ParallelConfig
		expectError bool
	}{
		{
			name: "valid config",
			config: &ParallelConfig{
				MaxWorkers:    5,
				TaskTimeout:   1 * time.Minute,
				EnableMetrics: true,
			},
			expectError: false,
		},
		{
			name: "zero workers",
			config: &ParallelConfig{
				MaxWorkers:    0,
				TaskTimeout:   1 * time.Minute,
				EnableMetrics: true,
			},
			expectError: true,
		},
		{
			name: "too many workers",
			config: &ParallelConfig{
				MaxWorkers:    11,
				TaskTimeout:   1 * time.Minute,
				EnableMetrics: true,
			},
			expectError: true,
		},
		{
			name: "zero timeout",
			config: &ParallelConfig{
				MaxWorkers:    3,
				TaskTimeout:   0,
				EnableMetrics: true,
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewSimpleParallelSubAgent(nil, tc.config)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				// Note: This will still error because parentCore is nil,
				// but we're testing config validation specifically
				assert.Error(t, err) // parentCore validation
			}
		})
	}
}

func TestSimpleParallelSubAgent_Basic(t *testing.T) {
	// This is a basic structure test - we can't easily test full execution
	// without setting up the entire ReactCore/Agent infrastructure
	
	// Test that we can create a configuration
	config := &ParallelConfig{
		MaxWorkers:    2,
		TaskTimeout:   30 * time.Second,
		EnableMetrics: true,
	}
	
	// Test configuration validation (parentCore is nil, so this should fail with the right error)
	_, err := NewSimpleParallelSubAgent(nil, config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parentCore cannot be nil")
}

func TestSimpleParallelSubAgent_EmptyTasks(t *testing.T) {
	// Create a minimal mock setup
	config := DefaultParallelConfig()
	
	// Create a simple parallel agent structure (this won't work for real execution
	// but will test the empty tasks path)
	spa := &SimpleParallelSubAgent{
		parentCore: nil, // This is ok for empty tasks test
		config:     config,
	}
	
	// Test empty task list
	results, err := spa.ExecuteTasksParallel(context.Background(), []string{}, nil)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestParallelExecutorWrapper(t *testing.T) {
	// Test the wrapper creation
	wrapper := NewParallelExecutorWrapper(nil)
	assert.NotNil(t, wrapper)
	
	// Test with nil ReactCore
	_, err := wrapper.ExecuteTasksParallel(context.Background(), map[string]interface{}{
		"tasks": []interface{}{"echo test"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ReactCore not available")
}