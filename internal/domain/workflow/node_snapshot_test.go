package workflow

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClassifyOutputInline(t *testing.T) {
	small := map[string]string{"result": "ok"}
	data, inline, err := ClassifyOutput(small)
	require.NoError(t, err)
	require.True(t, inline)
	require.NotEmpty(t, data)
}

func TestClassifyOutputLarge(t *testing.T) {
	large := strings.Repeat("x", MaxInlineOutputBytes+1)
	data, inline, err := ClassifyOutput(large)
	require.NoError(t, err)
	require.False(t, inline)
	require.Greater(t, len(data), MaxInlineOutputBytes)
}

func TestClassifyOutputExactThreshold(t *testing.T) {
	// A string of exactly MaxInlineOutputBytes-2 characters will serialize to
	// MaxInlineOutputBytes bytes (2 bytes for JSON quotes).
	exact := strings.Repeat("a", MaxInlineOutputBytes-2)
	data, inline, err := ClassifyOutput(exact)
	require.NoError(t, err)
	require.True(t, inline)
	require.Equal(t, MaxInlineOutputBytes, len(data))
}

func TestClassifyOutputMarshalError(t *testing.T) {
	// Channels cannot be marshaled to JSON.
	ch := make(chan int)
	_, _, err := ClassifyOutput(ch)
	require.Error(t, err)
}

func TestTruncateOutputSmall(t *testing.T) {
	small := map[string]string{"ok": "true"}
	result := TruncateOutputForSnapshot(small)
	require.NotContains(t, result, "...[truncated]")
}

func TestTruncateOutputLarge(t *testing.T) {
	large := strings.Repeat("x", MaxInlineOutputBytes+500)
	result := TruncateOutputForSnapshot(large)
	require.Contains(t, result, "...[truncated]")
	// The prefix before the truncation marker must be exactly MaxInlineOutputBytes.
	idx := strings.Index(result, "...[truncated]")
	require.Equal(t, MaxInlineOutputBytes, idx)
}

func TestTruncateOutputMarshalError(t *testing.T) {
	ch := make(chan int)
	result := TruncateOutputForSnapshot(ch)
	require.Equal(t, "[marshal error]", result)
}

func TestSnapshotGovernanceTruncatesLargeOutput(t *testing.T) {
	node := NewNode("big-node", nil, nil)
	_, err := node.Start()
	require.NoError(t, err)

	largeOutput := map[string]string{
		"data": strings.Repeat("z", MaxInlineOutputBytes+1000),
	}
	snapshot, err := node.CompleteSuccess(largeOutput)
	require.NoError(t, err)
	require.Equal(t, NodeStatusSucceeded, snapshot.Status)

	// Output should be truncated to a string.
	outputStr, ok := snapshot.Output.(string)
	require.True(t, ok, "large output should be truncated to string")
	require.Contains(t, outputStr, "...[truncated]")

	// OutputRef should be set.
	require.Equal(t, "node:big-node:output", snapshot.OutputRef)
}

func TestSnapshotGovernancePreservesSmallOutput(t *testing.T) {
	node := NewNode("small-node", nil, nil)
	_, err := node.Start()
	require.NoError(t, err)

	smallOutput := map[string]string{"result": "done"}
	snapshot, err := node.CompleteSuccess(smallOutput)
	require.NoError(t, err)

	// Output should remain the original map, not a string.
	outputMap, ok := snapshot.Output.(map[string]string)
	require.True(t, ok, "small output should remain original type")
	require.Equal(t, "done", outputMap["result"])

	// OutputRef should be empty.
	require.Empty(t, snapshot.OutputRef)
}

func TestSnapshotGovernanceNilOutput(t *testing.T) {
	node := NewNode("nil-node", nil, nil)
	_, err := node.Start()
	require.NoError(t, err)

	snapshot, err := node.CompleteSuccess(nil)
	require.NoError(t, err)
	require.Nil(t, snapshot.Output)
	require.Empty(t, snapshot.OutputRef)
}
