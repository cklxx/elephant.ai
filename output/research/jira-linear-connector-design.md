# Jira / Linear Read Connector Design

Date: 2026-03-10

Purpose:
- design the Phase 2 read-only Jira / Linear connector that unlocks:
  - Enhanced Blocker Radar
  - Enhanced 1:1 Prep Brief
  - Enhanced Weekly Pulse
  - Scope Change Detection

## Codebase Pattern Read

I reviewed the existing port / adapter patterns first.

Primary local references:
- [internal/domain/task/store.go](/Users/bytedance/code/elephant.ai/internal/domain/task/store.go)
- [internal/app/notification/notification.go](/Users/bytedance/code/elephant.ai/internal/app/notification/notification.go)
- [internal/domain/materialregistry/ports/remote_fetcher.go](/Users/bytedance/code/elephant.ai/internal/domain/materialregistry/ports/remote_fetcher.go)
- [internal/infra/lark/oauth/service.go](/Users/bytedance/code/elephant.ai/internal/infra/lark/oauth/service.go)
- [internal/infra/lark/oauth/token_store.go](/Users/bytedance/code/elephant.ai/internal/infra/lark/oauth/token_store.go)
- [internal/infra/taskstore/local_store.go](/Users/bytedance/code/elephant.ai/internal/infra/taskstore/local_store.go)
- [internal/shared/config/types.go](/Users/bytedance/code/elephant.ai/internal/shared/config/types.go)
- [internal/shared/config/file_config.go](/Users/bytedance/code/elephant.ai/internal/shared/config/file_config.go)
- [internal/app/blocker/radar.go](/Users/bytedance/code/elephant.ai/internal/app/blocker/radar.go)
- [internal/app/prepbrief/brief.go](/Users/bytedance/code/elephant.ai/internal/app/prepbrief/brief.go)
- [internal/app/pulse/weekly.go](/Users/bytedance/code/elephant.ai/internal/app/pulse/weekly.go)

Observed style:
- canonical domain types and store ports live together in `internal/domain/...`
- orchestration belongs in `internal/app/...`
- provider clients, OAuth, token stores, and persistence adapters belong in `internal/infra/...`
- config uses a top-level runtime struct plus a YAML-only mirror file config

Important note:
- there is no repo-wide `internal/ports/` package in this checkout
- the closest explicit `ports` example is [internal/domain/materialregistry/ports/remote_fetcher.go](/Users/bytedance/code/elephant.ai/internal/domain/materialregistry/ports/remote_fetcher.go)
- the dominant style in this repo is domain-first ports, not a global `internal/ports` directory

Design implication:
- the connector should follow the `internal/domain/task` pattern, not invent a new top-level shared `ports` package

## 1. Interface Design

### Architectural Decision

Create one canonical external-work domain and make Jira and Linear provider adapters feed it.

Do not overload [internal/domain/task/store.go](/Users/bytedance/code/elephant.ai/internal/domain/task/store.go):
- `task.Task` is runtime-owned work
- Jira / Linear issues are external system records
- mixing them will corrupt semantics and make ownership harder to reason about

Recommended new package layout:

```text
internal/domain/workitem/
  types.go
  store.go
  sync.go

internal/app/workitems/
  sync_service.go
  query_service.go
  user_mapping.go

internal/infra/workitems/store/
  local_store.go
  local_store_test.go

internal/infra/workitems/jira/
  client.go
  provider.go
  oauth_service.go
  token_store.go
  webhook.go

internal/infra/workitems/linear/
  client.go
  provider.go
  oauth_service.go
  token_store.go
  webhook.go

internal/delivery/server/http/
  workitems_webhooks.go

internal/delivery/server/bootstrap/
  workitems.go
```

Recommended config additions:

```text
internal/shared/config/types.go
internal/shared/config/file_config.go
internal/shared/config/proactive_merge.go
```

Add a new top-level runtime block instead of hiding this under `proactive`:

