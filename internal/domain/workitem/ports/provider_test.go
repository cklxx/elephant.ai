package ports_test

import (
	"context"
	"testing"
	"time"

	"alex/internal/domain/workitem"
	"alex/internal/domain/workitem/ports"
	"alex/internal/infra/workitems/jira"
	"alex/internal/infra/workitems/linear"
)

// TestJiraProviderImplementsInterface verifies the Jira provider satisfies
// the WorkItemReader interface at compile time and returns correct provider ID.
func TestJiraProviderImplementsInterface(t *testing.T) {
	var reader ports.WorkItemReader = jira.NewProvider(jira.Config{}, nil)
	if got := reader.Provider(); got != workitem.ProviderJira {
		t.Errorf("Provider() = %q, want %q", got, workitem.ProviderJira)
	}
}

// TestLinearProviderImplementsInterface verifies the Linear provider satisfies
// the WorkItemReader interface at compile time and returns correct provider ID.
func TestLinearProviderImplementsInterface(t *testing.T) {
	var reader ports.WorkItemReader = linear.NewProvider(linear.Config{}, nil)
	if got := reader.Provider(); got != workitem.ProviderLinear {
		t.Errorf("Provider() = %q, want %q", got, workitem.ProviderLinear)
	}
}

// TestJiraProviderStubBehavior verifies stub methods return empty results without error.
func TestJiraProviderStubBehavior(t *testing.T) {
	p := jira.NewProvider(jira.Config{BaseURL: "https://test.atlassian.net"}, nil)
	ctx := context.Background()

	t.Run("ListWorkItems returns empty page", func(t *testing.T) {
		page, err := p.ListWorkItems(ctx, ports.IssueQuery{WorkspaceID: "ws1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(page.Items) != 0 {
			t.Errorf("expected 0 items, got %d", len(page.Items))
		}
	})

	t.Run("GetWorkItem returns not-implemented", func(t *testing.T) {
		_, err := p.GetWorkItem(ctx, "ws1", "PROJ-1")
		if err == nil {
			t.Fatal("expected error for stub GetWorkItem")
		}
	})

	t.Run("ListComments returns empty page", func(t *testing.T) {
		page, err := p.ListComments(ctx, ports.CommentQuery{WorkItemID: "PROJ-1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(page.Comments) != 0 {
			t.Errorf("expected 0 comments, got %d", len(page.Comments))
		}
	})

	t.Run("ListStatusChanges returns empty page", func(t *testing.T) {
		page, err := p.ListStatusChanges(ctx, ports.StatusChangeQuery{WorkItemID: "PROJ-1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(page.Changes) != 0 {
			t.Errorf("expected 0 changes, got %d", len(page.Changes))
		}
	})

	t.Run("ResolveWorkspaces returns not-implemented", func(t *testing.T) {
		_, err := p.ResolveWorkspaces(ctx)
		if err == nil {
			t.Fatal("expected error for stub ResolveWorkspaces")
		}
	})
}

// TestLinearProviderStubBehavior verifies stub methods return empty results without error.
func TestLinearProviderStubBehavior(t *testing.T) {
	p := linear.NewProvider(linear.Config{APIKey: "lin_test"}, nil)
	ctx := context.Background()

	t.Run("ListWorkItems returns empty page", func(t *testing.T) {
		page, err := p.ListWorkItems(ctx, ports.IssueQuery{WorkspaceID: "ws1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(page.Items) != 0 {
			t.Errorf("expected 0 items, got %d", len(page.Items))
		}
	})

	t.Run("GetWorkItem returns not-implemented", func(t *testing.T) {
		_, err := p.GetWorkItem(ctx, "ws1", "abc-123")
		if err == nil {
			t.Fatal("expected error for stub GetWorkItem")
		}
	})

	t.Run("ListComments returns empty page", func(t *testing.T) {
		page, err := p.ListComments(ctx, ports.CommentQuery{WorkItemID: "abc-123"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(page.Comments) != 0 {
			t.Errorf("expected 0 comments, got %d", len(page.Comments))
		}
	})

	t.Run("ListStatusChanges returns empty page", func(t *testing.T) {
		page, err := p.ListStatusChanges(ctx, ports.StatusChangeQuery{WorkItemID: "abc-123"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(page.Changes) != 0 {
			t.Errorf("expected 0 changes, got %d", len(page.Changes))
		}
	})

	t.Run("ResolveWorkspaces returns not-implemented", func(t *testing.T) {
		_, err := p.ResolveWorkspaces(ctx)
		if err == nil {
			t.Fatal("expected error for stub ResolveWorkspaces")
		}
	})
}

// TestIssueFilterDefaults verifies zero-value filter is usable.
func TestIssueFilterDefaults(t *testing.T) {
	f := workitem.IssueFilter{}
	if f.Provider != "" {
		t.Errorf("expected empty provider, got %q", f.Provider)
	}
	if f.Limit != 0 {
		t.Errorf("expected 0 limit, got %d", f.Limit)
	}
	if !f.UpdatedAfter.IsZero() {
		t.Errorf("expected zero UpdatedAfter, got %v", f.UpdatedAfter)
	}
}

// TestWorkItemFields verifies a fully-populated WorkItem can be constructed.
func TestWorkItemFields(t *testing.T) {
	now := time.Now()
	started := now.Add(-1 * time.Hour)
	item := workitem.WorkItem{
		ID:          "PROJ-42",
		Provider:    workitem.ProviderJira,
		WorkspaceID: "cloud-123",
		ProjectID:   "10001",
		ProjectKey:  "PROJ",
		Key:         "PROJ-42",
		Title:       "Implement feature X",
		Description: "Detailed description",
		URL:         "https://test.atlassian.net/browse/PROJ-42",
		Assignee:    workitem.PersonRef{ExternalID: "user1", DisplayName: "Alice"},
		Reporter:    workitem.PersonRef{ExternalID: "user2", DisplayName: "Bob"},
		StatusID:    "3",
		StatusName:  "In Progress",
		StatusClass: workitem.StatusInProgress,
		Priority:    "High",
		Labels:      []string{"backend", "phase2"},
		IsBlocked:   false,
		CreatedAt:   now,
		UpdatedAt:   now,
		StartedAt:   &started,
		Metadata:    map[string]string{"issue_type": "Story"},
	}

	if item.StatusClass.IsTerminal() {
		t.Error("in_progress should not be terminal")
	}
	if item.Assignee.DisplayName != "Alice" {
		t.Errorf("expected assignee Alice, got %q", item.Assignee.DisplayName)
	}
	if len(item.Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(item.Labels))
	}
}

// TestCommentFields verifies a fully-populated Comment can be constructed.
func TestCommentFields(t *testing.T) {
	now := time.Now()
	c := workitem.Comment{
		ID:          "comment-1",
		Provider:    workitem.ProviderLinear,
		WorkspaceID: "org-1",
		WorkItemID:  "issue-1",
		Author:      workitem.PersonRef{ExternalID: "u1", DisplayName: "Charlie"},
		BodyText:    "This is blocked on API review",
		BodyRaw:     "<p>This is blocked on API review</p>",
		IsSystem:    false,
		Visibility:  "public",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if c.IsSystem {
		t.Error("expected non-system comment")
	}
	if c.Author.DisplayName != "Charlie" {
		t.Errorf("expected author Charlie, got %q", c.Author.DisplayName)
	}
}

// TestStatusChangeFields verifies a fully-populated StatusChange can be constructed.
func TestStatusChangeFields(t *testing.T) {
	now := time.Now()
	sc := workitem.StatusChange{
		ID:             "sc-1",
		Provider:       workitem.ProviderJira,
		WorkspaceID:    "cloud-123",
		WorkItemID:     "PROJ-42",
		FromStatusID:   "1",
		FromStatusName: "To Do",
		ToStatusID:     "3",
		ToStatusName:   "In Progress",
		ChangedBy:      workitem.PersonRef{ExternalID: "user1", DisplayName: "Alice"},
		ChangedAt:      now,
		Source:         "changelog",
	}

	if sc.Source != "changelog" {
		t.Errorf("expected source=changelog, got %q", sc.Source)
	}
	if sc.FromStatusName != "To Do" {
		t.Errorf("expected from=To Do, got %q", sc.FromStatusName)
	}
}
