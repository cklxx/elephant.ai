// Package jira implements a Jira Cloud REST API adapter for the workitem
// ProviderReader port. It uses the v3 REST API with either API-token or
// OAuth 2.0 3LO authentication.
package jira

import (
	"context"
	"fmt"
	"net/http"

	"alex/internal/domain/workitem"
	"alex/internal/domain/workitem/ports"
)

// Config holds Jira Cloud connection settings.
type Config struct {
	BaseURL  string   `yaml:"base_url"`
	Email    string   `yaml:"email"`
	APIToken string   `yaml:"api_token"`
	Projects []string `yaml:"projects"`
}

// Provider implements ports.WorkItemReader for Jira Cloud REST API v3.
type Provider struct {
	cfg    Config
	client *http.Client
}

// NewProvider creates a Jira provider with the given config and HTTP client.
// The client should have appropriate timeouts configured.
func NewProvider(cfg Config, client *http.Client) *Provider {
	if client == nil {
		client = &http.Client{}
	}
	return &Provider{cfg: cfg, client: client}
}

// Provider returns the provider identifier.
func (p *Provider) Provider() workitem.Provider {
	return workitem.ProviderJira
}

// ListWorkItems queries Jira issues via POST /rest/api/3/search/jql.
// TODO: implement real API call with JQL query, field selection, and pagination.
func (p *Provider) ListWorkItems(_ context.Context, q ports.IssueQuery) (ports.ProviderIssuePage, error) {
	_ = q
	// Stub: real implementation will POST to /rest/api/3/search/jql with:
	//   - JQL: project IN (...) AND updated >= "cursor" ORDER BY updated ASC
	//   - fields: summary, description, status, assignee, reporter, priority, labels, created, updated
	//   - maxResults: q.Limit
	//   - startAt: derived from q.Cursor
	return ports.ProviderIssuePage{}, nil
}

// GetWorkItem fetches a single issue via GET /rest/api/3/issue/{idOrKey}.
// TODO: implement real API call with field selection.
func (p *Provider) GetWorkItem(_ context.Context, workspaceID, workItemID string) (*workitem.WorkItem, error) {
	_ = workspaceID
	_ = workItemID
	// Stub: real implementation will GET /rest/api/3/issue/{workItemID}
	// with fields: summary, description, status, assignee, reporter, priority, labels, created, updated
	return nil, fmt.Errorf("jira: GetWorkItem not yet implemented")
}

// ListComments fetches comments via GET /rest/api/3/issue/{idOrKey}/comment.
// TODO: implement real API call with pagination.
func (p *Provider) ListComments(_ context.Context, q ports.CommentQuery) (ports.ProviderCommentPage, error) {
	_ = q
	// Stub: real implementation will GET /rest/api/3/issue/{q.WorkItemID}/comment
	// with startAt/maxResults pagination
	return ports.ProviderCommentPage{}, nil
}

// ListStatusChanges fetches changelog via POST /rest/api/3/changelog/bulkfetch.
// TODO: implement real API call filtering for status field changes.
func (p *Provider) ListStatusChanges(_ context.Context, q ports.StatusChangeQuery) (ports.ProviderStatusChangePage, error) {
	_ = q
	// Stub: real implementation will use the changelog API
	// filtering for items where field == "status"
	return ports.ProviderStatusChangePage{}, nil
}

// ResolveWorkspaces returns accessible Jira Cloud sites via the
// accessible-resources endpoint. Requires OAuth 2.0 3LO authentication.
// TODO: implement real API call.
func (p *Provider) ResolveWorkspaces(_ context.Context) ([]ports.WorkspaceRef, error) {
	// Stub: real implementation will GET https://api.atlassian.com/oauth/token/accessible-resources
	return nil, fmt.Errorf("jira: ResolveWorkspaces not yet implemented")
}

// apiURL builds a full API URL for the configured Jira Cloud instance.
func (p *Provider) apiURL(path string) string {
	base := p.cfg.BaseURL
	if base == "" {
		base = "https://api.atlassian.com"
	}
	return base + path
}
