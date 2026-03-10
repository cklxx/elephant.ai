// Package linear implements a Linear GraphQL API adapter for the workitem
// ProviderReader port. It supports both personal API key and OAuth 2.0
// authentication modes.
package linear

import (
	"context"
	"fmt"
	"net/http"

	"alex/internal/domain/workitem"
	"alex/internal/domain/workitem/ports"
)

// graphQLEndpoint is the Linear GraphQL API endpoint.
const graphQLEndpoint = "https://api.linear.app/graphql"

// Config holds Linear connection settings.
type Config struct {
	APIKey  string   `yaml:"api_key"`
	TeamIDs []string `yaml:"team_ids"`
}

// Provider implements ports.WorkItemReader for the Linear GraphQL API.
type Provider struct {
	cfg    Config
	client *http.Client
}

// NewProvider creates a Linear provider with the given config and HTTP client.
// The client should have appropriate timeouts configured.
func NewProvider(cfg Config, client *http.Client) *Provider {
	if client == nil {
		client = &http.Client{}
	}
	return &Provider{cfg: cfg, client: client}
}

// Provider returns the provider identifier.
func (p *Provider) Provider() workitem.Provider {
	return workitem.ProviderLinear
}

// ListWorkItems queries Linear issues via GraphQL with team and updatedAt filters.
// TODO: implement real GraphQL query with pagination.
func (p *Provider) ListWorkItems(_ context.Context, q ports.IssueQuery) (ports.ProviderIssuePage, error) {
	_ = q
	// Stub: real implementation will execute GraphQL query:
	//   query { issues(filter: { team: { id: { in: [...] } }, updatedAt: { gte: "..." } },
	//     first: N, after: cursor) { nodes { id title description state { name type } ... } pageInfo { ... } } }
	return ports.ProviderIssuePage{}, nil
}

// GetWorkItem fetches a single Linear issue by ID via GraphQL.
// TODO: implement real GraphQL query.
func (p *Provider) GetWorkItem(_ context.Context, workspaceID, workItemID string) (*workitem.WorkItem, error) {
	_ = workspaceID
	_ = workItemID
	// Stub: real implementation will execute: query { issue(id: "...") { ... } }
	return nil, fmt.Errorf("linear: GetWorkItem not yet implemented")
}

// ListComments fetches comments for a Linear issue via GraphQL.
// TODO: implement real GraphQL query with pagination.
func (p *Provider) ListComments(_ context.Context, q ports.CommentQuery) (ports.ProviderCommentPage, error) {
	_ = q
	// Stub: real implementation will execute:
	//   query { issue(id: "...") { comments(first: N, after: cursor) { nodes { ... } pageInfo { ... } } } }
	return ports.ProviderCommentPage{}, nil
}

// ListStatusChanges synthesizes status changes from Linear issue history.
// Linear does not have a dedicated changelog API; status changes are derived
// from webhook previous-value payloads or polling-diff comparisons.
// TODO: implement status change tracking.
func (p *Provider) ListStatusChanges(_ context.Context, q ports.StatusChangeQuery) (ports.ProviderStatusChangePage, error) {
	_ = q
	// Stub: Linear status changes are synthesized from:
	// 1. Webhook payloads with "previous" values for state changes
	// 2. Polling-diff comparison of cached vs current state
	return ports.ProviderStatusChangePage{}, nil
}

// ResolveWorkspaces returns the Linear organization visible to the
// authenticated user.
// TODO: implement real GraphQL query.
func (p *Provider) ResolveWorkspaces(_ context.Context) ([]ports.WorkspaceRef, error) {
	// Stub: real implementation will execute: query { organization { id name urlKey } }
	return nil, fmt.Errorf("linear: ResolveWorkspaces not yet implemented")
}
