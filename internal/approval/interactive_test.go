package approval

import (
	"alex/internal/agent/ports"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInteractiveApprover(t *testing.T) {
	approver := NewInteractiveApprover(60*time.Second, false, true)
	assert.NotNil(t, approver)
	assert.Equal(t, 60*time.Second, approver.timeout)
	assert.False(t, approver.autoApprove)
	assert.True(t, approver.colorEnabled)
}

func TestInteractiveApprover_AutoApprove(t *testing.T) {
	approver := NewInteractiveApprover(60*time.Second, true, false)

	request := &ports.ApprovalRequest{
		Operation: "file_edit",
		FilePath:  "/test/file.txt",
		Diff:      "some diff",
		Summary:   "+5 lines, -2 lines",
	}

	ctx := context.Background()
	response, err := approver.RequestApproval(ctx, request)
	require.NoError(t, err)
	assert.True(t, response.Approved)
	assert.Equal(t, "approve", response.Action)
}

func TestInteractiveApprover_RequestAutoApprove(t *testing.T) {
	approver := NewInteractiveApprover(60*time.Second, false, false)

	request := &ports.ApprovalRequest{
		Operation:   "file_edit",
		FilePath:    "/test/file.txt",
		Diff:        "some diff",
		Summary:     "+5 lines, -2 lines",
		AutoApprove: true,
	}

	ctx := context.Background()
	response, err := approver.RequestApproval(ctx, request)
	require.NoError(t, err)
	assert.True(t, response.Approved)
	assert.Equal(t, "approve", response.Action)
}

func TestInteractiveApprover_Colorize(t *testing.T) {
	tests := []struct {
		name         string
		colorEnabled bool
		text         string
	}{
		{
			name:         "with color enabled",
			colorEnabled: true,
			text:         "test",
		},
		{
			name:         "with color disabled",
			colorEnabled: false,
			text:         "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			approver := NewInteractiveApprover(60*time.Second, false, tt.colorEnabled)
			result := approver.colorize(tt.text)
			assert.NotEmpty(t, result)
			if tt.colorEnabled {
				// With color, result might be longer due to ANSI codes
				assert.Contains(t, result, tt.text)
			} else {
				// Without color, result should be exactly the same
				assert.Equal(t, tt.text, result)
			}
		})
	}
}

func TestNoOpApprover(t *testing.T) {
	approver := NewNoOpApprover()
	assert.NotNil(t, approver)

	request := &ports.ApprovalRequest{
		Operation: "file_edit",
		FilePath:  "/test/file.txt",
		Diff:      "some diff",
		Summary:   "+5 lines, -2 lines",
	}

	ctx := context.Background()
	response, err := approver.RequestApproval(ctx, request)
	require.NoError(t, err)
	assert.True(t, response.Approved)
	assert.Equal(t, "approve", response.Action)
}

func TestInteractiveApprover_DisplayDiff(t *testing.T) {
	approver := NewInteractiveApprover(60*time.Second, false, false)

	request := &ports.ApprovalRequest{
		Operation: "file_edit",
		FilePath:  "/test/file.txt",
		Diff:      "--- a/file.txt\n+++ b/file.txt\n@@ -1,1 +1,1 @@\n-old\n+new",
		Summary:   "+1 lines, -1 lines",
	}

	// This test just ensures displayDiff doesn't panic
	// We can't easily test terminal output without mocking
	assert.NotPanics(t, func() {
		approver.displayDiff(request)
	})
}

// Mock approver for testing
type MockApprover struct {
	Response *ports.ApprovalResponse
	Error    error
}

func (m *MockApprover) RequestApproval(ctx context.Context, request *ports.ApprovalRequest) (*ports.ApprovalResponse, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return m.Response, nil
}

func TestMockApprover(t *testing.T) {
	mockApprover := &MockApprover{
		Response: &ports.ApprovalResponse{
			Approved: true,
			Action:   "approve",
			Message:  "Mocked approval",
		},
	}

	request := &ports.ApprovalRequest{
		Operation: "file_edit",
		FilePath:  "/test/file.txt",
	}

	ctx := context.Background()
	response, err := mockApprover.RequestApproval(ctx, request)
	require.NoError(t, err)
	assert.True(t, response.Approved)
	assert.Equal(t, "approve", response.Action)
}
