package larktools

import (
	"strings"
	"testing"
)

func TestChannelDescriptionExpressesRoutingBoundaries(t *testing.T) {
	t.Parallel()

	desc := NewLarkChannel().Definition().Description
	for _, keyword := range []string{
		"send_message",
		"upload_file",
		"history",
		"create_event",
		"query_events",
		"update_event",
		"delete_event",
		"list_tasks",
		"create_task",
		"update_task",
		"delete_task",
	} {
		if !strings.Contains(desc, keyword) {
			t.Fatalf("expected channel description to mention action %q, got %q", keyword, desc)
		}
	}
}