```go
type RuntimeConfig struct {
	...
	WorkItems WorkItemsConfig `json:"work_items" yaml:"work_items"`
}
```

Reason:
- this is a platform data source, not a single proactive feature
- Blocker Radar, Prep Brief, Weekly Pulse, and Scope Change all consume it

### Canonical Domain Port

```go
package workitem

import (
	"context"
	"time"
)

type Provider string

const (
	ProviderJira   Provider = "jira"
	ProviderLinear Provider = "linear"
)

type StatusClass string

const (
	StatusTodo       StatusClass = "todo"
	StatusInProgress StatusClass = "in_progress"
	StatusBlocked    StatusClass = "blocked"
	StatusDone       StatusClass = "done"
	StatusCancelled  StatusClass = "cancelled"
	StatusUnknown    StatusClass = "unknown"
)

type WorkspaceRef struct {
	Provider    Provider
	WorkspaceID string
	Name        string
	URL         string
}

type PersonRef struct {
	ExternalID  string
	DisplayName string
	Email       string
}

type WorkItem struct {
	ID             string
	Provider       Provider
	WorkspaceID    string
	ProjectID      string
	ProjectKey     string
	Key            string
	Title          string
	Description    string
	URL            string
	Assignee       PersonRef
	Reporter       PersonRef
	StatusID       string
	StatusName     string
	StatusClass    StatusClass
	Priority       string
	Labels         []string
	IsBlocked      bool
	BlockedReason  string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	StartedAt      *time.Time
	CompletedAt    *time.Time
	SourceVersion  string
	Metadata       map[string]string
}

type Comment struct {
	ID          string
	Provider    Provider
	WorkspaceID string
	WorkItemID  string
	Author      PersonRef
	BodyText    string
	BodyRaw     string
	IsSystem    bool
	Visibility  string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

type StatusChange struct {
	ID             string
	Provider       Provider
	WorkspaceID    string
	WorkItemID     string
	FromStatusID   string
	FromStatusName string
	ToStatusID     string
	ToStatusName   string
	ChangedBy      PersonRef
	ChangedAt      time.Time
	Source         string // changelog | webhook_diff | polling_diff
}

type UserBinding struct {
	Provider       Provider
	WorkspaceID    string
	ExternalUserID string
	Email          string
	LarkOpenID     string
	InternalUserID string
	DisplayName    string
}

type SyncCursor struct {
	Provider       Provider
	WorkspaceID    string
	Stream         string
	LastSeenAt     time.Time
	OpaqueCursor   string
	UpdatedAt      time.Time
}

type IssueFilter struct {
	Provider     Provider
	WorkspaceID  string
	ProjectIDs   []string
	AssigneeIDs  []string
	Statuses     []StatusClass
	UpdatedAfter time.Time
	Limit        int
}

type ProviderReader interface {
	Provider() Provider
	ListWorkItems(ctx context.Context, q ProviderIssueQuery) (ProviderIssuePage, error)
	ListComments(ctx context.Context, q ProviderCommentQuery) (ProviderCommentPage, error)
	ListStatusChanges(ctx context.Context, q ProviderStatusChangeQuery) (ProviderStatusChangePage, error)
	GetWorkItem(ctx context.Context, workspaceID, workItemID string) (*WorkItem, error)
	ResolveWorkspace(ctx context.Context) ([]WorkspaceRef, error)
}

type Store interface {
	EnsureSchema(ctx context.Context) error
	UpsertWorkItems(ctx context.Context, items []*WorkItem) error
	UpsertComments(ctx context.Context, comments []*Comment) error
	AppendStatusChanges(ctx context.Context, changes []*StatusChange) error
	UpsertUserBindings(ctx context.Context, bindings []*UserBinding) error
	GetWorkItem(ctx context.Context, provider Provider, workspaceID, workItemID string) (*WorkItem, error)
	ListWorkItems(ctx context.Context, filter IssueFilter) ([]*WorkItem, error)
	ListComments(ctx context.Context, provider Provider, workspaceID, workItemID string, after time.Time) ([]*Comment, error)
	ListStatusChanges(ctx context.Context, provider Provider, workspaceID, workItemID string, after time.Time) ([]*StatusChange, error)
	GetUserBinding(ctx context.Context, provider Provider, workspaceID, externalUserID string) (*UserBinding, error)
	GetCursor(ctx context.Context, provider Provider, workspaceID, stream string) (SyncCursor, error)
	SetCursor(ctx context.Context, provider Provider, workspaceID, stream string, cursor SyncCursor) error
}
```

