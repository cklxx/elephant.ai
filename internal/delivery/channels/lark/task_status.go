package lark

import (
	"strings"

	"alex/internal/shared/utils"
)

const (
	taskStatusPending      = "pending"
	taskStatusRunning      = "running"
	taskStatusWaitingInput = "waiting_input"
	taskStatusCompleted    = "completed"
	taskStatusFailed       = "failed"
	taskStatusCancelled    = "cancelled"
)

// normalizeTaskStatus maps status aliases to canonical task status values.
func normalizeTaskStatus(status string) string {
	switch utils.TrimLower(status) {
	case taskStatusPending:
		return taskStatusPending
	case taskStatusRunning:
		return taskStatusRunning
	case taskStatusWaitingInput:
		return taskStatusWaitingInput
	case "done", "success", "succeeded", taskStatusCompleted:
		return taskStatusCompleted
	case "error", "errored", taskStatusFailed:
		return taskStatusFailed
	case "canceled", taskStatusCancelled:
		return taskStatusCancelled
	default:
		return utils.TrimLower(status)
	}
}

func isTerminalTaskStatus(status string) bool {
	switch normalizeTaskStatus(status) {
	case taskStatusCompleted, taskStatusFailed, taskStatusCancelled:
		return true
	default:
		return false
	}
}

func isActiveTaskStatus(status string) bool {
	switch normalizeTaskStatus(status) {
	case taskStatusPending, taskStatusRunning, taskStatusWaitingInput:
		return true
	default:
		return false
	}
}

// normalizeCompletionTaskStatus ensures completion events always produce a terminal status.
func normalizeCompletionTaskStatus(status, errText string) string {
	normalized := normalizeTaskStatus(status)
	if isTerminalTaskStatus(normalized) {
		return normalized
	}
	if strings.TrimSpace(errText) != "" {
		return taskStatusFailed
	}
	return taskStatusCompleted
}
