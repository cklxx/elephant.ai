package analytics

const (
	EventTaskSubmitted             = "task_submitted"
	EventTaskSubmissionFailed      = "task_submission_failed"
	EventTaskRetriedWithoutSession = "task_retried_without_session"
	EventTaskCancelRequested       = "task_cancel_requested"
	EventTaskCancelFailed          = "task_cancel_failed"
	EventTaskExecutionStarted      = "task_execution_started"
	EventTaskExecutionCompleted    = "task_execution_completed"
	EventTaskExecutionFailed       = "task_execution_failed"
	EventTaskExecutionCancelled    = "task_execution_cancelled"
)