### App-Layer Orchestration

Recommended services:

- `SyncService`
  - owns polling loops
  - consumes webhook deliveries
  - re-fetches authoritative issue state
  - updates store and cursors

- `QueryService`
  - returns feature-ready read models for:
    - blocker radar
    - prep brief
    - weekly pulse
    - scope change detection

- `UserMappingService`
  - resolves Jira account IDs / Linear user IDs into internal identities
  - isolates mapping from provider clients

### Why This Matches The Existing Style

- [internal/domain/task/store.go](/Users/bytedance/code/elephant.ai/internal/domain/task/store.go) is the strongest precedent for domain-owned types and store interfaces
- [internal/app/notification/notification.go](/Users/bytedance/code/elephant.ai/internal/app/notification/notification.go) shows a small stable interface with swappable adapters
- [internal/infra/lark/oauth/service.go](/Users/bytedance/code/elephant.ai/internal/infra/lark/oauth/service.go) and [internal/infra/lark/oauth/token_store.go](/Users/bytedance/code/elephant.ai/internal/infra/lark/oauth/token_store.go) are the right template for OAuth services and token persistence
- [internal/infra/taskstore/local_store.go](/Users/bytedance/code/elephant.ai/internal/infra/taskstore/local_store.go) is the right template for a file-backed canonical cache if we want the fastest initial slice

## 2. Data Model

### Core Canonical Records

#### `WorkItem`

Required for all four leader features.

Fields that must be normalized:
- identity: `provider`, `workspace_id`, `project_id`, `id`, `key`, `url`
- ownership: `assignee`, `reporter`
- lifecycle: `status_id`, `status_name`, `status_class`, `created_at`, `updated_at`, `started_at`, `completed_at`
- risk signals: `priority`, `labels`, `is_blocked`, `blocked_reason`
- content: `title`, `description`
- audit: `source_version`

#### `Comment`

Needed for:
- “waiting on X” or “blocked on Y” inference
- recent context in prep briefs
- scope-change explanations
- freshness scoring

Required fields:
- `provider`
- `workspace_id`
- `work_item_id`
- `author`
- `body_text`
- `body_raw`
- `is_system`
- `visibility`
- `created_at`
- `updated_at`
- `deleted_at`

#### `StatusChange`

Needed for:
- stale-state detection
- churn / reopen tracking
- scope-change timing
- pulse metrics

Required fields:
- `provider`
- `workspace_id`
- `work_item_id`
- `from_status_id`
- `from_status_name`
- `to_status_id`
- `to_status_name`
- `changed_by`
- `changed_at`
- `source`

#### `UserBinding`

This is not optional.

Without it:
- Blocker Radar cannot route alerts to the right owner
- Prep Brief cannot build “your work” or “this person’s work” views
- Weekly Pulse cannot roll up by person reliably

Required fields:
- `provider`
- `workspace_id`
- `external_user_id`
- `email`
- `lark_open_id`
- `internal_user_id`
- `display_name`

### Raw-Field Preservation

Keep a small provider-specific metadata bag on `WorkItem` and `Comment` for fields we do not normalize yet:

Examples:
- Jira issue type
- Jira story points custom field id/value
- Linear team id
- Linear cycle id
- provider-native URLs

