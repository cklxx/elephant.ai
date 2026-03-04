package taskfile

import "testing"

func TestBaseTaskID(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{input: "slow-task", want: "slow-task"},
		{input: "slow-task-retry-1", want: "slow-task"},
		{input: "slow-task-retry-1-retry-2", want: "slow-task"},
		{input: "slow-task-retry-x", want: "slow-task-retry-x"},
		{input: "task", want: "task"},
		{input: "", want: ""},
		{input: "-retry-1", want: "-retry-1"},
	}
	for _, tc := range cases {
		got := BaseTaskID(tc.input)
		if got != tc.want {
			t.Errorf("BaseTaskID(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestExtractRoleID(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{input: "team-planner", want: "planner"},
		{input: "team-planner-debate", want: "planner"},
		{input: "team-planner-retry-2", want: "planner"},
		{input: "team-planner-debate-retry-1", want: "planner-debate"},
		{input: "other", want: ""},
		{input: "", want: ""},
		{input: "team-", want: ""},
		{input: "team-multi-word-role", want: "multi-word-role"},
		{input: "team-multi-word-role-debate", want: "multi-word-role"},
		{input: "team-multi-word-role-retry-1", want: "multi-word-role"},
	}
	for _, tc := range cases {
		got := ExtractRoleID(tc.input)
		if got != tc.want {
			t.Errorf("ExtractRoleID(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
