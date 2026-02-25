package lark

import (
	"context"
	"fmt"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larktask "github.com/larksuite/oapi-sdk-go/v3/service/task/v2"
)

// TaskService provides typed access to Lark Task APIs (v2).
type TaskService struct {
	client *lark.Client
}

// Task is a simplified view of a Lark task.
type Task struct {
	TaskID    string
	Summary   string
	DueTime   time.Time
	Completed bool
	Creator   string
}

// ListTasksRequest defines parameters for listing tasks.
type ListTasksRequest struct {
	PageSize  int
	PageToken string
}

// ListTasksResponse contains paginated tasks.
type ListTasksResponse struct {
	Tasks     []Task
	PageToken string
	HasMore   bool
}

// ListTasks returns tasks for the authenticated user.
func (s *TaskService) ListTasks(ctx context.Context, req ListTasksRequest, opts ...CallOption) (*ListTasksResponse, error) {
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}

	builder := larktask.NewListTaskReqBuilder().
		PageSize(pageSize)

	if req.PageToken != "" {
		builder.PageToken(req.PageToken)
	}

	resp, err := s.client.Task.V2.Task.List(ctx, builder.Build(), buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	tasks := make([]Task, 0, len(resp.Data.Items))
	for _, item := range resp.Data.Items {
		tasks = append(tasks, parseLarkTask(item))
	}

	var pageToken string
	var hasMore bool
	if resp.Data.PageToken != nil {
		pageToken = *resp.Data.PageToken
	}
	if resp.Data.HasMore != nil {
		hasMore = *resp.Data.HasMore
	}

	return &ListTasksResponse{
		Tasks:     tasks,
		PageToken: pageToken,
		HasMore:   hasMore,
	}, nil
}

// CreateTaskRequest defines parameters for creating a task.
type CreateTaskRequest struct {
	Summary string
	DueTime *time.Time
}

// CreateTask creates a new task.
func (s *TaskService) CreateTask(ctx context.Context, req CreateTaskRequest, opts ...CallOption) (*Task, error) {
	inputBuilder := larktask.NewInputTaskBuilder().
		Summary(req.Summary)

	if req.DueTime != nil {
		inputBuilder.Due(larktask.NewDueBuilder().
			Timestamp(fmt.Sprintf("%d", req.DueTime.Unix())).
			IsAllDay(false).
			Build())
	}

	createReq := larktask.NewCreateTaskReqBuilder().
		InputTask(inputBuilder.Build()).
		Build()

	resp, err := s.client.Task.V2.Task.Create(ctx, createReq, buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	t := parseLarkTask(resp.Data.Task)
	return &t, nil
}

// PatchTaskRequest defines parameters for updating a task.
type PatchTaskRequest struct {
	TaskID  string
	Summary *string
	DueTime *time.Time
}

// PatchTask updates an existing task. Only non-nil fields are patched.
func (s *TaskService) PatchTask(ctx context.Context, req PatchTaskRequest, opts ...CallOption) (*Task, error) {
	inputBuilder := larktask.NewInputTaskBuilder()
	var updateFields []string

	if req.Summary != nil {
		inputBuilder.Summary(*req.Summary)
		updateFields = append(updateFields, "summary")
	}
	if req.DueTime != nil {
		inputBuilder.Due(larktask.NewDueBuilder().
			Timestamp(fmt.Sprintf("%d", req.DueTime.Unix())).
			IsAllDay(false).
			Build())
		updateFields = append(updateFields, "due")
	}

	body := larktask.NewPatchTaskReqBodyBuilder().
		Task(inputBuilder.Build()).
		UpdateFields(updateFields).
		Build()

	patchReq := larktask.NewPatchTaskReqBuilder().
		TaskGuid(req.TaskID).
		Body(body).
		Build()

	resp, err := s.client.Task.V2.Task.Patch(ctx, patchReq, buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("patch task: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	t := parseLarkTask(resp.Data.Task)
	return &t, nil
}

// DeleteTask deletes a task.
func (s *TaskService) DeleteTask(ctx context.Context, taskID string, opts ...CallOption) error {
	req := larktask.NewDeleteTaskReqBuilder().
		TaskGuid(taskID).
		Build()

	resp, err := s.client.Task.V2.Task.Delete(ctx, req, buildOpts(opts)...)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	if !resp.Success() {
		return &APIError{Code: resp.Code, Msg: resp.Msg}
	}
	return nil
}

// --- helpers ---

func parseLarkTask(item *larktask.Task) Task {
	if item == nil {
		return Task{}
	}
	t := Task{}
	if item.Guid != nil {
		t.TaskID = *item.Guid
	}
	if item.Summary != nil {
		t.Summary = *item.Summary
	}
	if item.Due != nil && item.Due.Timestamp != nil {
		t.DueTime = parseTimestamp(*item.Due.Timestamp)
	}
	if item.CompletedAt != nil && *item.CompletedAt != "" && *item.CompletedAt != "0" {
		t.Completed = true
	}
	if item.Creator != nil && item.Creator.Id != nil {
		t.Creator = *item.Creator.Id
	}
	return t
}