This avoids re-fetches when a Phase 2 feature needs one extra field.

### Storage Shape

For the first implementation, a single local persisted store is acceptable:
- one file-backed canonical cache mirroring [internal/infra/taskstore/local_store.go](/Users/bytedance/code/elephant.ai/internal/infra/taskstore/local_store.go)
- separate maps / indexes by:
  - `(provider, workspace_id, work_item_id)`
  - `(provider, workspace_id, assignee_external_id)`
  - `(provider, workspace_id, updated_at)`

If this grows beyond one team, switch the same `Store` port to SQLite.

## 3. Sync Strategy

Target freshness SLA: `<10 minutes`

### Decision

Use webhook-first sync with polling repair.

Do not choose webhook-only:
- webhooks fail
- tokens get revoked
- admin config drifts

Do not choose polling-only:
- too latent for blocker detection
- too expensive for comments and state changes

### End-To-End Topology

Fast path:
1. provider webhook received
2. verify signature
3. dedupe by delivery ID
4. fetch authoritative current record from provider API
5. update canonical store
6. emit freshness metrics

Repair path:
1. every 5 minutes per workspace
2. poll provider for recently updated work items
3. fetch comments and status changes for changed items only
4. reconcile missed events
5. advance cursor

### Freshness Objectives

- webhook-to-store p95: `<60s`
- polling repair p95: `<5m`
- end-user freshness SLA for leader features: `<10m`

### Cursor Model

Persist one cursor per:
- provider
- workspace
- stream

Streams:
- `issues`
- `comments`
- `status_changes`

### Jira Cloud Strategy

Recommended API shape:
- use REST v3, not the deprecated older search path
- use `POST /rest/api/3/search/jql` for polling issue snapshots
- request only explicit fields needed for the canonical model
- order by `updated ASC`
- use a small overlap window, e.g. `cursor - 2m`, to tolerate eventual consistency
- use `GET /rest/api/3/issue/{idOrKey}/comment` for paginated comments
- use `POST /rest/api/3/changelog/bulkfetch` for status changes on changed issues

Recommended webhook subscriptions:
- `jira:issue_created`
- `jira:issue_updated`
- `jira:issue_deleted`
- `comment_created`
- `comment_updated`
- `comment_deleted`

Important Jira-specific rules:
- do not rely on comment data being embedded in `jira:issue_*` webhooks
- use separate comment webhooks
- always re-fetch the issue or comment after webhook receipt

### Linear Strategy

Recommended API shape:
- use GraphQL
- poll issues by `updatedAt` with explicit team filters
- do not poll each issue individually
- fetch comments only for changed issues
- synthesize status changes from webhook `previous` values or from cached-snapshot diffs

Recommended webhook subscriptions:
- `Issue`
- `Comment`
- optionally `Project` and `Cycle` later if Weekly Pulse expands

Important Linear-specific rules:
- webhooks are organization-scoped and can be all-public-teams or single-team
- webhook payloads include previous values for changed properties
- use that previous-value payload to derive status-change events without extra per-item queries on every update

### Dedupe And Replay Protection

Store recent delivery ids:
- Jira: webhook id if provided, otherwise `(provider, signature, payload hash, received minute)`
- Linear: `Linear-Delivery` header

Retention:
- 24 hours is enough for replay prevention and retry dedupe

### Failure Policy

If webhook handling fails:
- return non-200 only when retry is useful
- otherwise persist an ingest error and let polling repair it

If polling fails:
- keep cursor unchanged
- surface a health signal
- do not partially advance cursor

## 4. Auth Flow

### Jira Cloud

#### Recommended rollout

Phase 2 alpha:
- support operator-managed API token first for speed

Phase 2 beta / hosted:
- add OAuth 2.0 3LO

#### Why not ship API token only

Atlassian’s current guidance clearly prefers proper app auth over collecting user API tokens. For internal or managed deployments, API token mode is still the fastest bootstrap path, but it should not be the long-term public integration mode.

