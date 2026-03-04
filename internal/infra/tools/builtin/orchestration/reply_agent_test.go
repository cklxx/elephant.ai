package orchestration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
)

type replyDispatcherOnly struct{}

func (d *replyDispatcherOnly) Dispatch(context.Context, agent.BackgroundDispatchRequest) error {
	return nil
}
func (d *replyDispatcherOnly) Status([]string) []agent.BackgroundTaskSummary { return nil }
func (d *replyDispatcherOnly) Collect([]string, bool, time.Duration) []agent.BackgroundTaskResult {
	return nil
}

type replyDispatcherWithInjector struct {
	replyDispatcherOnly
	taskID string
	input  string
	err    error
}

func (d *replyDispatcherWithInjector) InjectBackgroundInput(_ context.Context, taskID string, input string) error {
	d.taskID = taskID
	d.input = input
	return d.err
}

type replyDispatcherWithResponder struct {
	replyDispatcherOnly
	resp agent.InputResponse
	err  error
}

func (d *replyDispatcherWithResponder) ReplyExternalInput(_ context.Context, resp agent.InputResponse) error {
	d.resp = resp
	return d.err
}

func TestReplyAgentValidationErrors(t *testing.T) {
	tool := NewReplyAgent()
	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{
			name: "unsupported parameter",
			args: map[string]any{"task_id": "task-1", "x": "1"},
			want: "unsupported parameter",
		},
		{
			name: "missing task id",
			args: map[string]any{},
			want: "missing 'task_id'",
		},
		{
			name: "invalid request id type",
			args: map[string]any{"task_id": "task-1", "request_id": 1},
			want: "missing 'request_id'",
		},
		{
			name: "empty request id",
			args: map[string]any{"task_id": "task-1", "request_id": " "},
			want: "request_id cannot be empty",
		},
		{
			name: "invalid approved type",
			args: map[string]any{"task_id": "task-1", "request_id": "req-1", "approved": "yes"},
			want: "approved must be a boolean",
		},
		{
			name: "invalid option type",
			args: map[string]any{"task_id": "task-1", "request_id": "req-1", "option_id": 9},
			want: "option_id must be a string",
		},
		{
			name: "invalid message type",
			args: map[string]any{"task_id": "task-1", "request_id": "req-1", "message": 9},
			want: "message must be a string",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), ports.ToolCall{
				ID:        "call-1",
				Arguments: tc.args,
			})
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if result == nil || result.Error == nil {
				t.Fatalf("expected tool error, got %+v", result)
			}
			if !strings.Contains(result.Content, tc.want) {
				t.Fatalf("expected error containing %q, got %q", tc.want, result.Content)
			}
		})
	}
}

func TestReplyAgentInjectionPath(t *testing.T) {
	tool := NewReplyAgent()
	call := ports.ToolCall{
		ID: "call-inject",
		Arguments: map[string]any{
			"task_id": " task-1 ",
			"message": " continue ",
		},
	}

	t.Run("dispatcher missing", func(t *testing.T) {
		result, err := tool.Execute(context.Background(), call)
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if result.Error == nil || !strings.Contains(result.Content, "background task dispatch is not available") {
			t.Fatalf("expected missing dispatcher error, got %+v", result)
		}
	})

	t.Run("request id required without message", func(t *testing.T) {
		ctx := agent.WithBackgroundDispatcher(context.Background(), &replyDispatcherOnly{})
		result, err := tool.Execute(ctx, ports.ToolCall{
			ID: "call-empty",
			Arguments: map[string]any{
				"task_id": "task-1",
			},
		})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if result.Error == nil || !strings.Contains(result.Content, "request_id is required unless message injection is provided") {
			t.Fatalf("expected request_id requirement error, got %+v", result)
		}
	})

	t.Run("injector missing", func(t *testing.T) {
		ctx := agent.WithBackgroundDispatcher(context.Background(), &replyDispatcherOnly{})
		result, err := tool.Execute(ctx, call)
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if result.Error == nil || !strings.Contains(result.Content, "background input injector is not available") {
			t.Fatalf("expected injector missing error, got %+v", result)
		}
	})

	t.Run("injector failure", func(t *testing.T) {
		disp := &replyDispatcherWithInjector{err: errors.New("inject failed")}
		ctx := agent.WithBackgroundDispatcher(context.Background(), disp)
		result, err := tool.Execute(ctx, call)
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if result.Error == nil || !strings.Contains(result.Content, "inject failed") {
			t.Fatalf("expected injector failure, got %+v", result)
		}
	})

	t.Run("injector success", func(t *testing.T) {
		disp := &replyDispatcherWithInjector{}
		ctx := agent.WithBackgroundDispatcher(context.Background(), disp)
		result, err := tool.Execute(ctx, call)
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if result.Error != nil {
			t.Fatalf("unexpected error: %s", result.Content)
		}
		if disp.taskID != "task-1" || disp.input != "continue" {
			t.Fatalf("expected trimmed injection payload, got task=%q input=%q", disp.taskID, disp.input)
		}
		if !strings.Contains(result.Content, "Injected input into task") {
			t.Fatalf("unexpected success content: %q", result.Content)
		}
	})
}

func TestReplyAgentExternalInputPath(t *testing.T) {
	tool := NewReplyAgent()
	call := ports.ToolCall{
		ID: "call-reply",
		Arguments: map[string]any{
			"task_id":    " task-1 ",
			"request_id": " req-1 ",
			"approved":   true,
			"option_id":  " option-A ",
			"message":    " details ",
		},
	}

	t.Run("responder missing", func(t *testing.T) {
		ctx := agent.WithBackgroundDispatcher(context.Background(), &replyDispatcherOnly{})
		result, err := tool.Execute(ctx, call)
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if result.Error == nil || !strings.Contains(result.Content, "external input responder is not available") {
			t.Fatalf("expected responder missing error, got %+v", result)
		}
	})

	t.Run("responder failure", func(t *testing.T) {
		disp := &replyDispatcherWithResponder{err: errors.New("reply failed")}
		ctx := agent.WithBackgroundDispatcher(context.Background(), disp)
		result, err := tool.Execute(ctx, call)
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if result.Error == nil || !strings.Contains(result.Content, "reply failed") {
			t.Fatalf("expected responder failure, got %+v", result)
		}
	})

	t.Run("responder success", func(t *testing.T) {
		disp := &replyDispatcherWithResponder{}
		ctx := agent.WithBackgroundDispatcher(context.Background(), disp)
		result, err := tool.Execute(ctx, call)
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		if result.Error != nil {
			t.Fatalf("unexpected error: %s", result.Content)
		}
		if disp.resp.TaskID != "task-1" || disp.resp.RequestID != "req-1" {
			t.Fatalf("expected trimmed identifiers, got %+v", disp.resp)
		}
		if !disp.resp.Approved || disp.resp.OptionID != "option-A" || disp.resp.Text != "details" {
			t.Fatalf("unexpected responder payload: %+v", disp.resp)
		}
		if !strings.Contains(result.Content, "Reply sent for task") {
			t.Fatalf("unexpected success content: %q", result.Content)
		}
	})
}
