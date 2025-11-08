package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	agentports "alex/internal/agent/ports"
	"alex/internal/storage/blobstore"
	"alex/internal/storage/craftsync"
	"alex/internal/utils"
	id "alex/internal/utils/id"
)

const (
	defaultWorkbenchContentRuneLimit = 4000
	articleDraftSource               = "workbench-article"
	articleIllustrationSource        = "workbench-article-illustration"
	articleDraftURLExpiry            = 10 * time.Minute
	maxImageConceptReferences        = 5
	maxWebBlueprintMustHaves         = 6
	maxCodePlanFeatures              = 8
	maxCodePlanIntegrations          = 6
)

var (
	// ErrWorkbenchContentRequired indicates that the editor content was empty after sanitization.
	ErrWorkbenchContentRequired = errors.New("content is required")
	// ErrWorkbenchMissingUser indicates that no authenticated user was associated with the request context.
	ErrWorkbenchMissingUser = errors.New("missing user context")
	// ErrWorkbenchDraftNotFound indicates that the requested craft could not be found for the current user.
	ErrWorkbenchDraftNotFound = errors.New("article draft not found")
	// ErrWorkbenchServiceNameRequired indicates that the code workbench request is missing a service name.
	ErrWorkbenchServiceNameRequired = errors.New("service name is required")
)

// WorkbenchService orchestrates agent collaboration features for the Alex workbench.
type WorkbenchService struct {
	executor              AgentExecutor
	sessionStore          agentports.SessionStore
	blobStore             blobstore.BlobStore
	logger                *utils.Logger
	illustrationGenerator IllustrationGenerator

	craftMirror craftsync.Mirror

	maxContentRune int
}

// WorkbenchServiceOption configures optional behaviour for WorkbenchService.
type WorkbenchServiceOption func(*WorkbenchService)

// WithWorkbenchContentLimit overrides the maximum number of runes from the editor
// that will be forwarded to the agent when generating insights. Primarily used in tests.
func WithWorkbenchContentLimit(limit int) WorkbenchServiceOption {
	return func(s *WorkbenchService) {
		if limit > 0 {
			s.maxContentRune = limit
		}
	}
}

// WithCraftMirror configures a mirror for saving crafts into a sandbox-visible directory.
func WithCraftMirror(mirror craftsync.Mirror) WorkbenchServiceOption {
	return func(s *WorkbenchService) {
		s.craftMirror = mirror
	}
}

// WithIllustrationGenerator wires an illustration generator for article insights.
func WithIllustrationGenerator(generator IllustrationGenerator) WorkbenchServiceOption {
	return func(s *WorkbenchService) {
		s.illustrationGenerator = generator
	}
}

// NewWorkbenchService creates a new WorkbenchService instance.
func NewWorkbenchService(executor AgentExecutor, sessionStore agentports.SessionStore, blobStore blobstore.BlobStore, opts ...WorkbenchServiceOption) *WorkbenchService {
	service := &WorkbenchService{
		executor:       executor,
		sessionStore:   sessionStore,
		blobStore:      blobStore,
		logger:         utils.NewComponentLogger("WorkbenchService"),
		maxContentRune: defaultWorkbenchContentRuneLimit,
	}
	for _, opt := range opts {
		opt(service)
	}
	return service
}

// SaveArticleDraftRequest contains the payload required to persist an article draft as a craft artifact.
type SaveArticleDraftRequest struct {
	SessionID string `json:"session_id,omitempty"`
	Title     string `json:"title,omitempty"`
	Content   string `json:"content"`
	Summary   string `json:"summary,omitempty"`
}

// SaveArticleDraftResult describes the persisted craft along with the associated session identifier.
type SaveArticleDraftResult struct {
	Craft     Craft  `json:"craft"`
	SessionID string `json:"session_id"`
}

// ArticleDraft represents a saved article craft along with a download link.
type ArticleDraft struct {
	Craft       Craft  `json:"craft"`
	DownloadURL string `json:"download_url,omitempty"`
}

// ArticleInsights represents structured AI assistance for the article workspace.
type ArticleInsights struct {
	Summary       string                `json:"summary"`
	KeyPoints     []string              `json:"key_points"`
	Suggestions   []string              `json:"suggestions"`
	Citations     []ArticleCitation     `json:"citations"`
	Illustrations []ArticleIllustration `json:"illustrations,omitempty"`
	SessionID     string                `json:"session_id,omitempty"`
	TaskID        string                `json:"task_id,omitempty"`
	RawAnswer     string                `json:"raw_answer,omitempty"`
}

// ArticleIllustration summarises an article paragraph and its corresponding visual prompt idea.
type ArticleIllustration struct {
	ParagraphSummary string   `json:"paragraph_summary"`
	ImageIdea        string   `json:"image_idea"`
	Prompt           string   `json:"prompt"`
	Keywords         []string `json:"keywords,omitempty"`
	CraftID          string   `json:"craft_id,omitempty"`
	ImageURL         string   `json:"image_url,omitempty"`
	MediaType        string   `json:"media_type,omitempty"`
	Name             string   `json:"name,omitempty"`
}