#### OAuth design

Mirror the Lark OAuth split:

```text
internal/infra/workitems/jira/oauth_service.go
internal/infra/workitems/jira/token_store.go
internal/infra/workitems/jira/state_store.go
```

Recommended config:

```go
type WorkItemsJiraConfig struct {
	Enabled             bool     `yaml:"enabled"`
	Mode                string   `yaml:"mode"` // api_token | oauth
	BaseURL             string   `yaml:"base_url"`
	ClientID            string   `yaml:"client_id"`
	ClientSecret        string   `yaml:"client_secret"`
	RedirectBase        string   `yaml:"redirect_base"`
	APIToken            string   `yaml:"api_token"`
	Email               string   `yaml:"email"`
	WebhookSecret       string   `yaml:"webhook_secret"`
	PollIntervalSeconds int      `yaml:"poll_interval_seconds"`
	Projects            []string `yaml:"projects"`
}
```

OAuth flow:
1. redirect user to Atlassian authorize URL
2. request scopes needed for read-only issue/comment/project access
3. include `offline_access` so refresh tokens are issued
4. exchange code for access token
5. call `accessible-resources`
6. require explicit site binding in UI / config
7. persist selected `cloud_id`

Important Jira design constraint:
- Atlassian documents that one grant can span multiple sites for the same app and account
- therefore the connector should not “guess” which site to bind
- require explicit selection of the target site after callback

Refresh-token handling:
- store rotating refresh token
- replace stored refresh token after every refresh
- tolerate Atlassian’s documented small reuse interval when concurrent refresh happens

Webhook verification:
- validate `X-Hub-Signature`
- compute HMAC using UTF-8 raw body and configured webhook secret

#### Recommended scopes

Use the smallest read-only set that covers:
- issue read
- comment read
- project metadata read
- user/profile lookup as needed for assignee mapping
- `offline_access` for refresh

### Linear

#### Recommended rollout

Phase 2 alpha:
- support restricted personal API key first

Phase 2 beta / hosted:
- add OAuth 2.0

#### Auth design

Same split as Jira and Lark:

```text
internal/infra/workitems/linear/oauth_service.go
internal/infra/workitems/linear/token_store.go
internal/infra/workitems/linear/state_store.go
```

Recommended config:

```go
type WorkItemsLinearConfig struct {
	Enabled             bool     `yaml:"enabled"`
	Mode                string   `yaml:"mode"` // api_key | oauth
	ClientID            string   `yaml:"client_id"`
	ClientSecret        string   `yaml:"client_secret"`
	RedirectBase        string   `yaml:"redirect_base"`
	APIKey              string   `yaml:"api_key"`
	WebhookSecret       string   `yaml:"webhook_secret"`
	PollIntervalSeconds int      `yaml:"poll_interval_seconds"`
	TeamIDs             []string `yaml:"team_ids"`
}
```

OAuth flow:
1. redirect to `https://linear.app/oauth/authorize`
2. request `read`
3. request `admin` only if elephant.ai will programmatically create / read webhooks
4. exchange code on `https://api.linear.app/oauth/token`
5. store access token and refresh token
6. refresh before expiry

Important Linear design constraints:
- personal API keys can be team-scoped and permission-scoped, which is good for alpha
- OAuth apps created after October 1, 2025 have refresh tokens by default
- workspace admins or OAuth apps with `admin` scope are required for webhook creation / read

Webhook verification:
- verify `Linear-Signature`
- validate `webhookTimestamp` freshness
- use raw request body for HMAC

### Token Store Port

Use the same narrow interface style as [internal/infra/lark/oauth/token_store.go](/Users/bytedance/code/elephant.ai/internal/infra/lark/oauth/token_store.go):

