// Package ports defines the port interfaces for external work item providers.
// Adapters in internal/infra/workitems/{jira,linear}/ implement these
// interfaces for specific project management systems.
package ports

import (
	"context"

	"alex/internal/domain/workitem"
)

// ProviderIssuePage is a paginated result of work items from a provider.
type ProviderIssuePage struct {
	Items      []*workitem.WorkItem `json:"items"`
	Total      int                  `json:"total"`
	HasMore    bool                 `json:"has_more"`
	NextCursor string               `json:"next_cursor,omitempty"`
}

// ProviderCommentPage is a paginated result of comments from a provider.
type ProviderCommentPage struct {
	Comments   []*workitem.Comment `json:"comments"`
	Total      int                 `json:"total"`
	HasMore    bool                `json:"has_more"`
	NextCursor string              `json:"next_cursor,omitempty"`
}

// ProviderStatusChangePage is a paginated result of status changes from a provider.
type ProviderStatusChangePage struct {
	Changes    []*workitem.StatusChange `json:"changes"`
	Total      int                      `json:"total"`
	HasMore    bool                     `json:"has_more"`
	NextCursor string                   `json:"next_cursor,omitempty"`
}

// IssueQuery specifies criteria for fetching work items from a provider API.
type IssueQuery struct {
	WorkspaceID  string   `json:"workspace_id"`
	ProjectIDs   []string `json:"project_ids,omitempty"`
	UpdatedAfter string   `json:"updated_after,omitempty"` // RFC3339
	Cursor       string   `json:"cursor,omitempty"`
	Limit        int      `json:"limit,omitempty"`
}

// CommentQuery specifies criteria for fetching comments from a provider API.
type CommentQuery struct {
	WorkspaceID string `json:"workspace_id"`
	WorkItemID  string `json:"work_item_id"`
	Cursor      string `json:"cursor,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

// StatusChangeQuery specifies criteria for fetching status changes from a provider API.
type StatusChangeQuery struct {
	WorkspaceID string `json:"workspace_id"`
	WorkItemID  string `json:"work_item_id"`
	Cursor      string `json:"cursor,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

// WorkspaceRef describes a workspace available to the authenticated user.
type WorkspaceRef struct {
	Provider    workitem.Provider `json:"provider"`
	WorkspaceID string           `json:"workspace_id"`
	Name        string           `json:"name"`
	URL         string           `json:"url"`
}

// WorkItemReader abstracts read access to an external project management
// provider. Implementations must be safe for concurrent use.
type WorkItemReader interface {
	// Provider returns the provider identifier (e.g. "jira", "linear").
	Provider() workitem.Provider

	// ListWorkItems returns a paginated list of work items matching the query.
	ListWorkItems(ctx context.Context, q IssueQuery) (ProviderIssuePage, error)

	// GetWorkItem returns a single work item by its provider-native ID.
	GetWorkItem(ctx context.Context, workspaceID, workItemID string) (*workitem.WorkItem, error)

	// ListComments returns paginated comments for a work item.
	ListComments(ctx context.Context, q CommentQuery) (ProviderCommentPage, error)

	// ListStatusChanges returns paginated status changes (changelog) for a work item.
	ListStatusChanges(ctx context.Context, q StatusChangeQuery) (ProviderStatusChangePage, error)

	// ResolveWorkspaces returns the workspaces accessible to the authenticated user.
	ResolveWorkspaces(ctx context.Context) ([]WorkspaceRef, error)
}