// ArticleCitation contains a vetted external reference suggested by the agent.
type ArticleCitation struct {
	Title   string `json:"title"`
	Source  string `json:"source"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// GenerateImageConceptsRequest captures user input for the image workbench ideation flow.
type GenerateImageConceptsRequest struct {
	Brief      string   `json:"brief"`
	Style      string   `json:"style,omitempty"`
	References []string `json:"references,omitempty"`
}

// ImageConcept describes a single visual direction crafted by the agent.
type ImageConcept struct {
	Title       string   `json:"title"`
	Prompt      string   `json:"prompt"`
	StyleNotes  []string `json:"style_notes,omitempty"`
	AspectRatio string   `json:"aspect_ratio,omitempty"`
	SeedHint    string   `json:"seed_hint,omitempty"`
	Mood        string   `json:"mood,omitempty"`
}

// ImageConceptResult bundles the generated concepts together with execution metadata.
type ImageConceptResult struct {
	Concepts  []ImageConcept `json:"concepts"`
	SessionID string         `json:"session_id,omitempty"`
	TaskID    string         `json:"task_id,omitempty"`
	RawAnswer string         `json:"raw_answer,omitempty"`
}

// GenerateWebBlueprintRequest captures the desired outcome for the web workbench assistant.
type GenerateWebBlueprintRequest struct {
	Goal      string   `json:"goal"`
	Audience  string   `json:"audience,omitempty"`
	Tone      string   `json:"tone,omitempty"`
	MustHaves []string `json:"must_haves,omitempty"`
}

// WebBlueprint describes an executable plan for constructing a landing page.
type WebBlueprint struct {
	PageTitle     string            `json:"page_title"`
	Summary       string            `json:"summary"`
	Sections      []WebSection      `json:"sections"`
	CallToActions []WebCallToAction `json:"call_to_actions,omitempty"`
	SEOKeywords   []string          `json:"seo_keywords,omitempty"`
}

// WebSection outlines one block of the resulting page.
type WebSection struct {
	Title           string   `json:"title"`
	Purpose         string   `json:"purpose"`
	Components      []string `json:"components,omitempty"`
	CopySuggestions []string `json:"copy_suggestions,omitempty"`
}

// WebCallToAction highlights an interactive entry point on the page.
type WebCallToAction struct {
	Label       string `json:"label"`
	Destination string `json:"destination"`
	Variant     string `json:"variant,omitempty"`
	Messaging   string `json:"messaging,omitempty"`
}

// WebBlueprintResult bundles the generated blueprint with metadata from the agent invocation.
type WebBlueprintResult struct {
	Blueprint WebBlueprint `json:"blueprint"`
	SessionID string       `json:"session_id,omitempty"`
	TaskID    string       `json:"task_id,omitempty"`
	RawAnswer string       `json:"raw_answer,omitempty"`
}

// GenerateCodeServicePlanRequest captures the desired microservice demo parameters.
type GenerateCodeServicePlanRequest struct {
	ServiceName  string   `json:"service_name"`
	Objective    string   `json:"objective"`
	Language     string   `json:"language,omitempty"`
	Features     []string `json:"features,omitempty"`
	Integrations []string `json:"integrations,omitempty"`
}

// CodeServicePlan summarises the agent produced blueprint for the code workbench.
type CodeServicePlan struct {
	ServiceName  string              `json:"service_name"`
	Summary      string              `json:"summary"`
	Language     string              `json:"language,omitempty"`
	Runtime      string              `json:"runtime,omitempty"`
	Architecture []string            `json:"architecture,omitempty"`
	Components   []CodePlanComponent `json:"components"`
	APIEndpoints []CodePlanEndpoint  `json:"api_endpoints,omitempty"`
	DevTasks     []string            `json:"dev_tasks,omitempty"`
	Operations   []string            `json:"operations,omitempty"`
	Testing      []string            `json:"testing,omitempty"`
}

// CodePlanComponent highlights a subsystem required for the service.
type CodePlanComponent struct {
	Name           string   `json:"name"`
	Responsibility string   `json:"responsibility"`
	TechNotes      []string `json:"tech_notes,omitempty"`
}

// CodePlanEndpoint documents an HTTP endpoint that should be implemented.
type CodePlanEndpoint struct {
	Method         string `json:"method"`
	Path           string `json:"path"`
	Description    string `json:"description"`
	RequestSchema  string `json:"request_schema,omitempty"`
	ResponseSchema string `json:"response_schema,omitempty"`
}

// CodeServicePlanResult wraps the generated plan with session metadata.
type CodeServicePlanResult struct {
	Plan      CodeServicePlan `json:"plan"`
	SessionID string          `json:"session_id,omitempty"`
	TaskID    string          `json:"task_id,omitempty"`
	RawAnswer string          `json:"raw_answer,omitempty"`
}

var htmlTagPattern = regexp.MustCompile(`<[^>]+>`)
var whitespacePattern = regexp.MustCompile(`\s+`)
var headingPattern = regexp.MustCompile(`(?is)<h[1-6][^>]*>(.*?)</h[1-6]>`)

// GenerateArticleInsights asks the agent to analyse the current article draft and return
// structured research assistance.
func (s *WorkbenchService) GenerateArticleInsights(ctx context.Context, content string) (*ArticleInsights, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil, ErrWorkbenchContentRequired
	}

	userID := strings.TrimSpace(id.UserIDFromContext(ctx))
	if userID == "" {
		return nil, ErrWorkbenchMissingUser
	}

	ctx = id.WithUserID(ctx, userID)

	sanitized := sanitizeWorkbenchContent(trimmed)
	truncated := truncateRunes(sanitized, s.maxContentRune)
	if truncated == "" {
		return nil, ErrWorkbenchContentRequired
	}

	session, err := s.executor.GetSession(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to prepare session: %w", err)
	}
	if session == nil {
		return nil, errors.New("executor returned nil session")
	}

	prompt := buildArticleInsightPrompt(truncated)

	result, err := s.executor.ExecuteTask(ctx, prompt, session.ID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate insights: %w", err)
	}
	if result == nil {
		return nil, errors.New("missing task result")
	}

	insights, err := parseArticleInsights(result.Answer)
	if err != nil {
		s.logger.Warn("failed to parse article insights response: %v", err)
		insights = &ArticleInsights{
			Summary:   strings.TrimSpace(result.Answer),
			KeyPoints: nil,
		}
	}

	insights.SessionID = result.SessionID
	insights.TaskID = result.TaskID
	insights.RawAnswer = result.Answer

	if err := s.generateArticleIllustrations(ctx, userID, session, result, insights); err != nil {
		if s.logger != nil {
			s.logger.Warn("failed to prepare illustrations: %v", err)
		}
	}

	return insights, nil
}

func (s *WorkbenchService) generateArticleIllustrations(ctx context.Context, userID string, session *agentports.Session, result *agentports.TaskResult, insights *ArticleInsights) error {
	if s == nil || s.illustrationGenerator == nil || insights == nil {
		return nil
	}
	if len(insights.Illustrations) == 0 {
		return nil
	}
	if s.blobStore == nil || s.sessionStore == nil {
		return errors.New("illustration dependencies not configured")
	}
	if session == nil {
		if result != nil && strings.TrimSpace(result.SessionID) != "" {
			loaded, err := s.sessionStore.Get(ctx, result.SessionID)
			if err != nil {
				return fmt.Errorf("load session for illustrations: %w", err)
			}
			session = loaded
		} else {
			return errors.New("missing session for illustration generation")
		}
	}
	if session.UserID == "" {
		session.UserID = userID
	}

	newArtifacts := make([]agentports.Artifact, 0)

	for idx := range insights.Illustrations {
		prompt := strings.TrimSpace(insights.Illustrations[idx].Prompt)
		if prompt == "" {
			continue
		}
		asset, err := s.illustrationGenerator.GenerateIllustration(ctx, prompt)
		if err != nil {
			if s.logger != nil {
				s.logger.Warn("illustration generation failed for prompt %q: %v", truncateRunes(prompt, 48), err)
			}
			continue
		}
		if asset == nil {
			continue
		}
		if len(asset.Data) == 0 {
			if asset.RemoteURL != "" {
				insights.Illustrations[idx].ImageURL = asset.RemoteURL
			}
			continue
		}

		artifact, err := s.persistIllustration(ctx, userID, session, idx, insights.Illustrations[idx], asset)
		if err != nil {
			if s.logger != nil {
				s.logger.Warn("persist illustration failed: %v", err)
			}
			continue
		}
		newArtifacts = append(newArtifacts, *artifact)

		downloadURL := ""
		if artifact.StorageKey != "" {
			signed, err := s.blobStore.GetSignedURL(ctx, artifact.StorageKey, articleDraftURLExpiry)
			if err != nil {
				if s.logger != nil {
					s.logger.Warn("sign illustration %s failed: %v", artifact.ID, err)
				}
			} else {
				downloadURL = signed
			}
		}

		insights.Illustrations[idx].CraftID = artifact.ID
		insights.Illustrations[idx].ImageURL = downloadURL
		insights.Illustrations[idx].MediaType = artifact.MediaType
		insights.Illustrations[idx].Name = artifact.Name
	}

	if len(newArtifacts) == 0 {
		return nil
	}

	session.Artifacts = append(session.Artifacts, newArtifacts...)
	if err := s.sessionStore.Save(ctx, session); err != nil {
		return fmt.Errorf("save session with illustrations: %w", err)
	}
	return nil
}

// GenerateImageConcepts asks the agent to craft Seedream-ready prompt concepts for the image workbench.
func (s *WorkbenchService) GenerateImageConcepts(ctx context.Context, req GenerateImageConceptsRequest) (*ImageConceptResult, error) {
	trimmedBrief := strings.TrimSpace(req.Brief)
	if trimmedBrief == "" {
		return nil, ErrWorkbenchContentRequired
	}

	userID := strings.TrimSpace(id.UserIDFromContext(ctx))
	if userID == "" {
		return nil, ErrWorkbenchMissingUser
	}

	ctx = id.WithUserID(ctx, userID)

	sanitizedBrief := sanitizeWorkbenchContent(trimmedBrief)
	truncatedBrief := truncateRunes(sanitizedBrief, s.maxContentRune)
	if truncatedBrief == "" {
		return nil, ErrWorkbenchContentRequired
	}

	style := strings.TrimSpace(req.Style)
	if style != "" {
		style = truncateRunes(sanitizeWorkbenchContent(style), 400)
	}

	references := make([]string, 0, len(req.References))
	for _, ref := range req.References {
		if len(references) >= maxImageConceptReferences {
			break
		}
		trimmed := strings.TrimSpace(ref)
		if trimmed == "" {
			continue
		}
		sanitized := truncateRunes(sanitizeWorkbenchContent(trimmed), 400)
		if sanitized != "" {
			references = append(references, sanitized)
		}
	}

	session, err := s.executor.GetSession(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to prepare session: %w", err)
	}
	if session == nil {
		return nil, errors.New("executor returned nil session")
	}

	prompt := buildImageConceptPrompt(truncatedBrief, style, references)
	result, err := s.executor.ExecuteTask(ctx, prompt, session.ID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate image concepts: %w", err)
	}
	if result == nil {
		return nil, errors.New("missing task result")
	}

	concepts, err := parseImageConcepts(result.Answer)
	if err != nil {
		s.logger.Warn("failed to parse image concepts response: %v", err)
		fallbackPrompt := strings.TrimSpace(result.Answer)
		if fallbackPrompt == "" {
			return nil, fmt.Errorf("failed to parse image concepts: %w", err)
		}
		return &ImageConceptResult{
			Concepts: []ImageConcept{
				{
					Title:  "视觉概念草稿",
					Prompt: fallbackPrompt,
				},
			},
			SessionID: result.SessionID,
			TaskID:    result.TaskID,
			RawAnswer: result.Answer,
		}, nil
	}

	concepts.SessionID = result.SessionID
	concepts.TaskID = result.TaskID
	concepts.RawAnswer = result.Answer
	return concepts, nil
}

// GenerateWebBlueprint asks the agent to deliver a component-level plan for the requested web experience.
func (s *WorkbenchService) GenerateWebBlueprint(ctx context.Context, req GenerateWebBlueprintRequest) (*WebBlueprintResult, error) {
	trimmedGoal := strings.TrimSpace(req.Goal)
	if trimmedGoal == "" {
		return nil, ErrWorkbenchContentRequired
	}

	userID := strings.TrimSpace(id.UserIDFromContext(ctx))
	if userID == "" {
		return nil, ErrWorkbenchMissingUser
	}

	ctx = id.WithUserID(ctx, userID)

	sanitizedGoal := truncateRunes(sanitizeWorkbenchContent(trimmedGoal), s.maxContentRune)
	if sanitizedGoal == "" {
		return nil, ErrWorkbenchContentRequired
	}

	audience := strings.TrimSpace(req.Audience)
	if audience != "" {
		audience = truncateRunes(sanitizeWorkbenchContent(audience), 400)
	}

	tone := strings.TrimSpace(req.Tone)
	if tone != "" {
		tone = truncateRunes(sanitizeWorkbenchContent(tone), 200)
	}

	mustHaves := make([]string, 0, len(req.MustHaves))
	for _, entry := range req.MustHaves {
		if len(mustHaves) >= maxWebBlueprintMustHaves {
			break
		}
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}
		sanitized := truncateRunes(sanitizeWorkbenchContent(trimmed), 160)
		if sanitized != "" {
			mustHaves = append(mustHaves, sanitized)
		}
	}

	session, err := s.executor.GetSession(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to prepare session: %w", err)
	}
	if session == nil {
		return nil, errors.New("executor returned nil session")
	}

	prompt := buildWebBlueprintPrompt(sanitizedGoal, audience, tone, mustHaves)
	result, err := s.executor.ExecuteTask(ctx, prompt, session.ID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate web blueprint: %w", err)
	}
	if result == nil {
		return nil, errors.New("missing task result")
	}

	blueprint, err := parseWebBlueprint(result.Answer)
	if err != nil {
		s.logger.Warn("failed to parse web blueprint response: %v", err)
		fallback := buildFallbackWebBlueprint(sanitizedGoal, audience, tone, mustHaves)
		return &WebBlueprintResult{
			Blueprint: fallback,
			SessionID: result.SessionID,
			TaskID:    result.TaskID,
			RawAnswer: result.Answer,
		}, nil
	}

	return &WebBlueprintResult{
		Blueprint: *blueprint,
		SessionID: result.SessionID,
		TaskID:    result.TaskID,
		RawAnswer: result.Answer,
	}, nil
}

// GenerateCodeServicePlan asks the agent to create an implementation blueprint for the code workbench.
func (s *WorkbenchService) GenerateCodeServicePlan(ctx context.Context, req GenerateCodeServicePlanRequest) (*CodeServicePlanResult, error) {
	serviceName := strings.TrimSpace(req.ServiceName)
	if serviceName == "" {
		return nil, ErrWorkbenchServiceNameRequired
	}

	objective := strings.TrimSpace(req.Objective)
	if objective == "" {
		return nil, ErrWorkbenchContentRequired
	}

	userID := strings.TrimSpace(id.UserIDFromContext(ctx))
	if userID == "" {
		return nil, ErrWorkbenchMissingUser
	}

	ctx = id.WithUserID(ctx, userID)

	sanitizedName := truncateRunes(sanitizeWorkbenchContent(serviceName), 120)
	if sanitizedName == "" {
		return nil, ErrWorkbenchServiceNameRequired
	}

	sanitizedObjective := truncateRunes(sanitizeWorkbenchContent(objective), s.maxContentRune)
	if sanitizedObjective == "" {
		return nil, ErrWorkbenchContentRequired
	}

	languageCanonical, languageDisplay := normalizeCodeLanguage(strings.TrimSpace(req.Language))

	features := sanitizeWorkbenchList(req.Features, maxCodePlanFeatures, 200)
	integrations := sanitizeWorkbenchList(req.Integrations, maxCodePlanIntegrations, 200)

	session, err := s.executor.GetSession(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to prepare session: %w", err)
	}
	if session == nil {
		return nil, errors.New("executor returned nil session")
	}

	prompt := buildCodeServicePlanPrompt(sanitizedName, sanitizedObjective, languageDisplay, features, integrations)
	result, err := s.executor.ExecuteTask(ctx, prompt, session.ID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate code plan: %w", err)
	}
	if result == nil {
		return nil, errors.New("missing task result")
	}

	plan, err := parseCodeServicePlan(result.Answer)
	if err != nil {
		s.logger.Warn("failed to parse code plan response: %v", err)
		fallback := buildFallbackCodeServicePlan(sanitizedName, sanitizedObjective, languageCanonical, languageDisplay, features, integrations)
		return &CodeServicePlanResult{
			Plan:      fallback,
			SessionID: result.SessionID,
			TaskID:    result.TaskID,
			RawAnswer: result.Answer,
		}, nil
	}

	if plan.Language == "" {
		plan.Language = languageDisplay
	}
	if plan.Runtime == "" && languageDisplay != "" {
		plan.Runtime = defaultRuntimeForLanguage(languageCanonical, languageDisplay)
	}
	plan.ServiceName = sanitizedName

	return &CodeServicePlanResult{
		Plan:      *plan,
		SessionID: result.SessionID,
		TaskID:    result.TaskID,
		RawAnswer: result.Answer,
	}, nil
}

// SaveArticleDraft uploads the provided article draft to blob storage and records it as a craft artifact.
func (s *WorkbenchService) SaveArticleDraft(ctx context.Context, req SaveArticleDraftRequest) (*SaveArticleDraftResult, error) {
	if s == nil {
		return nil, errors.New("workbench service not initialized")
	}
	trimmed := strings.TrimSpace(req.Content)
	if trimmed == "" {
		return nil, ErrWorkbenchContentRequired
	}

	userID := strings.TrimSpace(id.UserIDFromContext(ctx))
	if userID == "" {
		return nil, ErrWorkbenchMissingUser
	}

	if s.sessionStore == nil {
		return nil, errors.New("session store not configured")
	}
	if s.blobStore == nil {
		return nil, errors.New("blob store not configured")
	}

	ctx = id.WithUserID(ctx, userID)

	sessionID := strings.TrimSpace(req.SessionID)
	session, err := s.executor.GetSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	if session == nil {
		return nil, errors.New("executor returned nil session")
	}
	sessionID = session.ID

	artifact, err := s.persistArticleDraft(ctx, userID, session, req)
	if err != nil {
		return nil, err
	}

	session.Artifacts = append(session.Artifacts, *artifact)
	if err := s.sessionStore.Save(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	craft := Craft{
		ID:          artifact.ID,
		SessionID:   artifact.SessionID,
		UserID:      artifact.UserID,
		Name:        artifact.Name,
		MediaType:   artifact.MediaType,
		Description: artifact.Description,
		Source:      artifact.Source,
		Size:        artifact.Size,
		Checksum:    artifact.Checksum,
		StorageKey:  artifact.StorageKey,
		CreatedAt:   artifact.CreatedAt,
	}

	return &SaveArticleDraftResult{Craft: craft, SessionID: sessionID}, nil
}

// DeleteArticleDraft removes the specified article draft craft and deletes the backing blob.
func (s *WorkbenchService) DeleteArticleDraft(ctx context.Context, craftID string) error {
	if s == nil {
		return errors.New("workbench service not initialized")
	}

	normalizedID := strings.TrimSpace(craftID)
	if normalizedID == "" {
		return fmt.Errorf("craft id is required")
	}

	userID := strings.TrimSpace(id.UserIDFromContext(ctx))
	if userID == "" {
		return ErrWorkbenchMissingUser
	}

	if s.sessionStore == nil {
		return errors.New("session store not configured")
	}
	if s.blobStore == nil {
		return errors.New("blob store not configured")
	}

	ctx = id.WithUserID(ctx, userID)

	sessionIDs, err := s.sessionStore.List(ctx)
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}

	type deletionTarget struct {
		session    *agentports.Session
		artifact   agentports.Artifact
		newListing []agentports.Artifact
	}

	var target *deletionTarget

	for _, sessionID := range sessionIDs {
		session, err := s.sessionStore.Get(ctx, sessionID)
		if err != nil {
			s.logger.Warn("failed to load session %s: %v", sessionID, err)
			continue
		}
		if session.UserID != "" && session.UserID != userID {
			continue
		}

		cleaned := make([]agentports.Artifact, 0, len(session.Artifacts))
		var match *agentports.Artifact
		for _, artifact := range session.Artifacts {
			if artifact.ID == normalizedID && strings.TrimSpace(artifact.Source) == articleDraftSource {
				copyArtifact := artifact
				match = &copyArtifact
				continue
			}
			cleaned = append(cleaned, artifact)
		}

		if match != nil {
			target = &deletionTarget{session: session, artifact: *match, newListing: cleaned}
			break
		}
	}

	if target == nil {
		return ErrWorkbenchDraftNotFound
	}

	original := target.session.Artifacts
	target.session.Artifacts = target.newListing
	if err := s.sessionStore.Save(ctx, target.session); err != nil {
		target.session.Artifacts = original
		return fmt.Errorf("save session: %w", err)
	}

	if key := strings.TrimSpace(target.artifact.StorageKey); key != "" {
		if err := s.blobStore.DeleteObject(ctx, key); err != nil {
			s.logger.Warn("failed to delete blob for draft %s: %v", target.artifact.ID, err)
		}
	}

	s.removeArtifactMirror(ctx, target.artifact)

	return nil
}

func (s *WorkbenchService) persistArticleDraft(ctx context.Context, userID string, session *agentports.Session, req SaveArticleDraftRequest) (*agentports.Artifact, error) {
	filename := deriveArticleFilename(req.Title, req.Content)
	description := deriveArticleDescription(req.Summary, req.Content)

	artifactID := id.NewArtifactID()
	storageKey := fmt.Sprintf("%s/%s.html", userID, artifactID)

	body := bytes.NewBufferString(normalizeHTMLDocument(req.Content))
	contentBytes := body.Bytes()
	checksum := sha256.Sum256(contentBytes)
	size := int64(len(contentBytes))

	reader := bytes.NewReader(contentBytes)
	storedKey, err := s.blobStore.PutObject(ctx, storageKey, reader, blobstore.PutOptions{ContentType: "text/html; charset=utf-8", ContentLength: size})
	if err != nil {
		return nil, fmt.Errorf("upload article draft: %w", err)
	}
	if strings.TrimSpace(storedKey) != "" {
		storageKey = storedKey
	}

	artifact := agentports.Artifact{
		ID:          artifactID,
		SessionID:   session.ID,
		UserID:      userID,
		Name:        filename,
		MediaType:   "text/html",
		StorageKey:  storageKey,
		Description: description,
		Size:        size,
		Checksum:    hex.EncodeToString(checksum[:]),
		Source:      articleDraftSource,
		CreatedAt:   time.Now(),
	}

	s.mirrorArtifact(ctx, artifact, contentBytes)

	return &artifact, nil
}

func (s *WorkbenchService) persistIllustration(ctx context.Context, userID string, session *agentports.Session, index int, illustration ArticleIllustration, asset *GeneratedIllustration) (*agentports.Artifact, error) {
	if s == nil || s.blobStore == nil {
		return nil, errors.New("blob store not configured")
	}
	if session == nil {
		return nil, errors.New("session is required")
	}
	if asset == nil || len(asset.Data) == 0 {
		return nil, errors.New("empty illustration payload")
	}

	mediaType := strings.TrimSpace(asset.MediaType)
	if mediaType == "" {
		mediaType = "image/png"
	}

	name := deriveIllustrationFilename(illustration.ImageIdea, asset.Name, index)
	description := deriveIllustrationDescription(illustration.ImageIdea, asset.Description)

	artifactID := id.NewArtifactID()
	storageExt := deriveIllustrationExtension(name)
	storageKey := fmt.Sprintf("%s/%s%s", userID, artifactID, storageExt)

	contentBytes := asset.Data
	checksum := sha256.Sum256(contentBytes)
	size := int64(len(contentBytes))

	reader := bytes.NewReader(contentBytes)
	storedKey, err := s.blobStore.PutObject(ctx, storageKey, reader, blobstore.PutOptions{ContentType: mediaType, ContentLength: size})
	if err != nil {
		return nil, fmt.Errorf("upload illustration: %w", err)
	}
	if strings.TrimSpace(storedKey) != "" {
		storageKey = storedKey
	}

	artifact := agentports.Artifact{
		ID:          artifactID,
		SessionID:   session.ID,
		UserID:      userID,
		Name:        name,
		MediaType:   mediaType,
		StorageKey:  storageKey,
		Description: description,
		Size:        size,
		Checksum:    hex.EncodeToString(checksum[:]),
		Source:      articleIllustrationSource,
		CreatedAt:   time.Now(),
	}
	if asset.Source != "" {
		artifact.Source = asset.Source
	}
	if asset.RemoteURL != "" {
		artifact.URI = asset.RemoteURL
	}

	s.mirrorArtifact(ctx, artifact, contentBytes)

	return &artifact, nil
}

func (s *WorkbenchService) mirrorArtifact(ctx context.Context, artifact agentports.Artifact, content []byte) {
	if s == nil || s.craftMirror == nil {
		return
	}
	meta := craftsync.ArtifactMetadata{
		ID:          artifact.ID,
		UserID:      artifact.UserID,
		SessionID:   artifact.SessionID,
		Name:        artifact.Name,
		MediaType:   artifact.MediaType,
		Description: artifact.Description,
		Source:      artifact.Source,
		StorageKey:  artifact.StorageKey,
		URI:         artifact.URI,
		Size:        artifact.Size,
		Checksum:    artifact.Checksum,
		CreatedAt:   artifact.CreatedAt,
	}
	if _, err := s.craftMirror.Mirror(ctx, meta, content); err != nil {
		if s.logger != nil {
			s.logger.Warn("failed to mirror artifact %s: %v", artifact.ID, err)
		}
	}
}

func (s *WorkbenchService) removeArtifactMirror(ctx context.Context, artifact agentports.Artifact) {
	if s == nil || s.craftMirror == nil {
		return
	}
	meta := craftsync.ArtifactMetadata{
		ID:        artifact.ID,
		UserID:    artifact.UserID,
		SessionID: artifact.SessionID,
	}
	if err := s.craftMirror.Remove(ctx, meta); err != nil {
		if s.logger != nil {
			s.logger.Warn("failed to remove mirrored artifact %s: %v", artifact.ID, err)
		}
	}
}

// ListArticleDrafts returns crafts created from the article workbench for the authenticated user.
func (s *WorkbenchService) ListArticleDrafts(ctx context.Context) ([]ArticleDraft, error) {
	if s == nil {
		return nil, errors.New("workbench service not initialized")
	}

	userID := strings.TrimSpace(id.UserIDFromContext(ctx))
	if userID == "" {
		return nil, ErrWorkbenchMissingUser
	}

	if s.sessionStore == nil {
		return nil, errors.New("session store not configured")
	}
	if s.blobStore == nil {
		return nil, errors.New("blob store not configured")
	}

	ctx = id.WithUserID(ctx, userID)

	sessionIDs, err := s.sessionStore.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	drafts := make([]ArticleDraft, 0)
	for _, sessionID := range sessionIDs {
		session, err := s.sessionStore.Get(ctx, sessionID)
		if err != nil {
			s.logger.Warn("failed to load session %s: %v", sessionID, err)
			continue
		}
		if session.UserID != "" && session.UserID != userID {
			continue
		}
		for _, artifact := range session.Artifacts {
			if strings.TrimSpace(artifact.Source) != articleDraftSource {
				continue
			}

			downloadURL := ""
			if artifact.StorageKey != "" {
				url, err := s.blobStore.GetSignedURL(ctx, artifact.StorageKey, articleDraftURLExpiry)
				if err != nil {
					s.logger.Warn("failed to sign draft %s: %v", artifact.ID, err)
				} else {
					downloadURL = url
				}
			}

			drafts = append(drafts, ArticleDraft{
				Craft: Craft{
					ID:          artifact.ID,
					SessionID:   artifact.SessionID,
					UserID:      artifact.UserID,
					Name:        artifact.Name,
					MediaType:   artifact.MediaType,
					Description: artifact.Description,
					Source:      artifact.Source,
					Size:        artifact.Size,
					Checksum:    artifact.Checksum,
					StorageKey:  artifact.StorageKey,
					CreatedAt:   artifact.CreatedAt,
				},
				DownloadURL: downloadURL,
			})
		}
	}

	sort.Slice(drafts, func(i, j int) bool {
		return drafts[i].Craft.CreatedAt.After(drafts[j].Craft.CreatedAt)
	})

	return drafts, nil
}

func sanitizeWorkbenchContent(content string) string {
	withoutTags := htmlTagPattern.ReplaceAllString(content, " ")
	withoutEntities := strings.NewReplacer("&nbsp;", " ", "&amp;", "&", "&lt;", "<", "&gt;", ">").Replace(withoutTags)
	collapsed := whitespacePattern.ReplaceAllString(withoutEntities, " ")
	return strings.TrimSpace(collapsed)
}

func truncateRunes(input string, limit int) string {
	if limit <= 0 {
		return input
	}
	if utf8.RuneCountInString(input) <= limit {
		return input
	}
	var builder strings.Builder
	builder.Grow(limit)
	count := 0
	for _, r := range input {
		if count >= limit {
			break
		}
		builder.WriteRune(r)
		count++
	}
	return builder.String()
}

func buildArticleInsightPrompt(content string) string {
	var builder strings.Builder
	builder.WriteString("你是一个严谨而谨慎的中文写作研究助理。\n")
	builder.WriteString("我会提供正在撰写的文章内容，请你只输出经过核实且与主题高度相关的信息。\n")
	builder.WriteString("\n")
	builder.WriteString("请严格按照下面的 JSON 结构直接返回结果（不要添加反引号或额外说明）：\n")
	builder.WriteString("{\n")
	builder.WriteString("  \"summary\": \"对文章重点的简短概括\",\n")
	builder.WriteString("  \"key_points\": [\"概括文章当前要点\"],\n")
	builder.WriteString("  \"suggestions\": [\"下一步写作建议，务必务实可执行\"],\n")
	builder.WriteString("  \"citations\": [{\n")
	builder.WriteString("    \"title\": \"资料标题\",\n")
	builder.WriteString("    \"source\": \"可信来源名称\",\n")
	builder.WriteString("    \"url\": \"可公开访问的链接\",\n")
	builder.WriteString("    \"snippet\": \"引用的关键语句或数据\"\n")
	builder.WriteString("  }],\n")
	builder.WriteString("  \"illustrations\": [{\n")
	builder.WriteString("    \"paragraph_summary\": \"段落核心观点\",\n")
	builder.WriteString("    \"image_idea\": \"描述插图需要呈现的场景与信息\",\n")
	builder.WriteString("    \"prompt\": \"英文提示词，便于直接投喂图像模型\",\n")
	builder.WriteString("    \"keywords\": [\"可选的视觉关键词\"]\n")
	builder.WriteString("  }]\n")
	builder.WriteString("}\n")
	builder.WriteString("\n")
	builder.WriteString("仅当资料能够追溯到公开可信来源时才填入 citations；若暂时找不到，请返回空数组，不要编造。\n")
	builder.WriteString("若对文章内容有风险提示，也放入 suggestions。\n")
	builder.WriteString("针对文章中的重要段落或章节，生成 2-4 条插图建议，提示词需为英文且可直接使用；当段落信息不足时可跳过。\n")
	builder.WriteString("\n")
	builder.WriteString("文章草稿如下：\n")
	builder.WriteString(content)
	builder.WriteString("\n")
	builder.WriteString("请保持回答为合法 JSON。")
	return builder.String()
}

func buildImageConceptPrompt(brief string, style string, references []string) string {
	var builder strings.Builder
	builder.WriteString("你是一位严谨的中文视觉设计总监，熟练掌握 Seedream 图像生成模型。\n")
	builder.WriteString("需要你基于下面的创作简报，输出可以直接用于 seedream_text_to_image 的提示词方案。\n")
	builder.WriteString("请确保所有输出信息真实、可执行，不要臆造不存在的品牌或素材。\n")
	builder.WriteString("\n")
	builder.WriteString("请严格以 JSON 格式回复，且不要附加反引号或额外说明，结构如下：\n")
	builder.WriteString("{\n")
	builder.WriteString("  \"concepts\": [\n")
	builder.WriteString("    {\n")
	builder.WriteString("      \"title\": \"方向名称\",\n")
	builder.WriteString("      \"prompt\": \"可直接投喂 seedream_text_to_image 的英文提示词\",\n")
	builder.WriteString("      \"style_notes\": [\"补充的视觉细节或素材建议\"],\n")
	builder.WriteString("      \"aspect_ratio\": \"例如 16:9\",\n")
	builder.WriteString("      \"seed_hint\": \"可选的随机种子或留空\",\n")
	builder.WriteString("      \"mood\": \"整体氛围描述\"\n")
	builder.WriteString("    }\n")
	builder.WriteString("  ]\n")
	builder.WriteString("}\n")
	builder.WriteString("\n")
	builder.WriteString("创作简报：\n")
	builder.WriteString(brief)
	builder.WriteString("\n")
	if style != "" {
		builder.WriteString("偏好风格：\n")
		builder.WriteString(style)
		builder.WriteString("\n")
	}
	if len(references) > 0 {
		builder.WriteString("可参考的素材或说明：\n")
		for i, ref := range references {
			builder.WriteString(fmt.Sprintf("%d. %s\n", i+1, ref))
		}
	}
	builder.WriteString("请最多给出 3 个不同方向，突出光线、材质、构图等关键点，prompt 中不要包含中文。")
	return builder.String()
}

func buildWebBlueprintPrompt(goal string, audience string, tone string, mustHaves []string) string {
	var builder strings.Builder
	builder.WriteString("你是一名资深的中文网页信息架构师，擅长把业务目标拆解为明确的页面结构。\n")
	builder.WriteString("请阅读下面的目标信息，输出一个可以直接交付给设计与开发团队的落地页蓝图。\n")
	builder.WriteString("所有建议必须基于事实且务实可执行，不要捏造数据或夸张承诺。\n\n")
	builder.WriteString("请严格按以下 JSON 结构回答，勿添加反引号或额外注释：\n")
	builder.WriteString("{\n")
	builder.WriteString("  \"blueprint\": {\n")
	builder.WriteString("    \"page_title\": \"页面标题\",\n")
	builder.WriteString("    \"summary\": \"一句话概述页面要解决的问题\",\n")
	builder.WriteString("    \"sections\": [\n")
	builder.WriteString("      {\n")
	builder.WriteString("        \"title\": \"模块标题\",\n")
	builder.WriteString("        \"purpose\": \"该模块要传达的价值或动作\",\n")
	builder.WriteString("        \"components\": [\"核心组件或交互元素\"],\n")
	builder.WriteString("        \"copy_suggestions\": [\"可直接使用的中文文案要点\"]\n")
	builder.WriteString("      }\n")
	builder.WriteString("    ],\n")
	builder.WriteString("    \"call_to_actions\": [\n")
	builder.WriteString("      {\n")
	builder.WriteString("        \"label\": \"按钮文案\",\n")
	builder.WriteString("        \"destination\": \"指向的页面或行为\",\n")
	builder.WriteString("        \"variant\": \"主按钮/次按钮等\",\n")
	builder.WriteString("        \"messaging\": \"补充提示信息\"\n")
	builder.WriteString("      }\n")
	builder.WriteString("    ],\n")
	builder.WriteString("    \"seo_keywords\": [\"相关搜索关键词\"]\n")
	builder.WriteString("  }\n")
	builder.WriteString("}\n\n")
	builder.WriteString("页面目标：\n")
	builder.WriteString(goal)
	builder.WriteString("\n")
	if audience != "" {
		builder.WriteString("目标受众：\n")
		builder.WriteString(audience)
		builder.WriteString("\n")
	}
	if tone != "" {
		builder.WriteString("品牌语气：\n")
		builder.WriteString(tone)
		builder.WriteString("\n")
	}
	if len(mustHaves) > 0 {
		builder.WriteString("必须包含：\n")
		for i, item := range mustHaves {
			builder.WriteString(fmt.Sprintf("%d. %s\n", i+1, item))
		}
	}
	builder.WriteString("请至少输出 4 个模块，并在文案建议中提供可直接使用的中文短句。")
	return builder.String()
}

func buildCodeServicePlanPrompt(serviceName string, objective string, language string, features []string, integrations []string) string {
	var builder strings.Builder
	builder.WriteString("你是一位严谨的全栈架构师，需要为一个代码微服务工作台提供脚手架方案。\n")
	builder.WriteString("请根据提供的信息，输出可以直接交付给工程团队的实现计划，确保内容真实、可执行。\n\n")
	builder.WriteString("请严格按以下 JSON 结构回答，禁止添加反引号或额外说明：\n")
	builder.WriteString("{\n")
	builder.WriteString("  \"plan\": {\n")
	builder.WriteString("    \"service_name\": \"服务名称\",\n")
	builder.WriteString("    \"summary\": \"一句话目标\",\n")
	builder.WriteString("    \"language\": \"首选语言或留空\",\n")
	builder.WriteString("    \"runtime\": \"建议运行时或框架\",\n")
	builder.WriteString("    \"architecture\": [\"核心架构要点\"],\n")
	builder.WriteString("    \"components\": [\n")
	builder.WriteString("      {\n")
	builder.WriteString("        \"name\": \"组件名称\",\n")
	builder.WriteString("        \"responsibility\": \"主要职责\",\n")
	builder.WriteString("        \"tech_notes\": [\"落地时的技术要点\"]\n")
	builder.WriteString("      }\n")
	builder.WriteString("    ],\n")
	builder.WriteString("    \"api_endpoints\": [\n")
	builder.WriteString("      {\n")
	builder.WriteString("        \"method\": \"HTTP 方法\",\n")
	builder.WriteString("        \"path\": \"/api/path\",\n")
	builder.WriteString("        \"description\": \"用途\",\n")
	builder.WriteString("        \"request_schema\": \"请求字段说明\",\n")
	builder.WriteString("        \"response_schema\": \"响应字段说明\"\n")
	builder.WriteString("      }\n")
	builder.WriteString("    ],\n")
	builder.WriteString("    \"dev_tasks\": [\"实现步骤\"],\n")
	builder.WriteString("    \"operations\": [\"运维与运行建议\"],\n")
	builder.WriteString("    \"testing\": [\"测试建议\"]\n")
	builder.WriteString("  }\n")
	builder.WriteString("}\n\n")
	builder.WriteString("服务名称：\n")
	builder.WriteString(serviceName)
	builder.WriteString("\n")
	builder.WriteString("业务目标：\n")
	builder.WriteString(objective)
	builder.WriteString("\n")
	if language != "" {
		builder.WriteString("首选语言或框架：\n")
		builder.WriteString(language)
		builder.WriteString("\n")
	}
	if len(features) > 0 {
		builder.WriteString("主要功能或特性：\n")
		for i, feature := range features {
			builder.WriteString(fmt.Sprintf("%d. %s\n", i+1, feature))
		}
	}
	if len(integrations) > 0 {
		builder.WriteString("外部依赖或集成：\n")
		for i, integration := range integrations {
			builder.WriteString(fmt.Sprintf("%d. %s\n", i+1, integration))
		}
	}
	builder.WriteString("请确保所有字段填充真实、具体的建议，不要返回空数组，无法确定的字段请省略。")
	return builder.String()
}

func deriveArticleFilename(title string, content string) string {
	candidate := strings.TrimSpace(title)
	if candidate == "" {
		candidate = extractHeading(content)
	}
	if candidate == "" {
		candidate = sanitizeWorkbenchContent(content)
	}
	if candidate == "" {
		candidate = "文章草稿"
	}
	candidate = strings.ReplaceAll(candidate, "/", "-")
	candidate = strings.ReplaceAll(candidate, "\\", "-")
	candidate = truncateRunes(candidate, 60)
	if !strings.HasSuffix(candidate, ".html") {
		candidate = candidate + ".html"
	}
	return candidate
}

func deriveArticleDescription(summary string, content string) string {
	candidate := strings.TrimSpace(summary)
	if candidate == "" {
		sanitized := sanitizeWorkbenchContent(content)
		candidate = truncateRunes(sanitized, 160)
	}
	return candidate
}

func deriveIllustrationFilename(idea string, fallback string, index int) string {
	candidate := strings.TrimSpace(fallback)
	if candidate == "" {
		candidate = strings.TrimSpace(idea)
	}
	if candidate == "" {
		candidate = fmt.Sprintf("文章插图-%02d", index+1)
	}
	candidate = sanitizeWorkbenchContent(candidate)
	candidate = strings.ReplaceAll(candidate, "/", "-")
	candidate = strings.ReplaceAll(candidate, "\\", "-")
	candidate = truncateRunes(candidate, 80)
	lower := strings.ToLower(candidate)
	switch {
	case strings.HasSuffix(lower, ".png"), strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"), strings.HasSuffix(lower, ".webp"), strings.HasSuffix(lower, ".gif"):
		return candidate
	default:
		return candidate + ".png"
	}
}

func deriveIllustrationExtension(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"):
		return ".jpg"
	case strings.HasSuffix(lower, ".webp"):
		return ".webp"
	case strings.HasSuffix(lower, ".gif"):
		return ".gif"
	default:
		return ".png"
	}
}

func deriveIllustrationDescription(idea string, existing string) string {
	candidate := strings.TrimSpace(existing)
	if candidate == "" {
		candidate = strings.TrimSpace(idea)
	}
	if candidate == "" {
		candidate = "文章插图"
	}
	return truncateRunes(candidate, 160)
}

func extractHeading(content string) string {
	matches := headingPattern.FindStringSubmatch(content)
	if len(matches) < 2 {
		return ""
	}
	inner := matches[1]
	inner = htmlTagPattern.ReplaceAllString(inner, " ")
	inner = whitespacePattern.ReplaceAllString(inner, " ")
	return strings.TrimSpace(inner)
}

func normalizeHTMLDocument(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "<!DOCTYPE html><html lang=\"zh-CN\"><head><meta charset=\"utf-8\"></head><body></body></html>"
	}
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "<html") {
		return trimmed
	}
	return "<!DOCTYPE html><html lang=\"zh-CN\"><head><meta charset=\"utf-8\"></head><body>" + trimmed + "</body></html>"
}

type articleInsightsPayload struct {
	Summary       string                       `json:"summary"`
	KeyPoints     []string                     `json:"key_points"`
	Suggestions   []string                     `json:"suggestions"`
	Citations     []articleCitationPayload     `json:"citations"`
	Illustrations []articleIllustrationPayload `json:"illustrations"`
}

type articleCitationPayload struct {
	Title   string `json:"title"`
	Source  string `json:"source"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

type articleIllustrationPayload struct {
	ParagraphSummary string   `json:"paragraph_summary"`
	ImageIdea        string   `json:"image_idea"`
	Prompt           string   `json:"prompt"`
	Keywords         []string `json:"keywords"`
}

func parseArticleInsights(answer string) (*ArticleInsights, error) {
	trimmed := strings.TrimSpace(answer)
	if trimmed == "" {
		return nil, fmt.Errorf("empty answer")
	}

	candidate := extractJSON(trimmed)
	if candidate == "" {
		return nil, fmt.Errorf("no json object found")
	}

	var payload articleInsightsPayload
	if err := json.Unmarshal([]byte(candidate), &payload); err != nil {
		return nil, fmt.Errorf("failed to decode json: %w", err)
	}

	insights := &ArticleInsights{
		Summary:     strings.TrimSpace(payload.Summary),
		KeyPoints:   normalizeStringSlice(payload.KeyPoints),
		Suggestions: normalizeStringSlice(payload.Suggestions),
	}

	if len(payload.Citations) > 0 {
		insights.Citations = make([]ArticleCitation, 0, len(payload.Citations))
		for _, citation := range payload.Citations {
			if strings.TrimSpace(citation.Title) == "" || strings.TrimSpace(citation.URL) == "" {
				continue
			}
			insights.Citations = append(insights.Citations, ArticleCitation{
				Title:   strings.TrimSpace(citation.Title),
				Source:  strings.TrimSpace(citation.Source),
				URL:     strings.TrimSpace(citation.URL),
				Snippet: strings.TrimSpace(citation.Snippet),
			})
		}
	}

	if len(payload.Illustrations) > 0 {
		insights.Illustrations = make([]ArticleIllustration, 0, len(payload.Illustrations))
		for _, illustration := range payload.Illustrations {
			prompt := strings.TrimSpace(illustration.Prompt)
			idea := strings.TrimSpace(illustration.ImageIdea)
			summary := strings.TrimSpace(illustration.ParagraphSummary)
			if prompt == "" && idea == "" {
				continue
			}
			insights.Illustrations = append(insights.Illustrations, ArticleIllustration{
				ParagraphSummary: summary,
				ImageIdea:        idea,
				Prompt:           prompt,
				Keywords:         normalizeStringSlice(illustration.Keywords),
			})
		}
	}

	return insights, nil
}

type imageConceptsPayload struct {
	Concepts []imageConceptPayload `json:"concepts"`
}

type imageConceptPayload struct {
	Title       string   `json:"title"`
	Prompt      string   `json:"prompt"`
	StyleNotes  []string `json:"style_notes"`
	AspectRatio string   `json:"aspect_ratio"`
	SeedHint    string   `json:"seed_hint"`
	Mood        string   `json:"mood"`
}

func parseImageConcepts(answer string) (*ImageConceptResult, error) {
	trimmed := strings.TrimSpace(answer)
	if trimmed == "" {
		return nil, fmt.Errorf("empty answer")
	}

	candidate := extractJSON(trimmed)
	if candidate == "" {
		return nil, fmt.Errorf("no json object found")
	}

	var payload imageConceptsPayload
	if err := json.Unmarshal([]byte(candidate), &payload); err != nil {
		return nil, fmt.Errorf("failed to decode json: %w", err)
	}

	if len(payload.Concepts) == 0 {
		return nil, fmt.Errorf("no concepts returned")
	}

	concepts := &ImageConceptResult{Concepts: make([]ImageConcept, 0, len(payload.Concepts))}
	for _, concept := range payload.Concepts {
		prompt := strings.TrimSpace(concept.Prompt)
		if prompt == "" {
			continue
		}
		concepts.Concepts = append(concepts.Concepts, ImageConcept{
			Title:       strings.TrimSpace(concept.Title),
			Prompt:      prompt,
			StyleNotes:  normalizeStringSlice(concept.StyleNotes),
			AspectRatio: strings.TrimSpace(concept.AspectRatio),
			SeedHint:    strings.TrimSpace(concept.SeedHint),
			Mood:        strings.TrimSpace(concept.Mood),
		})
	}

	if len(concepts.Concepts) == 0 {
		return nil, fmt.Errorf("no valid concepts found")
	}

	return concepts, nil
}

type webBlueprintPayload struct {
	Blueprint webBlueprintData `json:"blueprint"`
}

type webBlueprintData struct {
	PageTitle     string                       `json:"page_title"`
	Summary       string                       `json:"summary"`
	Sections      []webBlueprintSectionPayload `json:"sections"`
	CallToActions []webBlueprintCTAPayload     `json:"call_to_actions"`
	SEOKeywords   []string                     `json:"seo_keywords"`
}

type webBlueprintSectionPayload struct {
	Title           string   `json:"title"`
	Purpose         string   `json:"purpose"`
	Components      []string `json:"components"`
	CopySuggestions []string `json:"copy_suggestions"`
}

type webBlueprintCTAPayload struct {
	Label       string `json:"label"`
	Destination string `json:"destination"`
	Variant     string `json:"variant"`
	Messaging   string `json:"messaging"`
}

type codePlanPayload struct {
	Plan codePlanData `json:"plan"`
}

type codePlanData struct {
	ServiceName  string                     `json:"service_name"`
	Summary      string                     `json:"summary"`
	Language     string                     `json:"language"`
	Runtime      string                     `json:"runtime"`
	Architecture []string                   `json:"architecture"`
	Components   []codePlanComponentPayload `json:"components"`
	APIEndpoints []codePlanEndpointPayload  `json:"api_endpoints"`
	DevTasks     []string                   `json:"dev_tasks"`
	Operations   []string                   `json:"operations"`
	Testing      []string                   `json:"testing"`
}

type codePlanComponentPayload struct {
	Name           string   `json:"name"`
	Responsibility string   `json:"responsibility"`
	TechNotes      []string `json:"tech_notes"`
}

type codePlanEndpointPayload struct {
	Method         string `json:"method"`
	Path           string `json:"path"`
	Description    string `json:"description"`
	RequestSchema  string `json:"request_schema"`
	ResponseSchema string `json:"response_schema"`
}

func parseWebBlueprint(answer string) (*WebBlueprint, error) {
	trimmed := strings.TrimSpace(answer)
	if trimmed == "" {
		return nil, fmt.Errorf("empty answer")
	}

	candidate := extractJSON(trimmed)
	if candidate == "" {
		return nil, fmt.Errorf("no json object found")
	}

	var payload webBlueprintPayload
	if err := json.Unmarshal([]byte(candidate), &payload); err != nil {
		return nil, fmt.Errorf("failed to decode json: %w", err)
	}

	pageTitle := strings.TrimSpace(payload.Blueprint.PageTitle)
	summary := strings.TrimSpace(payload.Blueprint.Summary)
	if pageTitle == "" || summary == "" {
		return nil, fmt.Errorf("missing blueprint title or summary")
	}

	sections := make([]WebSection, 0, len(payload.Blueprint.Sections))
	for _, section := range payload.Blueprint.Sections {
		title := strings.TrimSpace(section.Title)
		purpose := strings.TrimSpace(section.Purpose)
		if title == "" || purpose == "" {
			continue
		}
		sections = append(sections, WebSection{
			Title:           title,
			Purpose:         purpose,
			Components:      normalizeStringSlice(section.Components),
			CopySuggestions: normalizeStringSlice(section.CopySuggestions),
		})
	}

	if len(sections) == 0 {
		return nil, fmt.Errorf("no valid sections found")
	}

	ctas := make([]WebCallToAction, 0, len(payload.Blueprint.CallToActions))
	for _, cta := range payload.Blueprint.CallToActions {
		label := strings.TrimSpace(cta.Label)
		destination := strings.TrimSpace(cta.Destination)
		if label == "" || destination == "" {
			continue
		}
		ctas = append(ctas, WebCallToAction{
			Label:       label,
			Destination: destination,
			Variant:     strings.TrimSpace(cta.Variant),
			Messaging:   strings.TrimSpace(cta.Messaging),
		})
	}

	blueprint := &WebBlueprint{
		PageTitle:     pageTitle,
		Summary:       summary,
		Sections:      sections,
		CallToActions: ctas,
		SEOKeywords:   normalizeStringSlice(payload.Blueprint.SEOKeywords),
	}

	return blueprint, nil
}

func buildFallbackWebBlueprint(goal string, audience string, tone string, mustHaves []string) WebBlueprint {
	title := truncateRunes(goal, 80)
	if title == "" {
		title = "落地页蓝图"
	}

	summaryParts := []string{goal}
	if audience != "" {
		summaryParts = append(summaryParts, "目标受众："+audience)
	}
	if tone != "" {
		summaryParts = append(summaryParts, "语气："+tone)
	}
	if len(mustHaves) > 0 {
		summaryParts = append(summaryParts, "必须包含："+strings.Join(mustHaves, "、"))
	}

	sections := []WebSection{
		{
			Title:   "首屏价值陈述",
			Purpose: "迅速传达产品/服务的核心价值并引导用户继续浏览",
			Components: []string{
				"主标题",
				"支持性副标题",
				"主行动按钮",
				"品牌背书图标",
			},
			CopySuggestions: []string{
				"一句话概括你要解决的问题",
				"补充一条可信的量化成果或客户评价",
			},
		},
		{
			Title:      "关键功能模块",
			Purpose:    "分点解释核心能力或产品特性，帮助用户建立信任",
			Components: []string{"三列特性介绍", "支持图片或插画"},
			CopySuggestions: []string{
				"每个要点保持 12-18 个中文字符",
				"聚焦可量化的收益或独特优势",
			},
		},
		{
			Title:      "案例或证言",
			Purpose:    "展示真实案例、合作客户或媒体报道",
			Components: []string{"客户 Logo 墙", "推荐语滑块"},
			CopySuggestions: []string{
				"引用一句最具代表性的客户反馈",
				"附上明确的公司或人物身份信息",
			},
		},
		{
			Title:      "转化行动",
			Purpose:    "重复强调转化路径，提供次级行动入口",
			Components: []string{"主 CTA 按钮", "次级 CTA 链接", "常见问题手风琴"},
			CopySuggestions: []string{
				"用动词开头强调下一步操作",
				"提供一个常见顾虑的即时解答",
			},
		},
	}

	blueprint := WebBlueprint{
		PageTitle: title,
		Summary:   strings.Join(summaryParts, "；"),
		Sections:  sections,
		CallToActions: []WebCallToAction{{
			Label:       "立即了解详情",
			Destination: "联系表单或产品演示预约",
			Variant:     "primary",
			Messaging:   "我们将在 1 个工作日内回复",
		}},
		SEOKeywords: mustHaves,
	}

	return blueprint
}

func parseCodeServicePlan(answer string) (*CodeServicePlan, error) {
	trimmed := strings.TrimSpace(answer)
	if trimmed == "" {
		return nil, fmt.Errorf("empty answer")
	}

	candidate := extractJSON(trimmed)
	if candidate == "" {
		return nil, fmt.Errorf("no json object found")
	}

	var payload codePlanPayload
	if err := json.Unmarshal([]byte(candidate), &payload); err != nil {
		return nil, fmt.Errorf("failed to decode json: %w", err)
	}

	summary := strings.TrimSpace(payload.Plan.Summary)
	if summary == "" {
		return nil, fmt.Errorf("missing plan summary")
	}

	components := make([]CodePlanComponent, 0, len(payload.Plan.Components))
	for _, component := range payload.Plan.Components {
		name := strings.TrimSpace(component.Name)
		resp := strings.TrimSpace(component.Responsibility)
		if name == "" || resp == "" {
			continue
		}
		components = append(components, CodePlanComponent{
			Name:           name,
			Responsibility: resp,
			TechNotes:      normalizeStringSlice(component.TechNotes),
		})
	}

	if len(components) == 0 {
		return nil, fmt.Errorf("no valid components found")
	}

	endpoints := make([]CodePlanEndpoint, 0, len(payload.Plan.APIEndpoints))
	for _, endpoint := range payload.Plan.APIEndpoints {
		method := strings.ToUpper(strings.TrimSpace(endpoint.Method))
		path := strings.TrimSpace(endpoint.Path)
		desc := strings.TrimSpace(endpoint.Description)
		if method == "" || path == "" || desc == "" {
			continue
		}
		endpoints = append(endpoints, CodePlanEndpoint{
			Method:         method,
			Path:           path,
			Description:    desc,
			RequestSchema:  strings.TrimSpace(endpoint.RequestSchema),
			ResponseSchema: strings.TrimSpace(endpoint.ResponseSchema),
		})
	}

	plan := &CodeServicePlan{
		ServiceName:  strings.TrimSpace(payload.Plan.ServiceName),
		Summary:      summary,
		Language:     strings.TrimSpace(payload.Plan.Language),
		Runtime:      strings.TrimSpace(payload.Plan.Runtime),
		Architecture: normalizeStringSlice(payload.Plan.Architecture),
		Components:   components,
		APIEndpoints: endpoints,
		DevTasks:     normalizeStringSlice(payload.Plan.DevTasks),
		Operations:   normalizeStringSlice(payload.Plan.Operations),
		Testing:      normalizeStringSlice(payload.Plan.Testing),
	}

	return plan, nil
}

func buildFallbackCodeServicePlan(serviceName string, objective string, languageCanonical string, languageDisplay string, features []string, integrations []string) CodeServicePlan {
	summary := truncateRunes(objective, 200)
	if summary == "" {
		summary = "为示例微服务生成开发脚手架"
	}

	architecture := []string{
		"REST API 层：通过统一路由暴露 JSON 接口",
		"业务服务层：封装核心逻辑，保持无状态设计",
		"数据访问层：抽象持久化接口，方便替换实现",
		"容器化与配置：12-Factor 配置、支持环境变量注入",
	}
	if len(features) > 0 {
		architecture = append(architecture, "特色功能："+strings.Join(features, "、"))
	}
	if len(integrations) > 0 {
		architecture = append(architecture, "外部集成："+strings.Join(integrations, "、"))
	}

	components := []CodePlanComponent{
		{
			Name:           "HTTP 入口",
			Responsibility: "初始化路由与中间件，处理请求/响应序列化",
			TechNotes: []string{
				"使用结构化日志与请求 ID",
				"暴露健康检查、指标端点",
			},
		},
		{
			Name:           "用例服务",
			Responsibility: "实现核心业务流程，封装与数据层的交互",
			TechNotes: []string{
				"保持函数无状态，便于单元测试",
				"对外暴露接口便于 Agent 编排",
			},
		},
		{
			Name:           "适配器/存储",
			Responsibility: "封装数据库或外部 API 调用，处理错误与重试",
			TechNotes: []string{
				"提供接口以便替换成内存实现用于演示",
				"集成指标与错误日志",
			},
		},
	}

	endpoints := []CodePlanEndpoint{
		{Method: "GET", Path: "/healthz", Description: "健康检查，返回服务状态", ResponseSchema: "{\"status\":\"ok\"}"},
		{Method: "GET", Path: "/api/version", Description: "返回服务版本与构建信息", ResponseSchema: "{\"version\":string}"},
	}
	for _, feature := range features {
		slug := sanitizeWorkbenchContent(feature)
		slug = strings.ReplaceAll(strings.ToLower(slug), " ", "-")
		if slug == "" {
			continue
		}
		path := "/api/" + slug
		endpoints = append(endpoints, CodePlanEndpoint{
			Method:      "POST",
			Path:        path,
			Description: "处理特性：" + feature,
		})
	}

	devTasks := []string{
		"初始化 Git 仓库并配置 CI lint/test 流水线",
		"生成基础目录结构：cmd、internal、pkg 等",
		"实现健康检查与日志中间件",
		"编写至少 3 个用例单元测试，覆盖核心流程",
	}
	for _, feature := range features {
		devTasks = append(devTasks, "实现功能模块："+feature)
	}

	operations := []string{
		"提供 Dockerfile 与 docker-compose 本地运行方案",
		"暴露 Prometheus 格式指标与结构化日志",
		"预留环境变量以配置外部依赖",
	}
	if len(integrations) > 0 {
		operations = append(operations, "集成外部依赖："+strings.Join(integrations, "、"))
	}

	testing := []string{
		"编写 API 层集成测试，覆盖 happy-path 与异常分支",
		"在 sandbox 中运行端到端验证脚本",
		"提供 Makefile 或 npm script 简化测试执行",
	}

	plan := CodeServicePlan{
		ServiceName:  serviceName,
		Summary:      summary,
		Language:     languageDisplay,
		Runtime:      defaultRuntimeForLanguage(languageCanonical, languageDisplay),
		Architecture: architecture,
		Components:   components,
		APIEndpoints: endpoints,
		DevTasks:     devTasks,
		Operations:   operations,
		Testing:      testing,
	}

	return plan
}

func sanitizeWorkbenchList(values []string, limit int, runeLimit int) []string {
	if limit <= 0 || runeLimit <= 0 || len(values) == 0 {
		return nil
	}
	sanitized := make([]string, 0, len(values))
	for _, value := range values {
		if len(sanitized) >= limit {
			break
		}
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		clean := truncateRunes(sanitizeWorkbenchContent(trimmed), runeLimit)
		if clean != "" {
			sanitized = append(sanitized, clean)
		}
	}
	if len(sanitized) == 0 {
		return nil
	}
	return sanitized
}

func normalizeCodeLanguage(value string) (string, string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", ""
	}
	lower := strings.ToLower(trimmed)
	switch lower {
	case "go", "golang":
		return "go", "Go"
	case "node", "nodejs", "node.js", "javascript", "js":
		return "nodejs", "Node.js"
	case "typescript", "ts":
		return "typescript", "TypeScript"
	case "python", "py":
		return "python", "Python"
	}
	sanitized := sanitizeWorkbenchContent(trimmed)
	if sanitized == "" {
		return "", ""
	}
	runes := []rune(sanitized)
	runes[0] = unicode.ToUpper(runes[0])
	return lower, string(runes)
}

func defaultRuntimeForLanguage(canonical string, display string) string {
	switch canonical {
	case "go":
		return "Go + chi/otel 中间件"
	case "nodejs":
		return "Node.js (TypeScript) + Fastify"
	case "typescript":
		return "TypeScript + Fastify"
	case "python":
		return "Python + FastAPI"
	}
	if display != "" {
		return display
	}
	return "容器化微服务运行时"
}

func extractJSON(answer string) string {
	start := strings.Index(answer, "{")
	end := strings.LastIndex(answer, "}")
	if start == -1 || end == -1 || end <= start {
		return ""
	}
	return answer[start : end+1]
}

func normalizeStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}