```go
type TokenStore interface {
	EnsureSchema(ctx context.Context) error
	Get(ctx context.Context, workspaceID string) (Token, error)
	Upsert(ctx context.Context, token Token) error
	Delete(ctx context.Context, workspaceID string) error
}
```

## 5. Mapping To Internal Domain Types

### Key Rule

Do not map Jira / Linear issues into `task.Task`.

Instead:
- keep external work in `internal/domain/workitem`
- map from `workitem.QueryService` into feature-local read models

This preserves the difference between:
- work elephant.ai owns and executes
- work the team owns in external systems

### Assignee Mapping

Use `UserBinding` as the stable bridge.

Resolution order:
1. explicit configured mapping
2. exact email match
3. exact external id match from previous binding
4. display-name match only as a manual-review fallback

Needed by:
- [internal/app/blocker/radar.go](/Users/bytedance/code/elephant.ai/internal/app/blocker/radar.go)
- [internal/app/prepbrief/brief.go](/Users/bytedance/code/elephant.ai/internal/app/prepbrief/brief.go)
- [internal/app/pulse/weekly.go](/Users/bytedance/code/elephant.ai/internal/app/pulse/weekly.go)

### Status Mapping

Canonical mapping:

| Canonical | Meaning |
|---|---|
| `todo` | not started |
| `in_progress` | actively being worked |
| `blocked` | progress blocked or waiting on dependency |
| `done` | completed |
| `cancelled` | intentionally stopped |
| `unknown` | does not map cleanly |

Jira mapping:
- Jira `To Do` category -> `todo`
- Jira `In Progress` category -> `in_progress`
- Jira `Done` category -> `done`
- blocked custom statuses or configured blocked labels -> `blocked`
- anything else -> `unknown`

Linear mapping:
- `unstarted` -> `todo`
- `started` -> `in_progress`
- `completed` -> `done`
- `canceled` -> `cancelled`
- configured blocked state names / blocked labels -> `blocked`
- anything else -> `unknown`

Blocked-state inference should be config-driven because neither provider guarantees one universal blocked primitive.

Recommended blocked signals:
- status name in configured blocked set
- label in configured blocked labels
- latest comment contains blocked markers
- no movement after dependency comment

### Comment Mapping

Normalize all comments into:
- plain text `BodyText`
- original provider body in `BodyRaw`
- `IsSystem`
- `Visibility`

Provider-specific notes:
- Jira comments can contain Atlassian Document Format in v3
- Linear comments are markdown-like rich text

Design rule:
- store both plain-text and raw-body forms
- leader features should consume `BodyText`
- keep `BodyRaw` for future richer rendering

### Feature-Specific Internal Read Models

Recommended derived models in `internal/app/workitems/query_service.go`:

```go
type BlockerWorkView struct {
	Item            *workitem.WorkItem
	Owner           *workitem.UserBinding
	LastCommentAt   *time.Time
	LastStatusAt    *time.Time
	Staleness       time.Duration
	Reasons         []string
}

type PrepBriefView struct {
	MemberID        string
	OpenItems       []*workitem.WorkItem
	BlockedItems    []*workitem.WorkItem
	CompletedItems  []*workitem.WorkItem
	RecentComments  []*workitem.Comment
	RecentChanges   []*workitem.StatusChange
}

type WeeklyPulseView struct {
	WindowStart      time.Time
	WindowEnd        time.Time
	CompletedByOwner map[string]int
	BlockedCount     int
	ReopenedCount    int
	AgeBuckets       map[string]int
}

type ScopeChangeView struct {
	Item            *workitem.WorkItem
	ChangedAt       time.Time
	ChangeType      string
	BeforeHash      string
	AfterHash       string
	SupportingNotes []string
}
```

### Scope Change Support

Store normalized fingerprints on `WorkItem.Metadata`:
- `title_hash`
- `description_hash`
- `acceptance_hash`

Then detect:
- title changed after `started_at`
- description changed after assignee started
- reopened item with spec delta
- comment references to changed scope with no status reset

