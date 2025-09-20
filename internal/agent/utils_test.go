package agent

import (
	"testing"
	"time"

	"alex/pkg/types"

	"github.com/stretchr/testify/assert"
)

func TestGenerateTaskID(t *testing.T) {
	// Test that generateTaskID creates unique IDs
	id1 := generateTaskID()
	time.Sleep(time.Microsecond) // Ensure different timestamp
	id2 := generateTaskID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)

	// Test ID format
	assert.Contains(t, id1, "task_")
	assert.Contains(t, id2, "task_")
}

func TestGenerateTaskIDFormat(t *testing.T) {
	// Test that ID follows expected format: task_{timestamp}_{random}
	id := generateTaskID()

	assert.True(t, len(id) > 10) // Should be reasonably long
	assert.Contains(t, id, "task_")

	// Should start with task_
	assert.Equal(t, "task_", id[:5])
}

func TestGenerateTaskIDUniqueness(t *testing.T) {
	// Generate multiple IDs quickly to test uniqueness
	ids := make(map[string]bool)
	numIDs := 100

	for i := 0; i < numIDs; i++ {
		id := generateTaskID()
		assert.False(t, ids[id], "Generated duplicate ID: %s", id)
		ids[id] = true
	}

	assert.Len(t, ids, numIDs, "Expected %d unique IDs", numIDs)
}

func TestBuildFinalResult(t *testing.T) {
	// Test buildFinalResult with successful result
	startTime := time.Now().Add(-5 * time.Second)
	taskCtx := &types.ReactTaskContext{
		StartTime:        startTime,
		History:          []types.ReactExecutionStep{
			{Number: 1, Thought: "Thinking about the problem", Action: "think", Observation: "Step completed", Timestamp: time.Now()},
			{Number: 2, Thought: "Taking action", Action: "act", Observation: "Action taken", Timestamp: time.Now()},
		},
		TokensUsed:       100,
		PromptTokens:     60,
		CompletionTokens: 40,
	}

	result := buildFinalResult(taskCtx, "Success message", 0.95, true)

	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, "Success message", result.Answer)
	assert.Equal(t, 0.95, result.Confidence)
	assert.Equal(t, taskCtx.History, result.Steps)
	assert.Equal(t, 100, result.TokensUsed)
	assert.Equal(t, 60, result.PromptTokens)
	assert.Equal(t, 40, result.CompletionTokens)
	assert.Greater(t, result.Duration, time.Duration(0))
	assert.LessOrEqual(t, result.Duration, 10*time.Second)
}

func TestBuildFinalResultFailure(t *testing.T) {
	// Test buildFinalResult with failure result
	startTime := time.Now().Add(-2 * time.Second)
	taskCtx := &types.ReactTaskContext{
		StartTime:        startTime,
		History:          []types.ReactExecutionStep{
			{Number: 1, Thought: "Trying to solve", Action: "think", Observation: "Problem analyzed", Timestamp: time.Now()},
			{Number: 2, Thought: "Action failed", Action: "act", Observation: "Action unsuccessful", Timestamp: time.Now()},
		},
		TokensUsed:       50,
		PromptTokens:     30,
		CompletionTokens: 20,
	}

	result := buildFinalResult(taskCtx, "Failed to complete task", 0.1, false)

	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, "Failed to complete task", result.Answer)
	assert.Equal(t, 0.1, result.Confidence)
	assert.Equal(t, taskCtx.History, result.Steps)
	assert.Equal(t, 50, result.TokensUsed)
	assert.Equal(t, 30, result.PromptTokens)
	assert.Equal(t, 20, result.CompletionTokens)
	assert.Greater(t, result.Duration, time.Duration(0))
}

func TestBuildFinalResultEmptyHistory(t *testing.T) {
	// Test buildFinalResult with empty history
	startTime := time.Now()
	taskCtx := &types.ReactTaskContext{
		StartTime:        startTime,
		History:          []types.ReactExecutionStep{},
		TokensUsed:       0,
		PromptTokens:     0,
		CompletionTokens: 0,
	}

	result := buildFinalResult(taskCtx, "No steps taken", 0.0, false)

	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, "No steps taken", result.Answer)
	assert.Equal(t, 0.0, result.Confidence)
	assert.Empty(t, result.Steps)
	assert.Equal(t, 0, result.TokensUsed)
	assert.Equal(t, 0, result.PromptTokens)
	assert.Equal(t, 0, result.CompletionTokens)
	assert.GreaterOrEqual(t, result.Duration, time.Duration(0))
}

func TestBuildFinalResultDuration(t *testing.T) {
	// Test that duration is calculated correctly
	startTime := time.Now().Add(-3 * time.Second)
	taskCtx := &types.ReactTaskContext{
		StartTime: startTime,
		History:   []types.ReactExecutionStep{},
	}

	result := buildFinalResult(taskCtx, "Duration test", 1.0, true)

	assert.NotNil(t, result)
	assert.Greater(t, result.Duration, 2*time.Second)
	assert.LessOrEqual(t, result.Duration, 5*time.Second)
}

func TestBuildFinalResultNilContext(t *testing.T) {
	// Test that buildFinalResult handles edge cases gracefully
	// This test verifies behavior when context has minimal data

	taskCtx := &types.ReactTaskContext{
		StartTime: time.Now(),
	}

	result := buildFinalResult(taskCtx, "", 0.0, false)

	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Empty(t, result.Answer)
	assert.Equal(t, 0.0, result.Confidence)
	assert.Nil(t, result.Steps) // History is nil, not empty slice
	assert.GreaterOrEqual(t, result.Duration, time.Duration(0))
}

func TestBuildFinalResultHighConfidence(t *testing.T) {
	// Test buildFinalResult with maximum confidence
	taskCtx := &types.ReactTaskContext{
		StartTime: time.Now(),
		History: []types.ReactExecutionStep{
			{Number: 1, Thought: "Perfect solution found", Action: "think", Observation: "Analysis complete", Timestamp: time.Now()},
			{Number: 2, Thought: "Executed flawlessly", Action: "act", Observation: "Action succeeded", Timestamp: time.Now()},
			{Number: 3, Thought: "Confirmed success", Action: "observe", Observation: "Verification complete", Timestamp: time.Now()},
		},
		TokensUsed:       200,
		PromptTokens:     120,
		CompletionTokens: 80,
	}

	result := buildFinalResult(taskCtx, "Perfect result", 1.0, true)

	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, "Perfect result", result.Answer)
	assert.Equal(t, 1.0, result.Confidence)
	assert.Len(t, result.Steps, 3)
	assert.Equal(t, 200, result.TokensUsed)
	assert.Equal(t, 120, result.PromptTokens)
	assert.Equal(t, 80, result.CompletionTokens)
}

// Benchmark tests for performance
func BenchmarkGenerateTaskID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		generateTaskID()
	}
}

func BenchmarkBuildFinalResult(b *testing.B) {
	taskCtx := &types.ReactTaskContext{
		StartTime: time.Now(),
		History: []types.ReactExecutionStep{
			{Number: 1, Thought: "Thinking", Action: "think", Observation: "Thought complete", Timestamp: time.Now()},
			{Number: 2, Thought: "Acting", Action: "act", Observation: "Action complete", Timestamp: time.Now()},
		},
		TokensUsed:       100,
		PromptTokens:     60,
		CompletionTokens: 40,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buildFinalResult(taskCtx, "Test result", 0.8, true)
	}
}