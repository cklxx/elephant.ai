package lark

import (
	"context"
	"fmt"
	"time"

	larktask "github.com/larksuite/oapi-sdk-go/v3/service/task/v2"
)

// BatchCreateTasks creates multiple tasks sequentially, collecting per-task errors.
// On success the corresponding error entry is nil; on failure the task entry is zero-valued.
func (s *TaskService) BatchCreateTasks(ctx context.Context, tasks []CreateTaskRequest, opts ...CallOption) ([]Task, []error) {
	results := make([]Task, len(tasks))
	errs := make([]error, len(tasks))

	for i, req := range tasks {
		t, err := s.CreateTask(ctx, req, opts...)
		if err != nil {
			errs[i] = fmt.Errorf("task[%d] %q: %w", i, req.Summary, err)
			continue
		}
		results[i] = *t
	}
	return results, errs
}

// BatchCompleteTasks marks multiple tasks as completed sequentially by patching
// their completed_at timestamp. Each entry in the returned slice corresponds
// to the taskID at the same index.
func (s *TaskService) BatchCompleteTasks(ctx context.Context, taskIDs []string, opts ...CallOption) []error {
	errs := make([]error, len(taskIDs))
	now := fmt.Sprintf("%d", time.Now().UnixMilli())

	for i, id := range taskIDs {
		if err := s.completeTask(ctx, id, now, opts...); err != nil {
			errs[i] = fmt.Errorf("task[%d] %q: %w", i, id, err)
		}
	}
	return errs
}

// completeTask patches a single task to set its completed_at timestamp.
func (s *TaskService) completeTask(ctx context.Context, taskID, completedAt string, opts ...CallOption) error {
	inputBuilder := larktask.NewInputTaskBuilder().
		CompletedAt(completedAt)

	body := larktask.NewPatchTaskReqBodyBuilder().
		Task(inputBuilder.Build()).
		UpdateFields([]string{"completed_at"}).
		Build()

	patchReq := larktask.NewPatchTaskReqBuilder().
		TaskGuid(taskID).
		Body(body).
		Build()

	resp, err := s.client.Task.V2.Task.Patch(ctx, patchReq, buildOpts(opts)...)
	if err != nil {
		return fmt.Errorf("complete task: %w", err)
	}
	if !resp.Success() {
		return &APIError{Code: resp.Code, Msg: resp.Msg}
	}
	return nil
}