## 6. Estimated Effort Breakdown

Estimate assumes one engineer already familiar with this repo.

| Slice | Effort | Risk | Notes |
|---|---:|---|---|
| `internal/domain/workitem` canonical model + ports | `1-2d` | low | mostly straightforward repo-native modeling |
| local persisted store | `2-3d` | low | can mirror `taskstore` design |
| query service for leader features | `2-3d` | medium | needs good feature contracts |
| Jira provider with API token mode | `3-4d` | medium | JQL polling + comment + changelog fetch |
| Linear provider with API key mode | `2-3d` | medium | GraphQL query layer + webhook diff mapping |
| sync service polling loop | `2-3d` | medium | cursor correctness matters |
| webhook HTTP ingestion | `2-3d` | medium | signature validation, retries, dedupe |
| Jira OAuth 3LO | `2-3d` | medium | site selection and rotating refresh token handling |
| Linear OAuth | `2d` | low | cleaner than Jira |
| user-binding service | `1-2d` | medium | correctness matters for leader routing |
| tests | `4-5d` | medium | store, provider, webhook, cursor, mapping |

Total:
- alpha, polling-first, API-key/API-token only: `~3 weeks`
- production-ready Jira + Linear + webhooks + OAuth: `~5-6 weeks`

## Recommended Delivery Order

### Slice 1: fastest useful alpha

- `internal/domain/workitem`
- local persisted store
- Jira API-token reader
- Linear API-key reader
- polling sync only
- query service for Blocker Radar + Prep Brief

Outcome:
- enough to unlock real external work visibility

### Slice 2: freshness and reliability

- webhook ingestion
- delivery dedupe
- cursor repair
- user binding
- status-change normalization hardening

Outcome:
- enough for `<10m` freshness SLA

### Slice 3: hosted / multi-team readiness

- Jira OAuth 3LO
- Linear OAuth
- admin-scope webhook automation
- scope-change specific query views
- operational metrics and health surfacing

Outcome:
- enough for broader rollout

## Final Recommendation

The most codebase-native design is:

1. `internal/domain/workitem`
   - one canonical external-work model
   - store + provider ports

2. `internal/infra/workitems/{jira,linear,store}`
   - provider-specific clients
   - OAuth/token persistence
   - durable cache adapter

3. `internal/app/workitems`
   - sync orchestration
   - assignee mapping
   - leader-facing query projections

4. `internal/delivery/server/{http,bootstrap}`
   - webhook endpoints
   - dependency wiring

This avoids scattering Jira / Linear logic across each leader feature and gives elephant.ai one durable external-work signal layer that all Phase 2 features can share.

## External Sources

Jira Cloud official docs:
- https://developer.atlassian.com/cloud/confluence/oauth-2-3lo-apps/
- https://developer.atlassian.com/cloud/jira/platform/scopes-for-oauth-2-3LO-and-forge-apps/
- https://developer.atlassian.com/cloud/jira/platform/rest/v3/intro/
- https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issues/
- https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-comments/
- https://developer.atlassian.com/cloud/jira/software/webhooks/
- https://developer.atlassian.com/cloud/jira/platform/change-notice-removal-of-comments-from-issue-webhooks/

Linear official docs:
- https://linear.app/developers/graphql
- https://linear.app/developers/oauth-2-0-authentication
- https://linear.app/developers/webhooks
- https://linear.app/docs/api-and-webhooks

Specific external facts used:
- Jira 3LO uses `accessible-resources` to resolve site `cloudid`; refresh tokens require `offline_access`; grants can span multiple sites for one app/account
- Jira webhooks support separate issue and comment events; signed delivery uses `X-Hub-Signature`
- Linear API supports personal API keys and OAuth2
- Linear OAuth apps created after October 1, 2025 have refresh tokens by default
- Linear webhook deliveries use `Linear-Signature`, include a delivery id, and include `previous` values for changed properties
